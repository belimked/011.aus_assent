package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"time"
)

// Property notes live inside properties.data.notes (the phone replaces the whole
// data object on PATCH). Each note added since the previous version becomes one
// Rex Notes::create job linked to the Rex property. Edits and deletions are not synced.

// propertyNoteKey is the stable identity of a note: the app's id, or for older
// notes saved before ids existed, a hash of when/who/what.
func propertyNoteKey(n map[string]any) string {
	if id := str(n, "id"); id != "" && len(id) <= 64 {
		return id
	}
	when := str(n, "at")
	if when == "" {
		when = str(n, "d")
	}
	sum := sha1.Sum([]byte(when + "\x1f" + str(n, "a") + "\x1f" + str(n, "t")))
	return "h" + hex.EncodeToString(sum[:])
}

func propertyNotes(v any) []map[string]any {
	list, _ := v.([]any)
	out := []map[string]any{}
	for _, item := range list {
		if n, ok := item.(map[string]any); ok && str(n, "t") != "" {
			out = append(out, n)
		}
	}
	return out
}

// copyPropertyNotes snapshots the notes before mutate overlays the request body.
func copyPropertyNotes(v any) []any {
	list, _ := v.([]any)
	return append([]any(nil), list...)
}

func enqueuePropertyNotes(tx *sql.Tx, propertyID string, diff any, prior map[string]any, actorID, stamp string) error {
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
		if old[key] {
			continue
		}
		old[key] = true // the same note twice in one body is one job
		entityID := propertyID + ":" + key
		var queued int
		if err := tx.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='rex_property_note' AND entity_id=?`, entityID).Scan(&queued); err != nil {
			return err
		}
		if queued > 0 { // removed and restored, or replayed: never a second Rex note
			continue
		}
		payload := ijson(map[string]any{"propertyId": propertyID, "noteKey": key, "t": str(n, "t"), "a": str(n, "a"), "d": str(n, "d"), "at": str(n, "at"), "actorId": actorID})
		if _, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_property_note',?,'POST',?,?)`, entityID, payload, stamp); err != nil {
			return err
		}
	}
	return nil
}

// propertyNoteText is the Rex note body. Rex stamps system_ctime itself and it
// cannot be written, so a note dated before the send day carries its date in the text.
func propertyNoteText(m map[string]any, author string, today time.Time) string {
	t := str(m, "t")
	if d := str(m, "d"); d != "" && d != today.In(brisbane).Format("2006-01-02") {
		t = "[" + d + "] " + t
	}
	return t + rexSignature(author)
}

// attachPropertyNoteSync gives the phone a per-note Rex status keyed by note id.
func attachPropertyNoteSync(db *sql.DB, propertyID string, item map[string]any) {
	attachNoteSync(db, "rex_property_note", "propertyId", propertyID, item)
}

// attachNoteSync: entity is rex_property_note or rex_contact_note, owner the payload key.
func attachNoteSync(db *sql.DB, entity, owner, ownerID string, item map[string]any) {
	// entity_id is "<ownerId>:<noteKey>"; ';' sorts right after ':'.
	rows, err := db.Query(`SELECT json_extract(payload,'$.noteKey'),status,COALESCE(last_error,''),created_at FROM rex_outbox WHERE entity=? AND entity_id>? AND entity_id<? AND json_extract(payload,'$.'||?)=? ORDER BY id`, entity, ownerID+":", ownerID+";", owner, ownerID)
	if err != nil {
		return
	}
	out := map[string]any{}
	for rows.Next() {
		var key sql.NullString
		var status, reason, at string
		if rows.Scan(&key, &status, &reason, &at) == nil && key.String != "" {
			out[key.String] = map[string]any{"status": status, "reason": reason, "at": at}
		}
	}
	rows.Close()
	if len(out) > 0 {
		item["noteRexSync"] = out
	} else {
		delete(item, "noteRexSync")
	}
}
