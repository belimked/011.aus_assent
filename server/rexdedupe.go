package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Same-mobile duplicate handling between the app and Rex (问题02), and the
// Rex lookups shared with contact search. See docs/rex-integration.md
// "同号码查重".

type rexHit struct {
	ID, First, Last, Name, Phone, Email, Owner string
}

// rexFindByPhone calls Contacts::findMatchingContacts. Rex expands the number
// server-side (+61 / 0 / spaced forms match), so the caller's form is sent as is.
func rexFindByPhone(ctx context.Context, c *rexClient, phone, contactType string) (verdict string, truncated bool, hits []rexHit, err error) {
	body := map[string]any{"contact_type": contactType, "phones": []string{phone}, "limit": 20}
	_, env, err := c.Call(ctx, "Contacts", "findMatchingContacts", body)
	if err != nil {
		return "", false, nil, err
	}
	var out struct {
		Verdict   string           `json:"verdict"`
		Truncated bool             `json:"truncated"`
		Contacts  []map[string]any `json:"contacts"`
	}
	if err = json.Unmarshal(env.Result, &out); err != nil {
		return "", false, nil, fmt.Errorf("findMatchingContacts result: %w", err)
	}
	for _, row := range out.Contacts {
		if h := rexHitFrom(row); h.ID != "" {
			hits = append(hits, h)
		}
	}
	return out.Verdict, out.Truncated, hits, nil
}

// rexNameSearchBody is the Contacts::search body for a name query.
// TODO(sandbox): criteria type "like" on contact.name_full is not in the Rex
// criteria docs; .tmp/search/sandbox-check.sh verifies it (待沙箱确认).
func rexNameSearchBody(q string) map[string]any {
	return map[string]any{"criteria": []any{map[string]any{"name": "contact.name_full", "type": "like", "value": "%" + q + "%"}}, "limit": 20}
}

func rexSearchByName(ctx context.Context, c *rexClient, q string) ([]rexHit, error) {
	_, env, err := c.Call(ctx, "Contacts", "search", rexNameSearchBody(q))
	if err != nil {
		return nil, err
	}
	var out struct {
		Rows []map[string]any `json:"rows"`
	}
	if err = json.Unmarshal(env.Result, &out); err != nil {
		return nil, fmt.Errorf("Contacts search result: %w", err)
	}
	hits := []rexHit{}
	for _, row := range out.Rows {
		if h := rexHitFrom(row); h.ID != "" {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

func rexHitFrom(row importRow) rexHit {
	flattenRexRead(row)
	h := rexHit{ID: iv(row, "id"), First: iv(row, "name_first"), Last: iv(row, "name_last"), Phone: iv(row, "phone_number"), Email: iv(row, "email_address")}
	if h.ID == "" {
		h.ID = iv(row, "_id")
	}
	h.Name = strings.TrimSpace(h.First + " " + h.Last)
	for _, k := range []string{"name", "name_full", "system_search_key"} {
		if h.Name == "" {
			h.Name = iv(row, k)
		}
	}
	h.Owner = iv(imap(row["system_owner_user"]), "name")
	return h
}

// markPossibleDuplicate records, on every contact in ids, the other ids as
// data.possibleDuplicateOf. The phone shows these through duplicatesOf.
func markPossibleDuplicate(db *sql.DB, ids ...string) error {
	if len(ids) < 2 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		var raw string
		if err = tx.QueryRow(`SELECT data FROM contacts WHERE id=?`, id).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		var data map[string]any
		if json.Unmarshal([]byte(raw), &data) != nil || data == nil {
			data = map[string]any{}
		}
		list := stringsFrom(data["possibleDuplicateOf"])
		seen := map[string]bool{}
		for _, x := range list {
			seen[x] = true
		}
		changed := false
		for _, other := range ids {
			if other != id && !seen[other] {
				list = append(list, other)
				seen[other] = true
				changed = true
			}
		}
		if !changed {
			continue
		}
		data["possibleDuplicateOf"] = list
		if _, err = tx.Exec(`UPDATE contacts SET data=?,updated_at=?,version=version+1 WHERE id=?`, ijson(data), now(), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// mergedAway reports whether id is the secondary of a completed local merge
// (contact_merge.go writes a "merged-into" audit row and soft-deletes it).
func mergedAway(db *sql.DB, id string) bool {
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM contacts c WHERE c.id=? AND c.deleted_at IS NOT NULL AND EXISTS(SELECT 1 FROM audit_log l WHERE l.action='merged-into' AND l.entity='contacts' AND l.entity_id=c.id)`, id).Scan(&n)
	return n > 0
}

// ---- App → Rex (worker, before Contacts::create) ----

type finishFunc func(status, reason, link, service string) error

// duplicateOfLinked: a local contact with the same mobile is already linked to rex.
// Never point two local contacts at one Rex record: flag both for a merge instead.
func (w *rexWorker) duplicateOfLinked(j rexJob, rex string, finish finishFunc) error {
	var other string
	_ = w.db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='contacts' AND account_id=? AND rex_id=?`, w.account, rex).Scan(&other)
	if other == j.localID {
		return finish("sent", "", "", "")
	}
	if other != "" {
		if err := markPossibleDuplicate(w.db, j.localID, other); err != nil {
			return err
		}
	}
	return finish("skipped", "possible duplicate of "+other+" (already in Rex "+rex+"); merge in app", "", "")
}

// rexPreCreate looks the mobile up in Rex before a create. done=false means
// nothing matched and the create may proceed.
func (w *rexWorker) rexPreCreate(ctx context.Context, j rexJob, c map[string]any, finish finishFunc) (done bool, err error) {
	mobile := str(c, "mobile")
	if len(normAUMobile(mobile)) < 10 {
		return false, nil
	}
	if w.client.Token == "" {
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return true, w.retry(j, "Rex login failed", 0)
		}
	}
	w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", j.localID, ""
	lookup := func() (string, bool, []rexHit, error) {
		mctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		return rexFindByPhone(mctx, w.client, mobile, "person")
	}
	verdict, truncated, hits, e := lookup()
	if e != nil && strings.Contains(e.Error(), "TokenException") {
		w.client.Token = ""
		if _, loginErr := w.client.Login(ctx, w.email, w.password, w.account); loginErr != nil {
			return true, w.retry(j, "Rex login failed", 0)
		}
		verdict, truncated, hits, e = lookup()
	}
	if e != nil {
		var rate rexRateError
		if errors.As(e, &rate) {
			return true, w.retry(j, "Rex rate limited", rate.delay)
		}
		// Unknown whether Rex already has this person: retry later, never create blindly.
		return true, w.retry(j, "Rex duplicate check failed; will retry", 0)
	}
	switch {
	case verdict == "none" && len(hits) == 0:
		return false, nil
	case verdict != "unique" || truncated || len(hits) != 1:
		return true, finish("failed", "several Rex contacts share this mobile; resolve in Rex before retry", "", "")
	}
	rid := hits[0].ID
	var other string
	e = w.db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='contacts' AND account_id=? AND rex_id=?`, w.account, rid).Scan(&other)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return true, e
	}
	if other != "" {
		return true, w.duplicateOfLinked(j, rid, finish)
	}
	return true, w.adoptRexContact(j, rid)
}

// adoptRexContact links the app contact to an existing Rex contact instead of
// creating one, then queues an update so the app's (newer) non-empty fields reach Rex.
func (w *rexWorker) adoptRexContact(j rexJob, rid string) error {
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	if _, err = tx.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('contacts',?,?,'Contacts',?,?)`, j.localID, rid, stamp, w.account); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE rex_outbox SET status='sent',last_error=?,next_attempt_at=NULL WHERE id=?`, "linked to existing Rex contact "+rid, j.id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contacts SET data=json_set(data,'$.rexId',?),updated_at=?,version=version+1 WHERE id=?`, rid, stamp, j.localID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at) VALUES('contacts',?,'PATCH',?,'pending',?)`, j.localID, `{"rexAdopt":true}`, stamp); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, stamp, "app_to_rex", "contacts", j.localID, rid, `{"linked":"existing Rex contact (same mobile)"}`, "ok"); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- Rex → App (poller / import of a Rex contact without rex_links) ----

type dupCandidate struct {
	id, updated, createStatus string
	linked                    bool
	data                      map[string]any
}

func localByMobile(db *sql.DB, mobile, exclude string) ([]dupCandidate, error) {
	norm := normAUMobile(mobile)
	if len(norm) < 10 {
		return nil, nil
	}
	rows, err := db.Query(`SELECT c.id,c.updated_at,c.data,COALESCE(c.mobile,''),
 EXISTS(SELECT 1 FROM rex_links l WHERE l.entity='contacts' AND l.local_id=c.id),
 COALESCE((SELECT o.status FROM rex_outbox o WHERE o.entity='contacts' AND o.entity_id=c.id AND o.op='POST' ORDER BY o.id DESC LIMIT 1),'')
 FROM contacts c WHERE c.deleted_at IS NULL AND c.id<>?
 AND (COALESCE(json_extract(c.data,'$.mobileNorm'),'')=? OR instr(`+sqlDigits(`COALESCE(NULLIF(c.mobile,''),json_extract(c.data,'$.mobile'),'')`)+`,?)>0)`, exclude, norm, norm[1:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dupCandidate{}
	for rows.Next() {
		var d dupCandidate
		var raw, col string
		var linked int
		if err = rows.Scan(&d.id, &d.updated, &raw, &col, &linked, &d.createStatus); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &d.data)
		phone := col
		if phone == "" {
			phone = str(d.data, "mobile")
		}
		if normAUMobile(phone) != norm && str(d.data, "mobileNorm") != norm {
			continue
		}
		d.linked = linked > 0
		out = append(out, d)
	}
	return out, rows.Err()
}

// rexModTime reads system_modtime (Unix seconds in Rex).
func rexModTime(r importRow) time.Time {
	s := iv(r, "system_modtime")
	if n, err := strconv.ParseFloat(s, 64); err == nil && n > 0 {
		return time.Unix(int64(n), 0)
	}
	return time.Time{}
}

// rexAdoptOrImport saves a Rex contact that has no rex_links row. A single
// unlinked local contact with the same mobile (whose create was never sent) is
// adopted instead of creating a second record; otherwise the contact is imported
// and same-mobile linked contacts are flagged as possible duplicates.
func rexAdoptOrImport(db *sql.DB, account string, detail importRow, owner string) (local string, created bool, err error) {
	rid := iv(detail, "id")
	newID := "rex-contacts-" + account + "-" + rid
	candidates, err := localByMobile(db, iv(detail, "phone_number"), newID)
	if err != nil {
		return "", false, err
	}
	var adoptable []dupCandidate
	var others []string
	for _, c := range candidates {
		if !c.linked && c.createStatus != "sent" {
			adoptable = append(adoptable, c)
		} else {
			others = append(others, c.id)
		}
	}
	if len(adoptable) == 1 {
		c := adoptable[0]
		if err = adoptLocalContact(db, account, detail, c); err != nil {
			return "", false, err
		}
		return c.id, false, markPossibleDuplicate(db, append([]string{c.id}, others...)...)
	}
	for _, c := range adoptable {
		others = append(others, c.id)
	}
	if err = importSave(db, "contacts", account, detail, owner, false, &importStats{}); err != nil {
		return "", false, err
	}
	return newID, true, markPossibleDuplicate(db, append([]string{newID}, others...)...)
}

func adoptLocalContact(db *sql.DB, account string, detail importRow, c dupCandidate) error {
	rid := iv(detail, "id")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	data := c.data
	if data == nil {
		data = map[string]any{}
	}
	localAt, _ := time.Parse(layout, c.updated)
	rexAt := rexModTime(detail)
	changes := map[string]any{"linked": "existing app contact (same mobile)"}
	rexWins := !rexAt.IsZero() && rexAt.After(localAt)
	if rexWins { // later writer wins; empty Rex values never overwrite
		for key, rexKey := range map[string]string{"first": "name_first", "last": "name_last", "mobile": "phone_number", "email": "email_address"} {
			if v := iv(detail, rexKey); v != "" && v != str(data, key) {
				changes[key] = map[string]any{"old": data[key], "new": v}
				data[key] = v
			}
		}
		data["mobileNorm"] = normAUMobile(str(data, "mobile"))
	}
	data["rexId"] = rid
	if _, err = tx.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id,etag) VALUES('contacts',?,?,'Contacts',?,?,?)`, c.id, rid, stamp, account, iv(detail, "etag")); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE rex_outbox SET status='sent',last_error=?,next_attempt_at=NULL WHERE entity='contacts' AND entity_id=? AND op='POST' AND status IN ('pending','waiting_auth')`, "linked to existing Rex contact "+rid, c.id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contacts SET first=?,last=?,mobile=?,email=?,data=?,updated_at=?,version=version+1 WHERE id=?`, str(data, "first"), str(data, "last"), str(data, "mobile"), str(data, "email"), ijson(data), stamp, c.id); err != nil {
		return err
	}
	if !rexWins { // app is newer: push its non-empty fields through the update path
		if _, err = tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at) VALUES('contacts',?,'PATCH',?,'pending',?)`, c.id, `{"rexAdopt":true}`, stamp); err != nil {
			return err
		}
		changes["winner"] = "app"
	} else {
		changes["winner"] = "rex"
	}
	if _, err = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, stamp, "rex_to_app", "contacts", c.id, rid, ijson(changes), "ok"); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- Read-only report: several local contacts pointing at one Rex record ----

type dupRexLink struct {
	RexID    string   `json:"rexId"`
	LocalIDs []string `json:"localIds"`
	Via      string   `json:"via"` // rex_links | data.rexId
}

func duplicateRexLinks(db querier) ([]dupRexLink, error) {
	out := []dupRexLink{}
	scan := func(via, q string) error {
		rows, err := db.Query(q)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rid, ids string
			if err = rows.Scan(&rid, &ids); err != nil {
				return err
			}
			out = append(out, dupRexLink{RexID: rid, LocalIDs: strings.Split(ids, ","), Via: via})
		}
		return rows.Err()
	}
	// Links across account ids (the unique index is per account) …
	if err := scan("rex_links", `SELECT rex_id,group_concat(DISTINCT local_id) FROM rex_links WHERE entity='contacts' GROUP BY rex_id HAVING count(DISTINCT local_id)>1 ORDER BY rex_id`); err != nil {
		return nil, err
	}
	// … and contacts whose data.rexId names a record linked to another contact
	// (the old findLocalDuplicate path finished "sent" without a link row).
	// Joined in Go: rex_links has no index usable on rex_id alone, and a SQL join
	// against json_extract scans contacts × links (minutes on production sizes).
	// Each query is fully read and closed before the next (single connection).
	linked := map[string][]string{}
	rows, err := db.Query(`SELECT rex_id,local_id FROM rex_links WHERE entity='contacts'`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid, local string
		if err = rows.Scan(&rid, &local); err != nil {
			rows.Close()
			return nil, err
		}
		linked[rid] = append(linked[rid], local)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	type ref struct{ id, rex string }
	refs := []ref{}
	rows, err = db.Query(`SELECT id,CAST(json_extract(data,'$.rexId') AS TEXT) FROM contacts WHERE deleted_at IS NULL AND json_extract(data,'$.rexId') IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r ref
		if err = rows.Scan(&r.id, &r.rex); err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, r)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	byRex := map[string][]string{}
	for _, r := range refs {
		if locals := linked[r.rex]; slices.ContainsFunc(locals, func(l string) bool { return l != r.id }) && !slices.Contains(byRex[r.rex], r.id) {
			byRex[r.rex] = append(byRex[r.rex], r.id)
		}
	}
	keys := make([]string, 0, len(byRex))
	for k := range byRex {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, dupRexLink{RexID: k, LocalIDs: byRex[k], Via: "data.rexId"})
	}
	return out, nil
}

// sqlDigits strips the separators people type in phone numbers (Rex stores "0401 999 088").
func sqlDigits(expr string) string {
	for _, ch := range []string{" ", "-", "+", "(", ")", "."} {
		expr = "replace(" + expr + ",'" + ch + "','')"
	}
	return expr
}
