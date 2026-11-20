package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func password(s string) (string, error) {
	if len(s) < 8 {
		return "", fmt.Errorf("password must have at least 8 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return string(b), err
}
func createUser(db *sql.DB, email, name, role, pass string) error {
	hash, err := password(pass)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO users(id,email,name,role,password_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(email) DO NOTHING`, ulid(), email, name, role, hash, now(), now())
	return err
}
func (a *app) admin(w http.ResponseWriter, r *http.Request, p principal) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/")
	if (p.role == "admin" && (path == "market-review" || strings.HasPrefix(path, "market-review/"))) ||
		path == "todos" || strings.HasPrefix(path, "todos/") || path == "captures" ||
		path == "ofi_attendees" || strings.HasPrefix(path, "ofi_attendees/") ||
		path == "listings" || path == "contacts" {
		a.adminWork(w, r, p, path)
		return
	}
	if path == "properties" {
		a.adminProperties(w, r, p)
		return
	}
	if path == "parcel-stats" && r.Method == "GET" {
		a.adminParcelStats(w, p)
		return
	}
	if path == "zones" || strings.HasPrefix(path, "zones/") {
		a.adminZones(w, r, p, path)
		return
	}
	if strings.HasPrefix(path, "market/") {
		a.marketAdmin(w, r, p)
		return
	}
	if strings.HasPrefix(path, "rex-outbox/") && strings.HasSuffix(path, "/retry") && r.Method == "POST" {
		id, err := strconvAtoi(strings.TrimSuffix(strings.TrimPrefix(path, "rex-outbox/"), "/retry"))
		if err != nil || id <= 0 {
			problem(w, 400, "invalid id")
			return
		}
		// An uncertain create cannot be replayed until an operator records its Rex id.
		var entity, localID, reason string
		if err := a.db.QueryRow(`SELECT entity,entity_id,COALESCE(last_error,'') FROM rex_outbox WHERE id=? AND status IN ('failed','skipped')`, id).Scan(&entity, &localID, &reason); err != nil {
			problem(w, 404, "row not retryable")
			return
		}
		if strings.Contains(reason, "reconcile before retry") {
			if entity == "contacts" {
				var raw string
				_ = a.db.QueryRow(`SELECT payload FROM rex_outbox WHERE id=?`, id).Scan(&raw)
				var m map[string]any
				_ = json.Unmarshal([]byte(raw), &m)
				if str(m, "contactId") != "" && str(m, "t") != "" {
					entity, localID = "contact_notes", str(m, "id")
				}
			}
			var linked int
			_ = a.db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity=? AND local_id=?`, entity, localID).Scan(&linked)
			if linked == 0 {
				problem(w, 409, "reconcile Rex record and link id before retry")
				return
			}
		}
		res, err := a.db.Exec(`UPDATE rex_outbox SET status='pending',tries=0,last_error=NULL,next_attempt_at=NULL WHERE id=? AND status IN ('failed','skipped')`, id)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			problem(w, 404, "row not retryable")
			return
		}
		respond(w, 200, map[string]any{"id": id, "status": "pending"})
		return
	}
	if path == "overview" && r.Method == "GET" {
		a.overview(w)
		return
	}
	if path == "data" && r.Method == "GET" {
		a.adminData(w, r, p)
		return
	}
	if path == "approvals" && r.Method == "GET" {
		a.adminApprovals(w, r)
		return
	}
	if path == "audit" || path == "rex-outbox" {
		if r.Method != "GET" {
			problem(w, 405, "method not allowed")
			return
		}
		limit, _ := strconvAtoi(r.URL.Query().Get("limit"))
		if limit < 1 || limit > 200 {
			limit = 50
		}
		table := "audit_log"
		if path == "rex-outbox" {
			table = "rex_outbox"
		}
		q := `SELECT * FROM ` + table + ` WHERE id<?`
		before, _ := strconvAtoi(r.URL.Query().Get("before"))
		if before <= 0 {
			before = 1 << 62
		}
		args := []any{before}
		filters := map[string]string{}
		if path == "audit" {
			filters = map[string]string{"user": "user_id", "entity": "entity", "action": "action", "from": "at >=", "to": "at <="}
		} else {
			filters = map[string]string{"status": "status"}
		}
		for _, key := range []string{"user", "entity", "action", "from", "to", "status"} {
			if val := r.URL.Query().Get(key); val != "" && filters[key] != "" {
				col := filters[key]
				if key == "from" || key == "to" {
					q += " AND " + col + " ?"
				} else {
					q += " AND " + col + "=?"
				}
				args = append(args, val)
			}
		}
		q += ` ORDER BY id DESC LIMIT ?`
		args = append(args, limit)
		rows, err := a.db.Query(q, args...)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		items := []any{}
		for rows.Next() {
			values := make([]any, len(cols))
			ptr := make([]any, len(cols))
			for i := range values {
				ptr[i] = &values[i]
			}
			if rows.Scan(ptr...) != nil {
				break
			}
			m := map[string]any{}
			for i, k := range cols {
				m[k] = values[i]
			}
			items = append(items, m)
		}
		respond(w, 200, items)
		return
	}
	if path == "users" {
		if p.role == "leader" {
			if r.Method != "GET" {
				problem(w, 403, "admin only")
				return
			}
			rows, e := a.db.Query(`SELECT id,name,role,team_id FROM users WHERE team_id=(SELECT team_id FROM users WHERE id=?) AND disabled=0 AND deleted_at IS NULL`, p.id)
			if e != nil {
				problem(w, 500, "database error")
				return
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, name, role, team string
				if rows.Scan(&id, &name, &role, &team) == nil {
					out = append(out, map[string]any{"id": id, "name": name, "role": role, "teamId": team})
				}
			}
			respond(w, 200, out)
			return
		}
		switch r.Method {
		case "GET":
			rows, err := a.db.Query(`SELECT u.id,u.email,u.name,u.role,u.team_id,u.disabled,u.version, (SELECT max(created_at) FROM sessions WHERE user_id=u.id), (SELECT device FROM sessions WHERE user_id=u.id ORDER BY created_at DESC LIMIT 1) FROM users u WHERE u.deleted_at IS NULL ORDER BY u.email`)
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, email, name, role, teamID string
				var disabled, ver int
				var last, device sql.NullString
				if rows.Scan(&id, &email, &name, &role, &teamID, &disabled, &ver, &last, &device) == nil {
					out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "disabled": disabled != 0, "version": ver, "lastLogin": last.String, "device": device.String, "teamId": teamID})
				}
			}
			respond(w, 200, out)
			return
		case "POST":
			m, err := body(r)
			if err != nil {
				problem(w, 400, "invalid JSON")
				return
			}
			role := str(m, "role")
			if strings.TrimSpace(str(m, "email")) == "" || strings.TrimSpace(str(m, "name")) == "" {
				problem(w, 400, "email and name required")
				return
			}
			if role != "agent" && role != "admin" && role != "leader" {
				problem(w, 400, "invalid role")
				return
			}
			hash, err := password(str(m, "password"))
			if err != nil {
				problem(w, 400, err.Error())
				return
			}
			id := ulid()
			tx, _ := a.db.Begin()
			defer tx.Rollback()
			_, err = tx.Exec(`INSERT INTO users(id,email,name,role,password_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, strings.ToLower(str(m, "email")), str(m, "name"), role, hash, now(), now())
			if err == nil {
				err = a.record(tx, p, "POST", "users", id, map[string]any{"email": str(m, "email"), "role": role})
			}
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				problem(w, 400, "invalid user")
				return
			}
			respond(w, 201, map[string]any{"id": id, "email": str(m, "email"), "role": role})
			return
		case "PATCH":
			m, err := body(r)
			if err != nil {
				problem(w, 400, "invalid JSON")
				return
			}
			id := str(m, "id")
			var role, name, teamID string
			var disabled, ver int
			err = a.db.QueryRow(`SELECT role,name,team_id,disabled,version FROM users WHERE id=? AND deleted_at IS NULL`, id).Scan(&role, &name, &teamID, &disabled, &ver)
			if err != nil {
				problem(w, 404, "not found")
				return
			}
			if v, ok := m["version"].(float64); !ok || int(v) != ver {
				respond(w, 409, map[string]any{"error": "version conflict", "current": map[string]any{"id": id, "role": role, "name": name, "disabled": disabled != 0, "version": ver}})
				return
			}
			previousRole := role
			if v, ok := m["teamId"].(string); ok {
				var n int
				_ = a.db.QueryRow(`SELECT count(*) FROM teams WHERE id=?`, v).Scan(&n)
				if n == 0 {
					problem(w, 400, "invalid team")
					return
				}
				teamID = v
			}
			if v, ok := m["role"]; ok {
				role, _ = v.(string)
				if role != "admin" && role != "agent" && role != "leader" {
					problem(w, 400, "invalid role")
					return
				}
			}
			if v, ok := m["name"].(string); ok {
				name = v
			}
			if v, ok := m["disabled"].(bool); ok {
				disabled = 0
				if v {
					disabled = 1
				}
			}
			hash := ""
			if v, ok := m["password"].(string); ok {
				hash, err = password(v)
				if err != nil {
					problem(w, 400, err.Error())
					return
				}
			}
			tx, _ := a.db.Begin()
			defer tx.Rollback()
			_, err = tx.Exec(`UPDATE users SET role=?,name=?,team_id=?,disabled=?,password_hash=CASE WHEN ?='' THEN password_hash ELSE ? END,version=version+1,updated_at=? WHERE id=? AND version=?`, role, name, teamID, disabled, hash, hash, now(), id, ver)
			if err == nil && (hash != "" || disabled != 0 || role != previousRole) {
				_, err = tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
			}
			delete(m, "password")
			if err == nil {
				err = a.record(tx, p, "PATCH", "users", id, m)
			}
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			respond(w, 200, map[string]any{"id": id, "role": role, "name": name, "disabled": disabled != 0, "version": ver + 1})
			return
		}
		problem(w, 405, "method not allowed")
		return
	}
	if strings.HasPrefix(path, "users/") && strings.HasSuffix(path, "/revoke") && r.Method == "POST" {
		id := strings.TrimSuffix(strings.TrimPrefix(path, "users/"), "/revoke")
		var n int
		_ = a.db.QueryRow(`SELECT count(*) FROM users WHERE id=?`, id).Scan(&n)
		if n == 0 {
			problem(w, 404, "not found")
			return
		}
		tx, _ := a.db.Begin()
		defer tx.Rollback()
		_, err := tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
		if err == nil {
			err = a.record(tx, p, "revoke", "users", id, map[string]any{})
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		respond(w, 200, map[string]any{"ok": true})
		return
	}
	problem(w, 404, "not found")
}
func strconvAtoi(s string) (int, error) { var n int; _, err := fmt.Sscan(s, &n); return n, err }
func seed(db *sql.DB) error {
	pass := os.Getenv("AXLEO_SEED_PASSWORD")
	var existing int
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE email IN ('admin@axleo.local','mary@axleo.local','john@axleo.local')`).Scan(&existing); err != nil {
		return err
	}
	if pass == "" {
		pass = random()
		if existing < 3 {
			fmt.Println("AXLEO seed password (save now):", pass)
		}
	}
	for _, u := range [][3]string{{"admin@axleo.local", "Admin", "admin"}, {"mary@axleo.local", "Mary", "agent"}, {"john@axleo.local", "John", "agent"}} {
		if err := createUser(db, u[0], u[1], u[2], pass); err != nil {
			return err
		}
	}
	return seedData(db)
}

type seedStore interface {
	Query(string, ...any) (*sql.Rows, error)
	Exec(string, ...any) (sql.Result, error)
}

// seedData is shared by seed and the atomic reset; reset never changes accounts.
func seedData(db seedStore) error {
	ids := map[string]string{}
	rows, err := db.Query(`SELECT name,id FROM users WHERE email IN ('mary@axleo.local','john@axleo.local')`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, id string
		_ = rows.Scan(&name, &id)
		ids[name] = id
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if ids["Mary"] == "" || ids["John"] == "" {
		return fmt.Errorf("Mary and John seed accounts required; run seed first")
	}
	owner := func(s string) string {
		if s == "Natalie Wong" || s == "John" {
			return ids["John"]
		}
		return ids["Mary"]
	}
	b, err := assets.ReadFile("seed/demo.json")
	if err != nil {
		return err
	}
	var demo struct{ Contacts, Properties, Todos, Captures, Enquiries, Leads, Listings, Sales, Contracts, Referrals, Market, Zones, MyTags []map[string]any }
	if err = json.Unmarshal(b, &demo); err != nil {
		return err
	}
	stamp := now()
	for _, m := range demo.Contacts {
		id := str(m, "id")
		ownerID := owner(str(m, "owner"))
		payload, _ := json.Marshal(m)
		_, err = db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,roles,tags,flags,consent,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, append(append([]any{id, ownerID, str(m, "first"), str(m, "last"), str(m, "mobile"), str(m, "email")}, contactColumns(m)...), string(payload), stamp, stamp)...)
		if err != nil {
			return err
		}
		for i, n := range sliceMaps(m["notes"]) {
			_, err = db.Exec(`INSERT INTO contact_notes(id,contact_id,d,a,t,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, fmt.Sprintf("seed-%s-%d", id, i), id, str(n, "d"), str(n, "a"), str(n, "t"), stamp)
			if err != nil {
				return err
			}
		}
	}
	for _, m := range demo.Properties {
		id := str(m, "id")
		payload, _ := json.Marshal(m)
		_, err = db.Exec(`INSERT INTO properties(id,no,street,suburb,geometry,pf_owner,last_sale,flags,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, str(m, "no"), str(m, "street"), str(m, "suburb"), jsonField(m, "geometry"), str(m, "pfOwner"), str(m, "lastSale"), jsonField(m, "flags"), string(payload), stamp, stamp, strings.HasPrefix(id, "p") && len(id) <= 3)
		if err != nil {
			return err
		}
		for _, cid := range stringsFrom(m["ownerIds"]) {
			_, err = db.Exec(`INSERT INTO property_owners(property_id,contact_id) SELECT ?,id FROM contacts WHERE id=? AND deleted_at IS NULL ON CONFLICT DO NOTHING`, id, cid)
			if err != nil {
				return err
			}
		}
	}
	for _, m := range demo.Todos {
		payload, _ := json.Marshal(m)
		_, err = db.Exec(`INSERT INTO todos(id,assignee_id,created_by,due,time,text,kind,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, str(m, "id"), owner(str(m, "who")), owner(str(m, "by")), str(m, "d"), str(m, "time"), str(m, "t"), str(m, "kind"), string(payload), stamp, stamp)
		if err != nil {
			return err
		}
	}
	groups := map[string][]map[string]any{"captures": demo.Captures, "enquiries": demo.Enquiries, "leads": demo.Leads, "listings": demo.Listings, "sales": demo.Sales, "contracts": demo.Contracts, "referrals": demo.Referrals, "market_items": demo.Market, "zones": demo.Zones, "tags": demo.MyTags}
	for kind, items := range groups {
		for i, m := range items {
			id := str(m, "id")
			if kind == "tags" {
				id = fmt.Sprintf("seed-tag-%d", i)
				m["id"] = id
			}
			who := str(m, "owner")
			switch kind {
			case "listings":
				who = str(m, "agent")
			case "tags":
				who = str(m, "by")
			}
			ownerID := owner(who)
			m["ownerId"] = ownerID
			payload, _ := json.Marshal(m)
			if kind == "referrals" || kind == "leads" {
				_, err = db.Exec(`INSERT INTO `+kind+`(id,owner_id,contact_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, ownerID, str(m, "contactId"), string(payload), stamp, stamp)
			} else {
				_, err = db.Exec(`INSERT INTO `+kind+`(id,owner_id,data,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, ownerID, string(payload), stamp, stamp)
			}
			if err != nil {
				return err
			}
			if kind == "listings" {
				for j, event := range sliceMaps(m["ofi"]) {
					for k, attendee := range sliceMaps(event["attendees"]) {
						aid := fmt.Sprintf("seed-ofi-%s-%d-%d", id, j, k)
						attendee["id"] = aid
						attendee["listingId"] = id
						attendee["eventIndex"] = j
						attendee["ownerId"] = ownerID
						raw, _ := json.Marshal(attendee)
						_, err = db.Exec(`INSERT INTO ofi_attendees(id,owner_id,event_id,contact_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, aid, ownerID, id, str(attendee, "contactId"), string(raw), stamp, stamp)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
func sliceMaps(v any) []map[string]any {
	var out []map[string]any
	for _, x := range v.([]any) {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
func main() {
	if len(os.Args) >= 2 && os.Args[1] == "rex-probe" {
		fatal(rexProbe(os.Args[2:]))
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: axleo serve|migrate|seed|reset-data [--yes]|passwd <email>|backup <dir>|verify <file>|restore <file>|rex-probe")
		os.Exit(2)
	}
	path := env("AXLEO_DB", "./axleo.db")
	if os.Args[1] == "verify" || os.Args[1] == "restore" {
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("file required"))
		}
		if os.Args[1] == "verify" {
			counts, err := verify(os.Args[2])
			fatal(err)
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s: %d\n", k, counts[k])
			}
		} else {
			saved, err := restore(path, os.Args[2])
			fatal(err)
			fmt.Println("previous database saved:", saved)
		}
		return
	}
	var lock *os.File
	if os.Args[1] == "serve" || os.Args[1] == "reset-data" {
		var err error
		lock, err = serviceLock(path)
		fatal(err)
		defer lock.Close()
	}
	db, err := open(path)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	if os.Args[1] == "reset-data" {
		if len(os.Args) != 2 && (len(os.Args) != 3 || os.Args[2] != "--yes") {
			fatal(fmt.Errorf("usage: axleo reset-data [--yes]"))
		}
		// Do not migrate or write anything during a dry run.
		fatal(resetData(db, path, len(os.Args) == 3))
		return
	}
	if err = migrate(db, path); err != nil {
		fatal(err)
	}
	switch os.Args[1] {
	case "backup":
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("directory required"))
		}
		file, err := backup(db, os.Args[2])
		fatal(err)
		fmt.Println(file)
	case "migrate":
		return
	case "seed":
		fatal(seed(db))
	case "passwd":
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("email required"))
		}
		var pass string
		if _, err = fmt.Fscanln(os.Stdin, &pass); err != nil {
			fatal(err)
		}
		hash, err := password(pass)
		if err != nil {
			fatal(err)
		}
		res, err := db.Exec(`UPDATE users SET password_hash=?,updated_at=?,version=version+1 WHERE email=?`, hash, now(), os.Args[2])
		if err != nil {
			fatal(err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			fatal(fmt.Errorf("user not found"))
		}
		_, err = db.Exec(`DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE email=?)`, os.Args[2])
		fatal(err)
	case "market-pull":
		_, err := (pricefinderAPISource{}).Fetch(context.Background(), time.Time{})
		if err != nil {
			if os.Getenv("AXLEO_PRICEFINDER_API_KEY") != "" {
				fatal(err)
			}
			fmt.Println(err)
		}
		return
	case "market-import":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("CSV file required"))
		}
		provider, kind, dry := "pricefinder", "auto", false
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--provider", "--kind":
				if i+1 >= len(os.Args) {
					fatal(fmt.Errorf("missing flag value"))
				}
				i++
				if os.Args[i-1] == "--provider" {
					provider = os.Args[i]
				} else {
					kind = os.Args[i]
				}
			case "--dry-run":
				dry = true
			default:
				fatal(fmt.Errorf("unknown flag %s", os.Args[i]))
			}
		}
		info, err := os.Stat(os.Args[2])
		fatal(err)
		if info.Size() > 20<<20 {
			fatal(fmt.Errorf("file exceeds 20MB"))
		}
		b, err := os.ReadFile(os.Args[2])
		fatal(err)
		var id string
		fatal(db.QueryRow(`SELECT id FROM users WHERE role='admin' ORDER BY created_at LIMIT 1`).Scan(&id))
		report, err := (&app{db: db}).importMarket(b, os.Args[2], provider, kind, dry, principal{id: id, role: "admin", device: "CLI"})
		fatal(err)
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
	case "serve":
		a := &app{db: db, failures: map[string][]time.Time{}}
		if worker := newRexWorker(db); worker != nil {
			go worker.run(context.Background())
		}
		fatal(http.ListenAndServe(env("AXLEO_ADDR", "127.0.0.1:8082"), a.routes()))
	default:
		fatal(fmt.Errorf("unknown command"))
	}
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
