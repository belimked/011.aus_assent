package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type rexWorker struct {
	db                       *sql.DB
	client                   *rexClient
	email, password, account string
}
type rexJob struct {
	id                           int64
	entity, localID, op, payload string
	tries                        int
}

func newRexWorker(db *sql.DB) *rexWorker {
	if os.Getenv("AXLEO_REX_ENABLED") != "1" {
		return nil
	}
	for _, k := range []string{"REX_BASE_URL", "REX_APP_ID", "REX_EMAIL", "REX_PASSWORD", "REX_ACCOUNT_ID"} {
		if os.Getenv(k) == "" {
			return nil
		}
	}
	w := &rexWorker{db: db, client: newRexClient(os.Getenv("REX_BASE_URL"), os.Getenv("REX_APP_ID")), email: os.Getenv("REX_EMAIL"), password: os.Getenv("REX_PASSWORD"), account: os.Getenv("REX_ACCOUNT_ID")}
	if ms, e := strconv.Atoi(os.Getenv("REX_INTERVAL_MS")); e == nil && ms >= 1000 {
		w.client.Interval = time.Duration(ms) * time.Millisecond
	}
	// One worker per process; the service lock prevents concurrent server processes.
	w.client.Retries = 0 // persist Retry-After rather than sleeping with a claimed job
	return w
}
func (w *rexWorker) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := w.step(ctx); err != nil { /* transient DB errors are retried on the next tick */
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func (w *rexWorker) link(entity, id string) string {
	var rex string
	_ = w.db.QueryRow(`SELECT rex_id FROM rex_links WHERE entity=? AND local_id=?`, entity, id).Scan(&rex)
	return rex
}
func (w *rexWorker) agent(id string) (name, rex string) {
	_ = w.db.QueryRow(`SELECT name,COALESCE(rex_user_id,'') FROM users WHERE id=?`, id).Scan(&name, &rex)
	if name == "" {
		name = "Agent"
	}
	return
}
func rexRef(id string) map[string]any { return map[string]any{"id": id} }
func (w *rexWorker) step(ctx context.Context) error {
	var j rexJob
	err := w.db.QueryRow(`SELECT id,entity,entity_id,op,payload,tries FROM rex_outbox WHERE status IN ('waiting_auth','pending') AND (next_attempt_at IS NULL OR next_attempt_at<=?) ORDER BY id LIMIT 1`, now()).Scan(&j.id, &j.entity, &j.localID, &j.op, &j.payload, &j.tries)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	finish := func(status, reason, link, service string) error {
		tx, e := w.db.Begin()
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if link != "" {
			if _, e = tx.Exec(`INSERT OR IGNORE INTO rex_links(entity,local_id,rex_id,rex_service,created_at) VALUES(?,?,?,?,?)`, j.entity, j.localID, link, service, now()); e != nil {
				return e
			}
		}
		_, e = tx.Exec(`UPDATE rex_outbox SET status=?,last_error=?,next_attempt_at=NULL WHERE id=?`, status, reason, j.id)
		if e != nil {
			return e
		}
		return tx.Commit()
	}
	var m map[string]any
	if json.Unmarshal([]byte(j.payload), &m) != nil {
		return finish("failed", "invalid payload", "", "")
	}
	if j.op == "DELETE" {
		return finish("skipped", "not mapped yet", "", "")
	}
	service, method, entityLink := "", "create", ""
	body := map[string]any{}
	update := false
	var data map[string]any
	actor := ""
	switch j.entity {
	case "rex_merge":
		winning, losing := w.link("contacts", str(m, "primaryId")), w.link("contacts", str(m, "secondaryId"))
		if winning == "" || losing == "" {
			return w.deferJob(j, "waiting for contact links")
		}
		service, method = "Dedupe", "combineRecords"
		body = map[string]any{"service_name": "Contacts", "winning_id": winning, "losing_ids": []string{losing}}
	case "rex_contact_tag", "rex_contact_relation", "rex_contact_letter", "rex_contact_dnd":
		contact := w.link("contacts", str(m, "contactId"))
		if contact == "" {
			return w.deferJob(j, "waiting for contact")
		}
		service, method, update = "Contacts", "update", true
		data = map[string]any{"id": contact}
		switch j.entity {
		case "rex_contact_tag":
			tag := str(m, "tag")
			switch str(m, "group") {
			case "DK":
				tag = "DK - " + tag
			case "OFI":
				tag = "OFI - " + tag
			case "Enquiry":
				tag = "Enquiry - " + tag
			}
			data["related"] = map[string]any{"contact_tags": []any{map[string]any{"tag": tag}}}
		case "rex_contact_relation":
			rel, _ := m["relation"].(map[string]any)
			if propertyID := str(rel, "propertyId"); propertyID != "" {
				property := w.link("properties", propertyID)
				if property == "" {
					return w.deferJob(j, "waiting for property")
				}
				kind := map[string]string{"Tenant of": "tenant", "Previous owner of": "past_owner", "Owner of": "owner", "Previous tenant of": "past_tenant"}[str(rel, "type")]
				if kind == "" {
					return finish("skipped", "unsupported property relation", "", "")
				}
				service = "Properties"
				data = map[string]any{"id": property, "related": map[string]any{"contact_reln_property": []any{map[string]any{"contact": rexRef(contact), "reln_type": rexRef(kind)}}}}
				break
			}
			targetID := str(rel, "contactId")
			if targetID == "" {
				return finish("skipped", "relation has no local contact ID", "", "")
			}
			target := w.link("contacts", targetID)
			if target == "" {
				return w.deferJob(j, "waiting for related contact")
			}
			// TODO(sandbox): contact-to-contact relation subtable and reln_type IDs are not confirmed by rexprobe.
			data["related"] = map[string]any{"contact_reln_contact": []any{map[string]any{"contact": rexRef(target), "reln_type": rexRef(str(rel, "type"))}}}
		case "rex_contact_letter":
			data["contact_by_letter"] = m["value"]
		case "rex_contact_dnd":
			data["is_dnd"] = m["value"] // Never combine with channel preferences: Rex resets them.
		}
	case "rex_todo_completion":
		reminder := w.link("todos", str(m, "todoId"))
		if reminder == "" {
			return w.deferJob(j, "waiting for reminder")
		}
		service, method = "Reminders", "toggleCompletionState"
		body = map[string]any{"id": reminder, "is_completed": true, "create_activity_entry": true}
	case "rex_completion_note":
		if str(m, "contactId") == "" {
			return finish("skipped", "completion note needs a contact", "", "")
		}
		contact := w.link("contacts", str(m, "contactId"))
		if contact == "" {
			return w.deferJob(j, "waiting for contact")
		}
		service = "Notes"
		actor, _ := w.agent(str(m, "actorId"))
		data = map[string]any{"note": str(m, "note") + rexSignature(actor), "note_type": rexRef("note"), "related": map[string]any{"note_contacts": []any{map[string]any{"contact": rexRef(contact)}}}}
	case "contacts":
		if str(m, "contactId") != "" && str(m, "t") != "" { // contact_notes are enqueued as contacts with their own id
			j.entity = "contact_notes"
			j.localID = str(m, "id")
			entityLink = w.link("contact_notes", j.localID)
			contact := w.link("contacts", str(m, "contactId"))
			if contact == "" {
				return w.deferJob(j, "waiting for contact")
			}
			service = "Notes"
			actor = str(m, "a")
			var userID string
			_ = w.db.QueryRow(`SELECT user_id FROM contact_notes WHERE id=?`, j.localID).Scan(&userID)
			if userID != "" {
				actor, _ = w.agent(userID)
			}
			data = map[string]any{"note": str(m, "t") + rexSignature(actor), "note_type": rexRef("note"), "related": map[string]any{"note_contacts": []any{map[string]any{"contact": rexRef(contact)}}}}
		} else {
			service = "Contacts"
			entityLink = w.link("contacts", j.localID)
			var raw, owner string
			if e := w.db.QueryRow(`SELECT data,owner_id FROM contacts WHERE id=?`, j.localID).Scan(&raw, &owner); e != nil {
				return finish("skipped", "local contact missing", "", "")
			}
			var c map[string]any
			if json.Unmarshal([]byte(raw), &c) != nil {
				return finish("failed", "invalid contact", "", "")
			}
			if entityLink != "" {
				if j.op == "POST" { // create already done (link stored)
					return finish("sent", "", "", "")
				}
				// Contacts::update with related rows would append duplicate phones/emails in Rex; not mapped until the update shape is sandbox-verified.
				return finish("skipped", "contact update not mapped yet", "", "")
			}
			// Shape verified in sandbox 2026-11-17 (.tmp/rex/extra run.sh / run2.sh).
			data = map[string]any{"contact_type": "person"}
			related := map[string]any{"contact_names": []any{map[string]any{"name_first": str(c, "first"), "name_last": str(c, "last")}}}
			if s := str(c, "mobile"); s != "" {
				related["contact_phones"] = []any{map[string]any{"phone_number": s, "phone_type": "mobile", "phone_primary": true}}
			}
			if s := str(c, "email"); s != "" {
				related["contact_emails"] = []any{map[string]any{"email_address": s, "email_desc": "home", "email_primary": true}}
			}
			data["related"] = related
		}
	case "leads":
		service = "Leads"
		entityLink = w.link("leads", j.localID)
		var contactID, owner, raw string
		if e := w.db.QueryRow(`SELECT contact_id,owner_id,data FROM leads WHERE id=?`, j.localID).Scan(&contactID, &owner, &raw); e != nil {
			return finish("skipped", "local lead missing", "", "")
		}
		var lead map[string]any
		_ = json.Unmarshal([]byte(raw), &lead)
		if contactID == "" {
			contactID = str(lead, "contactId")
		}
		contact := w.link("contacts", contactID)
		if contact == "" {
			return w.deferJob(j, "waiting for contact")
		}
		actor, assignee := w.agent(owner)
		data = map[string]any{"contact": rexRef(contact), "lead_type": rexRef("general"), "note": str(lead, "note") + rexSignature(actor)}
		if assignee != "" {
			data["assignee"] = rexRef(assignee)
		}
	case "todos":
		service = "Reminders"
		entityLink = w.link("todos", j.localID)
		var raw, assignee, creator, contactID, propertyID string
		if e := w.db.QueryRow(`SELECT data,assignee_id,created_by,COALESCE(contact_id,''),COALESCE(property_id,'') FROM todos WHERE id=?`, j.localID).Scan(&raw, &assignee, &creator, &contactID, &propertyID); e != nil {
			return finish("skipped", "local todo missing", "", "")
		}
		var todo map[string]any
		_ = json.Unmarshal([]byte(raw), &todo)
		if contactID == "" {
			contactID = str(todo, "cid")
		}
		if propertyID == "" {
			propertyID = str(todo, "propId")
		}
		contact, property := w.link("contacts", contactID), w.link("properties", propertyID)
		if contactID == "" && propertyID == "" {
			return finish("skipped", "reminder needs contact or property", "", "")
		}
		if contact == "" && property == "" {
			return w.deferJob(j, "waiting for contact/property link")
		}
		actor, _ = w.agent(creator)
		_, remindee := w.agent(assignee)
		if remindee == "" {
			return finish("skipped", "remindee not mapped to Rex user", "", "")
		}
		data = map[string]any{"remindee": map[string]any{"id": remindee, "type": "user"}, "reminder_type": rexRef("task"), "reminder_date": str(todo, "d"), "reminder_description": str(todo, "t") + rexSignature(actor)}
		if contact != "" {
			data["contact"] = rexRef(contact)
		} else {
			data["property"] = rexRef(property)
		}
		if str(todo, "d") == "" {
			return finish("skipped", "reminder needs date", "", "")
		}
	default:
		return finish("skipped", "not mapped yet", "", "")
	}
	if entityLink != "" {
		if j.entity != "contacts" || service != "Contacts" {
			return finish("sent", "", "", "")
		}
		method = "update"
		data["id"] = entityLink
	}
	if data != nil {
		body = map[string]any{"data": data, "return_id": true}
	}
	if w.client.Token == "" {
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return w.retry(j, "Rex login failed", 0)
		}
	}
	call := func() (json.RawMessage, json.RawMessage, error) {
		_, env, x := w.client.Call(ctx, service, method, body)
		return env.Result, env.Error, x
	}
	result, rexErr, e := call()
	if e != nil && strings.Contains(string(rexErr), "TokenException") {
		w.client.Token = ""
		if _, loginErr := w.client.Login(ctx, w.email, w.password, w.account); loginErr != nil {
			return w.retry(j, "Rex login failed", 0)
		}
		result, _, e = call()
	}
	if e != nil {
		var rate rexRateError
		if errors.As(e, &rate) {
			return w.retry(j, "Rex rate limited", rate.delay)
		}
		// Rex answered with an error envelope: the write was rejected, not ambiguous.
		if len(rexErr) > 0 && string(rexErr) != "null" {
			return finish("failed", "Rex rejected: "+safeRexError(rexErr), "", "")
		}
		// No envelope (timeout, empty 5xx): a create may have succeeded despite a lost response. Never replay an ambiguous write.
		if method == "create" || method == "combineRecords" || method == "toggleCompletionState" || j.entity == "rex_contact_tag" || j.entity == "rex_contact_relation" || j.entity == "rex_contact_dnd" {
			return finish("failed", "Rex write outcome unknown; reconcile before retry", "", "")
		}
		return w.retry(j, "Rex update failed", 0)
	}
	if method != "create" || update {
		return finish("sent", "", "", "")
	}
	id := probeID(result)
	if id == "" {
		return finish("failed", "Rex returned no id; reconcile before retry", "", "")
	}
	return finish("sent", "", id, service)
}
func rexSignature(name string) string {
	if name == "" {
		name = "Agent"
	}
	return "\n— via AxleoCollect · " + name
}
func (w *rexWorker) deferJob(j rexJob, reason string) error {
	_, e := w.db.Exec(`UPDATE rex_outbox SET status='pending',last_error=?,next_attempt_at=? WHERE id=?`, reason, time.Now().UTC().Add(15*time.Second).Format(layout), j.id)
	return e
}
func (w *rexWorker) retry(j rexJob, reason string, delay time.Duration) error {
	tries := j.tries + 1
	status := "pending"
	if tries >= 6 {
		status = "failed"
	}
	if delay == 0 {
		delay = time.Duration(1<<min(tries, 8)) * time.Second
	}
	if delay > time.Hour {
		delay = time.Hour
	}
	_, e := w.db.Exec(`UPDATE rex_outbox SET status=?,tries=?,last_error=?,next_attempt_at=? WHERE id=?`, status, tries, reason, time.Now().UTC().Add(delay).Format(layout), j.id)
	return e
}
