package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
		q := `SELECT * FROM ` + table + ` WHERE id<? ORDER BY id DESC LIMIT ?`
		before, _ := strconvAtoi(r.URL.Query().Get("before"))
		if before <= 0 {
			before = 1 << 62
		}
		rows, err := a.db.Query(q, before, limit)
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
		switch r.Method {
		case "GET":
			rows, err := a.db.Query(`SELECT id,email,name,role,disabled,version FROM users WHERE deleted_at IS NULL ORDER BY email`)
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, email, name, role string
				var disabled, ver int
				if rows.Scan(&id, &email, &name, &role, &disabled, &ver) == nil {
					out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "disabled": disabled != 0, "version": ver})
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
			if role != "agent" && role != "admin" {
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
			var role, name string
			var disabled, ver int
			err = a.db.QueryRow(`SELECT role,name,disabled,version FROM users WHERE id=? AND deleted_at IS NULL`, id).Scan(&role, &name, &disabled, &ver)
			if err != nil {
				problem(w, 404, "not found")
				return
			}
			if v, ok := m["version"].(float64); !ok || int(v) != ver {
				respond(w, 409, map[string]any{"error": "version conflict", "current": map[string]any{"id": id, "role": role, "name": name, "disabled": disabled != 0, "version": ver}})
				return
			}
			if v, ok := m["role"]; ok {
				role, _ = v.(string)
				if role != "admin" && role != "agent" {
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
			_, err = tx.Exec(`UPDATE users SET role=?,name=?,disabled=?,password_hash=CASE WHEN ?='' THEN password_hash ELSE ? END,version=version+1,updated_at=? WHERE id=? AND version=?`, role, name, disabled, hash, hash, now(), id, ver)
			if err == nil && hash != "" {
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
	rows.Close()
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
	var demo struct{ Contacts, Properties, Todos []map[string]any }
	if err = json.Unmarshal(b, &demo); err != nil {
		return err
	}
	stamp := now()
	for _, m := range demo.Contacts {
		id := str(m, "id")
		ownerID := owner(str(m, "owner"))
		payload, _ := json.Marshal(m)
		_, err = db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,roles,tags,flags,consent,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, ownerID, str(m, "first"), str(m, "last"), str(m, "mobile"), str(m, "email"), jsonField(m, "roles"), jsonField(m, "tags"), jsonField(m, "flags"), jsonField(m, "consent"), string(payload), stamp, stamp)
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
			_, err = db.Exec(`INSERT INTO property_owners(property_id,contact_id) VALUES(?,?) ON CONFLICT DO NOTHING`, id, cid)
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
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: axleo serve|migrate|seed|passwd <email>")
		os.Exit(2)
	}
	path := env("AXLEO_DB", "./axleo.db")
	db, err := open(path)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	if err = migrate(db, path); err != nil {
		fatal(err)
	}
	switch os.Args[1] {
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
	case "serve":
		a := &app{db: db, failures: map[string][]time.Time{}}
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
