package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Contact mailing lists (UAT 3 item 9). docs/rex-integration.md "联系人 Mailing lists".
//
// contact_mailing_lists is the server's copy of Rex related.contact_mailing_lists.
// The phone sends contacts.data.mailingLists (the names it wants); only the
// names it changed since its previous push are queued (stale phone copies
// never undo a Rex-side change). Add and remove are separate outbox jobs:
// rex_contact_mailing_add / rex_contact_mailing_remove, entity_id
// "<contactId>:<lower(name)>", payload {contactId,name}.
//
// Conflict rule: while a job for (contact,name) is pending/waiting_auth, Rex
// poll/import results never change that name. Once the job is done, Rex wins:
// the worker saves the Rex state it read (plus its own change), and the next
// poll read replaces everything else.

const (
	mailingAdd    = "rex_contact_mailing_add"
	mailingRemove = "rex_contact_mailing_remove"
)

func mailingKey(name string) string  { return strings.ToLower(mailingName(name)) }
func mailingName(name string) string { return strings.Join(strings.Fields(name), " ") }

// mailingActive: a row the contact is subscribed to. ” is an app add not yet
// seen in Rex, or Rex returning no status; other statuses ("unsubscribed", ...)
// are history rows (model_Contacts: "subscribed to (or has previously been subscribed to)").
func mailingActive(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "" || s == "active" || s == "subscribed"
}

// mailingNames reads a phone list: strings or {name}/{mailing_list_name} objects, deduplicated.
func mailingNames(v any) []string {
	list, _ := v.([]any)
	out, seen := []string{}, map[string]bool{}
	for _, x := range list {
		name := istring(x)
		if m := imap(x); m != nil {
			name = iv(m, "name")
			if name == "" {
				name = iv(m, "mailing_list_name")
			}
		}
		name = mailingName(name)
		if name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			out = append(out, name)
		}
	}
	return out
}

type mailingRow struct{ name, status string }

func loadMailingRows(q querier, contactID string) ([]mailingRow, error) {
	rows, err := q.Query(`SELECT name,status FROM contact_mailing_lists WHERE contact_id=? ORDER BY name COLLATE NOCASE`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []mailingRow{}
	for rows.Next() {
		var r mailingRow
		if err = rows.Scan(&r.name, &r.status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// openMailingJobs: lower(name) of every not yet finished job for the contact.
func openMailingJobs(q querier, contactID string) (map[string]bool, error) {
	rows, err := q.Query(`SELECT entity_id FROM rex_outbox WHERE entity IN (?,?) AND entity_id>? AND entity_id<? AND status IN ('pending','waiting_auth')`, mailingAdd, mailingRemove, contactID+":", contactID+";")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		out[strings.TrimPrefix(key, contactID+":")] = true
	}
	return out, rows.Err()
}

func enqueueMailing(tx *sql.Tx, entity, contactID, name, stamp string) error {
	_, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?,'POST',?,?)`, entity, contactID+":"+mailingKey(name), ijson(map[string]any{"contactId": contactID, "name": name}), stamp)
	return err
}

// enqueueMailingLists runs inside the contact write (enqueueRexDetails). It is a
// no-op unless the phone sent mailingLists. Only the phone's own changes are
// queued: adds against mailingListsSeen (the list it last got from the server),
// removes against mailingListsSeen plus its previous push (two pushes before a
// pull). Names already in that state are idempotent no-ops.
func enqueueMailingLists(tx *sql.Tx, id string, current map[string]any, stamp string) error {
	// mailingListSync/rexLinked are computed on pull (attachMailingLists); never store a phone's copy.
	if _, a := current["mailingListSync"]; a || current["rexLinked"] != nil {
		if _, err := tx.Exec(`UPDATE contacts SET data=json_remove(data,'$.mailingListSync','$.rexLinked') WHERE id=?`, id); err != nil {
			return err
		}
	}
	raw, ok := current["mailingLists"]
	if !ok {
		return nil
	}
	want := mailingNames(raw)
	rows, err := loadMailingRows(tx, id)
	if err != nil {
		return err
	}
	have := map[string]mailingRow{}
	for _, r := range rows {
		have[strings.ToLower(r.name)] = r
	}
	addBase, removeBase := map[string]bool{}, map[string]bool{}
	if seen, ok := current["mailingListsSeen"]; ok {
		for _, n := range mailingNames(seen) {
			addBase[strings.ToLower(n)], removeBase[strings.ToLower(n)] = true, true
		}
		// The current write's own outbox row is the newest; OFFSET 1 is the previous push.
		var prior string
		_ = tx.QueryRow(`SELECT payload FROM rex_outbox WHERE entity='contacts' AND entity_id=? AND op IN ('POST','PATCH') AND json_type(payload,'$.mailingLists')='array' ORDER BY id DESC LIMIT 1 OFFSET 1`, id).Scan(&prior)
		var p map[string]any
		_ = json.Unmarshal([]byte(prior), &p)
		for _, n := range mailingNames(p["mailingLists"]) {
			removeBase[strings.ToLower(n)] = true
		}
	} else { // no baseline from the phone: compare against the server state
		for k, r := range have {
			addBase[k], removeBase[k] = mailingActive(r.status), mailingActive(r.status)
		}
	}
	wanted := map[string]bool{}
	for _, name := range want {
		k := strings.ToLower(name)
		wanted[k] = true
		if addBase[k] {
			continue // not changed by the phone
		}
		if r, ok := have[k]; ok && mailingActive(r.status) {
			continue // idempotent: already a member, no second job
		}
		if _, err = tx.Exec(`INSERT INTO contact_mailing_lists(contact_id,name,status,updated_at) VALUES(?,?,'',?) ON CONFLICT(contact_id,name) DO UPDATE SET status='',updated_at=excluded.updated_at`, id, name, stamp); err != nil {
			return err
		}
		if err = enqueueMailing(tx, mailingAdd, id, name, stamp); err != nil {
			return err
		}
	}
	for k, r := range have {
		if wanted[k] || !removeBase[k] || !mailingActive(r.status) {
			continue
		}
		if _, err = tx.Exec(`DELETE FROM contact_mailing_lists WHERE contact_id=? AND name=?`, id, r.name); err != nil {
			return err
		}
		if err = enqueueMailing(tx, mailingRemove, id, r.name, stamp); err != nil {
			return err
		}
	}
	// The push response (read after commit) carries the new server state and baseline.
	if rows, err = loadMailingRows(tx, id); err != nil {
		return err
	}
	names := []string{}
	for _, r := range rows {
		if mailingActive(r.status) {
			names = append(names, r.name)
		}
	}
	_, err = tx.Exec(`UPDATE contacts SET data=json_set(data,'$.mailingLists',json(?),'$.mailingListsSeen',json(?)) WHERE id=?`, ijson(names), ijson(names), id)
	return err
}

// saveMailingMemberships replaces the contact's rows with Rex's
// related.contact_mailing_lists, except names with an open app job. ok=false
// (key absent, e.g. a search row without the extra field) leaves rows alone.
func saveMailingMemberships(tx *sql.Tx, contactID string, rex []any, stamp string) (bool, error) {
	open, err := openMailingJobs(tx, contactID)
	if err != nil {
		return false, err
	}
	rows, err := loadMailingRows(tx, contactID)
	if err != nil {
		return false, err
	}
	type rexRow struct{ name, status, id, sub, unsub, reason string }
	latest := map[string]rexRow{}
	for _, x := range rex {
		m := imap(x)
		name := mailingName(iv(m, "mailing_list_name"))
		if name == "" {
			name = mailingName(iv(imap(m["mailing_list"]), "name"))
		}
		if name == "" {
			continue
		}
		r := rexRow{name, iv(m, "subscription_status"), iv(m, "id"), iv(m, "subscribe_timestamp"), iv(m, "unsubscribe_timestamp"), iv(m, "unsubscribe_reason")}
		k := strings.ToLower(name)
		if old, ok := latest[k]; ok && mailingActive(old.status) && !mailingActive(r.status) {
			continue // an active row beats a history row for the same list
		}
		latest[k] = r
	}
	changed := false
	for _, r := range rows {
		k := strings.ToLower(r.name)
		if _, ok := latest[k]; ok || open[k] {
			continue
		}
		if _, err = tx.Exec(`DELETE FROM contact_mailing_lists WHERE contact_id=? AND name=?`, contactID, r.name); err != nil {
			return false, err
		}
		changed = true
	}
	before := map[string]string{}
	for _, r := range rows {
		before[strings.ToLower(r.name)] = r.status
	}
	for k, r := range latest {
		if open[k] {
			continue
		}
		if s, ok := before[k]; !ok || s != r.status {
			changed = true
		}
		if _, err = tx.Exec(`INSERT INTO contact_mailing_lists(contact_id,name,status,rex_row_id,subscribed_at,unsubscribed_at,unsubscribe_reason,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(contact_id,name) DO UPDATE SET name=excluded.name,status=excluded.status,rex_row_id=excluded.rex_row_id,subscribed_at=excluded.subscribed_at,unsubscribed_at=excluded.unsubscribed_at,unsubscribe_reason=excluded.unsubscribe_reason,updated_at=excluded.updated_at`, contactID, r.name, r.status, r.id, r.sub, r.unsub, r.reason, stamp); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// rexMailingRows returns related.contact_mailing_lists and whether Rex sent it.
func rexMailingRows(r importRow) ([]any, bool) {
	v, ok := imap(r["related"])["contact_mailing_lists"]
	if !ok {
		return nil, false
	}
	list, _ := v.([]any)
	return list, true
}

// importMailingLists is called by importSave inside its transaction.
func importMailingLists(tx *sql.Tx, contactID string, r importRow, stamp string) error {
	list, ok := rexMailingRows(r)
	if !ok {
		return nil
	}
	_, err := saveMailingMemberships(tx, contactID, list, stamp)
	return err
}

// adoptMailingLists: an unlinked app contact adopted by a Rex contact (问题02)
// keeps its app-only lists; they are queued for Rex before Rex's rows are saved.
func adoptMailingLists(tx *sql.Tx, contactID string, detail importRow, stamp string) error {
	list, ok := rexMailingRows(detail)
	inRex := map[string]bool{}
	for _, x := range list {
		if m := imap(x); mailingActive(iv(m, "subscription_status")) {
			inRex[mailingKey(iv(m, "mailing_list_name"))] = true
		}
	}
	rows, err := loadMailingRows(tx, contactID)
	if err != nil {
		return err
	}
	open, err := openMailingJobs(tx, contactID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if k := strings.ToLower(r.name); mailingActive(r.status) && !inRex[k] && !open[k] {
			if err = enqueueMailing(tx, mailingAdd, contactID, r.name, stamp); err != nil {
				return err
			}
		}
	}
	if !ok {
		return nil
	}
	_, err = saveMailingMemberships(tx, contactID, list, stamp)
	return err
}

// mergeMailingLists moves the secondary's lists to the primary (one row per
// name, the primary's row wins), cancels the secondary's open jobs and queues
// adds on the primary so the surviving Rex record has every list. combineRecords
// is not known to merge mailing lists (unverified); the adds are idempotent and
// wait for the rex_merge job, so a list combineRecords did carry is not added twice.
func mergeMailingLists(tx *sql.Tx, primary, secondary, rex, stamp string) error {
	prows, err := loadMailingRows(tx, primary)
	if err != nil {
		return err
	}
	srows, err := loadMailingRows(tx, secondary)
	if err != nil {
		return err
	}
	open, err := openMailingJobs(tx, primary)
	if err != nil {
		return err
	}
	had := map[string]bool{}
	for _, r := range prows {
		had[strings.ToLower(r.name)] = mailingActive(r.status)
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO contact_mailing_lists(contact_id,name,status,rex_row_id,subscribed_at,unsubscribed_at,unsubscribe_reason,updated_at) SELECT ?,name,status,'',subscribed_at,unsubscribed_at,unsubscribe_reason,? FROM contact_mailing_lists WHERE contact_id=?`, primary, stamp, secondary); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contact_mailing_lists SET status='',updated_at=? WHERE contact_id=? AND status<>'' AND lower(status) NOT IN ('active','subscribed') AND name IN (SELECT name FROM contact_mailing_lists WHERE contact_id=? AND lower(status) IN ('','active','subscribed'))`, stamp, primary, secondary); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM contact_mailing_lists WHERE contact_id=?`, secondary); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE rex_outbox SET status='skipped',last_error='contact merged',next_attempt_at=NULL WHERE entity IN (?,?) AND entity_id>? AND entity_id<? AND status IN ('pending','waiting_auth')`, mailingAdd, mailingRemove, secondary+":", secondary+";"); err != nil {
		return err
	}
	if rex == "none" {
		return nil
	}
	// Secondary lists are new to the primary; when only the secondary is in Rex,
	// the surviving Rex record also lacks the primary's lists.
	candidates := append([]mailingRow{}, srows...)
	if rex == "secondary" {
		candidates = append(candidates, prows...)
	}
	queued := map[string]bool{}
	for _, r := range candidates {
		k := strings.ToLower(r.name)
		if !mailingActive(r.status) || queued[k] || open[k] || (had[k] && rex != "secondary") {
			continue
		}
		queued[k] = true
		if err = enqueueMailing(tx, mailingAdd, primary, r.name, stamp); err != nil {
			return err
		}
	}
	return nil
}

// attachMailingLists puts the server state on a pulled contact: mailingLists
// (current names), mailingListSync (latest job per lower(name)) and rexLinked.
func attachMailingLists(db *sql.DB, id string, item map[string]any) {
	rows, err := loadMailingRows(db, id)
	if err != nil {
		return
	}
	names := []string{}
	for _, r := range rows {
		if mailingActive(r.status) {
			names = append(names, r.name)
		}
	}
	item["mailingLists"] = names
	item["mailingListsSeen"] = names
	jobs, err := db.Query(`SELECT entity,entity_id,status,COALESCE(last_error,''),created_at FROM rex_outbox WHERE entity IN (?,?) AND entity_id>? AND entity_id<? ORDER BY id`, mailingAdd, mailingRemove, id+":", id+";")
	if err != nil {
		return
	}
	out := map[string]any{}
	for jobs.Next() {
		var entity, key, status, reason, at string
		if jobs.Scan(&entity, &key, &status, &reason, &at) == nil {
			out[strings.TrimPrefix(key, id+":")] = map[string]any{"status": status, "reason": reason, "at": at, "op": strings.TrimPrefix(entity, "rex_contact_mailing_")}
		}
	}
	jobs.Close()
	if len(out) > 0 {
		item["mailingListSync"] = out
	} else {
		delete(item, "mailingListSync")
	}
	var linked int
	_ = db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='contacts' AND local_id=?`, id).Scan(&linked)
	item["rexLinked"] = linked > 0
}

// stepMailing writes one add/remove. Unlinked contacts are handled exactly like
// rex_contact_tag (rexworker.go). Both writes are idempotent: Contacts::read
// first; an existing active row is not added again, an absent list is a
// successful remove, so an ambiguous failure is retried, never duplicated.
func (w *rexWorker) stepMailing(ctx context.Context, j rexJob, finish func(status, reason, link, service string) error) error {
	var m map[string]any
	if json.Unmarshal([]byte(j.payload), &m) != nil || str(m, "contactId") == "" || mailingName(str(m, "name")) == "" {
		return finish("failed", "invalid payload", "", "")
	}
	cid, name := str(m, "contactId"), mailingName(str(m, "name"))
	var merging int
	_ = w.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='rex_merge' AND status IN ('pending','waiting_auth') AND (json_extract(payload,'$.primaryId')=? OR json_extract(payload,'$.secondaryId')=?)`, cid, cid).Scan(&merging)
	if merging > 0 {
		return w.deferJob(j, "waiting for contact merge")
	}
	linked := w.link("contacts", cid)
	var source string
	var exists int
	_ = w.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),''),deleted_at IS NULL FROM contacts WHERE id=?`, cid).Scan(&source, &exists)
	if linked == "" {
		if exists == 0 {
			return finish("skipped", "not linked to Rex sandbox record", "", "")
		}
		var createStatus string
		_ = w.db.QueryRow(`SELECT status FROM rex_outbox WHERE entity='contacts' AND entity_id=? AND op='POST' ORDER BY id DESC LIMIT 1`, cid).Scan(&createStatus)
		if createStatus == "failed" {
			return finish("failed", "contact creation failed; reconcile before retry", "", "")
		}
		if createStatus == "" || createStatus == "skipped" || createStatus == "sent" {
			return finish("skipped", "contact not in Rex", "", "")
		}
		return w.deferJob(j, "waiting for contact link")
	}
	if source != "rex_import" {
		return finish("skipped", "not linked to Rex sandbox record", "", "")
	}
	w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", cid, linked
	if w.client.Token == "" {
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return w.retry(j, "Rex login failed", 0)
		}
	}
	call := func(method string, body any) (json.RawMessage, json.RawMessage, error) {
		_, env, e := w.client.Call(ctx, "Contacts", method, body)
		if e != nil && strings.Contains(string(env.Error), "TokenException") {
			w.client.Token = ""
			if _, loginErr := w.client.Login(ctx, w.email, w.password, w.account); loginErr != nil {
				return nil, nil, loginErr
			}
			_, env, e = w.client.Call(ctx, "Contacts", method, body)
		}
		return env.Result, env.Error, e
	}
	fail := func(rexErr json.RawMessage, e error) error {
		var rate rexRateError
		if errors.As(e, &rate) {
			return w.retry(j, "Rex rate limited", rate.delay)
		}
		if len(rexErr) > 0 && string(rexErr) != "null" {
			return finish("failed", "Rex rejected: "+safeRexError(rexErr), "", "")
		}
		return w.retry(j, "Rex update failed", 0) // safe to replay: read-then-write
	}
	result, rexErr, e := call("read", map[string]any{"id": linked})
	if e != nil {
		return fail(rexErr, e)
	}
	var detail importRow
	_ = json.Unmarshal(result, &detail)
	list, _ := rexMailingRows(detail)
	key := mailingKey(name)
	var active, history []importRow
	for _, x := range list {
		if r := imap(x); mailingKey(iv(r, "mailing_list_name")) == key {
			if mailingActive(iv(r, "subscription_status")) {
				active = append(active, r)
			} else {
				history = append(history, r)
			}
		}
	}
	var ops []any
	if j.entity == mailingAdd {
		if len(active) == 0 && len(history) > 0 {
			// Usually the person unsubscribed from a Rex email; do not silently re-subscribe.
			return finish("failed", "unsubscribed in Rex ("+iv(history[0], "subscription_status")+"); re-subscribe in Rex", "", "")
		}
		if len(active) == 0 {
			ops = []any{map[string]any{"mailing_list_name": name}} // B3: the name is required, an id is rejected
			list = append(list, map[string]any{"mailing_list_name": name, "subscription_status": "active"})
		}
	} else {
		keep := []any{}
		for _, x := range list {
			if r := imap(x); mailingKey(iv(r, "mailing_list_name")) == key && mailingActive(iv(r, "subscription_status")) {
				ops = append(ops, map[string]any{"id": r["id"], "_destroy": true})
				continue
			}
			keep = append(keep, x)
		}
		list = keep
	}
	if len(ops) > 0 {
		body := map[string]any{"data": map[string]any{"id": linked, "related": map[string]any{"contact_mailing_lists": ops}}}
		if _, rexErr, e = call("update", body); e != nil {
			return fail(rexErr, e)
		}
	}
	_, _ = w.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "app_to_rex", j.entity, j.localID, linked, ijson(map[string]any{"name": name, "ops": ops}), "ok")
	if err := finish("sent", "", "", ""); err != nil {
		return err
	}
	// Rex wins once the job is done: save what Rex now holds (other open jobs keep their names).
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	if _, err = saveMailingMemberships(tx, cid, list, stamp); err != nil {
		return err
	}
	// Re-send the contact to phones (new mailingListSync) without a version bump,
	// which would turn the phone's next queued edit into a conflict.
	if _, err = tx.Exec(`UPDATE contacts SET updated_at=? WHERE id=?`, stamp, cid); err != nil {
		return err
	}
	return tx.Commit()
}

// Company catalogue: AdminMailingLists::search, refreshed rarely (REX_MAILING_LIST_HOURS,
// default 6, min 1; 30 minutes after a failure). One search page per 100 lists.
func pollRexMailingLists(ctx context.Context, db *sql.DB, c *rexClient, account string, at time.Time) error {
	if !loadRexConfig().canRead() {
		return nil
	}
	hours := 6
	if n, e := strconv.Atoi(os.Getenv("REX_MAILING_LIST_HOURS")); e == nil && n >= 1 {
		hours = n
	}
	var last, result string
	_ = db.QueryRow(`SELECT at,result FROM rex_sync_events WHERE entity='mailing_list_poll' AND rex_id=? ORDER BY id DESC LIMIT 1`, account).Scan(&last, &result)
	if previous, e := time.Parse(layout, last); e == nil {
		wait := time.Duration(hours) * time.Hour
		if result != "ok" {
			wait = 30 * time.Minute
		}
		if at.Sub(previous) < wait {
			return nil
		}
	}
	lists, err := rexMailingCatalog(ctx, c)
	if err != nil {
		_, _ = db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result,error) VALUES(?,?,?,?,?,?,?,?)`, at.UTC().Format(layout), "rex_to_app", "mailing_list_poll", "", account, `{}`, "error", err.Error())
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := at.UTC().Format(layout)
	if _, err = tx.Exec(`DELETE FROM rex_mailing_lists WHERE account_id=?`, account); err != nil {
		return err
	}
	for _, r := range lists {
		if _, err = tx.Exec(`INSERT OR REPLACE INTO rex_mailing_lists(account_id,name,rex_id,channel,data,updated_at) VALUES(?,?,?,?,?,?)`, account, r["name"], r["id"], r["channel"], ijson(r["raw"]), stamp); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, stamp, "rex_to_app", "mailing_list_poll", "", account, ijson(map[string]any{"lists": len(lists)}), "ok"); err != nil {
		return err
	}
	return tx.Commit()
}

// rexMailingCatalog: field names of AdminMailingLists rows are unverified (no
// openapi docs); the name and channel are read from the likely keys.
func rexMailingCatalog(ctx context.Context, c *rexClient) ([]map[string]any, error) {
	out := []map[string]any{}
	for offset := 0; ; offset += 100 {
		c.Entity, c.LocalID, c.RexID = "mailing_lists", "", ""
		_, env, err := c.Call(ctx, "AdminMailingLists", "search", map[string]any{"limit": 100, "offset": offset})
		if err != nil {
			return nil, err
		}
		var page struct {
			Rows  []importRow `json:"rows"`
			Total int         `json:"total"`
		}
		if err = json.Unmarshal(env.Result, &page); err != nil {
			return nil, fmt.Errorf("AdminMailingLists search: %w", err)
		}
		for _, r := range page.Rows {
			name := ""
			for _, k := range []string{"name", "mailing_list_name", "list_name", "title"} {
				if name = mailingName(iv(r, k)); name != "" {
					break
				}
			}
			if name != "" {
				out = append(out, map[string]any{"name": name, "id": iv(r, "id"), "channel": mailingChannel(r), "raw": r})
			}
		}
		if len(page.Rows) < 100 || (page.Total > 0 && offset+len(page.Rows) >= page.Total) {
			return out, nil
		}
	}
}

// mailingChannel maps a list type to a consent channel; ” when unknown.
func mailingChannel(r importRow) string {
	for _, k := range []string{"list_type", "type", "channel", "medium"} {
		v := strings.ToLower(iv(r, k))
		if m := imap(r[k]); m != nil {
			v = strings.ToLower(iv(m, "id") + " " + iv(m, "text"))
		}
		switch {
		case strings.Contains(v, "email"):
			return "email"
		case strings.Contains(v, "sms") || strings.Contains(v, "text"):
			return "sms"
		case strings.Contains(v, "letter") || strings.Contains(v, "post") || strings.Contains(v, "mail"):
			return "letter"
		}
	}
	return ""
}

// GET /api/v1/mailing-lists: the cached catalogue (never calls Rex).
func (a *app) mailingListsHandler(w http.ResponseWriter, p principal) {
	account := loadRexConfig().AccountID
	rows, err := a.db.Query(`SELECT name,channel FROM rex_mailing_lists WHERE account_id=? ORDER BY name COLLATE NOCASE`, account)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	items := []map[string]any{}
	for rows.Next() {
		var name, channel string
		if rows.Scan(&name, &channel) == nil {
			items = append(items, map[string]any{"name": name, "channel": channel})
		}
	}
	rows.Close()
	var at string
	_ = a.db.QueryRow(`SELECT at FROM rex_sync_events WHERE entity='mailing_list_poll' AND rex_id=? AND result='ok' ORDER BY id DESC LIMIT 1`, account).Scan(&at)
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i]["name"].(string)) < strings.ToLower(items[j]["name"].(string))
	})
	respond(w, 200, map[string]any{"items": items, "at": at})
}

// rex-probe ML (read-only): AdminMailingLists row fields (name/channel keys),
// its search fields, whether Contacts::search accepts extra_fields
// contact_mailing_lists (bulk import) and the related row shape on Contacts::read.
func init() {
	probeMethods["mailingLists"] = probeSpec{"AdminMailingLists", "search", func(_ map[string]string) any { return map[string]any{"limit": 3} }}
	probeMethods["mailingListFields"] = probeSpec{"AdminMailingLists", "describeSearchFields", func(_ map[string]string) any { return map[string]any{} }}
	probeMethods["contactsMailing"] = probeSpec{"Contacts", "search", func(_ map[string]string) any {
		return map[string]any{"limit": 3, "extra_options": map[string]any{"extra_fields": []string{"contact_mailing_lists"}}}
	}}
	probeMethods["contactMailingRead"] = probeSpec{"Contacts", "read", func(v map[string]string) any { return map[string]any{"id": v["mailingContact"]} }}
}
func (p *probeRun) probeMailing(call func(string) (json.RawMessage, error)) (string, string) {
	_, e1 := call("mailingLists")
	_, e2 := call("mailingListFields")
	raw, e3 := call("contactsMailing")
	var e4 error
	if p.vars["mailingContact"] = probeFirst(raw); p.vars["mailingContact"] != "" {
		_, e4 = call("contactMailingRead")
	}
	verdict := "UNKNOWN"
	if e1 == nil && e3 == nil {
		verdict = "PASS"
	}
	return verdict, "AdminMailingLists 行字段（name/类型）、Contacts search extra_fields contact_mailing_lists 是否被接受、read 的 related.contact_mailing_lists 见 key_tree；错误：" + probeError(errors.Join(e1, e2, e3, e4), p.c)
}
