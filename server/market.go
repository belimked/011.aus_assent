package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
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

//go:embed market-mappings/*.json
var builtInMarketMappings embed.FS

var aliases = map[string][]string{
	"address": {"Address", "Street Address", "Property Address", "Street"}, "lot_plan": {"Lot/Plan", "Lot Plan", "Legal Description"}, "suburb": {"Suburb", "Locality"}, "postcode": {"Postcode", "Post Code"}, "price": {"Sale Price", "Price", "Last Sale"}, "date": {"Sale Date", "Contract Date", "Listed Date", "Event Date", "Last Sale Date"}, "agency": {"Agency", "Listing Agency"}, "agent": {"Agent", "Listing Agent"}, "owners": {"Owner Name(s)", "Owner Names", "Owner Name", "Current Owners"}, "beds": {"Beds", "Bedrooms"}, "baths": {"Baths", "Bathrooms"}, "cars": {"Car", "Cars", "Parking", "Car Parks"}, "land": {"Land Size", "Land", "Area"}, "kind": {"Kind", "Event Type", "Type"}, "provider_id": {"Property ID", "Listing ID", "Sale ID", "Record ID", "Provider ID", "PDS ID"},
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
	// Built-in defaults are independent of the process working directory. Only
	// the explicit server configuration can override them.
	for _, read := range []func() ([]byte, error){
		func() ([]byte, error) { return builtInMarketMappings.ReadFile("market-mappings/" + provider + ".json") },
		func() ([]byte, error) {
			return os.ReadFile(filepath.Join("/etc/axleo/market-mappings", provider+".json"))
		},
	} {
		b, err := read()
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
	s = regexp.MustCompile(`\b(?:unit|u)\s+(\d+)\s*[,/]\s*(\d+)\b|\b(?:unit|u)\s+(\d+)\s+(\d+)\b`).ReplaceAllStringFunc(s, func(part string) string {
		n := regexp.MustCompile(`\d+`).FindAllString(part, -1)
		return n[0] + "/" + n[1]
	})
	s = strings.ReplaceAll(s, "/", " ")
	s = punctuation.ReplaceAllString(s, " ")
	words := strings.Fields(s)
	for i, w := range words {
		// St is a street type only at the end; St Albans is a name.
		// Mt is a name prefix only (Mount Gravatt), never a street type.
		if w == "st" && i == len(words)-1 {
			words[i] = "street"
			continue
		}
		if w == "mt" && i > 0 && i < len(words)-1 {
			words[i] = "mount"
			continue
		}
		switch w {
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
		case "cl":
			words[i] = "close"
		case "cct":
			words[i] = "circuit"
		case "cir":
			words[i] = "circle"
		case "gr", "gve":
			words[i] = "grove"
		case "hwy":
			words[i] = "highway"
		case "la", "ln":
			words[i] = "lane"
		case "bvd", "blvd":
			words[i] = "boulevard"
		case "esp":
			words[i] = "esplanade"
		case "pkwy", "pwy":
			words[i] = "parkway"
		case "sq":
			words[i] = "square"
		case "crt":
			words[i] = "court"
		case "gdns":
			words[i] = "gardens"
		case "cr", "crs":
			words[i] = "crescent"
		case "wy":
			words[i] = "way"
		case "rdg":
			words[i] = "ridge"
		}
	}
	return strings.Join(words, " ")
}
func lotKey(s string) string {
	s = key(s)
	s = strings.TrimPrefix(s, "lot")
	if len(s) > 1 && s[0] == 'l' && s[1] >= '0' && s[1] <= '9' {
		s = s[1:]
	}
	return s
}

var seedPostcodes = map[string]string{"sunnybank": "4109", "sunnybankhills": "4109", "robertson": "4109", "macgregor": "4109", "runcorn": "4113"}

func (a *app) matchProperty(e MarketEvent) string { return matchProperty(a.db, e) }
func matchProperty(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, e MarketEvent) string {
	rows, err := q.Query(`SELECT id,no,street,suburb,data FROM properties WHERE deleted_at IS NULL`)
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
		if lk := lotKey(e.LotPlan); lk != "" && (lk == lotKey(str(p, "lot")) || lk == lotKey(str(p, "lotplan"))) {
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
func (a *app) covered(id, user string) bool { return coveredFrom(a.db, id, user) }
func coveredFrom(db querier, id, user string) bool {
	var raw string
	if db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw) != nil {
		return false
	}
	var p map[string]any
	_ = json.Unmarshal([]byte(raw), &p)
	rows, err := db.Query(`SELECT data FROM zones WHERE owner_id=? AND deleted_at IS NULL`, user)
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
func (a *app) marketCovered(m map[string]any, p principal) bool {
	return a.marketCoveredFrom(a.db, m, p)
}
func (a *app) marketCoveredFrom(db querier, m map[string]any, p principal) bool {
	if p.zones != nil {
		for _, z := range p.zones {
			if zoneCovers(z, m) {
				return true
			}
		}
		return false
	}
	return coveredFrom(db, str(m, "id"), p.id)
}
func (a *app) marketProperty(m map[string]any, p principal) { a.marketPropertyFrom(a.db, m, p) }
func (a *app) marketPropertyFrom(db querier, m map[string]any, p principal) {
	id := str(m, "id")
	visible := p.role == "admin" || a.marketCoveredFrom(db, m, p)
	if attrs, ok := m["rpdataAttributes"].(map[string]any); ok && !visible {
		delete(attrs, "owners")
		delete(attrs, "vendors")
	}
	var names, source, stamp string
	var absentee bool
	var price sql.NullInt64
	var date sql.NullString
	if db.QueryRow(`SELECT owner_names,last_sale_price,last_sale_date,source,updated_at,absentee_owner FROM property_ownership WHERE property_id=?`, id).Scan(&names, &price, &date, &source, &stamp, &absentee) != nil {
		return
	}
	market := map[string]any{"lastSalePrice": nil, "lastSaleDate": date.String, "source": source, "updatedAt": stamp}
	if price.Valid {
		market["lastSalePrice"] = price.Int64
	}
	if visible {
		market["owners"] = names
		market["absenteeOwner"] = absentee
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
	MatchedLot         int                 `json:"matchedLot"`
	MatchedAddress     int                 `json:"matchedAddress"`
	Misaligned         int                 `json:"misaligned"`
	Ignored            int                 `json:"ignored"`
	OwnershipUpdated   int                 `json:"ownershipUpdated"`
	SalesRecorded      int                 `json:"salesRecorded"`
	SalesNotified      int                 `json:"salesNotified"`
	UnmatchedAddresses []map[string]string `json:"unmatchedAddresses,omitempty"`
	Details            []string            `json:"errorDetails"`
	Agents             map[string]int      `json:"agentCounts"`
	DryRun             bool                `json:"dryRun"`
	HeaderRow          int                 `json:"headerRow"`
	WithParcel         int                 `json:"withParcel"`
	WithOwners         int                 `json:"withOwners"`
	WithCorelogicID    int                 `json:"withCorelogicId"`
	Synthetic          bool                `json:"synthetic"`
}

func (a *app) importMarket(data []byte, filename, provider, kind string, dry bool, p principal) (importReport, error) {
	r := importReport{ID: ulid(), Details: []string{}, Agents: map[string]int{}, DryRun: dry, Synthetic: syntheticMarketFile(filename)}
	if provider != "pricefinder" && provider != "rpdata" && provider != "other" {
		return r, errors.New("invalid provider")
	}
	if kind != "auto" && kind != "sale" && kind != "listing" && kind != "ownership" {
		return r, errors.New("invalid kind")
	}
	if len(data) > 100<<20 {
		return r, errors.New("file exceeds 100MB")
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
	rows, err := marketRows(data)
	if err != nil {
		return r, err
	}
	if len(rows) == 0 {
		return r, io.EOF
	}
	headerIndex := -1
	var head []string
	cols := map[string]int{}
	for i, candidate := range rows {
		if i >= 20 {
			break
		}
		found := map[string]int{}
		for field, names := range mapping {
			for j, h := range candidate {
				for _, name := range names {
					if key(h) == key(name) {
						found[field] = j
						break
					}
				}
			}
		}
		if hasColumn(found, "address") && (provider != "rpdata" || hasColumn(found, "lot_plan") && hasHeader(candidate, "Parcel Details")) {
			headerIndex, head, cols = i, candidate, found
			break
		}
	}
	if headerIndex < 0 {
		return r, errors.New("Address column required in first 20 rows")
	}
	r.Ignored = headerIndex
	r.Rows = headerIndex
	r.HeaderRow = headerIndex + 1
	pf := provider == "pricefinder" && hasColumn(cols, "lot_plan") && hasColumn(cols, "suburb") && hasHeader(head, "Street Name")
	if _, ok := cols["address"]; !ok && !pf {
		return r, errors.New("Address column required")
	}
	index, err := loadMarketIndex(a.db)
	if err != nil {
		return r, err
	}
	allowed := map[string]bool{"sunnybank": true, "sunnybankhills": true, "calamvale": true, "parkinson": true}
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
	snapshot := kind == "auto" && (provider == "rpdata" || !hasColumn(cols, "kind") && hasColumn(cols, "owners") && hasColumn(cols, "date") && key(head[cols["date"]]) == "lastsaledate")
	for rowIndex, line := range rows[headerIndex+1:] {
		var e error
		rowNum := rowIndex + headerIndex + 2
		r.Rows++
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(line) {
				v := strings.TrimSpace(line[i])
				if provider == "rpdata" && v == "-" {
					return ""
				}
				return v
			}
			return ""
		}
		address := get("address")
		if provider == "rpdata" && address != "" {
			if get("lot_plan") != "" {
				r.WithParcel++
			}
			if rpNames(head, line, "Owner") != "" {
				r.WithOwners++
			}
			if corelogicID(rpColumn(head, line, "Open in RPData")) != "" {
				r.WithCorelogicID++
			}
		}
		if pf {
			if !allowed[key(get("suburb"))] {
				r.Misaligned++
				continue
			}
			address = pfAddress(head, line)
		}
		if (ignoredMarketRow(line) && (address == "" || footerOnlyMarketRow(line))) || (address == "" && footerOnlyMarketRow(line) && len(line) > 0 && strings.TrimSpace(line[0]) != "" && cols["address"] != 0) {
			r.Ignored++
			continue
		}
		eventKind := kind
		if snapshot {
			eventKind = "snapshot"
		}
		if kind == "auto" && !snapshot {
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
		if eventKind != "sale" && eventKind != "listing" && eventKind != "withdrawn" && eventKind != "ownership" && eventKind != "snapshot" || address == "" {
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
		dateValue := normalMarketExcelDate(get("date"), data)
		date, dateErr := marketDate(dateValue)
		if dateErr != nil {
			r.Errors++
			r.Details = append(r.Details, fmt.Sprintf("row %d: date must be YYYY-MM-DD or DD/MM/YYYY", rowNum))
			continue
		}
		lotPlan := get("lot_plan")
		if pf {
			lotPlan = pfLotDescription(lotPlan)
		}
		providerID := get("provider_id")
		if provider == "rpdata" {
			providerID = corelogicID(rpColumn(head, line, "Open in RPData"))
		}
		event := MarketEvent{Kind: eventKind, Address: address, LotPlan: lotPlan, Suburb: get("suburb"), Postcode: get("postcode"), ProviderID: providerID}
		prop, method := index.match(event, pf && (pfColumn(head, line, "Unit") != "" || strings.Contains(event.Address, "/")))
		if method == "lot" {
			r.MatchedLot++
		} else if method == "address" {
			r.MatchedAddress++
		}
		if prop == "" {
			r.Unmatched++
			if len(r.UnmatchedAddresses) < 30 {
				r.UnmatchedAddresses = append(r.UnmatchedAddresses, map[string]string{"address": address, "lotplan": event.LotPlan, "suburb": event.Suburb, "postcode": event.Postcode, "kind": eventKind})
			}
		}
		if event.ProviderID == "" {
			h := sha256.Sum256([]byte(eventKind + "|" + addressKey(address) + "|" + date))
			event.ProviderID = hex.EncodeToString(h[:])
		}
		if snapshot && date != "" {
			event.ProviderID += ":" + date
		}
		raw := map[string]string{}
		for i, h := range head {
			if i < len(line) && safeMarketHeader(h) {
				value := line[i]
				switch key(h) {
				case "lastsaledate", "saledate", "valuationdate", "loaddate", "listeddate", "eventdate":
					value = normalMarketExcelDate(value, data)
				}
				raw[h] = value
			}
		}
		rawJSON, _ := json.Marshal(raw)
		var previous string
		if snapshot && prop != "" {
			if dry {
				_ = a.db.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id=?`, prop).Scan(&previous)
			} else {
				_ = tx.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id=?`, prop).Scan(&previous)
			}
		}
		notifySnapshot := snapshot && prop != "" && date != "" && date > previous && recentMarketSale(date)
		if snapshot {
			if prop != "" {
				r.OwnershipUpdated++
			}
			if date != "" {
				r.SalesRecorded++
			}
			if notifySnapshot {
				r.SalesNotified++
			}
		}
		if dry {
			r.Success++
			continue
		}
		if eventKind != "ownership" && (eventKind != "snapshot" || date != "") {
			storedKind := eventKind
			if snapshot {
				storedKind = "sale"
			}
			id := ulid()
			_, e = tx.Exec(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,lot_plan,suburb,postcode,price,event_date,agency,agent,beds,baths,cars,land,import_id,property_id,raw) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(provider,provider_id,kind) DO UPDATE SET address=excluded.address,address_key=excluded.address_key,lot_plan=excluded.lot_plan,suburb=excluded.suburb,postcode=excluded.postcode,price=excluded.price,event_date=excluded.event_date,agency=excluded.agency,agent=excluded.agent,beds=excluded.beds,baths=excluded.baths,cars=excluded.cars,land=excluded.land,import_id=excluded.import_id,property_id=excluded.property_id,raw=excluded.raw`, id, provider, event.ProviderID, storedKind, address, addressKey(address), event.LotPlan, event.Suburb, event.Postcode, price, date, get("agency"), get("agent"), get("beds"), get("baths"), get("cars"), get("land"), r.ID, sql.NullString{String: prop, Valid: prop != ""}, string(rawJSON))
			if e == nil && prop != "" && (!snapshot && (eventKind == "sale" || eventKind == "listing") || notifySnapshot) {
				var eid string
				e = tx.QueryRow(`SELECT id FROM market_events WHERE provider=? AND provider_id=? AND kind=?`, provider, event.ProviderID, storedKind).Scan(&eid)
				if e == nil {
					e = a.notifyMarket(tx, eid, prop, storedKind, address, get("agency"), date, r.Agents)
				}
			}
		}
		if e == nil && prop != "" && (eventKind == "sale" || eventKind == "ownership" || eventKind == "snapshot") {
			owners := get("owners")
			if provider == "rpdata" {
				owners = rpNames(head, line, "Owner")
			}
			if strings.EqualFold(owners, "N/A") {
				owners = ""
			}
			absentee := absenteeMarket(getOwnerAddress(head, line), address)
			if pf && strings.EqualFold(pfColumn(head, line, "Owner Occupier"), "true") {
				absentee = false
			} else if pf && strings.EqualFold(pfColumn(head, line, "Owner Occupier"), "false") {
				absentee = true
			}
			e = applyMarketOwnership(tx, prop, owners, price, date, provider, r.ID, absentee)
			if e == nil && pf {
				e = storePFAttributes(tx, prop, head, line)
			} else if e == nil && provider == "rpdata" {
				e = storeRPAttributes(tx, prop, head, line, providerID, filename, date, previous)
			}
		}
		if e != nil {
			return r, e
		}
		r.Success++
	}
	r.Skipped = r.Errors + r.Ignored + r.Misaligned
	if dry {
		return r, nil
	}
	details, _ := json.Marshal(r.Details)
	agents, _ := json.Marshal(r.Agents)
	unmatchedJSON, _ := json.Marshal(r.UnmatchedAddresses)
	_, err = tx.Exec(`UPDATE market_imports SET rows=?,success=?,skipped=?,errors=?,ignored=?,ownership_updated=?,sales_recorded=?,sales_notified=?,status='completed',error_details=?,agent_counts=?,unmatched_details=? WHERE id=?`, r.Rows, r.Success, r.Skipped, r.Errors, r.Ignored, r.OwnershipUpdated, r.SalesRecorded, r.SalesNotified, string(details), string(agents), string(unmatchedJSON), r.ID)
	if err == nil {
		err = a.record(tx, p, "IMPORT", "market_imports", r.ID, map[string]any{"sha256": digest, "rows": r.Rows, "success": r.Success, "errors": r.Errors})
	}
	if err == nil {
		err = tx.Commit()
	}
	return r, err
}

// applyMarketOwnership is shared by import and the manual match review path.
func applyMarketOwnership(tx *sql.Tx, prop, owners string, price int64, date, provider, importID string, absentee bool) error {
	_, err := tx.Exec(`INSERT INTO property_ownership
		(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at,absentee_owner)
		VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(property_id) DO UPDATE SET
		owner_names=CASE WHEN excluded.owner_names='' OR (excluded.last_sale_date!='' AND property_ownership.last_sale_date > excluded.last_sale_date) THEN property_ownership.owner_names
			ELSE excluded.owner_names END,
		last_sale_price=CASE WHEN excluded.last_sale_price IS NULL OR
			(property_ownership.last_sale_date > excluded.last_sale_date AND excluded.last_sale_date != '')
			THEN property_ownership.last_sale_price ELSE excluded.last_sale_price END,
		last_sale_date=CASE WHEN excluded.last_sale_date='' OR
			property_ownership.last_sale_date > excluded.last_sale_date
			THEN property_ownership.last_sale_date ELSE excluded.last_sale_date END,
		source=excluded.source,import_id=excluded.import_id,updated_at=excluded.updated_at,
		absentee_owner=CASE WHEN excluded.owner_names='' OR (excluded.last_sale_date!='' AND property_ownership.last_sale_date > excluded.last_sale_date) THEN property_ownership.absentee_owner
			ELSE excluded.absentee_owner END`,
		prop, owners, sql.NullInt64{Int64: price, Valid: price > 0},
		date, provider, importID, now(), absentee,
	)
	if err == nil {
		_, err = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1,registered=1 WHERE id=?`, now(), prop)
	}
	return err
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
		r.Body = http.MaxBytesReader(w, r.Body, (100<<20)+(1<<20))
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			problem(w, 400, "invalid upload")
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			problem(w, 400, "file required")
			return
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, (100<<20)+1))
		if err != nil || len(b) > 100<<20 {
			problem(w, 400, "file exceeds 100MB")
			return
		}
		provider := r.FormValue("provider")
		if provider == "" {
			provider = "rpdata"
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
		rows, err := a.rd().Query(`SELECT id,source,provider,filename,sha256,rows,success,skipped,errors,imported_by,created_at,status,agent_counts,unmatched_details,ignored,ownership_updated,sales_recorded,sales_notified FROM market_imports ORDER BY created_at DESC LIMIT 100`)
		if err != nil {
			problem(w, 500, err.Error())
			return
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, source, provider, file, sha, by, at, status, agents, unmatchedRaw string
			var total, ok, skip, errs, ignored, ownershipUpdated, salesRecorded, salesNotified int
			if rows.Scan(&id, &source, &provider, &file, &sha, &total, &ok, &skip, &errs, &by, &at, &status, &agents, &unmatchedRaw, &ignored, &ownershipUpdated, &salesRecorded, &salesNotified) == nil {
				var unmatched []any
				_ = json.Unmarshal([]byte(unmatchedRaw), &unmatched)
				var counts any
				_ = json.Unmarshal([]byte(agents), &counts)
				out = append(out, map[string]any{"id": id, "source": source, "provider": provider, "filename": file, "sha256": sha, "rows": total, "success": ok, "skipped": skip, "errors": errs, "importedBy": by, "createdAt": at, "status": status, "unmatched": len(unmatched), "agentCounts": counts, "ignored": ignored, "ownershipUpdated": ownershipUpdated, "salesRecorded": salesRecorded, "salesNotified": salesNotified})
			}
		}
		respond(w, 200, out)
		return
	}
	if strings.HasPrefix(path, "imports/") && r.Method == "GET" {
		id := strings.TrimPrefix(path, "imports/")
		var raw, unmatchedRaw, agentRaw, status string
		var total, success, skipped, errorsCount, ignored, ownershipUpdated, salesRecorded, salesNotified int
		if a.rd().QueryRow(`SELECT error_details,unmatched_details,agent_counts,rows,success,skipped,errors,status,ignored,ownership_updated,sales_recorded,sales_notified FROM market_imports WHERE id=?`, id).Scan(&raw, &unmatchedRaw, &agentRaw, &total, &success, &skipped, &errorsCount, &status, &ignored, &ownershipUpdated, &salesRecorded, &salesNotified) != nil {
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
		respond(w, 200, map[string]any{"id": id, "rows": total, "success": success, "skipped": skipped, "errors": errorsCount, "status": status, "agentCounts": agents, "errorDetails": details, "unmatched": unmatched, "ignored": ignored, "ownershipUpdated": ownershipUpdated, "salesRecorded": salesRecorded, "salesNotified": salesNotified})
		return
	}
	problem(w, 404, "not found")
}
