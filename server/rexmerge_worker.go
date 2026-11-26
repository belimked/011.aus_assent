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
// Then the rule-merged contact is written to Rex through the normal jobs queued in
// the same transaction that closes this one: Contacts::update (names, phone,
// email, contact_by_*), DND, property relations (rex_merge_relation) and the merge
// log note (contact note -> Notes::create). A contact still being created in Rex is
// awaited until the payload deadline (mergeLinkWait), never indefinitely.

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
		queue("rex_merge_relation", primary+":"+str(r, "propertyId"), "POST", map[string]any{"contactId": primary, "propertyId": str(r, "propertyId"), "kind": str(r, "kind"), "replaces": r["replaces"], "losingRexId": losing})
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

// stepMergeRelation sets the contact's relation to one property after a merge:
// superseded relation types are removed with the row id + _destroy convention of
// Properties::update ("To remove sub records include the _destroy flag"), then the
// resolved type is added if Rex does not already have it.
// TODO(sandbox): confirm Properties::read returns contact_reln_property row ids and
// that _destroy removes a relation row; until then a missing row id fails the job
// with a reconcile reason instead of guessing.
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
	replaces := map[string]bool{}
	for _, k := range stringsFrom(m["replaces"]) {
		replaces[k] = true
	}
	losing := str(m, "losingRexId")
	w.client.Entity, w.client.LocalID, w.client.RexID = "properties", str(m, "propertyId"), property
	result, rexErr, err := w.mergeCall(ctx, "Properties", "read", map[string]any{"id": property})
	if err != nil {
		return w.mergeFail(j, err, rexErr, false, finish)
	}
	var row map[string]any
	_ = json.Unmarshal(result, &row)
	current, _ := imap(row["related"])["contact_reln_property"].([]any)
	ops := []any{}
	have, missingID := false, false
	for _, v := range current {
		r := imap(v)
		cid := iv(imap(r["contact"]), "id")
		if cid == "" {
			cid = iv(r, "contact_id")
		}
		rk := iv(imap(r["reln_type"]), "id")
		if rk == "" {
			rk = iv(r, "reln_type_id")
		}
		if cid != contact && (losing == "" || cid != losing) {
			continue
		}
		if cid == contact && rk == kind {
			have = true
			continue
		}
		if !replaces[rk] {
			continue
		}
		if id := iv(r, "id"); id != "" {
			ops = append(ops, map[string]any{"id": id, "_destroy": true})
		} else {
			missingID = true
		}
	}
	if !have {
		ops = append(ops, map[string]any{"contact": rexRef(contact), "reln_type": rexRef(kind)})
	}
	if len(ops) > 0 {
		_, rexErr, err = w.mergeCall(ctx, "Properties", "update", map[string]any{"data": map[string]any{"id": property, "related": map[string]any{"contact_reln_property": ops}}})
		if err != nil {
			return w.mergeFail(j, err, rexErr, true, finish)
		}
	}
	if missingID {
		return finish("failed", "superseded relation has no row id in Properties::read; remove it in Rex", "", "")
	}
	return finish("sent", "", "", "")
}
