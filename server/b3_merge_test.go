package main

import "testing"

func TestB3MergeReferencesAndTodoReassignTombstone(t *testing.T) {
	a, tokens := setup(t)
	mary, admin := tokens["mary"], tokens["admi"]
	for _, id := range []string{"main", "other"} {
		_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": id, "first": id})
	}
	_ = request(a, "POST", "/api/v1/properties", mary, map[string]any{"id": "prop", "ownerIds": []string{"other"}})
	_ = request(a, "POST", "/api/v1/todos", mary, map[string]any{"id": "task", "cid": "other", "t": "Call"})
	for _, kind := range []string{"leads", "referrals", "ofi_attendees"} {
		_ = request(a, "POST", "/api/v1/"+kind, mary, map[string]any{"id": "link-" + kind, "contactId": "other"})
	}
	merge := request(a, "POST", "/api/v1/approvals", mary, map[string]any{"kind": "merge", "primaryId": "main", "secondaryId": "other"})
	if merge.Code != 201 {
		t.Fatal(merge.Body)
	}
	id := obj(t, merge)["id"].(string)
	if w := request(a, "POST", "/api/v1/approvals/"+id+"/decide", admin, map[string]any{"decision": "approve"}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	for _, kind := range []string{"leads", "referrals", "ofi_attendees"} {
		var ref string
		_ = a.db.QueryRow(`SELECT contact_id FROM `+kind+` WHERE id=?`, "link-"+kind).Scan(&ref)
		if ref != "main" {
			t.Fatalf("%s reference %s", kind, ref)
		}
	}
	property := obj(t, request(a, "GET", "/api/v1/properties/prop", mary, nil))
	ids := property["ownerIds"].([]any)
	if len(ids) != 1 || ids[0] != "main" {
		t.Fatal(ids)
	}
	var target string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&target)
	cursor := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))["cursor"].(string)
	req := request(a, "POST", "/api/v1/approvals", mary, map[string]any{"kind": "reassign", "entity": "todos", "id": "task", "targetId": target})
	if req.Code != 201 {
		t.Fatal(req.Body)
	}
	id = obj(t, req)["id"].(string)
	if w := request(a, "POST", "/api/v1/approvals/"+id+"/decide", admin, map[string]any{"decision": "approve"}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil))
	if len(inc["deleted"].(map[string]any)["todos"].([]any)) != 1 {
		t.Fatal(inc)
	}
}
