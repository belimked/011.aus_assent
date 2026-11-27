package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Production-sized synthetic database for the performance tests. Everything is
// generated from a fixed seed; names, phone numbers and addresses are made up.
// Shape (production in brackets, see docs/db-performance.md):
//   70,000 properties: 62,000 public QLD parcels, 2,800 legacy seed parcels,
//     5,000 Rex properties (500 per agent), 200 phone-created; 2,500 carry
//     RP Data market data (1,900 public parcels, 600 Rex properties)
//   8,000 contacts over 10 agents [7,901], 7,900 contact rex_links [7,887],
//   2,000 property_owners [1,621], 20,000 audit_log, 20,000 rex_request_log,
//   6,000 rex_outbox, 2,000 todos, 3,000 contact notes, 300 listings, 200 photos.
// An agent's full /sync is ~2,600 properties [~2,400, 2.7MB].

const perfGenVersion = "3" // bump whenever the generated data changes

const (
	perfParcels       = 62000
	perfLegacyParcels = 2800
	perfRexProps      = 5000
	perfAppProps      = 200
	perfMarketParcels = 1900
	perfMarketRex     = 600
	perfContacts      = 8000
	perfContactLinks  = 7900
	perfOwnerLinks    = 2000
	perfAgents        = 10
	perfAudit         = 20000
	perfRexLog        = 20000
	perfOutbox        = 6000
	perfTodos         = 2000
	perfNotes         = 3000
	perfListings      = 300
	perfPhotos        = 200
	perfAccount       = "2057"
)

type perfSuburb struct {
	name     string
	lat, lng float64
}

var perfSuburbs = []perfSuburb{
	{"Sunnybank", -27.580, 153.060}, {"Sunnybank Hills", -27.610, 153.050}, {"Robertson", -27.565, 153.058},
	{"Macgregor", -27.565, 153.075}, {"Runcorn", -27.595, 153.075}, {"Eight Mile Plains", -27.580, 153.092},
	{"Calamvale", -27.622, 153.048}, {"Stretton", -27.622, 153.070}, {"Kuraby", -27.606, 153.092},
}

var perfFirst = []string{"Alex", "Bailey", "Casey", "Dana", "Elliot", "Frankie", "Georgie", "Harper", "Indy", "Jordan", "Kai", "Logan", "Morgan", "Noel", "Oakley", "Parker", "Quinn", "Riley", "Sage", "Taylor"}
var perfLast = []string{"Ashford", "Brookes", "Carrow", "Dunmore", "Ellery", "Fairley", "Garnet", "Hollis", "Ingram", "Jessop", "Kendal", "Lowther", "Marlow", "Norland", "Orwell", "Pryor", "Quill", "Rowan", "Selby", "Thorne"}
var perfStreets = []string{"Acacia St", "Banksia Ave", "Coral Rd", "Daisy Ct", "Elm Pl", "Fern St", "Grevillea Dr", "Hakea Cres", "Iris Way", "Jacaranda St", "Kurrajong Rd", "Lilly Pilly Ct"}

// perfFixture is what scenarios need to know about the generated data.
type perfFixture struct {
	path     string
	admin    string
	agents   []string // user ids
	tokens   map[string]string
	contacts []string // contact ids by index
	rexProps []string // Rex property ids
}

// The template database is generated once per schema + generator version and
// copied for each run. Its directory is AXLEO_PERF_CACHE (default .tmp/perf).
func perfTemplate(t testing.TB) string {
	t.Helper()
	h := sha256.New()
	entries, err := assets.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		b, _ := assets.ReadFile("migrations/" + n)
		h.Write([]byte(n))
		h.Write(b)
	}
	dir := env("AXLEO_PERF_CACHE", filepath.Join(".tmp", "perf"))
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Timestamps are relative to the generation day (retention and report windows
	// depend on the wall clock), so the template is also keyed by that day.
	day := time.Now().UTC().Truncate(24 * time.Hour)
	prefix := fmt.Sprintf("perf-s%d-%s-g%s-", len(names), hex.EncodeToString(h.Sum(nil))[:10], perfGenVersion)
	file := filepath.Join(dir, prefix+day.Format("20060102")+".db")
	if _, err = os.Stat(file); err == nil {
		return file
	}
	if old, _ := filepath.Glob(filepath.Join(dir, "perf-s*.db")); len(old) > 0 {
		for _, o := range old {
			os.Remove(o)
		}
	}
	start := time.Now()
	tmp := fmt.Sprintf("%s.%d.tmp", file, os.Getpid())
	_ = os.Remove(tmp)
	if err = perfGenerate(tmp, day); err != nil {
		os.Remove(tmp)
		t.Fatal(err)
	}
	for _, s := range []string{"-wal", "-shm"} {
		os.Remove(tmp + s)
	}
	if err = os.Rename(tmp, file); err != nil {
		t.Fatal(err)
	}
	t.Logf("generated perf template %s in %v", file, time.Since(start).Round(time.Millisecond))
	return file
}

// perfCopy copies the template into dir and returns the fixture description.
func perfCopy(t testing.TB, template, dir string) *perfFixture {
	t.Helper()
	path := filepath.Join(dir, "perf.db")
	src, err := os.Open(template)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err = dst.Close(); err != nil {
		t.Fatal(err)
	}
	f := &perfFixture{path: path, admin: "u-admin", tokens: map[string]string{"admin": perfToken("u-admin")}}
	for i := 0; i < perfAgents; i++ {
		id := fmt.Sprintf("u-agent%02d", i)
		f.agents = append(f.agents, id)
		f.tokens[id] = perfToken(id)
	}
	for i := 0; i < perfContacts; i++ {
		f.contacts = append(f.contacts, perfContactID(i))
	}
	for i := 0; i < perfRexProps; i++ {
		f.rexProps = append(f.rexProps, perfRexPropID(i))
	}
	return f
}

func perfToken(user string) string {
	s := sha256.Sum256([]byte("perf-session-" + user))
	return hex.EncodeToString(s[:])
}
func perfContactID(i int) string { return fmt.Sprintf("rex-contacts-%s-%d", perfAccount, 300000+i) }
func perfRexPropID(i int) string { return fmt.Sprintf("rex-properties-%s-%d", perfAccount, 500000+i) }
func perfJSON(v any) string      { b, _ := json.Marshal(v); return string(b) }

func perfGenerate(path string, base time.Time) error {
	db, err := open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = migrate(db, path); err != nil {
		return err
	}
	r := rand.New(rand.NewPCG(20261125, 7))
	stamp := func(maxDays int) string {
		return base.Add(-time.Duration(r.IntN(maxDays*24*3600)) * time.Second).Format(layout)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ins := func(q string, args ...any) {
		if err == nil {
			_, err = tx.Exec(q, args...)
		}
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("perf-password"), bcrypt.MinCost)
	// The Rex poller requires the sandbox accounts (sandboxUsers): the first agents use them.
	users := []struct{ id, email, name, role string }{{"u-admin", "admin@axleo.local", "Perf Admin", "admin"}}
	for i := 0; i < perfAgents; i++ {
		email := fmt.Sprintf("agent%02d@perf.local", i)
		if i < 3 {
			email = []string{"mary", "john", "import"}[i] + "@axleo.local"
		}
		users = append(users, struct{ id, email, name, role string }{fmt.Sprintf("u-agent%02d", i), email, fmt.Sprintf("Agent %02d", i), "agent"})
	}
	created := base.AddDate(-1, 0, 0).Format(layout)
	for _, u := range users {
		ins(`INSERT INTO users(id,email,name,role,password_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, u.id, u.email, u.name, u.role, string(hash), created, created)
		raw, _ := hex.DecodeString(perfToken(u.id))
		sum := sha256.Sum256(raw)
		ins(`INSERT INTO sessions VALUES (?,?,?,?,?)`, hex.EncodeToString(sum[:]), u.id, "perf", "2099-01-01T00:00:00Z", created)
	}
	agent := func(i int) string { return fmt.Sprintf("u-agent%02d", i%perfAgents) }
	// Each agent covers one suburb with a zone (market owner visibility).
	for i := 0; i < perfAgents; i++ {
		s := perfSuburbs[i%len(perfSuburbs)]
		z := map[string]any{"id": fmt.Sprintf("zone-%d", i), "name": s.name, "suburb": s.name}
		ins(`INSERT INTO zones(id,owner_id,name,data,created_at,updated_at) VALUES(?,?,?,?,?,?)`, z["id"], agent(i), s.name, perfJSON(z), created, created)
	}
	ins(`INSERT INTO market_imports(id,source,provider,filename,sha256,rows,success,created_at,status) VALUES('mi-perf','file','rpdata','perf.xlsx','perf-sha',2500,2500,?,'completed')`, created)
	point := func() (string, float64, float64) {
		s := perfSuburbs[r.IntN(len(perfSuburbs))]
		return s.name, s.lat + (r.Float64()-0.5)*0.024, s.lng + (r.Float64()-0.5)*0.024
	}
	marketAttrs := func(i int) map[string]any {
		return map[string]any{"propertyType": "House", "landArea": 400 + r.IntN(600), "owners": fmt.Sprintf("%s %s", perfFirst[i%20], perfLast[(i/20)%20]), "lastSalePrice": 600000 + r.IntN(900000), "corelogicId": fmt.Sprint(9000000 + i)}
	}
	addMarket := func(id string, i int) {
		ins(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at,absentee_owner) VALUES(?,?,?,?,?,?,?,?)`, id, fmt.Sprintf("%s %s", perfFirst[i%20], perfLast[(i/20)%20]), 600000+r.IntN(900000), base.AddDate(0, 0, -r.IntN(3000)).Format("2006-01-02"), "rpdata", "mi-perf", stamp(90), r.IntN(5) == 0)
		ins(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,suburb,price,event_date,import_id,property_id,raw,review_status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, "me-"+id, "rpdata", "rp-"+id, "sale", "addr "+id, "addr"+id, "", 700000, base.AddDate(0, 0, -r.IntN(400)).Format("2006-01-02"), "mi-perf", id, "{}", "linked")
	}
	n := 0
	for i := 0; i < perfParcels; i++ {
		sub, lat, lng := point()
		id := fmt.Sprintf("qp-%06d", i)
		no, street := fmt.Sprint(1+r.IntN(200)), perfStreets[r.IntN(len(perfStreets))]
		lp := fmt.Sprintf("%dRP%d", 1+r.IntN(30), 100000+i)
		d := map[string]any{"id": id, "no": no, "street": street, "suburb": sub, "lotplan": lp, "lot": lp, "unit": "", "ll": []float64{lat, lng}, "source": "qld_parcels", "ownerIds": []string{}, "pfOwner": "", "lastSale": "", "flags": map[string]any{}}
		market := i%(perfParcels/perfMarketParcels) == 0 && n < perfMarketParcels
		if market {
			d["rpdataAttributes"] = marketAttrs(i)
		}
		geom := perfJSON(map[string]any{"type": "Polygon", "coordinates": [][][]float64{{{lng, lat}, {lng + 0.0002, lat}, {lng + 0.0002, lat + 0.0002}, {lng, lat + 0.0002}, {lng, lat}}}})
		ins(`INSERT INTO properties(id,no,street,suburb,geometry,pf_owner,last_sale,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,'','',?,?,?,1)`, id, no, street, sub, geom, perfJSON(d), created, created)
		if market {
			addMarket(id, i)
			n++
		}
	}
	for i := 0; i < perfLegacyParcels; i++ {
		sub, lat, lng := point()
		id := fmt.Sprintf("L%05d", i)
		d := map[string]any{"id": id, "no": fmt.Sprint(1 + r.IntN(200)), "street": perfStreets[r.IntN(len(perfStreets))], "suburb": sub, "lotplan": fmt.Sprintf("%dSP%d", 1+r.IntN(9), 200000+i), "lat": lat, "lng": lng, "ll": []float64{lat, lng}, "ownerIds": []string{}, "flags": map[string]any{}}
		ins(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,1)`, id, d["no"], d["street"], sub, perfJSON(d), created, created)
	}
	for i := 0; i < perfRexProps; i++ {
		sub, lat, lng := point()
		id, rid := perfRexPropID(i), fmt.Sprint(500000+i)
		no, street := fmt.Sprint(1+r.IntN(200)), perfStreets[r.IntN(len(perfStreets))]
		rex := map[string]any{"_id": rid, "etag": "e1", "adr_street_number": no, "adr_street_name": street, "adr_suburb_or_town": sub, "adr_state_or_region": "QLD", "adr_postcode": "4109", "adr_latitude": fmt.Sprint(lat), "adr_longitude": fmt.Sprint(lng), "attr_bedrooms": 3, "attr_bathrooms": 2, "attr_garages": 1, "system_owner_user": map[string]any{"id": "1", "name": "Office"}, "system_ctime": "1700000000", "system_modtime": "1720000000"}
		d := map[string]any{"id": id, "no": no, "street": street, "suburb": sub, "address": no + " " + street + ", " + sub, "state": "QLD", "postcode": "4109", "lat": fmt.Sprint(lat), "lng": fmt.Sprint(lng), "ll": []float64{lat, lng}, "beds": 3, "baths": 2, "cars": 1, "flags": map[string]any{"dnc": false}, "ownerIds": []string{}, "pfOwner": "", "lastSale": "", "source": "rex_import", "rex": rex}
		if i < perfMarketRex {
			d["rpdataAttributes"] = marketAttrs(i)
		}
		if i%10 == 0 {
			d["notes"] = []any{map[string]any{"id": fmt.Sprintf("pn-%d", i), "d": "2026-10-01", "a": "Agent", "t": "Door knocked, follow up"}}
		}
		ins(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES(?,?,?,?,?,?,1,?,?)`, id, agent(i), no, street, sub, perfJSON(d), created, stamp(60))
		ins(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id,etag) VALUES('properties',?,?,'Properties',?,?,'e1')`, id, rid, created, perfAccount)
		if i < perfMarketRex {
			addMarket(id, i)
		}
	}
	for i := 0; i < perfAppProps; i++ {
		sub, lat, lng := point()
		id := fmt.Sprintf("parcel-app-%04d", i)
		d := map[string]any{"id": id, "no": fmt.Sprint(1 + r.IntN(200)), "street": perfStreets[r.IntN(len(perfStreets))], "suburb": sub, "ll": []float64{lat, lng}, "ownerIds": []string{}, "flags": map[string]any{}, "notes": []any{map[string]any{"id": "n1", "d": "2026-10-02", "a": "Agent", "t": "Interested in selling"}}}
		ins(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,1)`, id, d["no"], d["street"], sub, perfJSON(d), created, stamp(60))
	}
	for i := 0; i < perfContacts; i++ {
		id := perfContactID(i)
		first, last := perfFirst[r.IntN(20)], perfLast[r.IntN(20)]
		mobile := fmt.Sprintf("04%02d %03d %03d", r.IntN(100), r.IntN(1000), r.IntN(1000))
		d := map[string]any{"id": id, "first": first, "last": last, "mobile": mobile, "mobileNorm": normAUMobile(mobile), "email": strings.ToLower(first+"."+last) + fmt.Sprintf("%d@example.invalid", i), "owner": "Agent", "roles": []string{"owner"}, "tags": []string{}, "notes": []any{}, "rels": []any{}, "source": "rex_import", "flags": map[string]any{"dnc": false}, "consent": map[string]any{"sms": true}}
		if i%8 == 0 {
			d["rexId"] = fmt.Sprint(300000 + i)
		}
		if i >= perfContactLinks && i < perfContactLinks+5 {
			d["rexId"] = fmt.Sprint(300000 + i - perfContactLinks) // duplicate reference for duplicateRexLinks
		}
		upd := stamp(60)
		ins(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,roles,tags,flags,consent,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, agent(i), first, last, mobile, d["email"], `["owner"]`, `[]`, `{"dnc":false}`, `{"sms":true}`, perfJSON(d), created, upd)
		if i < perfContactLinks {
			ins(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id,etag) VALUES('contacts',?,?,'Contacts',?,?,'e1')`, id, fmt.Sprint(300000+i), created, perfAccount)
		}
	}
	for i := 0; i < perfOwnerLinks; i++ {
		// Owners belong to the property's agent (same i%10).
		ins(`INSERT INTO property_owners(property_id,contact_id,relation,updated_at) VALUES(?,?,'owner',?)`, perfRexPropID(i), perfContactID(i), stamp(60))
	}
	for i := 0; i < perfTodos; i++ {
		d := map[string]any{"id": fmt.Sprintf("todo-%d", i), "t": "Call back", "d": base.AddDate(0, 0, r.IntN(30)).Format("2006-01-02"), "kind": "call"}
		if i%3 == 0 {
			d["propId"] = perfRexPropID(i % perfRexProps)
		}
		ins(`INSERT INTO todos(id,assignee_id,created_by,team_id,due,time,text,kind,data,created_at,updated_at) VALUES(?,?,?,'default',?,'',?,?,?,?,?)`, d["id"], agent(i), agent(i), d["d"], d["t"], "call", perfJSON(d), created, stamp(60))
	}
	for i := 0; i < perfNotes; i++ {
		c := r.IntN(perfContacts)
		ins(`INSERT INTO contact_notes(id,contact_id,user_id,d,a,t,created_at) VALUES(?,?,?,?,?,?,?)`, fmt.Sprintf("note-%d", i), perfContactID(c), agent(c), "2026-10-01", "Agent", "Spoke about the market", stamp(200))
	}
	for i := 0; i < perfListings; i++ {
		d := map[string]any{"id": fmt.Sprintf("rex-listings-%s-%d", perfAccount, 700000+i), "propertyId": perfRexPropID(i), "ofi": []any{map[string]any{"id": fmt.Sprint(i), "date": "2026-11-08", "attendees": []any{}}}}
		ins(`INSERT INTO listings(id,owner_id,property_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?)`, d["id"], agent(i), perfRexPropID(i), perfJSON(d), created, stamp(60))
	}
	for i := 0; i < perfPhotos; i++ {
		ins(`INSERT INTO photos(id,property_id,created_by,mime,size,created_at) VALUES(?,?,?,?,?,?)`, fmt.Sprintf("photo-%d", i), perfRexPropID(i*7%perfRexProps), agent(i), "image/jpeg", 120000, stamp(60))
	}
	for i := 0; i < perfAudit; i++ {
		entity, id := "contacts", perfContactID(r.IntN(perfContacts))
		if i%3 == 0 {
			entity, id = "properties", perfRexPropID(r.IntN(perfRexProps))
		}
		ins(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, agent(i), "perf", []string{"PATCH", "POST", "PATCH", "DELETE"}[i%4], entity, id, `{"first":"x"}`, stamp(200))
	}
	// Spread over 190 days: the first startup prunes the ~5% past the 180-day retention.
	for i := 0; i < perfRexLog; i++ {
		svc := []string{"Contacts", "Properties", "Listings", "AccountSettings"}[i%4]
		kind := []string{"read", "read", "write", "auth"}[i%4]
		ins(`INSERT INTO rex_request_log(at,account_id,direction,kind,service,method,path,caller,attempt,request_json,http_status,ok,error,duration_ms,result_summary,response_excerpt,entity,local_id,rex_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			stamp(190), perfAccount, "out", kind, svc, "search", "/v1/rex/"+strings.ToLower(svc)+"/search", "poller", 1, `{"limit":100}`, 200, 1, "", 120+r.IntN(400), "rows=100", `{"result":{"rows":[]}}`, "contacts", perfContactID(i%perfContacts), fmt.Sprint(300000+i%perfContacts))
	}
	for i := 0; i < perfOutbox; i++ {
		entity, id, payload := "contacts", perfContactID(i%perfContacts), `{"first":"x"}`
		switch i % 6 {
		case 1:
			entity, id, payload = "rex_contact_relation", fmt.Sprintf("%s:%s", perfRexPropID(i%perfRexProps), perfContactID(i%perfContacts)), perfJSON(map[string]any{"relation": map[string]any{"propertyId": perfRexPropID(i % perfRexProps), "contactId": perfContactID(i % perfContacts)}})
		case 2:
			key := fmt.Sprintf("pn-%d", i)
			entity, id, payload = "rex_property_note", perfRexPropID(i%perfRexProps)+":"+key, perfJSON(map[string]any{"propertyId": perfRexPropID(i % perfRexProps), "noteKey": key})
		case 3:
			entity, id = "todos", fmt.Sprintf("todo-%d", i%perfTodos)
		}
		ins(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,tries,created_at) VALUES(?,?,?,?,?,?,?)`, entity, id, "PATCH", payload, []string{"sent", "sent", "sent", "skipped", "failed", "sent"}[i%6], 1, stamp(120))
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// No ANALYZE: production has no sqlite_stat1, so plans here must match its
	// statistics-free planner.
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}
