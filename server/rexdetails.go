package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// enqueueRexDetails stores immutable operations alongside the local write. A later
// edit cannot silently change what an already queued Rex operation will do.
func enqueueRexDetails(tx *sql.Tx, kind, id string, diff any, stamp string) error {
	current, _ := diff.(map[string]any)
	if current == nil || (kind == "contacts" && str(current, "contactId") != "") {
		return nil
	}
	if kind == "contacts" {
		if err := enqueueMailingLists(tx, id, current, stamp); err != nil {
			return err
		}
	}
	var raw string
	_ = tx.QueryRow(`SELECT payload FROM rex_outbox WHERE entity=? AND entity_id=? AND op IN ('POST','PATCH') ORDER BY id DESC LIMIT 1 OFFSET 1`, kind, id).Scan(&raw)
	var prior map[string]any
	_ = json.Unmarshal([]byte(raw), &prior)
	enqueue := func(entity, key string, payload map[string]any) error {
		b, _ := json.Marshal(payload)
		_, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?,'POST',?,?)`, entity, key, string(b), stamp)
		return err
	}
	if kind == "todos" {
		if current["done"] == true && (prior == nil || prior["done"] != true) {
			if err := enqueue("rex_todo_completion", id, map[string]any{"todoId": id, "done": true}); err != nil {
				return err
			}
			// TODO(sandbox): completionNote is an optional sync field; the current mobile Todo has no completion-note UI.
			// A completion comment is a contact note, not a Rex activity entry.
			if note := str(current, "completionNote"); note != "" {
				if err := enqueue("rex_completion_note", id, map[string]any{"todoId": id, "contactId": str(current, "cid"), "note": note, "actorId": str(current, "createdBy")}); err != nil {
					return err
				}
			}
		}
		return nil
	}
	oldTags := map[string]bool{}
	for _, tag := range stringsFrom(prior["tags"]) {
		oldTags[tag] = true
	}
	seen := map[string]bool{}
	for _, tag := range stringsFrom(current["tags"]) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] || oldTags[tag] {
			continue
		}
		seen[tag] = true
		if err := enqueue("rex_contact_tag", id+":"+tag, map[string]any{"contactId": id, "tag": tag, "group": rexTagGroup(current, tag)}); err != nil {
			return err
		}
	}
	oldRels := map[string]bool{}
	oldRelations, _ := prior["rels"].([]any)
	for _, v := range sliceMaps(oldRelations) {
		b, _ := json.Marshal(v)
		oldRels[string(b)] = true
	}
	relations, _ := current["rels"].([]any)
	for i, v := range sliceMaps(relations) {
		b, _ := json.Marshal(v)
		if oldRels[string(b)] {
			continue
		}
		if err := enqueue("rex_contact_relation", fmt.Sprintf("%s:%d:%s", id, i, string(b)), map[string]any{"contactId": id, "relation": v}); err != nil {
			return err
		}
	}
	flags, _ := current["flags"].(map[string]any)
	prevFlags, _ := prior["flags"].(map[string]any)
	if flags != nil && flags["dnc"] != prevFlags["dnc"] {
		if dnd, ok := flags["dnc"].(bool); ok {
			if err := enqueue("rex_contact_dnd", id, map[string]any{"contactId": id, "value": dnd}); err != nil {
				return err
			}
		}
	}
	consent, _ := current["consent"].(map[string]any)
	prevConsent, _ := prior["consent"].(map[string]any)
	letter := func(v map[string]any) any {
		if v == nil {
			return nil
		}
		return v["letter"]
	}
	// Clearing DND restores all channels in Rex; reapply the letter opt-out afterwards.
	clearingDND := flags != nil && flags["dnc"] == false && prevFlags != nil && prevFlags["dnc"] == true
	if letter(consent) != nil && (letter(consent) != letter(prevConsent) || clearingDND) {
		if err := enqueue("rex_contact_letter", id, map[string]any{"contactId": id, "value": letter(consent)}); err != nil {
			return err
		}
	} else if seen["No junk mail"] && letter(consent) == nil {
		if err := enqueue("rex_contact_letter", id, map[string]any{"contactId": id, "value": 0}); err != nil {
			return err
		}
	}
	return nil
}

func rexTagGroup(m map[string]any, tag string) string {
	// TODO(sandbox): mobile tags have no explicit group in the contact payload;
	// source and known tag names are a best-effort grouping until the sync contract carries it.
	// The mobile tags are ungrouped strings. Capture source takes precedence;
	// otherwise the visit/intent groups correspond to DK and enquiry respectively.
	switch strings.ToLower(str(m, "source")) {
	case "dk", "ofi", "enquiry":
		return strings.ToUpper(str(m, "source"))
	}
	switch tag {
	case "Not at home", "Left card", "Friendly", "Neighbour", "Do not contact", "No junk mail":
		return "DK"
	case "Hot", "Warm", "Cold", "Seller lead", "Buyer lead":
		return "Enquiry"
	}
	return ""
}
