package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Australia/Brisbane must load even without system zoneinfo

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql seed/demo.json
var assets embed.FS
var version = "dev"

const layout = time.RFC3339Nano

func now() string { return time.Now().UTC().Format(layout) }
func env(k, v string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return v
}
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func ulid() string {
	b := make([]byte, 16)
	ms := uint64(time.Now().UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	if _, err := rand.Read(b[6:]); err != nil {
		panic(err)
	}
	n := new(big.Int).SetBytes(b)
	base := big.NewInt(32)
	alphabet := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	out := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(n, base, r)
		out[i] = alphabet[r.Int64()]
		n = q
	}
	return string(out)
}
func open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON"} {
		if _, err = db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}
func migrate(db *sql.DB, path string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	entries, err := assets.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var existingTables int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT IN ('schema_migrations','sqlite_sequence')`).Scan(&existingTables); err != nil {
		return err
	}
	backedUp := false
	for _, e := range entries {
		name := e.Name()
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		// SQLite's online backup is needed for WAL databases: VACUUM INTO yields a consistent snapshot.
		if existingTables > 0 && !backedUp {
			dir := env("AXLEO_BACKUP_DIR", filepath.Join(filepath.Dir(path), "backup"))
			if err = os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			dest := filepath.Join(dir, fmt.Sprintf("axleo-%d.db", time.Now().UnixNano()))
			if _, err = db.Exec(`VACUUM INTO '` + strings.ReplaceAll(dest, "'", "''") + `'`); err != nil {
				return err
			}
			backedUp = true
		}
		b, err := assets.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		// SQLite cannot drop a referenced users table while FK enforcement is on.
		// Keep the existing WAL backup and single DB connection; rebuild in a
		// transaction, check every reference before committing, then restore FKs.
		if name == "010_console.sql" {
			if _, err = db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				return err
			}
		}
		tx, err := db.Begin()
		if err != nil {
			if name == "010_console.sql" {
				_, _ = db.Exec(`PRAGMA foreign_keys=ON`)
			}
			return err
		}
		if _, err = tx.Exec(string(b)); err == nil {
			_, err = tx.Exec(`INSERT INTO schema_migrations VALUES (?,?)`, name, now())
		}
		if err == nil && name == "010_console.sql" {
			var check *sql.Rows
			check, err = tx.Query(`PRAGMA foreign_key_check`)
			if err == nil {
				if check.Next() {
					err = fmt.Errorf("migration 010: broken foreign key")
				}
				check.Close()
			}
		}
		if err != nil {
			tx.Rollback()
			if name == "010_console.sql" {
				_, _ = db.Exec(`PRAGMA foreign_keys=ON`)
			}
			return err
		}
		if err = tx.Commit(); err != nil {
			if name == "010_console.sql" {
				_, _ = db.Exec(`PRAGMA foreign_keys=ON`)
			}
			return err
		}
		if name == "010_console.sql" {
			if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
				return err
			}
		}
	}
	return nil
}

type app struct {
	db       *sql.DB
	mu       sync.Mutex
	failures map[string][]time.Time
	calls    map[string][]time.Time
}
type principal struct{ id, role, device, name, email string }

func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, code int, msg string) {
	respond(w, code, map[string]any{"error": msg})
}
func body(r *http.Request) (map[string]any, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	defer r.Body.Close()
	var m map[string]any
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("object required")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return m, nil
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func (a *app) auth(r *http.Request) (principal, bool) {
	var p principal
	h := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(h) != 2 || !strings.EqualFold(h[0], "Bearer") {
		return p, false
	}
	raw, err := hex.DecodeString(h[1])
	if err != nil || len(raw) != 32 {
		return p, false
	}
	hash := sha256.Sum256(raw)
	var expires string
	var disabled int
	err = a.db.QueryRow(`SELECT u.id,u.role,s.device,u.name,u.email,s.expires_at,u.disabled FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?`, hex.EncodeToString(hash[:])).Scan(&p.id, &p.role, &p.device, &p.name, &p.email, &expires, &disabled)
	if err != nil || disabled != 0 {
		return p, false
	}
	t, err := time.Parse(layout, expires)
	if err != nil || !t.After(time.Now()) {
		return p, false
	}
	// Session expiry is fixed at login; never extend beyond the configured lifetime.
	return p, true
}
func (a *app) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		var n int
		_ = a.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n)
		respond(w, 200, map[string]any{"ok": true, "version": version, "schema": n})
	})
	m.HandleFunc("POST /api/v1/auth/login", a.login)
	m.HandleFunc("POST /api/v1/rex/webhook/{token}", a.rexWebhook)
	m.Handle("GET /admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(webRoot()))))
	m.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	m.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := a.auth(r)
		if !ok {
			problem(w, 401, "unauthorized")
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/stt/transcribe":
			if !a.allowCall(p) {
				problem(w, 429, "rate limited")
				return
			}
			a.transcribe(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/tts":
			if !a.allowEndpointCall(p, "tts", 60) {
				problem(w, 429, "rate limited")
				return
			}
			a.synthesize(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/ai/translate":
			if !a.allowEndpointCall(p, "ai_translate", 30) {
				problem(w, 429, "rate limited")
				return
			}
			a.translateAI(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/photos":
			if !a.allowEndpointCall(p, "photos", 60) {
				problem(w, 429, "rate limited")
				return
			}
			a.uploadPhoto(w, r, p)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/photos/"):
			a.servePhoto(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/ai/image":
			if !a.allowCall(p) {
				problem(w, 429, "rate limited")
				return
			}
			a.imageAI(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/ai/extract":
			if !a.allowCall(p) {
				problem(w, 429, "rate limited")
				return
			}
			a.extractAI(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/auth/logout":
			raw, _ := hex.DecodeString(strings.Fields(r.Header.Get("Authorization"))[1])
			h := sha256.Sum256(raw)
			_, _ = a.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, hex.EncodeToString(h[:]))
			respond(w, 200, map[string]any{"ok": true})
		case r.URL.Path == "/api/v1/devices":
			a.devices(w, r, p)
		case r.Method == "GET" && r.URL.Path == "/api/v1/me":
			respond(w, 200, map[string]any{"id": p.id, "name": p.name, "email": p.email, "role": p.role})
		case r.Method == "GET" && r.URL.Path == "/api/v1/team":
			a.team(w, p)
		case r.Method == "GET" && r.URL.Path == "/api/v1/sync":
			a.syncPull(w, r, p)
		case r.Method == "POST" && r.URL.Path == "/api/v1/sync/push":
			a.syncPush(w, r, p)
		case strings.HasPrefix(r.URL.Path, "/api/v1/approvals") || strings.HasSuffix(r.URL.Path, "/merge") || strings.HasSuffix(r.URL.Path, "/reassign"):
			a.approvalRoute(w, r, p)
		case strings.HasPrefix(r.URL.Path, "/api/v1/admin/"):
			if p.role != "admin" && p.role != "leader" {
				path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/")
				if path == "todos" || strings.HasPrefix(path, "todos/") ||
					path == "captures" || path == "listings" || path == "contacts" ||
					path == "ofi_attendees" || strings.HasPrefix(path, "ofi_attendees/") ||
					path == "market-review" || strings.HasPrefix(path, "market-review/") {
					problem(w, 403, "admin only")
					return
				}
				problem(w, 404, "not found")
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/")
			if p.role == "leader" && path != "metrics" && path != "teams" && path != "todos" && !strings.HasPrefix(path, "todos/") && path != "captures" && path != "ofi_attendees" && path != "listings" && path != "parcel-stats" && path != "properties" && path != "users" && path != "zones" && path != "contacts" {
				problem(w, 403, "admin only")
				return
			}
			if !a.console(w, r, p, path) {
				a.admin(w, r, p)
			}
		default:
			a.resource(w, r, p)
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' blob: data: https://*.tile.openstreetmap.org; "+
					"connect-src 'self'; font-src 'self'; base-uri 'none'; "+
					"form-action 'self'; frame-ancestors 'none'")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		m.ServeHTTP(w, r)
	})
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	m, err := body(r)
	if err != nil {
		problem(w, 400, "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(str(m, "email")))
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	key := ip + "|" + email
	a.mu.Lock()
	valid := a.failures[key][:0]
	for _, t := range a.failures[key] {
		if time.Since(t) < 15*time.Minute {
			valid = append(valid, t)
		}
	}
	a.failures[key] = valid
	blocked := len(valid) >= 5
	a.mu.Unlock()
	if blocked {
		problem(w, 429, "too many attempts")
		return
	}
	var id, name, role, hash string
	var disabled int
	err = a.db.QueryRow(`SELECT id,name,role,password_hash,disabled FROM users WHERE email=?`, email).Scan(&id, &name, &role, &hash, &disabled)
	if err != nil || disabled != 0 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(str(m, "password"))) != nil {
		a.mu.Lock()
		a.failures[key] = append(a.failures[key], time.Now())
		a.mu.Unlock()
		problem(w, 401, "invalid credentials")
		return
	}
	a.mu.Lock()
	delete(a.failures, key)
	a.mu.Unlock()
	token := random()
	raw, _ := hex.DecodeString(token)
	sum := sha256.Sum256(raw)
	var hours int
	if e := a.db.QueryRow(`SELECT value FROM settings WHERE key='session_hours'`).Scan(&hours); e != nil || hours < 1 {
		hours = 720
	}
	_, err = a.db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?)`, hex.EncodeToString(sum[:]), id, str(m, "device"), time.Now().UTC().Add(time.Duration(hours)*time.Hour).Format(layout), now())
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"token": token, "user": map[string]any{"id": id, "name": name, "email": email, "role": role}})
}

// scope is the only authority for resource visibility. Every read and write uses it.
func scope(kind string, p principal) (string, []any) {
	if p.role == "admin" || kind == "listings" {
		return "", nil
	}
	if kind == "properties" {
		return " AND (owner_id IS NULL OR owner_id=?)", []any{p.id}
	}
	if kind == "tags" {
		return " AND (owner_id=? OR owner_id='')", []any{p.id}
	}
	if kind == "contacts" || b3Kind(kind) || kind == "approvals" {
		return " AND owner_id=?", []any{p.id}
	}
	return " AND (assignee_id=? OR created_by=?)", []any{p.id, p.id}
}
func writeScope(kind string, p principal) (string, []any) {
	if p.role != "admin" && (kind == "listings" || kind == "tags") {
		return " AND owner_id=?", []any{p.id}
	}
	return scope(kind, p)
}
func b3Kind(k string) bool {
	switch k {
	case "captures", "enquiries", "leads", "listings", "ofi_attendees", "sales", "contracts", "referrals", "market_items", "zones", "tags", "activity":
		return true
	}
	return false
}
func allowedKind(k string) bool {
	return k == "contacts" || k == "properties" || k == "todos" || b3Kind(k)
}
func (a *app) b3Owner(kind, id string) string {
	var owner string
	_ = a.db.QueryRow("SELECT owner_id FROM "+kind+" WHERE id=?", id).Scan(&owner)
	return owner
}

func (a *app) get(kind, id string, p principal, deleted bool) (map[string]any, error) {
	s, args := scope(kind, p)
	q := `SELECT data,version,updated_at,deleted_at`
	if kind == "contacts" {
		q += `,owner_id`
	}
	if kind == "todos" {
		q += `,assignee_id,created_by`
	}
	if b3Kind(kind) || kind == "approvals" {
		q += `,owner_id`
	}
	q += ` FROM ` + kind + ` WHERE id=?` + s
	if !deleted {
		q += ` AND deleted_at IS NULL`
	}
	args = append([]any{id}, args...)
	var data, updated string
	var ver int
	var del sql.NullString
	var owner, assignee, creator string
	scan := []any{&data, &ver, &updated, &del}
	if kind == "contacts" {
		scan = append(scan, &owner)
	}
	if kind == "todos" {
		scan = append(scan, &assignee, &creator)
	}
	if b3Kind(kind) || kind == "approvals" {
		scan = append(scan, &owner)
	}
	if err := a.db.QueryRow(q, args...).Scan(scan...); err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return nil, err
	}
	if kind == "contacts" {
		m["ownerId"] = owner
	}
	if kind == "todos" {
		m["assigneeId"] = assignee
		m["createdBy"] = creator
	}
	if b3Kind(kind) || kind == "approvals" {
		m["ownerId"] = owner
	}
	m["version"] = ver
	m["updated_at"] = updated
	if del.Valid {
		m["deleted_at"] = del.String
	}
	if kind == "properties" {
		a.owners(m, p)
		a.marketProperty(m, p)
		if p.role != "admin" && !a.covered(str(m, "id"), p.id) {
			delete(m, "pfOwner")
		}
	}
	if kind == "listings" {
		stripOFI(m)
	}
	return m, nil
}
func (a *app) owners(m map[string]any, p principal) {
	rows, err := a.db.Query(`SELECT contact_id FROM property_owners WHERE property_id=?`, str(m, "id"))
	if err != nil {
		m["ownerIds"] = []string{}
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []string{}
	for _, id := range ids {
		s, args := scope("contacts", p)
		args = append([]any{id}, args...)
		var n int
		_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE id=? AND deleted_at IS NULL`+s, args...).Scan(&n)
		if n > 0 {
			out = append(out, id)
		}
	}
	m["ownerIds"] = out
}
func (a *app) resource(w http.ResponseWriter, r *http.Request, p principal) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(path, "/")
	kind := parts[0]
	if !allowedKind(kind) || len(parts) > 3 {
		problem(w, 404, "not found")
		return
	}
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	if len(parts) == 3 {
		if kind == "contacts" && id != "" && parts[2] == "notes" && r.Method == "POST" {
			a.note(w, r, p, id)
			return
		}
		problem(w, 404, "not found")
		return
	}
	if r.Method == "GET" {
		if id != "" {
			v, err := a.get(kind, id, p, false)
			if err != nil {
				problem(w, 404, "not found")
				return
			}
			respond(w, 200, v)
			return
		}
		s, args := scope(kind, p)
		since := r.URL.Query().Get("since")
		q := `SELECT id FROM ` + kind + ` WHERE 1=1` + s
		if since != "" {
			q += ` AND updated_at>?`
			args = append(args, since)
		} else {
			q += ` AND deleted_at IS NULL`
		}
		q += ` ORDER BY updated_at,id`
		rows, err := a.db.Query(q, args...)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		ids := []string{}
		for rows.Next() {
			var v string
			if rows.Scan(&v) == nil {
				ids = append(ids, v)
			}
		}
		rows.Close()
		items := []any{}
		for _, v := range ids {
			item, err := a.get(kind, v, p, since != "")
			if err == nil {
				items = append(items, item)
			}
		}
		respond(w, 200, items)
		return
	}
	if r.Method != "POST" && r.Method != "PATCH" && r.Method != "DELETE" {
		problem(w, 405, "method not allowed")
		return
	}
	if (r.Method == "POST") != (id == "") {
		problem(w, 404, "not found")
		return
	}
	var m map[string]any
	var err error
	if r.Method != "DELETE" {
		m, err = body(r)
		if err != nil {
			problem(w, 400, "invalid JSON")
			return
		}
	}
	a.mutate(w, r, p, kind, id, m)
}
func (a *app) mutate(w http.ResponseWriter, r *http.Request, p principal, kind, id string, m map[string]any) {
	method := r.Method
	old := map[string]any{}
	_, ownersChanged := m["ownerIds"]
	if method == "POST" {
		id = str(m, "id")
		if id == "" {
			id = ulid()
		}
		if len(id) > 128 {
			problem(w, 400, "invalid id")
			return
		}
		if existing, err := a.get(kind, id, p, false); err == nil {
			respond(w, 200, existing)
			return
		}
		var exists int
		_ = a.db.QueryRow(`SELECT count(*) FROM `+kind+` WHERE id=?`, id).Scan(&exists)
		if exists > 0 {
			problem(w, 404, "not found")
			return
		}
		m["id"] = id
	} else {
		var err error
		old, err = a.get(kind, id, p, false)
		if err != nil {
			problem(w, 404, "not found")
			return
		}
		if kind == "listings" && p.role != "admin" && a.b3Owner(kind, id) != p.id {
			problem(w, 404, "not found")
			return
		}
		if method == "PATCH" {
			v, ok := m["version"].(float64)
			if !ok {
				problem(w, 400, "version required")
				return
			}
			if int(v) != old["version"] {
				respond(w, 409, map[string]any{"error": "version conflict", "current": old})
				return
			}
		}
		if method == "DELETE" {
			m = old
		} else {
			for k, v := range m {
				if k != "id" && k != "version" && k != "updated_at" && k != "deleted_at" {
					old[k] = v
				}
			}
			m = old
		}
	}
	previousVersion := old["version"]
	previousAssignee := ""
	if kind == "todos" && method != "POST" {
		_ = a.db.QueryRow(`SELECT assignee_id FROM todos WHERE id=?`, id).Scan(&previousAssignee)
	}
	owner := p.id
	assignee := p.id
	creator := p.id
	if kind == "contacts" {
		if method == "POST" && p.role == "admin" && str(m, "ownerId") != "" {
			owner = str(m, "ownerId")
		}
		if method != "POST" {
			owner = a.ownerID(kind, id)
			if p.role == "admin" && method == "PATCH" && str(m, "ownerId") != "" {
				owner = str(m, "ownerId") // admin reassigns directly
			}
		}
		if p.role != "admin" {
			owner = p.id
			// Agents hand a contact to a teammate directly (prototype: no approval step).
			if method == "PATCH" && a.teammate(p, str(m, "ownerId")) {
				owner = str(m, "ownerId")
			}
		}
		m["ownerId"] = owner
		if str(m, "owner") == "" {
			m["owner"] = p.name
		}
	}
	if kind == "todos" {
		if method == "POST" {
			if p.role == "admin" && str(m, "assigneeId") != "" {
				assignee = str(m, "assigneeId")
			}
			if p.role == "admin" && str(m, "createdBy") != "" {
				creator = str(m, "createdBy")
			}
			if p.role != "admin" && a.teammate(p, str(m, "assigneeId")) {
				assignee = str(m, "assigneeId")
			}
		} else {
			_ = a.db.QueryRow(`SELECT assignee_id,created_by FROM todos WHERE id=?`, id).Scan(&assignee, &creator)
			if p.role == "admin" {
				if str(m, "assigneeId") != "" {
					assignee = str(m, "assigneeId")
				}
				if str(m, "createdBy") != "" {
					creator = str(m, "createdBy")
				}
			} else if method == "PATCH" && str(m, "assigneeId") != assignee && a.teammate(p, str(m, "assigneeId")) {
				// Reassigning keeps it under the agent's "Assigned by me" (prototype sets by=ME()).
				assignee = str(m, "assigneeId")
				creator = p.id
			}
		}
		m["assigneeId"] = assignee
		m["createdBy"] = creator
	}
	if b3Kind(kind) {
		if method != "POST" {
			owner = a.b3Owner(kind, id)
		}
		if p.role == "admin" && str(m, "ownerId") != "" {
			owner = str(m, "ownerId")
		}
		if kind == "tags" && p.role == "admin" && method == "POST" && str(m, "ownerId") == "" {
			owner = ""
		}
		if p.role != "admin" {
			owner = p.id
		}
		m["ownerId"] = owner
		if kind == "listings" && method == "POST" && p.role != "admin" {
			m["agent"] = p.name
		}
	}
	if kind == "properties" && method != "DELETE" {
		if ownersChanged {
			for _, cid := range stringsFrom(m["ownerIds"]) {
				if _, err := a.get("contacts", cid, p, false); err != nil {
					problem(w, 404, "not found")
					return
				}
			}
		}
		if method == "PATCH" {
			if !ownersChanged { // Preserve every association, including invisible ones.
				rows, err := a.db.Query(`SELECT contact_id FROM property_owners WHERE property_id=?`, id)
				if err != nil {
					problem(w, 500, "database error")
					return
				}
				all := []string{}
				for rows.Next() {
					var cid string
					if rows.Scan(&cid) == nil {
						all = append(all, cid)
					}
				}
				rows.Close()
				m["ownerIds"] = all
			} else if p.role != "admin" {
				rows, err := a.db.Query(`SELECT po.contact_id FROM property_owners po JOIN contacts c ON c.id=po.contact_id WHERE po.property_id=? AND c.owner_id<>?`, id, p.id)
				if err != nil {
					problem(w, 500, "database error")
					return
				}
				all := stringsFrom(m["ownerIds"])
				for rows.Next() {
					var cid string
					if rows.Scan(&cid) == nil {
						all = append(all, cid)
					}
				}
				rows.Close()
				m["ownerIds"] = all
			}
		}
	}
	if kind == "listings" {
		stripOFI(m)
	}
	if kind == "properties" {
		delete(m, "market")
		if p.role != "admin" {
			if method == "PATCH" {
				var raw string
				if a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw) == nil {
					var original map[string]any
					_ = json.Unmarshal([]byte(raw), &original)
					m["pfOwner"] = str(original, "pfOwner")
				}
			} else {
				delete(m, "pfOwner")
			}
		}
	} // third-party market data is read-only
	stamp := now()
	m["id"] = id
	delete(m, "version")
	delete(m, "updated_at")
	delete(m, "deleted_at")
	b, _ := json.Marshal(m)
	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	var result sql.Result
	switch method {
	case "POST":
		switch kind {
		case "contacts":
			_, err = tx.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,data,created_at,updated_at,roles,tags,flags,consent) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, append([]any{id, owner, str(m, "first"), str(m, "last"), str(m, "mobile"), str(m, "email"), string(b), stamp, stamp}, contactColumns(m)...)...)
		case "todos":
			_, err = tx.Exec(`INSERT INTO todos(id,assignee_id,created_by,team_id,due,time,text,kind,data,created_at,updated_at) VALUES(?,?,?,(SELECT team_id FROM users WHERE id=?),?,?,?,?,?,?,?)`, id, assignee, creator, assignee, str(m, "d"), str(m, "time"), str(m, "t"), str(m, "kind"), string(b), stamp, stamp)
		case "properties":
			_, err = tx.Exec(`INSERT INTO properties(id,no,street,suburb,geometry,pf_owner,last_sale,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,?,?,?,1)`, id, str(m, "no"), str(m, "street"), str(m, "suburb"), jsonField(m, "geometry"), str(m, "pfOwner"), str(m, "lastSale"), string(b), stamp, stamp)
		case "leads", "referrals", "ofi_attendees":
			if kind == "ofi_attendees" {
				_, err = tx.Exec(`INSERT INTO ofi_attendees(id,owner_id,event_id,contact_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, owner, str(m, "listingId"), str(m, "contactId"), string(b), stamp, stamp)
			} else {
				_, err = tx.Exec(`INSERT INTO `+kind+`(id,owner_id,contact_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id, owner, str(m, "contactId"), string(b), stamp, stamp)
			}
		default:
			_, err = tx.Exec(`INSERT INTO `+kind+`(id,owner_id,data,created_at,updated_at) VALUES(?,?,?,?,?)`, id, owner, string(b), stamp, stamp)
		}
	case "PATCH":
		q := `UPDATE ` + kind + ` SET data=?,updated_at=?,version=version+1`
		args := []any{string(b), stamp}
		if kind == "contacts" {
			q += `,first=?,last=?,mobile=?,email=?,owner_id=?,roles=?,tags=?,flags=?,consent=?`
			args = append(args, str(m, "first"), str(m, "last"), str(m, "mobile"), str(m, "email"), owner)
			args = append(args, contactColumns(m)...)
		}
		if kind == "todos" {
			q += `,due=?,time=?,text=?,kind=?,assignee_id=?,created_by=?,team_id=(SELECT team_id FROM users WHERE id=?)`
			args = append(args, str(m, "d"), str(m, "time"), str(m, "t"), str(m, "kind"), assignee, creator, assignee)
		}
		if b3Kind(kind) {
			q += `,owner_id=?`
			args = append(args, owner)
			if kind == "leads" || kind == "referrals" || kind == "ofi_attendees" {
				q += `,contact_id=?`
				args = append(args, str(m, "contactId"))
			}
			if kind == "ofi_attendees" {
				q += `,event_id=?`
				args = append(args, str(m, "listingId"))
			}
		}
		if kind == "properties" {
			q += `,no=?,street=?,suburb=?,geometry=?,pf_owner=?,last_sale=?,registered=1`
			args = append(args, str(m, "no"), str(m, "street"), str(m, "suburb"), jsonField(m, "geometry"), str(m, "pfOwner"), str(m, "lastSale"))
		}
		s, sc := writeScope(kind, p)
		q += ` WHERE id=? AND version=? AND deleted_at IS NULL` + s
		args = append(args, id, previousVersion)
		args = append(args, sc...)
		result, err = tx.Exec(q, args...)
	case "DELETE":
		s, sc := writeScope(kind, p)
		args := append([]any{stamp, stamp, id, previousVersion}, sc...)
		result, err = tx.Exec(`UPDATE `+kind+` SET deleted_at=?,updated_at=?,version=version+1 WHERE id=? AND version=? AND deleted_at IS NULL`+s, args...)
	}
	if err != nil {
		problem(w, 400, "invalid record")
		return
	}
	if result != nil {
		n, _ := result.RowsAffected()
		if n == 0 {
			_ = tx.Rollback()
			current, e := a.get(kind, id, p, false)
			if e != nil {
				problem(w, 404, "not found")
			} else {
				respond(w, 409, map[string]any{"error": "version conflict", "current": current})
			}
			return
		}
	}
	if kind == "properties" && method != "DELETE" {
		if _, err = tx.Exec(`DELETE FROM property_owners WHERE property_id=?`, id); err != nil {
			problem(w, 400, "invalid owners")
			return
		}
		for _, cid := range stringsFrom(m["ownerIds"]) {
			if _, err = tx.Exec(`INSERT INTO property_owners(property_id,contact_id) VALUES (?,?)`, id, cid); err != nil {
				problem(w, 400, "invalid owners")
				return
			}
		}
	}
	if err = a.record(tx, p, method, kind, id, m); err != nil {
		problem(w, 500, "database error")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "database error")
		return
	}
	a.notifyMutation(kind, method, id, m, previousAssignee, p.id)
	if method == "DELETE" {
		respond(w, 200, map[string]any{"id": id, "deleted_at": stamp})
		return
	}
	item, err := a.get(kind, id, p, false)
	if err != nil && method == "PATCH" {
		// Handed to a teammate: no longer in this agent's scope, so echo it unscoped once.
		item, err = a.get(kind, id, principal{id: p.id, role: "admin"}, false)
	}
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	if method == "POST" {
		respond(w, 201, item)
	} else {
		respond(w, 200, item)
	}
}
func stringsFrom(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		return x
	}
	return out
}
func jsonField(m map[string]any, k string) string { b, _ := json.Marshal(m[k]); return string(b) }
func (a *app) teammate(p principal, id string) bool {
	if id == "" {
		return false
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM users WHERE id=? AND team_id=(SELECT team_id FROM users WHERE id=?) AND disabled=0 AND deleted_at IS NULL`, id, p.id).Scan(&n)
	return n == 1
}
func (a *app) ownerID(kind, id string) string {
	var v string
	_ = a.db.QueryRow(`SELECT owner_id FROM contacts WHERE id=?`, id).Scan(&v)
	return v
}
func (a *app) record(tx *sql.Tx, p principal, action, kind, id string, diff any) error {
	b, _ := json.Marshal(diff)
	stamp := now()
	if _, err := tx.Exec(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, p.id, p.device, action, kind, id, string(b), stamp); err != nil {
		return err
	}
	if kind == "contacts" || kind == "todos" || kind == "enquiries" || kind == "leads" || kind == "ofi_attendees" || kind == "referrals" || (kind == "captures" && captureApproved(diff)) {
		if kind == "contacts" {
			// TODO(write-phase): imported records must not write back to Rex yet.
			var source string
			if err := tx.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),'') FROM contacts WHERE id=?`, id).Scan(&source); err == nil && source == "rex_import" {
				return nil
			}
		}
		_, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?,?,?,?)`, kind, id, action, string(b), stamp)
		if err != nil {
			return err
		}
		if (kind == "contacts" || kind == "todos") && action != "DELETE" {
			return enqueueRexDetails(tx, kind, id, diff, stamp)
		}
		return nil
	}
	return nil
}
func (a *app) note(w http.ResponseWriter, r *http.Request, p principal, id string) {
	if _, err := a.get("contacts", id, p, false); err != nil {
		problem(w, 404, "not found")
		return
	}
	m, err := body(r)
	if err != nil {
		problem(w, 400, "invalid JSON")
		return
	}
	nid := random()[:26]
	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	m["id"] = nid
	m["contactId"] = id
	if _, err = tx.Exec(`INSERT INTO contact_notes(id,contact_id,user_id,d,a,t,created_at) VALUES(?,?,?,?,?,?,?)`, nid, id, p.id, str(m, "d"), str(m, "a"), str(m, "t"), now()); err == nil {
		var raw string
		err = tx.QueryRow(`SELECT data FROM contacts WHERE id=?`, id).Scan(&raw)
		if err == nil {
			var contact map[string]any
			err = json.Unmarshal([]byte(raw), &contact)
			if err == nil {
				notes, _ := contact["notes"].([]any)
				contact["notes"] = append(notes, map[string]any{"d": str(m, "d"), "a": str(m, "a"), "t": str(m, "t")})
				var b []byte
				b, err = json.Marshal(contact)
				if err == nil {
					_, err = tx.Exec(`UPDATE contacts SET data=?,version=version+1,updated_at=?,roles=?,tags=?,flags=?,consent=? WHERE id=?`, append([]any{string(b), now()}, append(contactColumns(contact), id)...)...)
				}
			}
		}
		if err == nil {
			err = a.record(tx, p, "POST", "contacts", id, m)
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 201, m)
}

func stripOFI(m map[string]any) {
	if events, ok := m["ofi"].([]any); ok {
		for _, v := range events {
			if event, ok := v.(map[string]any); ok {
				event["attendees"] = []any{}
			}
		}
	}
}
func captureApproved(v any) bool {
	m, _ := v.(map[string]any)
	return str(m, "status") == "approved" || str(m, "sync") == "approved"
}
