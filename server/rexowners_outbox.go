package main

import (
	"database/sql"
)

// The relation job follows the contact create job (outbox id order).
func enqueuePropertyOwners(tx *sql.Tx, id string, diff any, prior map[string]any, stamp string) error {
	m, ok := diff.(map[string]any)
	if !ok {
		return nil
	}
	owners, ok := m["ownerIds"]
	if !ok {
		return nil
	}
	old := map[string]bool{}
	for _, cid := range stringsFrom(prior["ownerIds"]) {
		old[cid] = true
	}
	for _, cid := range stringsFrom(owners) {
		if old[cid] {
			continue
		}
		payload := ijson(map[string]any{"contactId": cid, "relation": map[string]any{"type": "Owner of", "propertyId": id}})
		if _, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_contact_relation',?,'POST',?,?)`, cid+":"+id, payload, stamp); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('properties',?,'PATCH',?,?)`, id, ijson(m), stamp)
	return err
}
