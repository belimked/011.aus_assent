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
	c := loadRexConfig()
	if !c.canWrite() {
		return nil
	}
	w := &rexWorker{db: db, client: newRexClient(c.BaseURL, c.AppID), email: c.Email, password: c.Password, account: c.AccountID}
	w.client.AccountID = c.AccountID
	w.client.Caller = "worker"
	w.client.Password = c.Password
	w.client.Log = rexDBLogger(db)
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
	_ = w.db.QueryRow(`SELECT rex_id FROM rex_links WHERE entity=? AND local_id=? AND account_id=?`, entity, id, w.account).Scan(&rex)
	return rex
}
func (w *rexWorker) agent(id string) (name, rex string) {
	_ = w.db.QueryRow(`SELECT name,COALESCE(rex_user_id,'') FROM users WHERE id=?`, id).Scan(&name, &rex)
	if name == "" {
		name = "Agent"
	}
	return
}
func (w *rexWorker) userName(id string) string {
	var name string
	if id != "" {
		_ = w.db.QueryRow(`SELECT name FROM users WHERE id=?`, id).Scan(&name)
	}
	return name
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
			if _, e = tx.Exec(`INSERT OR IGNORE INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES(?,?,?,?,?,?)`, j.entity, j.localID, link, service, now(), w.account); e != nil {
				return e
			}
		}
		_, e = tx.Exec(`UPDATE rex_outbox SET status=?,last_error=?,next_attempt_at=NULL WHERE id=?`, status, reason, j.id)
		if e != nil {
			return e
		}
		if j.entity == "contacts" {
			if link != "" {
				_, e = tx.Exec(`UPDATE contacts SET data=json_set(data,'$.rexId',?) WHERE id=?`, link, j.localID)
				if e != nil {
					return e
				}
			}
			_, e = tx.Exec(`UPDATE contacts SET updated_at=?,version=version+1 WHERE id=?`, now(), j.localID)
			if e != nil {
				return e
			}
		}
		if j.entity == "rex_contact_relation" || j.entity == "rex_property_note" {
			var payload map[string]any
			_ = json.Unmarshal([]byte(j.payload), &payload)
			pid := str(payload, "propertyId")
			if j.entity == "rex_contact_relation" {
				pid = str(imap(payload["relation"]), "propertyId")
			}
			if pid != "" {
				_, e = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1 WHERE id=?`, now(), pid)
				if e != nil {
					return e
				}
			}
		}
		if status == "skipped" || status == "failed" {
			_, e = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result,error) VALUES(?,?,?,?,?,?,?,?)`, now(), "app_to_rex", j.entity, j.localID, link, `{}`, map[string]string{"skipped": "skipped", "failed": "error"}[status], reason)
			if e != nil {
				return e
			}
		}
		return tx.Commit()
	}
	// Contact merge jobs have their own bounded wait and follow-up writes (rexmerge_worker.go).
	if j.entity == "rex_merge" || j.entity == "rex_merge_relation" {
		return w.stepMerge(ctx, j, finish)
	}
	// Outbox is not a create queue. No job may write an unlinked demo/app record.
	linkID := j.localID
	if strings.HasPrefix(j.entity, "rex_contact_") {
		var payload map[string]any
		_ = json.Unmarshal([]byte(j.payload), &payload)
		linkID = str(payload, "contactId")
	}
	linked := w.link("contacts", linkID)
	if j.entity == "properties" {
		if w.link("properties", j.localID) == "" {
			return finish("skipped", "not in Rex", "", "")
		}
		return finish("sent", "", "", "")
	}
	if j.entity != "contacts" && j.entity != "rex_contact_relation" && j.entity != "rex_contact_dnd" && j.entity != "rex_contact_tag" && j.entity != "rex_contact_letter" && j.entity != "rex_property_note" {
		return finish("skipped", "not linked to Rex sandbox record", "", "")
	}
	if j.entity != "rex_property_note" { // property notes need the Rex property, not a contact
		var source string
		_ = w.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),'') FROM contacts WHERE id=?`, linkID).Scan(&source)
		if linked == "" && j.entity != "contacts" {
			var exists int
			_ = w.db.QueryRow(`SELECT count(*) FROM contacts WHERE id=? AND deleted_at IS NULL`, linkID).Scan(&exists)
			if exists == 0 {
				return finish("skipped", "not linked to Rex sandbox record", "", "")
			}
			var createStatus string
			_ = w.db.QueryRow(`SELECT status FROM rex_outbox WHERE entity='contacts' AND entity_id=? AND op='POST' ORDER BY id DESC LIMIT 1`, linkID).Scan(&createStatus)
			if createStatus == "failed" {
				return finish("failed", "contact creation failed; reconcile before retry", "", "")
			}
			if createStatus == "" || createStatus == "skipped" || createStatus == "sent" {
				return finish("skipped", "contact not in Rex", "", "")
			}
			return w.deferJob(j, "waiting for contact link")
		}
		if linked != "" && source != "rex_import" && j.entity != "contacts" && j.entity != "rex_contact_relation" {
			return finish("skipped", "not linked to Rex sandbox record", "", "")
		}
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
					return finish("skipped", "not in Rex", "", "")
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
	case "rex_property_note":
		property := w.link("properties", str(m, "propertyId"))
		if property == "" {
			return finish("skipped", "not in Rex", "", "")
		}
		if str(m, "t") == "" {
			return finish("skipped", "empty note", "", "")
		}
		entityLink = w.link("rex_property_note", j.localID) // already created: never a second note
		service = "Notes"
		actor = w.userName(str(m, "actorId"))
		if actor == "" {
			actor = str(m, "a")
		}
		data = map[string]any{"note": propertyNoteText(m, actor, time.Now()), "note_type": rexRef("note"), "related": map[string]any{"note_properties": []any{map[string]any{"property": rexRef(property)}}}}
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
			if entityLink == "" {
				var otherAccount int
				_ = w.db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='contacts' AND local_id=?`, j.localID).Scan(&otherAccount)
				if otherAccount > 0 {
					return finish("skipped", "linked to another Rex account", "", "")
				}
				if j.op != "POST" {
					return finish("skipped", "not linked to Rex sandbox record", "", "")
				}
				// Only contacts owned by a user with access to this write account.
				var permitted int
				_ = w.db.QueryRow(`SELECT count(*) FROM users WHERE id=? AND disabled=0 AND deleted_at IS NULL`, owner).Scan(&permitted)
				if permitted == 0 {
					return finish("skipped", "contact owner not in write account", "", "")
				}
				if found := w.findLocalDuplicate(j.localID, str(c, "mobile")); found != "" {
					return finish("sent", "", found, "Contacts")
				}
				related := map[string]any{"contact_names": []any{map[string]any{"name_first": str(c, "first"), "name_last": str(c, "last")}}}
				if phone := str(c, "mobile"); phone != "" {
					related["contact_phones"] = []any{map[string]any{"phone_number": phone, "phone_type": "mobile", "phone_primary": true}}
				}
				if email := str(c, "email"); email != "" {
					related["contact_emails"] = []any{map[string]any{"email_address": email, "email_desc": "home", "email_primary": true}}
				}
				if str(c, "first") == "" && str(c, "last") == "" {
					return finish("failed", "contact name required", "", "")
				}
				data = map[string]any{"related": related}
				break
			}
			method = "update"
			// Read the current sub-row identifiers before updating, never append duplicates.
			if w.client.Token == "" {
				if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
					return w.retry(j, "Rex login failed", 0)
				}
			}
			w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", j.localID, entityLink
			_, current, readErr := w.client.Call(ctx, "Contacts", "read", map[string]any{"id": entityLink})
			if readErr != nil {
				return w.retry(j, "Rex contact read failed", 0)
			}
			var currentRow map[string]any
			if json.Unmarshal(current.Result, &currentRow) != nil {
				return w.retry(j, "invalid Rex contact read", 0)
			}
			c["rex"] = currentRow
			data = map[string]any{"id": entityLink}
			rex := map[string]any{}
			if r, ok := c["rex"].(map[string]any); ok {
				rex = r
			}
			subID := func(key string) any {
				rows, _ := imap(rex["related"])[key].([]any)
				if len(rows) > 0 {
					return imap(rows[0])["id"]
				}
				return nil
			}
			name := map[string]any{"name_first": str(c, "first"), "name_last": str(c, "last")}
			if id := subID("contact_names"); id != nil {
				name["id"] = id
			} else {
				return finish("failed", "contact name subrow id missing; re-import detailed Rex record", "", "")
			}
			related := map[string]any{"contact_names": []any{name}}
			if s := str(c, "mobile"); s != "" {
				phone := map[string]any{"phone_number": s, "phone_type": "mobile", "phone_primary": true}
				if id := subID("contact_phones"); id != nil {
					phone["id"] = id
				} else {
					return finish("failed", "contact phone subrow id missing; re-import detailed Rex record", "", "")
				}
				related["contact_phones"] = []any{phone}
			}
			if s := str(c, "email"); s != "" {
				email := map[string]any{"email_address": s, "email_desc": "home", "email_primary": true}
				if id := subID("contact_emails"); id != nil {
					email["id"] = id
				} else {
					return finish("failed", "contact email subrow id missing; re-import detailed Rex record", "", "")
				}
				related["contact_emails"] = []any{email}
			}
			data["related"] = related
			for _, field := range []string{"letter", "email", "phone", "sms"} {
				if value, ok := imap(c["consent"])[field]; ok {
					data["contact_by_"+field] = value
				}
			}
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
		// Contacts::update only accepts data/fields/extra_fields (verified against sandbox 2057).
		body = map[string]any{"data": data}
		if method != "update" {
			body["return_id"] = true
		}
	}
	w.client.Entity, w.client.LocalID, w.client.RexID = "contacts", linkID, linked
	if j.entity == "rex_property_note" {
		w.client.Entity, w.client.LocalID, w.client.RexID = j.entity, j.localID, ""
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
		var resultRow map[string]any
		_ = json.Unmarshal(result, &resultRow)
		if entityLink != "" && str(resultRow, "etag") == "" {
			_, read, readErr := w.client.Call(ctx, service, "read", map[string]any{"id": entityLink})
			if readErr == nil {
				_ = json.Unmarshal(read.Result, &resultRow)
			}
		}
		if entityLink != "" && str(resultRow, "etag") != "" {
			_, _ = w.db.Exec(`UPDATE rex_links SET etag=? WHERE entity=? AND local_id=?`, str(resultRow, "etag"), j.entity, j.localID)
		}
		_, _ = w.db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "app_to_rex", j.entity, j.localID, entityLink, ijson(map[string]any{"etag": map[string]any{"old": nil, "new": str(resultRow, "etag")}}), "ok")
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

// Search linked contacts locally by canonical phone before any create request.
func (w *rexWorker) findLocalDuplicate(id, mobile string) string {
	norm := normAUMobile(mobile)
	if len(norm) < 10 {
		return ""
	}
	rows, err := w.db.Query(`SELECT l.rex_id,c.mobile FROM rex_links l JOIN contacts c ON c.id=l.local_id WHERE l.entity='contacts' AND l.account_id=? AND l.local_id<>? AND c.deleted_at IS NULL`, w.account, id)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var rex, phone string
		if rows.Scan(&rex, &phone) == nil && normAUMobile(phone) == norm {
			return rex
		}
	}
	return ""
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
	if e == nil && status == "failed" && j.entity == "contacts" {
		_, e = w.db.Exec(`UPDATE contacts SET updated_at=?,version=version+1 WHERE id=?`, now(), j.localID)
	}
	return e
}
