package main

import "testing"

func TestPropertyOwnerOutboxOnlyNewLinks(t *testing.T) {
	a := workerDB(t)
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = enqueuePropertyOwners(tx, "p", map[string]any{"ownerIds": []string{"existing", "new"}}, map[string]any{"ownerIds": []string{"existing"}}, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='rex_contact_relation' AND json_extract(payload,'$.contactId')='new'`).Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='rex_contact_relation' AND json_extract(payload,'$.contactId')='existing'`).Scan(&n)
	if n != 0 {
		t.Fatal(n)
	}
}
