package main

import (
	"database/sql"
)

// Contact notes live inside contacts.data.notes (the phone replaces the whole data
// object on PATCH), like property notes (rexproperty_notes.go). Each note added since
// the previous version becomes one Rex Notes::create job on the Rex contact, and on
// the Rex property when the note names a linked one (propId). Edits and deletions are
// not synced. The legacy POST /contacts/{id}/notes endpoint queues the same job.

// contactNoteToRex: app activity notes are not written to Rex. The phone marks them
// with "sys" (to-do, market monitor, preference and field-edit logs); author
// "AxleoCollect" covers app versions from before the marker.
func contactNoteToRex(n map[string]any) bool {
	return str(n, "sys") == "" && str(n, "a") != "AxleoCollect"
}

func enqueueContactNotes(tx *sql.Tx, contactID string, diff any, prior map[string]any, actorID, stamp string) error {
	m, ok := diff.(map[string]any)
	if !ok {
		return nil
	}
	old := map[string]bool{}
	for _, n := range propertyNotes(prior["notes"]) {
		old[propertyNoteKey(n)] = true
	}
	for _, n := range propertyNotes(m["notes"]) {
		key := propertyNoteKey(n)
		if old[key] || !contactNoteToRex(n) {
			continue
		}
		old[key] = true // the same note twice in one body is one job
		entityID := contactID + ":" + key
		var queued int
		if err := tx.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='rex_contact_note' AND entity_id=?`, entityID).Scan(&queued); err != nil {
			return err
		}
		if queued > 0 { // removed and restored, or replayed: never a second Rex note
			continue
		}
		payload := ijson(map[string]any{"contactId": contactID, "noteKey": key, "t": str(n, "t"), "a": str(n, "a"), "d": str(n, "d"), "at": str(n, "at"), "propertyId": str(n, "propId"), "actorId": actorID})
		if _, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_contact_note',?,'POST',?,?)`, entityID, payload, stamp); err != nil {
			return err
		}
	}
	return nil
}

// moveContactNoteJobs re-keys the secondary's note jobs and links to the primary
// during a merge: queued notes go to the primary's Rex record, statuses follow the
// notes. A queued secondary note whose key the primary already has is skipped.
func moveContactNoteJobs(tx *sql.Tx, primary, secondary string) error {
	from, to := secondary+":", secondary+";" // entity_id is "<contactId>:<noteKey>"
	cut := len(secondary) + 1
	if _, err := tx.Exec(`UPDATE rex_outbox SET status='skipped',last_error='same note already on merged contact',next_attempt_at=NULL WHERE entity='rex_contact_note' AND entity_id>? AND entity_id<? AND status IN ('pending','waiting_auth') AND ?||substr(entity_id,?) IN (SELECT entity_id FROM rex_outbox WHERE entity='rex_contact_note' AND entity_id>? AND entity_id<?)`, from, to, primary, cut, primary+":", primary+";"); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE rex_outbox SET entity_id=?||substr(entity_id,?),payload=json_set(payload,'$.contactId',?,'$.mergedFrom',?) WHERE entity='rex_contact_note' AND entity_id>? AND entity_id<?`, primary, cut, primary, secondary, from, to); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE OR IGNORE rex_links SET local_id=?||substr(local_id,?) WHERE entity='rex_contact_note' AND local_id>? AND local_id<?`, primary, cut, from, to)
	return err
}

// attachContactNoteSync gives the phone a per-note Rex status keyed by note id.
func attachContactNoteSync(db querier, contactID string, item map[string]any) {
	attachNoteSync(db, "rex_contact_note", "contactId", contactID, item)
}
