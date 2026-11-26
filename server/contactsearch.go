package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Contact search (docs/rex-integration.md "联系人搜索").
// The phone searches its own contacts first; only when that finds nothing it calls
// GET /api/v1/contacts/search, which queries the database and Rex in parallel and
// returns one merged list within ~3 seconds.

const (
	searchLimit       = 20
	searchRexBudget   = 2500 * time.Millisecond // login + queue + call
	searchRexPerMin   = 20                      // per user, Rex calls only (cache hits are free)
	searchCacheTTL    = 60 * time.Second
	searchOfferTTL    = 10 * time.Minute // a Rex id may be imported only after this user saw it
	searchImportLimit = 10 * time.Second
)

type searchView int

const (
	viewLimited searchView = iota // exists + owner name only
	viewFull
)

// contactSearchView is the single rule for how much of a local contact a search
// result shows. 待 Ping 确认: today the caller's own contacts and admins get full
// details; other agents' and unassigned contacts show existence + owner only.
func contactSearchView(p principal, ownerID string) searchView {
	if p.role == "admin" || (ownerID != "" && ownerID == p.id) {
		return viewFull
	}
	return viewLimited
}

// rexOnlyView is what a Rex contact that is not in the app shows (待 Ping 确认):
// name, masked mobile and the Rex owner, never email.
func rexOnlyView(h rexHit) map[string]any {
	return map[string]any{"source": "rex", "rexId": h.ID, "inRex": true, "full": false, "name": h.Name, "mobileMasked": maskMobile(h.Phone), "ownerName": h.Owner}
}

func maskMobile(s string) string {
	d := normAUMobile(s)
	if len(d) < 8 {
		if s == "" {
			return ""
		}
		return "••••"
	}
	return d[:4] + " " + d[4:7] + " •••"
}

// maskName keeps the initials only ("Teddie Bear" → "T••• B•••").
func maskName(first, last string) string {
	parts := []string{}
	for _, s := range []string{first, last} {
		s = strings.TrimSpace(s)
		if r, _ := utf8.DecodeRuneInString(s); s != "" {
			parts = append(parts, string(r)+"•••")
		}
	}
	return strings.Join(parts, " ")
}

var phoneQuery = regexp.MustCompile(`^[0-9+()\-. ]+$`)

type searchQuery struct {
	raw, norm, key string
	phone          bool
	rexAllowed     bool
}

// parseSearchQuery: ≥2 characters; a digits-only query with ≥3 digits matches by
// normalised mobile. Rex is asked for names and numbers with ≥8 digits
// (findMatchingContacts needs a whole number).
func parseSearchQuery(q string) (searchQuery, bool) {
	q = strings.TrimSpace(q)
	s := searchQuery{raw: q}
	if utf8.RuneCountInString(q) < 2 || len(q) > 100 {
		return s, false
	}
	digits := 0
	for _, r := range q {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if phoneQuery.MatchString(q) {
		if digits < 3 {
			return s, false
		}
		s.phone, s.norm = true, normAUMobile(q)
		s.key, s.rexAllowed = "phone:"+s.norm, len(s.norm) >= 8
		return s, true
	}
	s.norm = strings.ToLower(strings.Join(strings.Fields(q), " "))
	s.key, s.rexAllowed = "name:"+s.norm, true
	return s, true
}

type rexCacheEntry struct {
	at   time.Time
	hits []rexHit
}

// contactSearch owns the read-only Rex client used by search and import.
type contactSearch struct {
	cfg     rexConfig
	client  *rexClient
	sem     chan struct{} // the client carries per-call state: one call at a time
	mu      sync.Mutex
	cache   map[string]rexCacheEntry
	offered map[string]map[string]time.Time
	budget  time.Duration
}

func newContactSearch(db *sql.DB) *contactSearch {
	s := &contactSearch{cfg: loadRexConfig(), sem: make(chan struct{}, 1), cache: map[string]rexCacheEntry{}, offered: map[string]map[string]time.Time{}, budget: searchRexBudget}
	if s.cfg.canRead() {
		c := newRexClient(s.cfg.BaseURL, s.cfg.AppID)
		c.ReadOnly, c.AccountID, c.Caller, c.Password, c.Log = true, s.cfg.AccountID, "search", s.cfg.Password, rexDBLogger(db)
		c.Gate = rexAccountGate
		c.Retries = 0 // interactive: a 429 degrades to database results
		c.HTTP.Timeout = 10 * time.Second
		s.client = c
	}
	return s
}

func (a *app) contactSearcher() *contactSearch {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.search == nil {
		a.search = newContactSearch(a.db)
	}
	return a.search
}

func (s *contactSearch) cached(key string) ([]rexHit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[key]
	if !ok || time.Since(e.at) > searchCacheTTL {
		return nil, false
	}
	return e.hits, true
}
func (s *contactSearch) store(key string, hits []rexHit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) > 500 {
		for k, e := range s.cache {
			if time.Since(e.at) > searchCacheTTL {
				delete(s.cache, k)
			}
		}
	}
	s.cache[key] = rexCacheEntry{at: time.Now(), hits: hits}
}
func (s *contactSearch) offer(user string, hits []rexHit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.offered[user]
	if m == nil {
		m = map[string]time.Time{}
		s.offered[user] = m
	}
	for k, t := range m {
		if time.Since(t) > searchOfferTTL {
			delete(m, k)
		}
	}
	for _, h := range hits {
		m[h.ID] = time.Now()
	}
}
func (s *contactSearch) wasOffered(user, rexID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.offered[user][rexID]
	return ok && time.Since(t) <= searchOfferTTL
}

// withClient serialises use of the Rex client, logs in on demand and refuses any
// account other than REX_ACCOUNT_ID. ctx bounds the queue, login and call.
func (s *contactSearch) withClient(ctx context.Context, fn func(*rexClient) error) error {
	if s.client == nil {
		return errRexDisabled
	}
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.sem }()
	c := s.client
	login := func() error {
		if _, err := c.Login(ctx, s.cfg.Email, s.cfg.Password, s.cfg.AccountID); err != nil {
			return err
		}
		if c.VerifiedAccount == "" || c.VerifiedAccount != s.cfg.AccountID {
			c.Token = ""
			return errors.New("Rex active account does not match REX_ACCOUNT_ID")
		}
		return nil
	}
	if c.Token == "" {
		if err := login(); err != nil {
			return err
		}
	}
	err := fn(c)
	if err != nil && strings.Contains(err.Error(), "TokenException") && ctx.Err() == nil {
		c.Token = ""
		if err = login(); err != nil {
			return err
		}
		err = fn(c)
	}
	return err
}

var errRexDisabled = errors.New("Rex reads disabled")

func (s *contactSearch) rexLookup(ctx context.Context, q searchQuery) ([]rexHit, error) {
	var hits []rexHit
	err := s.withClient(ctx, func(c *rexClient) error {
		c.Entity, c.LocalID, c.RexID = "contacts", "", ""
		var e error
		if q.phone {
			_, _, hits, e = rexFindByPhone(ctx, c, q.raw, "any")
		} else {
			hits, e = rexSearchByName(ctx, c, q.raw)
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []rexHit{}
	}
	s.store(q.key, hits)
	return hits, nil
}

type searchRow struct {
	id, owner, ownerName, first, last, mobile, email, rexID string
}

const searchSelect = `SELECT c.id,COALESCE(c.owner_id,''),COALESCE(u.name,''),
 COALESCE(NULLIF(json_extract(c.data,'$.first'),''),c.first,''),COALESCE(NULLIF(json_extract(c.data,'$.last'),''),c.last,''),
 COALESCE(NULLIF(json_extract(c.data,'$.mobile'),''),c.mobile,''),COALESCE(NULLIF(json_extract(c.data,'$.email'),''),c.email,''),
 COALESCE((SELECT l.rex_id FROM rex_links l WHERE l.entity='contacts' AND l.local_id=c.id ORDER BY l.account_id=? DESC LIMIT 1),'')
 FROM contacts c LEFT JOIN users u ON u.id=c.owner_id WHERE c.deleted_at IS NULL`

func scanSearchRows(rows *sql.Rows) ([]searchRow, error) {
	defer rows.Close()
	out := []searchRow{}
	for rows.Next() {
		var r searchRow
		if err := rows.Scan(&r.id, &r.owner, &r.ownerName, &r.first, &r.last, &r.mobile, &r.email, &r.rexID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (a *app) searchDB(q searchQuery, p principal, account string) ([]searchRow, error) {
	args := []any{account}
	where := ""
	if q.phone {
		digits := q.norm
		tail := digits
		if strings.HasPrefix(tail, "0") && len(tail) > 3 {
			tail = tail[1:] // "+61 401…" is stored without the leading 0
		}
		where = ` AND (instr(COALESCE(json_extract(c.data,'$.mobileNorm'),''),?)>0 OR instr(` + sqlDigits(`COALESCE(NULLIF(json_extract(c.data,'$.mobile'),''),c.mobile,'')`) + `,?)>0)`
		args = append(args, digits, tail)
	} else {
		like := "%" + likeEscape(q.norm) + "%"
		where = ` AND (lower(COALESCE(json_extract(c.data,'$.first'),c.first,'')||' '||COALESCE(json_extract(c.data,'$.last'),c.last,'')) LIKE ? ESCAPE '\' OR lower(COALESCE(json_extract(c.data,'$.email'),c.email,'')) LIKE ? ESCAPE '\')`
		args = append(args, like, like)
	}
	args = append(args, p.id, searchLimit)
	rows, err := a.db.Query(searchSelect+where+` ORDER BY c.owner_id=? DESC,c.updated_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	return scanSearchRows(rows)
}

// localForRex maps Rex ids to linked, undeleted local contacts.
func (a *app) localForRex(ids []string, account string) (map[string]searchRow, error) {
	out := map[string]searchRow{}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{account, account}
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	rows, err := a.db.Query(searchSelect+` AND c.id IN (SELECT local_id FROM rex_links WHERE entity='contacts' AND account_id=? AND rex_id IN (`+strings.Join(marks, ",")+`))`, args...)
	if err != nil {
		return nil, err
	}
	list, err := scanSearchRows(rows)
	if err != nil {
		return nil, err
	}
	for _, r := range list {
		out[r.rexID] = r
	}
	return out, nil
}

func searchItem(p principal, r searchRow) map[string]any {
	item := map[string]any{"source": "local", "id": r.id, "ownerName": r.ownerName, "inRex": r.rexID != ""}
	if contactSearchView(p, r.owner) == viewFull {
		item["full"] = true
		item["ownerId"] = r.owner
		item["first"], item["last"], item["mobile"], item["email"] = r.first, r.last, r.mobile, r.email
		item["name"] = strings.TrimSpace(r.first + " " + r.last)
		item["rexId"] = r.rexID
		item["mine"] = r.owner == p.id
	} else {
		item["full"] = false
		item["name"] = maskName(r.first, r.last)
	}
	return item
}

// GET /api/v1/contacts/search?q=
func (a *app) contactSearchHandler(w http.ResponseWriter, r *http.Request, p principal) {
	q, ok := parseSearchQuery(r.URL.Query().Get("q"))
	if !ok {
		respond(w, 200, map[string]any{"items": []any{}, "rexSkipped": true, "rexReason": "query too short"})
		return
	}
	s := a.contactSearcher()
	type rexResult struct {
		hits []rexHit
		err  error
	}
	var rexCh chan rexResult
	resp := map[string]any{"rexTimedOut": false, "rexSkipped": false, "rexError": false}
	var hits []rexHit
	switch {
	case s.client == nil:
		resp["rexSkipped"], resp["rexReason"] = true, "disabled"
	case !q.rexAllowed:
		resp["rexSkipped"], resp["rexReason"] = true, "partial number"
	default:
		if cached, ok := s.cached(q.key); ok {
			hits = cached
		} else if !a.allowEndpointCall(p, "rex_search", searchRexPerMin) {
			resp["rexSkipped"], resp["rexReason"] = true, "rate limited"
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), s.budget)
			defer cancel()
			rexCh = make(chan rexResult, 1)
			go func() { h, e := s.rexLookup(ctx, q); rexCh <- rexResult{h, e} }()
			// The database query runs meanwhile; its rows are closed before waiting on Rex.
			local, err := a.searchDB(q, p, s.cfg.AccountID)
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			select {
			case res := <-rexCh:
				if res.err != nil {
					if errors.Is(res.err, context.DeadlineExceeded) || ctx.Err() != nil {
						resp["rexTimedOut"] = true
					} else {
						resp["rexError"] = true
					}
				}
				hits = res.hits
			case <-ctx.Done():
				resp["rexTimedOut"] = true
			}
			a.finishSearch(w, p, s, local, hits, resp)
			return
		}
	}
	local, err := a.searchDB(q, p, s.cfg.AccountID)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	a.finishSearch(w, p, s, local, hits, resp)
}

// finishSearch merges database and Rex results: Rex hits linked to a local
// contact become that contact (scope rules apply), others are "In Rex".
func (a *app) finishSearch(w http.ResponseWriter, p principal, s *contactSearch, local []searchRow, hits []rexHit, resp map[string]any) {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	linked, err := a.localForRex(ids, s.cfg.AccountID)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	items := []map[string]any{}
	seen := map[string]bool{}
	for _, r := range local {
		if !seen[r.id] {
			seen[r.id] = true
			items = append(items, searchItem(p, r))
		}
	}
	offered := []rexHit{}
	for _, h := range hits {
		if r, ok := linked[h.ID]; ok {
			if !seen[r.id] {
				seen[r.id] = true
				items = append(items, searchItem(p, r))
			}
			continue
		}
		if !seen["rex:"+h.ID] {
			seen["rex:"+h.ID] = true
			items = append(items, rexOnlyView(h))
			offered = append(offered, h)
		}
	}
	if len(items) > searchLimit {
		items = items[:searchLimit]
	}
	s.offer(p.id, offered)
	resp["items"] = items
	respond(w, 200, resp)
}

// POST /api/v1/contacts/import-rex {rexId}: imports a Rex contact the caller was
// just shown, owned by the caller (the caller wins over REX_USER_MAP).
func (a *app) contactImportRex(w http.ResponseWriter, r *http.Request, p principal) {
	m, err := body(r)
	if err != nil {
		problem(w, 400, "invalid JSON")
		return
	}
	rexID := strings.TrimSpace(istring(m["rexId"]))
	if rexID == "" || strings.Trim(rexID, "0123456789") != "" || len(rexID) > 20 {
		problem(w, 400, "rexId required")
		return
	}
	s := a.contactSearcher()
	if s.client == nil {
		problem(w, 503, "Rex unavailable")
		return
	}
	account := s.cfg.AccountID
	if found, err := a.localForRex([]string{rexID}, account); err != nil {
		problem(w, 500, "database error")
		return
	} else if row, ok := found[rexID]; ok {
		respond(w, 200, map[string]any{"imported": false, "item": searchItem(p, row)})
		return
	}
	if !s.wasOffered(p.id, rexID) {
		problem(w, 404, "not found")
		return
	}
	if !a.allowEndpointCall(p, "rex_search", searchRexPerMin) {
		problem(w, 429, "rate limited")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), searchImportLimit)
	defer cancel()
	var detail importRow
	err = s.withClient(ctx, func(c *rexClient) error {
		c.Entity, c.LocalID, c.RexID = "contacts", "", rexID
		_, env, e := c.Call(ctx, "Contacts", "read", map[string]any{"id": rexID})
		if e != nil {
			return e
		}
		detail = importRow{}
		return json.Unmarshal(env.Result, &detail)
	})
	if err != nil || iv(detail, "id") != rexID {
		problem(w, 502, "Rex read failed")
		return
	}
	flattenRexRead(detail)
	local, created, err := rexAdoptOrImport(a.db, account, detail, p.id)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	if created {
		_, _ = a.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "rex_to_app", "contacts", local, rexID, ijson(map[string]any{"imported_by": p.id}), "ok")
	}
	found, err := a.localForRex([]string{rexID}, account)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	row, ok := found[rexID]
	if !ok {
		problem(w, 500, "import failed")
		return
	}
	respond(w, 201, map[string]any{"imported": created, "item": searchItem(p, row)})
}
