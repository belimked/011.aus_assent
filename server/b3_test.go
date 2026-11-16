package main

import "testing"

func TestB3IsolationAndApproval(t *testing.T) {
	a, tokens := setup(t)
	mary, john, admin := tokens["mary"], tokens["john"], tokens["admi"]
	for _, kind := range []string{"captures", "enquiries", "leads", "listings", "ofi_attendees", "sales", "contracts", "referrals", "market_items", "zones", "tags"} {
		id := "m-" + kind
		if w := request(a, "POST", "/api/v1/"+kind, mary, map[string]any{"id": id}); w.Code != 201 {
			t.Fatalf("%s create %d %s", kind, w.Code, w.Body)
		}
		if kind == "listings" {
			if w := request(a, "GET", "/api/v1/"+kind+"/"+id, john, nil); w.Code != 200 {
				t.Fatalf("listing read: %d", w.Code)
			}
		} else {
			if w := request(a, "GET", "/api/v1/"+kind+"/"+id, john, nil); w.Code != 404 {
				t.Fatalf("%s leaked: %d", kind, w.Code)
			}
		}
		if w := request(a, "PATCH", "/api/v1/"+kind+"/"+id, john, map[string]any{"version": 1}); w.Code != 404 {
			t.Fatalf("%s patch %d", kind, w.Code)
		}
		if w := request(a, "DELETE", "/api/v1/"+kind+"/"+id, john, nil); w.Code != 404 {
			t.Fatalf("%s delete %d", kind, w.Code)
		}
	}
	full := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))
	cursor := full["cursor"].(string)
	for _, kind := range []string{"captures", "enquiries", "leads", "listings", "ofi_attendees", "sales", "contracts", "referrals", "market_items", "zones", "tags"} {
		if len(full[kind].([]any)) != 1 {
			t.Fatalf("%s missing full sync", kind)
		}
	}
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "one", "first": "First"})
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "two", "mobile": "123", "tags": []string{"warm"}})
	var target string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&target)
	req := request(a, "POST", "/api/v1/approvals", mary, map[string]any{"kind": "reassign", "entity": "contacts", "id": "one", "targetId": target})
	if req.Code != 201 {
		t.Fatal(req.Body)
	}
	id := obj(t, req)["id"].(string)
	if w := request(a, "POST", "/api/v1/approvals/"+id+"/decide", mary, map[string]any{"decision": "approve"}); w.Code != 404 {
		t.Fatal("agent decided")
	}
	if w := request(a, "POST", "/api/v1/approvals/"+id+"/decide", admin, map[string]any{"decision": "approve"}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil))
	if len(inc["deleted"].(map[string]any)["contacts"].([]any)) != 1 || len(inc["approvals"].([]any)) != 1 {
		t.Fatal(inc)
	}
	if request(a, "GET", "/api/v1/contacts/one", john, nil).Code != 200 {
		t.Fatal("not transferred")
	}
	merge := request(a, "POST", "/api/v1/approvals", mary, map[string]any{"kind": "merge", "primaryId": "two", "secondaryId": "one"})
	if merge.Code != 404 {
		t.Fatal("merge of invisible record accepted")
	}
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "three", "first": "Third"})
	merge = request(a, "POST", "/api/v1/approvals", mary, map[string]any{"kind": "merge", "primaryId": "three", "secondaryId": "two"})
	if merge.Code != 201 {
		t.Fatal(merge.Body)
	}
	mid := obj(t, merge)["id"].(string)
	if w := request(a, "POST", "/api/v1/approvals/"+mid+"/decide", admin, map[string]any{"decision": "approve"}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if request(a, "GET", "/api/v1/contacts/two", mary, nil).Code != 404 || obj(t, request(a, "GET", "/api/v1/contacts/three", mary, nil))["mobile"] != "123" {
		t.Fatal("merge failed")
	}
	reject := request(a, "POST", "/api/v1/approvals", john, map[string]any{"kind": "reassign", "entity": "contacts", "id": "one", "targetId": target})
	rid := obj(t, reject)["id"].(string)
	_ = request(a, "POST", "/api/v1/approvals/"+rid+"/decide", admin, map[string]any{"decision": "reject"})
	if request(a, "GET", "/api/v1/contacts/one", john, nil).Code != 200 {
		t.Fatal("reject changed owner")
	}
}
