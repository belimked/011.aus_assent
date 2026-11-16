package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MarketEvent struct{ Kind, Address, LotPlan, Suburb, Postcode, ProviderID string }
type MarketSource interface {
	Name() string
	Fetch(context.Context, time.Time) ([]MarketEvent, error)
}
type fileSource struct {
	data     []byte
	provider string
}

func (fileSource) Name() string { return "file" }

// Fetch reads normalized identifiers; importMarket performs the full per-row validation and writes.
func (f fileSource) Fetch(ctx context.Context, _ time.Time) ([]MarketEvent, error) {
	m, err := mapping(f.provider)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(f.data, []byte{0xef, 0xbb, 0xbf})))
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	cols := map[string]int{}
	for field, names := range m {
		for i, h := range head {
			for _, alias := range names {
				if key(h) == key(alias) {
					cols[field] = i
				}
			}
		}
	}
	if _, ok := cols["address"]; !ok {
		return nil, errors.New("Address column required")
	}
	out := []MarketEvent{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(line) {
				return strings.TrimSpace(line[i])
			}
			return ""
		}
		out = append(out, MarketEvent{Address: get("address"), LotPlan: get("lot_plan"), Suburb: get("suburb"), Postcode: get("postcode"), Kind: get("kind"), ProviderID: get("provider_id")})
	}
	return out, nil
}

type pricefinderAPISource struct{}

func (pricefinderAPISource) Name() string { return "pricefinder" }
func (pricefinderAPISource) Fetch(context.Context, time.Time) ([]MarketEvent, error) {
	if os.Getenv("AXLEO_PRICEFINDER_API_KEY") == "" {
		return nil, errors.New("Pricefinder API 未配置：需要中介提供授权与 API 文档")
	}
	return nil, errors.New("Pricefinder API 尚未实现：待提供接口文档")
}

var aliases = map[string][]string{
	"address": {"Address", "Street Address", "Property Address"}, "lot_plan": {"Lot/Plan", "Lot Plan"}, "suburb": {"Suburb", "Locality"}, "postcode": {"Postcode", "Post Code"}, "price": {"Sale Price", "Price"}, "date": {"Sale Date", "Contract Date", "Listed Date", "Event Date"}, "agency": {"Agency", "Listing Agency"}, "agent": {"Agent", "Listing Agent"}, "owners": {"Owner Name(s)", "Owner Names", "Owner Name"}, "beds": {"Beds", "Bedrooms"}, "baths": {"Baths", "Bathrooms"}, "cars": {"Car", "Cars", "Parking"}, "land": {"Land Size", "Land"}, "kind": {"Kind", "Event Type", "Type"}, "provider_id": {"Property ID", "Listing ID", "Sale ID", "Record ID", "Provider ID"},
}

func key(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.ToLower(s))
}
func mapping(provider string) (map[string][]string, error) {
	m := map[string][]string{}
	for k, v := range aliases {
		m[k] = append([]string{}, v...)
	}
	for _, base := range []string{"market-mappings", "/etc/axleo/market-mappings"} {
		b, err := os.ReadFile(filepath.Join(base, provider+".json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var overrides map[string][]string
		if err = json.Unmarshal(b, &overrides); err != nil {
			return nil, err
		}
		for k, v := range overrides {
			if _, ok := m[k]; !ok {
				return nil, fmt.Errorf("unknown mapping field %s", k)
			}
			m[k] = v
		}
	}
	return m, nil
}

var punctuation = regexp.MustCompile(`[^a-z0-9]+`)

func addressKey(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`\bunit\s+(\d+)\s+`).ReplaceAllString(s, "$1/")
	s = strings.ReplaceAll(s, "/", " ")
	s = punctuation.ReplaceAllString(s, " ")
	words := strings.Fields(s)
	for i, w := range words {
		switch w {
		case "st":
			words[i] = "street"
		case "rd":
			words[i] = "road"
		case "ave", "av":
			words[i] = "avenue"
		case "dr":
			words[i] = "drive"
		case "ct":
			words[i] = "court"
		case "cres":
			words[i] = "crescent"
		case "pl":
			words[i] = "place"
		case "pde":
			words[i] = "parade"
		case "tce":
			words[i] = "terrace"
		}
	}
	return strings.Join(words, " ")
}
func lotKey(s string) string { return key(strings.ReplaceAll(strings.ToLower(s), "lot", "")) }

var seedPostcodes = map[string]string{"sunnybank": "4109", "sunnybankhills": "4109", "robertson": "4109", "macgregor": "4109", "runcorn": "4113"}

func (a *app) matchProperty(e MarketEvent) string { return matchProperty(a.db, e) }
func matchProperty(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, e MarketEvent) string {
	rows, err := q.Query(`SELECT id,no,street,suburb,data FROM properties WHERE deleted_at IS NULL AND registered=1`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	addr := addressKey(e.Address)
	var found string
	for rows.Next() {
		var id, no, street, sub, raw string
		if rows.Scan(&id, &no, &street, &sub, &raw) != nil {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal([]byte(raw), &p)
		if e.LotPlan != "" && lotKey(e.LotPlan) != "" && lotKey(e.LotPlan) == lotKey(str(p, "lot")) {
			return id
		}
		if addressKey(no+" "+street) == addr && key(sub) == key(e.Suburb) && (e.Postcode == "" || seedPostcodes[key(sub)] == e.Postcode || str(p, "postcode") == e.Postcode) {
			if found != "" {
				return ""
			}
			found = id
		}
	}
	return found
}
func polygon(pt []float64, poly [][]float64) bool {
	if len(pt) < 2 || len(poly) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if len(a) < 2 || len(b) < 2 {
			continue
		}
		if (a[1] > pt[1]) != (b[1] > pt[1]) && pt[0] < (b[0]-a[0])*(pt[1]-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}
func zoneCovers(z map[string]any, p map[string]any) bool {
	if key(str(z, "suburb")) != "" && key(str(z, "suburb")) == key(str(p, "suburb")) {
		return true
	}
	if subs, ok := z["suburbs"].([]any); ok {
		for _, v := range subs {
			if key(fmt.Sprint(v)) == key(str(p, "suburb")) {
				return true
			}
		}
	}
	raw, _ := json.Marshal(p["ll"])
	var pt []float64
	_ = json.Unmarshal(raw, &pt)
	if len(pt) < 2 {
		return false
	}
	raw, _ = json.Marshal(z["pts"])
	var poly [][]float64
	_ = json.Unmarshal(raw, &poly)
	if polygon(pt, poly) {
		return true
	}
	centerRaw, _ := json.Marshal(z["center"])
	var center []float64
	_ = json.Unmarshal(centerRaw, &center)
	radius, _ := z["radius"].(float64)
	if len(center) == 2 && radius > 0 {
		dlat := (pt[0] - center[0]) * 111195
		dlon := (pt[1] - center[1]) * 111195 * math.Cos(center[0]*math.Pi/180)
		return math.Hypot(dlat, dlon) <= radius
	}
	return false
}
func (a *app) covered(id, user string) bool {
	var raw string
	if a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw) != nil {
		return false
	}
	var p map[string]any
	_ = json.Unmarshal([]byte(raw), &p)
	rows, err := a.db.Query(`SELECT data FROM zones WHERE owner_id=? AND deleted_at IS NULL`, user)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var zraw string
		if rows.Scan(&zraw) == nil {
			var z map[string]any
			_ = json.Unmarshal([]byte(zraw), &z)
			if zoneCovers(z, p) {
				return true
			}
		}
	}
	return false
}
func (a *app) marketProperty(m map[string]any, p principal) {
	id := str(m, "id")
	var names, source, stamp string
	var price sql.NullInt64
	var date sql.NullString
	if a.db.QueryRow(`SELECT owner_names,last_sale_price,last_sale_date,source,updated_at FROM property_ownership WHERE property_id=?`, id).Scan(&names, &price, &date, &source, &stamp) != nil {
		return
	}
	market := map[string]any{"lastSalePrice": nil, "lastSaleDate": date.String, "source": source, "updatedAt": stamp}
	if price.Valid {
		market["lastSalePrice"] = price.Int64
	}
	if p.role == "admin" || a.covered(id, p.id) {
		market["owners"] = names
	}
	m["market"] = market
}

type importReport struct {
	ID                 string              `json:"id"`
	Rows               int                 `json:"rows"`
	Success            int                 `json:"success"`
	Skipped            int                 `json:"skipped"`
	Errors             int                 `json:"errors"`
	Unmatched          int                 `json:"unmatched"`
	UnmatchedAddresses []map[string]string `json:"unmatchedAddresses,omitempty"`
	Details            []string            `json:"errorDetails"`
	Agents             map[string]int      `json:"agentCounts"`
	DryRun             bool                `json:"dryRun"`
}

func (a *app) importMarket(data []byte, filename, provider, kind string, dry bool, p principal) (importReport, error) {
	r := importReport{ID: ulid(), Details: []string{}, Agents: map[string]int{}, DryRun: dry}
	if provider != "pricefinder" && provider != "other" {
		return r, errors.New("invalid provider")
	}
	if kind != "auto" && kind != "sale" && kind != "listing" && kind != "ownership" {
		return r, errors.New("invalid kind")
	}
	if len(data) > 20<<20 {
		return r, errors.New("file exceeds 20MB")
	}
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM market_imports WHERE sha256=?`, digest).Scan(&n)
	if n > 0 {
		return r, errors.New("duplicate file sha256")
	}
	mapping, err := mapping(provider)
	if err != nil {
		return r, err
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = -1
	head, err := reader.Read()
	if err != nil {
		return r, err
	}
	cols := map[string]int{}
	for field, names := range mapping {
		for i, h := range head {
			for _, name := range names {
				if key(h) == key(name) {
					cols[field] = i
					break
				}
			}
		}
	}
	if _, ok := cols["address"]; !ok {
		return r, errors.New("Address column required")
	}
	var tx *sql.Tx
	if !dry {
		tx, err = a.db.Begin()
		if err != nil {
			return r, err
		}
		defer tx.Rollback()
		_, err = tx.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,imported_by,created_at,status) VALUES(?,'file',?,?,?,?,?,'processing')`, r.ID, provider, filepath.Base(filename), digest, p.id, now())
		if err != nil {
			return r, err
		}
	}
	rowNum := 1
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}
		rowNum++
		r.Rows++
		if e != nil {
			r.Errors++
			r.Details = append(r.Details, fmt.Sprintf("row %d: %v", rowNum, e))
			continue
		}
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(line) {
				return strings.TrimSpace(line[i])
			}
			return ""
		}
		address := get("address")
		eventKind := kind
		if kind == "auto" {
			eventKind = strings.ToLower(get("kind"))
			if eventKind == "" {
				if get("owners") != "" && get("date") == "" {
					eventKind = "ownership"
				} else if _, ok := cols["date"]; ok && key(head[cols["date"]]) == "listeddate" {
					eventKind = "listing"
				} else {
					eventKind = "sale"
				}
			}
		}
		if eventKind != "sale" && eventKind != "listing" && eventKind != "withdrawn" && eventKind != "ownership" || address == "" {
			r.Errors++
			r.Details = append(r.Details, fmt.Sprintf("row %d: missing address or invalid kind", rowNum))
			continue
		}
		price := int64(0)
		if value := get("price"); value != "" {
			price, e = strconv.ParseInt(strings.Map(func(c rune) rune {
				if c >= '0' && c <= '9' {
					return c
				}
				return -1
			}, value), 10, 64)
			if e != nil || price <= 0 {
				r.Errors++
				r.Details = append(r.Details, fmt.Sprintf("row %d: invalid price", rowNum))
				continue
			}
		}
		date := get("date")
		if date != "" {
			if _, e = time.Parse("2006-01-02", date); e != nil {
				r.Errors++
				r.Details = append(r.Details, fmt.Sprintf("row %d: date must be YYYY-MM-DD", rowNum))
				continue
			}
		}
		event := MarketEvent{Kind: eventKind, Address: address, LotPlan: get("lot_plan"), Suburb: get("suburb"), Postcode: get("postcode"), ProviderID: get("provider_id")}
		var prop string
		if dry {
			prop = a.matchProperty(event)
		} else {
			prop = matchProperty(tx, event)
		}
		if prop == "" {
			r.Unmatched++
			r.UnmatchedAddresses = append(r.UnmatchedAddresses, map[string]string{"address": address, "suburb": event.Suburb, "postcode": event.Postcode, "kind": eventKind})
		}
		if event.ProviderID == "" {
			h := sha256.Sum256([]byte(eventKind + "|" + addressKey(address) + "|" + date))
			event.ProviderID = hex.EncodeToString(h[:])
		}
		raw := map[string]string{}
		for i, h := range head {
			if i < len(line) {
				raw[h] = line[i]
			}
		}
		rawJSON, _ := json.Marshal(raw)
		if dry {
			r.Success++
			continue
		}
		if eventKind != "ownership" {
			id := ulid()
			_, e = tx.Exec(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,lot_plan,suburb,postcode,price,event_date,agency,agent,beds,baths,cars,land,import_id,property_id,raw) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(provider,provider_id,kind) DO UPDATE SET address=excluded.address,address_key=excluded.address_key,lot_plan=excluded.lot_plan,suburb=excluded.suburb,postcode=excluded.postcode,price=excluded.price,event_date=excluded.event_date,agency=excluded.agency,agent=excluded.agent,beds=excluded.beds,baths=excluded.baths,cars=excluded.cars,land=excluded.land,import_id=excluded.import_id,property_id=excluded.property_id,raw=excluded.raw`, id, provider, event.ProviderID, eventKind, address, addressKey(address), event.LotPlan, event.Suburb, event.Postcode, price, date, get("agency"), get("agent"), get("beds"), get("baths"), get("cars"), get("land"), r.ID, sql.NullString{String: prop, Valid: prop != ""}, string(rawJSON))
			if e == nil && prop != "" && (eventKind == "sale" || eventKind == "listing") {
				var eid string
				e = tx.QueryRow(`SELECT id FROM market_events WHERE provider=? AND provider_id=? AND kind=?`, provider, event.ProviderID, eventKind).Scan(&eid)
				if e == nil {
					e = a.notifyMarket(tx, eid, prop, eventKind, address, get("agency"), date, r.Agents)
				}
			}
		}
		if e == nil && prop != "" && (eventKind == "sale" || eventKind == "ownership") {
			_, e = tx.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(property_id) DO UPDATE SET owner_names=CASE WHEN excluded.owner_names='' THEN property_ownership.owner_names ELSE excluded.owner_names END,last_sale_price=CASE WHEN excluded.last_sale_price IS NULL THEN property_ownership.last_sale_price ELSE excluded.last_sale_price END,last_sale_date=CASE WHEN excluded.last_sale_date='' THEN property_ownership.last_sale_date ELSE excluded.last_sale_date END,source=excluded.source,import_id=excluded.import_id,updated_at=excluded.updated_at`, prop, get("owners"), sql.NullInt64{Int64: price, Valid: price > 0}, date, provider, r.ID, now())
			if e == nil {
				_, e = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1 WHERE id=?`, now(), prop)
			}
		}
		if e != nil {
			return r, e
		}
		r.Success++
	}
	r.Skipped = r.Errors
	if dry {
		return r, nil
	}
	details, _ := json.Marshal(r.Details)
	agents, _ := json.Marshal(r.Agents)
	unmatchedJSON, _ := json.Marshal(r.UnmatchedAddresses)
	_, err = tx.Exec(`UPDATE market_imports SET rows=?,success=?,skipped=?,errors=?,status='completed',error_details=?,agent_counts=?,unmatched_details=? WHERE id=?`, r.Rows, r.Success, r.Skipped, r.Errors, string(details), string(agents), string(unmatchedJSON), r.ID)
	if err == nil {
		err = a.record(tx, p, "IMPORT", "market_imports", r.ID, map[string]any{"sha256": digest, "rows": r.Rows, "success": r.Success, "errors": r.Errors})
	}
	if err == nil {
		err = tx.Commit()
	}
	return r, err
}
func (a *app) notifyMarket(tx *sql.Tx, eventID, prop, kind, address, agency, date string, counts map[string]int) error {
	var raw, sub string
	if err := tx.QueryRow(`SELECT data,suburb FROM properties WHERE id=?`, prop).Scan(&raw, &sub); err != nil {
		return err
	}
	var property map[string]any
	_ = json.Unmarshal([]byte(raw), &property)
	rows, err := tx.Query(`SELECT z.owner_id,z.data FROM zones z JOIN users u ON u.id=z.owner_id WHERE z.deleted_at IS NULL AND u.disabled=0 AND u.role='agent'`)
	if err != nil {
		return err
	}
	owners := map[string]bool{}
	for rows.Next() {
		var owner, data string
		if rows.Scan(&owner, &data) == nil {
			var z map[string]any
			_ = json.Unmarshal([]byte(data), &z)
			if zoneCovers(z, property) {
				owners[owner] = true
			}
		}
	}
	rows.Close()
	for owner := range owners {
		item := map[string]any{"id": ulid(), "kind": kind, "propId": prop, "addr": address, "meta": date, "sub": sub, "owner": "", "agency": agency, "ownerId": owner}
		b, _ := json.Marshal(item)
		result, e := tx.Exec(`INSERT INTO market_items(id,owner_id,data,event_id,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT DO NOTHING`, item["id"], owner, string(b), eventID, now(), now())
		if e != nil {
			return e
		}
		n, _ := result.RowsAffected()
		counts[owner] += int(n)
	}
	return nil
}
func (a *app) marketAdmin(w http.ResponseWriter, r *http.Request, p principal) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/market/")
	if path == "imports" && r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 21<<20)
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			problem(w, 400, "invalid upload")
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			problem(w, 400, "file required")
			return
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, (20<<20)+1))
		if err != nil || len(b) > 20<<20 {
			problem(w, 400, "file exceeds 20MB")
			return
		}
		provider := r.FormValue("provider")
		if provider == "" {
			provider = "pricefinder"
		}
		kind := r.FormValue("kind")
		if kind == "" {
			kind = "auto"
		}
		report, err := a.importMarket(b, h.Filename, provider, kind, r.FormValue("dryRun") == "true", p)
		if err != nil {
			problem(w, 409, err.Error())
			return
		}
		respond(w, 201, report)
		return
	}
	if path == "imports" && r.Method == "GET" {
		rows, err := a.db.Query(`SELECT id,source,provider,filename,sha256,rows,success,skipped,errors,imported_by,created_at,status,agent_counts,unmatched_details FROM market_imports ORDER BY created_at DESC LIMIT 100`)
		if err != nil {
			problem(w, 500, err.Error())
			return
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, source, provider, file, sha, by, at, status, agents, unmatchedRaw string
			var total, ok, skip, errs int
			if rows.Scan(&id, &source, &provider, &file, &sha, &total, &ok, &skip, &errs, &by, &at, &status, &agents, &unmatchedRaw) == nil {
				var unmatched []any
				_ = json.Unmarshal([]byte(unmatchedRaw), &unmatched)
				var counts any
				_ = json.Unmarshal([]byte(agents), &counts)
				out = append(out, map[string]any{"id": id, "source": source, "provider": provider, "filename": file, "sha256": sha, "rows": total, "success": ok, "skipped": skip, "errors": errs, "importedBy": by, "createdAt": at, "status": status, "unmatched": len(unmatched), "agentCounts": counts})
			}
		}
		respond(w, 200, out)
		return
	}
	if strings.HasPrefix(path, "imports/") && r.Method == "GET" {
		id := strings.TrimPrefix(path, "imports/")
		var raw, unmatchedRaw, agentRaw, status string
		var total, success, skipped, errorsCount int
		if a.db.QueryRow(`SELECT error_details,unmatched_details,agent_counts,rows,success,skipped,errors,status FROM market_imports WHERE id=?`, id).Scan(&raw, &unmatchedRaw, &agentRaw, &total, &success, &skipped, &errorsCount, &status) != nil {
			problem(w, 404, "not found")
			return
		}
		var details []string
		_ = json.Unmarshal([]byte(raw), &details)
		if len(details) > 50 {
			details = details[:50]
		}
		var unmatched []any
		_ = json.Unmarshal([]byte(unmatchedRaw), &unmatched)
		var agents any
		_ = json.Unmarshal([]byte(agentRaw), &agents)
		respond(w, 200, map[string]any{"id": id, "rows": total, "success": success, "skipped": skipped, "errors": errorsCount, "status": status, "agentCounts": agents, "errorDetails": details, "unmatched": unmatched})
		return
	}
	problem(w, 404, "not found")
}
