package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// rex_merge follows a local contact merge (contact_merge.go, docs/contact-merge.md).
// Links are read when the job runs, not when the merge happened:
//   both in Rex      -> Dedupe::combineRecords(winning=primary, losing=secondary)
//   only secondary   -> the primary adopts the secondary's Rex record (rex_links moves)
//   only primary     -> no combine
//   neither          -> skipped
// Then the rule-merged contact is written to Rex through jobs queued in the same
// transaction that closes this one, each of which reads the Rex record back first:
// Contacts::update {"merge":true} (stepMergeContact: name, phone/email de-dupe and
// primary, tag de-dupe, contact_by_*), DND, one rex_merge_relation per merged
// property (stepMergeRelation: align the contact's rows with the merge result) and
// the merge log note (contact note -> Notes::create on the winner). combineRecords is
// synchronous and moves the loser's notes to the winner itself (sandbox 2057,
// 2026-11-26). A contact still being created in Rex is awaited until the payload
// deadline (mergeLinkWait), never indefinitely.

var errRexLogin = errors.New("Rex login failed")

func (w *rexWorker) stepMerge(ctx context.Context, j rexJob, finish func(status, reason, link, service string) error) error {
	var m map[string]any
	if json.Unmarshal([]byte(j.payload), &m) != nil {
		return finish("failed", "invalid payload", "", "")
	}
	if j.entity == "rex_merge_relation" {
		return w.stepMergeRelation(ctx, j, m, finish)
	}
	primary, secondary := str(m, "primaryId"), str(m, "secondaryId")
	var alive int
	_ = w.db.QueryRow(`SELECT count(*) FROM contacts WHERE id=? AND deleted_at IS NULL`, primary).Scan(&alive)
	if alive == 0 {
		return finish("skipped", "merged contact no longer exists", "", "")
	}
	winning, losing := w.link("contacts", primary), w.link("contacts", secondary)
	if winning == "" || losing == "" {
		deadline := mergeTime(str(m, "deadline"))
		if deadline.IsZero() { // jobs queued before the deadline existed
			var created string
			_ = w.db.QueryRow(`SELECT created_at FROM rex_outbox WHERE id=?`, j.id).Scan(&created)
			if t := mergeTime(created); !t.IsZero() {
				deadline = t.Add(mergeLinkWait)
			}
		}
		waiting := (winning == "" && w.createPending(primary)) || (losing == "" && w.createPending(secondary))
		if waiting && time.Now().Before(deadline) {
			return w.deferJob(j, "waiting for contact link")
		}
	}
	switch {
	case winning != "" && losing != "":
		if winning != losing {
			w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", primary, winning
			_, rexErr, err := w.mergeCall(ctx, "Dedupe", "combineRecords", map[string]any{"service_name": "Contacts", "winning_id": winning, "losing_ids": []string{losing}})
			if err != nil {
				return w.mergeFail(j, err, rexErr, true, finish)
			}
		}
		return w.mergeDone(j, m, primary, secondary, winning, losing, "combined", false)
	case losing != "":
		return w.mergeDone(j, m, primary, secondary, losing, "", "primary adopted the secondary's Rex record", true)
	case winning != "":
		return w.mergeDone(j, m, primary, secondary, winning, "", "only the primary is in Rex", false)
	}
	reason := "neither contact in Rex"
	if w.createPending(primary) || w.createPending(secondary) {
		reason = "timed out waiting for the Rex contact link"
	}
	return finish("skipped", reason, "", "")
}

func (w *rexWorker) createPending(id string) bool {
	var n int
	_ = w.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='contacts' AND entity_id=? AND op='POST' AND status IN ('pending','waiting_auth') AND COALESCE(json_extract(payload,'$.t'),'')=''`, id).Scan(&n)
	return n > 0
}

func (w *rexWorker) mergeCall(ctx context.Context, service, method string, body map[string]any) (json.RawMessage, json.RawMessage, error) {
	if w.client.Token == "" {
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return nil, nil, errRexLogin
		}
	}
	_, env, err := w.client.Call(ctx, service, method, body)
	if err != nil && strings.Contains(string(env.Error), "TokenException") {
		w.client.Token = ""
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return nil, nil, errRexLogin
		}
		_, env, err = w.client.Call(ctx, service, method, body)
	}
	return env.Result, env.Error, err
}

func (w *rexWorker) mergeFail(j rexJob, err error, rexErr json.RawMessage, write bool, finish func(status, reason, link, service string) error) error {
	var rate rexRateError
	switch {
	case errors.Is(err, errRexLogin):
		return w.retry(j, "Rex login failed", 0)
	case errors.As(err, &rate):
		return w.retry(j, "Rex rate limited", rate.delay)
	case len(rexErr) > 0 && string(rexErr) != "null":
		return finish("failed", "Rex rejected: "+safeRexError(rexErr), "", "")
	case write:
		return finish("failed", "Rex write outcome unknown; reconcile before retry", "", "")
	}
	return w.retry(j, "Rex read failed", 0)
}

// mergeDone closes the rex_merge job and queues the follow-up writes atomically,
// so a crash can never repeat combineRecords or lose the follow-ups.
func (w *rexWorker) mergeDone(j rexJob, m map[string]any, primary, secondary, rexID, losing, reason string, adopt bool) error {
	source := ""
	if adopt {
		var ps, ss string
		_ = w.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),'') FROM contacts WHERE id=?`, primary).Scan(&ps)
		_ = w.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),'') FROM contacts WHERE id=?`, secondary).Scan(&ss)
		source = ps
		if ss != "" {
			source = ss // the Rex record's origin decides which Rex writes are allowed
		}
	}
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	if adopt {
		if _, err = tx.Exec(`UPDATE rex_links SET local_id=? WHERE entity='contacts' AND local_id=? AND account_id=?`, primary, secondary, w.account); err != nil {
			_ = tx.Rollback()
			return w.finishMerge(j, "failed", "primary already linked to another Rex record: "+err.Error())
		}
		if _, err = tx.Exec(`UPDATE contacts SET data=json_set(data,'$.rexId',?,'$.source',?),updated_at=?,version=version+1 WHERE id=?`, rexID, source, stamp, primary); err != nil {
			return err
		}
	}
	queue := func(entity, key, op string, payload any) {
		if err == nil {
			_, err = tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?,?,?,?)`, entity, key, op, ijson(payload), stamp)
		}
	}
	queue("contacts", primary, "PATCH", map[string]any{"id": primary, "merge": true})
	if m["dnc"] == true {
		queue("rex_contact_dnd", primary, "POST", map[string]any{"contactId": primary, "value": true})
	}
	rels, _ := m["relations"].([]any)
	for _, v := range rels {
		r := imap(v)
		queue("rex_merge_relation", primary+":"+str(r, "propertyId"), "POST", map[string]any{"contactId": primary, "propertyId": str(r, "propertyId"), "kind": str(r, "kind"), "replaces": r["replaces"]})
	}
	if note := imap(m["note"]); str(note, "t") != "" {
		note["contactId"] = primary
		queue("contacts", primary, "POST", note)
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE rex_outbox SET status='sent',last_error=?,next_attempt_at=NULL WHERE id=?`, reason, j.id)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, stamp, "app_to_rex", "rex_merge", primary, rexID, ijson(map[string]any{"secondaryId": secondary, "losingRexId": losing, "case": reason}), "ok")
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (w *rexWorker) finishMerge(j rexJob, status, reason string) error {
	_, err := w.db.Exec(`UPDATE rex_outbox SET status=?,last_error=?,next_attempt_at=NULL WHERE id=?`, status, reason, j.id)
	if err == nil {
		_, err = w.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result,error) VALUES(?,?,?,?,?,?,?,?)`, now(), "app_to_rex", j.entity, j.localID, "", `{}`, "error", reason)
	}
	return err
}

// mergeFailRetry handles a failed call in an idempotent step (everything is read
// back and recomputed on the next attempt), so an unknown outcome is retried
// instead of being parked for manual reconciliation.
func (w *rexWorker) mergeFailRetry(j rexJob, err error, rexErr json.RawMessage, reason string, finish func(status, reason, link, service string) error) error {
	var rate rexRateError
	if errors.Is(err, errRexLogin) || errors.As(err, &rate) || (len(rexErr) > 0 && string(rexErr) != "null") {
		return w.mergeFail(j, err, rexErr, false, finish)
	}
	return w.retry(j, reason, 0)
}

func rexRejected(rexErr json.RawMessage) bool {
	return len(rexErr) > 0 && string(rexErr) != "null"
}

// relRow is one contact_reln_property row of Properties::read.
type relRow struct{ id, contact, kind string }

func relationRows(result json.RawMessage) []relRow {
	var row map[string]any
	_ = json.Unmarshal(result, &row)
	list, _ := imap(row["related"])["contact_reln_property"].([]any)
	out := []relRow{}
	for _, v := range list {
		r := imap(v)
		cid := iv(imap(r["contact"]), "id")
		if cid == "" {
			cid = iv(r, "contact_id")
		}
		rk := iv(imap(r["reln_type"]), "id")
		if rk == "" {
			rk = iv(r, "reln_type_id")
		}
		out = append(out, relRow{id: iv(r, "id"), contact: cid, kind: rk})
	}
	return out
}

// alignRelationOps returns the contact_reln_property operations that leave the
// contact with exactly one row of type kind on the property. Rows of any other
// contact are never part of the result. typeUpdate selects whether a row of the
// wrong type is corrected in place (same id, new reln_type) or destroyed and a
// new row added. fixed reports that an in-place type update is included.
func alignRelationOps(rows []relRow, contact, kind string, typeUpdate bool) (ops []any, fixed bool, unaddressable int) {
	mine := []relRow{}
	for _, r := range rows {
		if r.contact == contact {
			mine = append(mine, r)
		}
	}
	keep := -1
	for i, r := range mine {
		if r.kind == kind {
			keep = i
			break
		}
	}
	if keep < 0 && typeUpdate {
		for i, r := range mine {
			if r.id != "" {
				keep = i
				break
			}
		}
	}
	ops = []any{}
	for i, r := range mine {
		if i == keep {
			continue
		}
		if r.id == "" {
			unaddressable++
			continue
		}
		ops = append(ops, map[string]any{"id": r.id, "_destroy": true})
	}
	switch {
	case keep < 0:
		ops = append(ops, map[string]any{"contact": rexRef(contact), "reln_type": rexRef(kind)})
	case mine[keep].kind != kind:
		ops = append(ops, map[string]any{"id": mine[keep].id, "reln_type": rexRef(kind)})
		fixed = true
	}
	return ops, fixed, unaddressable
}

// stepMergeRelation aligns the merged contact's relation to one property with the
// local merge result, from what Rex actually holds (sandbox 2057, 2026-11-26):
// Dedupe::combineRecords keeps the winner's row id on a shared property but gives
// it the loser's relation type, and drops the loser's row. The job reads the
// property, keeps one row of the expected type for the contact (correcting the
// type in place, or destroy + add when Rex rejects or ignores that), destroys the
// contact's extra rows and adds the row when missing. Rows of other contacts are
// never touched. Every pass re-reads, so a retry cannot duplicate a row.
func (w *rexWorker) stepMergeRelation(ctx context.Context, j rexJob, m map[string]any, finish func(status, reason, link, service string) error) error {
	contact := w.link("contacts", str(m, "contactId"))
	if contact == "" {
		return finish("skipped", "contact not in Rex", "", "")
	}
	property := w.link("properties", str(m, "propertyId"))
	if property == "" {
		return finish("skipped", "not in Rex", "", "")
	}
	kind := str(m, "kind")
	if kind != "owner" && kind != "tenant" && kind != "past_owner" && kind != "past_tenant" {
		return finish("skipped", "unsupported property relation", "", "")
	}
	w.client.Entity, w.client.LocalID, w.client.RexID = "properties", str(m, "propertyId"), property
	typeUpdate, reason := true, ""
	for pass := 0; pass < 3; pass++ {
		result, rexErr, err := w.mergeCall(ctx, "Properties", "read", map[string]any{"id": property})
		if err != nil {
			return w.mergeFail(j, err, rexErr, false, finish)
		}
		ops, fixed, unaddressable := alignRelationOps(relationRows(result), contact, kind, typeUpdate)
		if unaddressable > 0 {
			reason = "relation rows without id left in Rex"
		}
		if len(ops) == 0 {
			return finish("sent", reason, "", "")
		}
		if pass == 2 {
			break
		}
		_, rexErr, err = w.mergeCall(ctx, "Properties", "update", map[string]any{"data": map[string]any{"id": property, "related": map[string]any{"contact_reln_property": ops}}})
		if err != nil {
			if fixed && rexRejected(rexErr) {
				typeUpdate, reason = false, "relation type replaced by destroy + add"
				continue
			}
			return w.mergeFailRetry(j, err, rexErr, "Rex write outcome unknown; will re-read and align", finish)
		}
		if !fixed {
			return finish("sent", reason, "", "")
		}
		// Verify the in-place type change; if Rex ignored it the next pass
		// destroys the row and adds the expected type.
		typeUpdate = false
	}
	return finish("failed", "relation still not aligned after update; check the property in Rex", "", "")
}

// mergeRelated builds the Contacts::update related block that writes the local
// merge result c over what Dedupe::combineRecords left on the winner rex (sandbox
// 2057, 2026-11-26): Rex joins both names ("A & B") into the winner's name and
// appends every phone, email and tag of the loser without de-duplicating.
//   - names: the first contact_names row gets the local first/last, other rows are destroyed;
//   - phones (normAUMobile) / emails (lower case): duplicates destroyed, the local
//     mobile / email becomes primary (added when missing); other numbers stay;
//   - tags (case-insensitive) duplicates are returned separately (tagOps): Rex
//     tag removal is not confirmed, so the caller sends them on their own.
func mergeRelated(c, rex map[string]any) (related map[string]any, tagOps []any, fail string) {
	rows := func(key string) []map[string]any {
		list, _ := imap(rex["related"])[key].([]any)
		out := []map[string]any{}
		for _, v := range list {
			out = append(out, imap(v))
		}
		return out
	}
	related = map[string]any{}
	names := rows("contact_names")
	if len(names) == 0 || names[0]["id"] == nil {
		return nil, nil, "contact name subrow id missing; re-import detailed Rex record"
	}
	nameOps := []any{map[string]any{"id": names[0]["id"], "name_first": str(c, "first"), "name_last": str(c, "last")}}
	for _, r := range names[1:] {
		if r["id"] != nil {
			nameOps = append(nameOps, map[string]any{"id": r["id"], "_destroy": true})
		}
	}
	related["contact_names"] = nameOps

	dedupe := func(key, valueKey, primaryKey, want string, norm func(string) string, add map[string]any) {
		seen := map[string]bool{}
		ops := []any{}
		found := false
		type kept struct {
			row     map[string]any
			primary bool
			match   bool
		}
		keep := []kept{}
		for _, r := range rows(key) {
			k := norm(iv(r, valueKey))
			if k == "" {
				k = "\x00" + iv(r, "id")
			}
			if seen[k] {
				if r["id"] != nil {
					ops = append(ops, map[string]any{"id": r["id"], "_destroy": true})
				}
				continue
			}
			seen[k] = true
			match := want != "" && k == norm(want)
			found = found || match
			keep = append(keep, kept{r, ibool(r[primaryKey]), match})
		}
		for _, x := range keep {
			if x.row["id"] == nil || want == "" {
				continue
			}
			// Only the local value is primary; every other kept row is left as it is
			// except that it stops being primary.
			if x.match != x.primary {
				ops = append(ops, map[string]any{"id": x.row["id"], valueKey: x.row[valueKey], primaryKey: x.match})
			}
		}
		if want != "" && !found {
			ops = append(ops, add)
		}
		if len(ops) > 0 {
			related[key] = ops
		}
	}
	mobile, email := str(c, "mobile"), str(c, "email")
	dedupe("contact_phones", "phone_number", "phone_primary", mobile, normAUMobile,
		map[string]any{"phone_number": mobile, "phone_type": "mobile", "phone_primary": true})
	dedupe("contact_emails", "email_address", "email_primary", email, func(s string) string { return strings.ToLower(strings.TrimSpace(s)) },
		map[string]any{"email_address": email, "email_desc": "home", "email_primary": true})

	seen := map[string]bool{}
	for _, r := range rows("contact_tags") {
		k := strings.ToLower(strings.TrimSpace(iv(r, "tag")))
		if seen[k] && r["id"] != nil {
			tagOps = append(tagOps, map[string]any{"id": r["id"], "tag": r["tag"], "_destroy": true})
		}
		seen[k] = true
	}
	return related, tagOps, ""
}

// stepMergeContact writes the merged contact to its Rex record after rex_merge
// (Contacts::update queued as {"merge":true}). It always reads the record back
// first, so it is safe to retry and corrects what combineRecords left behind.
func (w *rexWorker) stepMergeContact(ctx context.Context, j rexJob, c map[string]any, rexID string, finish func(status, reason, link, service string) error) error {
	w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", j.localID, rexID
	result, rexErr, err := w.mergeCall(ctx, "Contacts", "read", map[string]any{"id": rexID})
	if err != nil {
		return w.mergeFail(j, err, rexErr, false, finish)
	}
	var row map[string]any
	if json.Unmarshal(result, &row) != nil {
		return w.retry(j, "invalid Rex contact read", 0)
	}
	related, tagOps, fail := mergeRelated(c, row)
	if fail != "" {
		return finish("failed", fail, "", "")
	}
	data := map[string]any{"id": rexID, "related": related}
	for _, field := range []string{"letter", "email", "phone", "sms"} {
		if value, ok := imap(c["consent"])[field]; ok {
			data["contact_by_"+field] = value
		}
	}
	result, rexErr, err = w.mergeCall(ctx, "Contacts", "update", map[string]any{"data": data})
	if err != nil {
		return w.mergeFailRetry(j, err, rexErr, "Rex update failed", finish)
	}
	reason := ""
	if len(tagOps) > 0 {
		// Separate request: a rejected tag removal must not undo the name/phone fix.
		if _, tagErr, e := w.mergeCall(ctx, "Contacts", "update", map[string]any{"data": map[string]any{"id": rexID, "related": map[string]any{"contact_tags": tagOps}}}); e != nil {
			reason = "duplicate tags left in Rex"
			if rexRejected(tagErr) {
				reason += ": " + safeRexError(tagErr)
			}
			_, _ = w.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result,error) VALUES(?,?,?,?,?,?,?,?)`, now(), "app_to_rex", "contacts", j.localID, rexID, ijson(map[string]any{"contact_tags": tagOps}), "skipped", reason)
		} else {
			result = nil // the etag changed again
		}
	}
	var updated map[string]any
	_ = json.Unmarshal(result, &updated)
	if str(updated, "etag") == "" {
		if read, _, e := w.mergeCall(ctx, "Contacts", "read", map[string]any{"id": rexID}); e == nil {
			_ = json.Unmarshal(read, &updated)
		}
	}
	if etag := str(updated, "etag"); etag != "" {
		_, _ = w.db.Exec(`UPDATE rex_links SET etag=? WHERE entity='contacts' AND local_id=?`, etag, j.localID)
	}
	_, _ = w.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "app_to_rex", "contacts", j.localID, rexID, ijson(map[string]any{"merge": related, "etag": str(updated, "etag")}), "ok")
	return finish("sent", reason, "", "")
}
