package main

import (
	"sort"
	"strings"
	"testing"
)

// mergeRex is a stateful Rex mock that reproduces what sandbox 2057 showed for
// Dedupe::combineRecords (2026-11-26):
//   - the loser is trashed synchronously;
//   - the winner's name becomes "<winner name> & <loser name>";
//   - every phone, email and tag of the loser is appended to the winner (loser
//     phones lose phone_primary), nothing is de-duplicated;
//   - the loser's notes move to the winner;
//   - on a property both are related to, only the winner's row survives (same id)
//     but it takes the loser's relation type; the loser's row disappears.
//     Rows on other properties move to the winner with the same id (not measured).
type mergeRex struct {
	t        *testing.T
	contacts map[string]map[string][]map[string]any // rex id -> sub table -> rows
	state    map[string]string
	props    map[string][]map[string]any // rex property id -> contact_reln_property rows
	notes    map[string]string           // note id -> contact
	bodies   map[string][]map[string]any
	next     int
	etag     int
	// rejectTagDestroy answers contact_tags _destroy with an error envelope;
	// typeUpdate "" applies an in-place reln_type change, "ignore" drops it
	// silently, "reject" answers with an error envelope.
	rejectTagDestroy bool
	typeUpdate       string
}

func newMergeRex(t *testing.T) *mergeRex {
	return &mergeRex{t: t, contacts: map[string]map[string][]map[string]any{}, state: map[string]string{}, props: map[string][]map[string]any{}, notes: map[string]string{}, bodies: map[string][]map[string]any{}, next: 9000}
}

func (f *mergeRex) id() float64 { f.next++; return float64(f.next) }

func (f *mergeRex) contact(id string) map[string][]map[string]any {
	c, ok := f.contacts[id]
	if !ok {
		c = map[string][]map[string]any{"contact_names": {{"id": f.id(), "name_first": "Rex", "name_last": id}}}
		f.contacts[id] = c
		f.state[id] = "active"
	}
	return c
}

// addContact seeds a Rex contact: phones/emails are "value" or "value*" (primary).
func (f *mergeRex) addContact(id, first, last string, phones, emails, tags []string) {
	c := map[string][]map[string]any{"contact_names": {{"id": f.id(), "name_first": first, "name_last": last}}}
	for _, p := range phones {
		c["contact_phones"] = append(c["contact_phones"], map[string]any{"id": f.id(), "phone_number": strings.TrimSuffix(p, "*"), "phone_type": "mobile", "phone_primary": strings.HasSuffix(p, "*")})
	}
	for _, e := range emails {
		c["contact_emails"] = append(c["contact_emails"], map[string]any{"id": f.id(), "email_address": strings.TrimSuffix(e, "*"), "email_desc": "home", "email_primary": strings.HasSuffix(e, "*")})
	}
	for _, tag := range tags {
		c["contact_tags"] = append(c["contact_tags"], map[string]any{"id": f.id(), "tag": tag})
	}
	f.contacts[id], f.state[id] = c, "active"
}

func (f *mergeRex) addRelation(property, row, contact, kind string) {
	f.props[property] = append(f.props[property], map[string]any{"id": row, "contact": map[string]any{"id": contact}, "reln_type": map[string]any{"id": kind}})
}

func (f *mergeRex) rows(property string) map[string]string { // row id -> "contact:kind"
	out := map[string]string{}
	for _, r := range f.props[property] {
		out[iv(r, "id")] = iv(imap(r["contact"]), "id") + ":" + iv(imap(r["reln_type"]), "id")
	}
	return out
}

func mergeRexName(c map[string][]map[string]any) string {
	n := c["contact_names"][0]
	return strings.TrimSpace(str(n, "name_first") + " " + str(n, "name_last"))
}

func copyRow(r map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range r {
		out[k] = v
	}
	return out
}

// apply runs Rex sub-record operations: {id,_destroy:true} removes, {id,...}
// updates the fields given, a row without id is appended.
func (f *mergeRex) apply(rows []map[string]any, ops []any) []map[string]any {
	for _, v := range ops {
		op := imap(v)
		id := iv(op, "id")
		if id == "" {
			row := copyRow(op)
			row["id"] = f.id()
			rows = append(rows, row)
			continue
		}
		for i, r := range rows {
			if iv(r, "id") != id {
				continue
			}
			if op["_destroy"] == true {
				rows = append(rows[:i:i], rows[i+1:]...)
				break
			}
			for k, x := range op {
				if k != "id" {
					r[k] = x
				}
			}
			break
		}
	}
	return rows
}

func (f *mergeRex) handle(path string, b map[string]any) (any, int) {
	f.bodies[path] = append(f.bodies[path], b)
	f.etag++
	switch path {
	case "/v1/rex/dedupe/combine-records":
		win := iv(b, "winning_id")
		w := f.contact(win)
		for _, v := range b["losing_ids"].([]any) {
			lose := istring(v)
			l := f.contact(lose)
			w["contact_names"][0]["name_first"] = mergeRexName(w) + " & " + mergeRexName(l)
			w["contact_names"][0]["name_last"] = ""
			for _, key := range []string{"contact_phones", "contact_emails", "contact_tags"} {
				for _, r := range l[key] {
					row := copyRow(r)
					row["id"] = f.id()
					if key == "contact_phones" {
						row["phone_primary"] = nil
					}
					if key == "contact_emails" {
						row["email_primary"] = nil
					}
					w[key] = append(w[key], row)
				}
			}
			f.state[lose] = "trashed"
			for n, c := range f.notes {
				if c == lose {
					f.notes[n] = win
				}
			}
			for pid, rows := range f.props {
				var winRow map[string]any
				for _, r := range rows {
					if iv(imap(r["contact"]), "id") == win {
						winRow = r
					}
				}
				kept := []map[string]any{}
				for _, r := range rows {
					if iv(imap(r["contact"]), "id") != lose {
						kept = append(kept, r)
						continue
					}
					if winRow != nil {
						winRow["reln_type"] = r["reln_type"] // the loser's type wins, the winner's id stays
						continue
					}
					r["contact"] = map[string]any{"id": win}
					kept = append(kept, r)
				}
				f.props[pid] = kept
			}
		}
		return []any{}, 200
	case "/v1/rex/contacts/read":
		id := iv(b, "id")
		c := f.contact(id)
		related := map[string]any{}
		for k, rows := range c {
			list := []any{}
			for _, r := range rows {
				list = append(list, copyRow(r))
			}
			related[k] = list
		}
		return map[string]any{"id": id, "etag": "e" + istring(float64(f.etag)), "system_record_state": f.state[id], "related": related}, 200
	case "/v1/rex/contacts/update":
		data := imap(b["data"])
		id := iv(data, "id")
		c := f.contact(id)
		for k, v := range imap(data["related"]) {
			ops, _ := v.([]any)
			if k == "contact_tags" && f.rejectTagDestroy {
				for _, op := range ops {
					if imap(op)["_destroy"] == true {
						return rexTestError("contact_tags cannot be removed"), 400
					}
				}
			}
			c[k] = f.apply(c[k], ops)
		}
		return map[string]any{"id": id, "etag": "u" + istring(float64(f.etag))}, 200
	case "/v1/rex/properties/read":
		id := iv(b, "id")
		list := []any{}
		for _, r := range f.props[id] {
			list = append(list, copyRow(r))
		}
		return map[string]any{"id": id, "related": map[string]any{"contact_reln_property": list}}, 200
	case "/v1/rex/properties/update":
		data := imap(b["data"])
		id := iv(data, "id")
		ops, _ := imap(data["related"])["contact_reln_property"].([]any)
		apply := []any{}
		for _, v := range ops {
			op := imap(v)
			if iv(op, "id") != "" && op["reln_type"] != nil {
				switch f.typeUpdate {
				case "reject":
					return rexTestError("reln_type cannot be changed"), 400
				case "ignore":
					continue
				}
			}
			apply = append(apply, v)
		}
		f.props[id] = f.apply(f.props[id], apply)
		return map[string]any{"id": id}, 200
	case "/v1/rex/notes/create":
		n := f.id()
		nc := imap(imap(imap(b["data"])["related"])["note_contacts"].([]any)[0])
		f.notes[istring(n)] = iv(imap(nc["contact"]), "id")
		return n, 200
	}
	f.t.Errorf("unexpected Rex call %s", path)
	return nil, 500
}

func mergeRexWorker(t *testing.T, a *app, f *mergeRex) *rexWorker {
	t.Helper()
	w, _ := sandboxWorker(t, a, f.handle)
	return w
}

// rexMergeFixSetup: a (Rex 91) and b (Rex 92), both in Rex. a owns p1 (newest),
// b was the tenant of p1 and owns p2. b's Rex record also holds a's mobile in
// international format and a's email in upper case, plus a shared tag.
func rexMergeFixSetup(t *testing.T) (*app, *mergeRex) {
	t.Helper()
	a := workerDB(t)
	mergeContactRow(t, a, "a", "u", t1, t1, map[string]any{"first": "Pat", "last": "A", "mobile": "0400000001", "email": "a@x.com", "source": "rex_import"})
	mergeContactRow(t, a, "b", "u", t2, t2, map[string]any{"first": "Pat", "last": "B", "mobile": "0400000002", "email": "b@x.com", "source": "rex_import"})
	for _, l := range [][3]string{{"contacts", "a", "91"}, {"contacts", "b", "92"}, {"properties", "p1", "501"}, {"properties", "p2", "502"}} {
		if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES(?,?,?,?,'2057',?)`, l[0], l[1], l[2], map[string]string{"contacts": "Contacts", "properties": "Properties"}[l[0]], now()); err != nil {
			t.Fatal(err)
		}
	}
	mergeProperty(t, a, "p1", "16", "Gager St")
	mergeProperty(t, a, "p2", "3", "Feltwell St")
	mergeOwner(t, a, "p1", "a", "owner", t3)
	mergeOwner(t, a, "p1", "b", "tenant", t1)
	mergeOwner(t, a, "p2", "b", "owner", t2)
	// b's note already in Rex: never created again after the merge moves it to a.
	if _, err := a.db.Exec(`INSERT INTO contact_notes(id,contact_id,d,a,t,created_at) VALUES('nb','b','2026-02-01','Mary','old note on b',?)`, t2); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contact_notes','nb','7000','Notes','2057',?)`, now()); err != nil {
		t.Fatal(err)
	}
	f := newMergeRex(t)
	f.addContact("91", "Pat", "A", []string{"0400 000 001*"}, []string{"a@x.com*"}, []string{"Buyer"})
	f.addContact("92", "Pat", "B", []string{"0400000002*", "+61 400 000 001"}, []string{"b@x.com*", "A@X.com"}, []string{"buyer", "Seller"})
	f.notes["7000"] = "92"
	f.addRelation("501", "17077130", "91", "owner")
	f.addRelation("501", "17077142", "92", "tenant")
	f.addRelation("501", "17016203", "55", "owner") // Teddie Bear: never touched
	f.addRelation("502", "805", "92", "owner")
	return a, f
}

func phoneSummary(rows []map[string]any) []string {
	out := []string{}
	for _, r := range rows {
		s := normAUMobile(iv(r, "phone_number"))
		if ibool(r["phone_primary"]) {
			s += "*"
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func emailSummary(rows []map[string]any) []string {
	out := []string{}
	for _, r := range rows {
		s := strings.ToLower(iv(r, "email_address"))
		if ibool(r["email_primary"]) {
			s += "*"
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func TestRexMergeCorrectsCombineResult(t *testing.T) {
	a, f := rexMergeFixSetup(t)
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" || plan.Rex != "both" {
		t.Fatal(plan.PrimaryID, plan.Rex)
	}
	local := contactData(t, a, "a")
	w := mergeRexWorker(t, a, f)
	drain(t, a, w, 10)
	for _, j := range outboxJobs(t, a) {
		if !strings.HasSuffix(j, "|sent") {
			t.Fatalf("job not sent: %v", outboxJobs(t, a))
		}
	}
	if c := f.bodies["/v1/rex/dedupe/combine-records"]; len(c) != 1 || f.state["92"] != "trashed" {
		t.Fatalf("combine %v state %s", c, f.state["92"])
	}
	// The worker overwrote the concatenated name with the local merge result.
	upd := f.bodies["/v1/rex/contacts/update"]
	names := imap(imap(upd[0]["data"])["related"])["contact_names"].([]any)
	if len(names) != 1 || iv(imap(names[0]), "id") == "" || str(imap(names[0]), "name_first") != str(local, "first") || str(imap(names[0]), "name_last") != str(local, "last") {
		t.Fatalf("name fix %v", names)
	}
	if got := mergeRexName(f.contacts["91"]); got != strings.TrimSpace(str(local, "first")+" "+str(local, "last")) || len(f.contacts["91"]["contact_names"]) != 1 {
		t.Fatalf("Rex name %q", got)
	}
	// Duplicate phone (B's +61 copy of A's number) destroyed; the local mobile is primary.
	phones := ijson(imap(imap(upd[0]["data"])["related"])["contact_phones"])
	if !strings.Contains(phones, `"_destroy":true`) {
		t.Fatalf("no duplicate phone destroyed: %s", phones)
	}
	want := []string{"0400000001", "0400000002"}
	for i, p := range want {
		if normAUMobile(str(local, "mobile")) == p {
			want[i] += "*"
		}
	}
	sort.Strings(want)
	if got := phoneSummary(f.contacts["91"]["contact_phones"]); ijson(got) != ijson(want) {
		t.Fatalf("Rex phones %v want %v", got, want)
	}
	wantEmails := []string{"a@x.com", "b@x.com"}
	for i, e := range wantEmails {
		if strings.ToLower(str(local, "email")) == e {
			wantEmails[i] += "*"
		}
	}
	sort.Strings(wantEmails)
	if got := emailSummary(f.contacts["91"]["contact_emails"]); ijson(got) != ijson(wantEmails) {
		t.Fatalf("Rex emails %v want %v", got, wantEmails)
	}
	// Tags: "Buyer"/"buyer" de-duplicated case-insensitively in a separate request.
	tags := []string{}
	for _, r := range f.contacts["91"]["contact_tags"] {
		tags = append(tags, iv(r, "tag"))
	}
	if ijson(tags) != `["Buyer","Seller"]` {
		t.Fatalf("Rex tags %v", tags)
	}
	// p1: combine left row 17077130 on A as tenant; the worker set it back to owner
	// in place. Teddie Bear's row is untouched; no request mentions it.
	if r := f.rows("501"); len(r) != 2 || r["17077130"] != "91:owner" || r["17016203"] != "55:owner" {
		t.Fatalf("p1 rows %v", r)
	}
	for _, b := range f.bodies["/v1/rex/properties/update"] {
		if s := ijson(b); strings.Contains(s, "17016203") || strings.Contains(s, `"55"`) {
			t.Fatalf("touched another contact's row: %s", s)
		}
	}
	p1 := f.bodies["/v1/rex/properties/update"]
	if len(p1) != 1 || ijson(p1[0]) != `{"data":{"id":"501","related":{"contact_reln_property":[{"id":"17077130","reln_type":{"id":"owner"}}]}}}` {
		t.Fatalf("property updates %v", p1)
	}
	// p2: B's row moved to A with the expected type: read only, no write.
	if r := f.rows("502"); r["805"] != "91:owner" {
		t.Fatalf("p2 rows %v", r)
	}
	// One Notes::create: the merge log on A. B's synced note moved with combine.
	notes := f.bodies["/v1/rex/notes/create"]
	if len(notes) != 1 || !strings.Contains(ijson(notes[0]), `"note_contacts":[{"contact":{"id":"91"}}]`) || !strings.Contains(ijson(notes[0]), "Merged with") {
		t.Fatalf("notes %v", notes)
	}
	if f.notes["7000"] != "91" {
		t.Fatal("b's note not on a")
	}
	// Running the follow-ups again changes nothing in Rex.
	before := len(f.bodies["/v1/rex/properties/update"])
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='pending' WHERE entity IN ('rex_merge_relation')`); err != nil {
		t.Fatal(err)
	}
	drain(t, a, w, 3)
	if len(f.bodies["/v1/rex/properties/update"]) != before {
		t.Fatal("re-run wrote relations again")
	}
}

// Rex ignores or rejects an in-place reln_type change: destroy the row and add
// the expected type instead.
func TestRexMergeRelationTypeFallback(t *testing.T) {
	for _, mode := range []string{"ignore", "reject"} {
		t.Run(mode, func(t *testing.T) {
			a, f := rexMergeFixSetup(t)
			f.typeUpdate = mode
			runMerge(t, a)
			drain(t, a, mergeRexWorker(t, a, f), 10)
			var status, reason string
			_ = a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE entity='rex_merge_relation' AND entity_id='a:p1'`).Scan(&status, &reason)
			if status != "sent" {
				t.Fatal(status, reason)
			}
			rows := f.rows("501")
			if _, ok := rows["17077130"]; ok || len(rows) != 2 || rows["17016203"] != "55:owner" {
				t.Fatalf("rows %v", rows)
			}
			for id, v := range rows {
				if id != "17016203" && v != "91:owner" {
					t.Fatalf("rows %v", rows)
				}
			}
			last := f.bodies["/v1/rex/properties/update"]
			if s := ijson(last[len(last)-1]); s != `{"data":{"id":"501","related":{"contact_reln_property":[{"_destroy":true,"id":"17077130"},{"contact":{"id":"91"},"reln_type":{"id":"owner"}}]}}}` {
				t.Fatalf("fallback %s", s)
			}
		})
	}
}

// A rejected tag removal is recorded and skipped; the name/phone fix stays.
func TestRexMergeTagRemovalRejected(t *testing.T) {
	a, f := rexMergeFixSetup(t)
	f.rejectTagDestroy = true
	runMerge(t, a)
	drain(t, a, mergeRexWorker(t, a, f), 10)
	var status, reason string
	_ = a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE entity='contacts' AND entity_id='a' AND json_extract(payload,'$.merge')=1`).Scan(&status, &reason)
	if status != "sent" || !strings.HasPrefix(reason, "duplicate tags left in Rex") {
		t.Fatal(status, reason)
	}
	var events int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_sync_events WHERE entity='contacts' AND local_id='a' AND result='skipped' AND error LIKE 'duplicate tags left%'`).Scan(&events)
	if events != 1 || len(f.contacts["91"]["contact_names"]) != 1 || strings.Contains(mergeRexName(f.contacts["91"]), "&") {
		t.Fatal(events, mergeRexName(f.contacts["91"]))
	}
}

// A contact with several name rows keeps one (updated) and destroys the others;
// phones not in the merge result are kept, only duplicates go.
func TestMergeRelatedRows(t *testing.T) {
	rex := map[string]any{"related": map[string]any{
		"contact_names":  []any{map[string]any{"id": float64(1), "name_first": "A & B"}, map[string]any{"id": float64(2), "name_first": "B"}},
		"contact_phones": []any{map[string]any{"id": float64(3), "phone_number": "0411 111 111", "phone_primary": true}, map[string]any{"id": float64(4), "phone_number": "0733334444"}, map[string]any{"id": float64(5), "phone_number": "61411111111"}},
		"contact_emails": []any{map[string]any{"id": float64(6), "email_address": "X@Y.com", "email_primary": true}},
	}}
	related, tags, fail := mergeRelated(map[string]any{"first": "Pat", "last": "Lee", "mobile": "0422 222 222", "email": "x@y.com"}, rex)
	if fail != "" || tags != nil {
		t.Fatal(fail, tags)
	}
	if s := ijson(related["contact_names"]); s != `[{"id":1,"name_first":"Pat","name_last":"Lee"},{"_destroy":true,"id":2}]` {
		t.Fatal(s)
	}
	// 0411 duplicate destroyed, 0411 kept but no longer primary, landline kept, new mobile added as primary.
	if s := ijson(related["contact_phones"]); s != `[{"_destroy":true,"id":5},{"id":3,"phone_number":"0411 111 111","phone_primary":false},{"phone_number":"0422 222 222","phone_primary":true,"phone_type":"mobile"}]` {
		t.Fatal(s)
	}
	if _, ok := related["contact_emails"]; ok { // already primary, same address ignoring case
		t.Fatal(related["contact_emails"])
	}
}

// A note of the secondary still queued at merge time follows contact_notes to
// the primary's Rex record, never the trashed loser; it is created once.
func TestRexMergeQueuedSecondaryNoteGoesToPrimary(t *testing.T) {
	a, f := rexMergeFixSetup(t)
	if _, err := a.db.Exec(`INSERT INTO contact_notes(id,contact_id,d,a,t,created_at) VALUES('nb2','b','2026-02-02','Mary','late note on b',?)`, t2); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','b','POST','{"id":"nb2","contactId":"b","t":"late note on b"}',?)`, now()); err != nil {
		t.Fatal(err)
	}
	runMerge(t, a)
	drain(t, a, mergeRexWorker(t, a, f), 12)
	late := 0
	for _, n := range f.bodies["/v1/rex/notes/create"] {
		if s := ijson(n); strings.Contains(s, "late note on b") {
			late++
			if !strings.Contains(s, `"note_contacts":[{"contact":{"id":"91"}}]`) {
				t.Fatalf("late note not on primary: %s", s)
			}
		}
	}
	if late != 1 || len(f.bodies["/v1/rex/notes/create"]) != 2 {
		t.Fatalf("notes %v", f.bodies["/v1/rex/notes/create"])
	}
}
