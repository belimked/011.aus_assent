package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
)

// assignPollOwners only touches newly imported rows. Unmapped records retain the
// sandbox allocator as a last resort; linked contacts inherit the property agent.
func assignPollOwners(db *sql.DB, props, contacts map[string]bool) error {
	resolve := func(raw string) string {
		user := imap(rawOwner(raw))
		candidates := []string{iv(user, "id"), iv(user, "email"), iv(user, "email_address")}
		for _, pair := range strings.Split(os.Getenv("REX_USER_MAP"), ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) != 2 {
				continue
			}
			for _, key := range candidates {
				if key != "" && strings.EqualFold(key, strings.TrimSpace(parts[0])) {
					candidates = append([]string{strings.TrimSpace(parts[1])}, candidates...)
					break
				}
			}
		}
		candidates = append(candidates, os.Getenv("REX_DEFAULT_OWNER_EMAIL"))
		for _, key := range candidates {
			if key == "" {
				continue
			}
			var id string
			if db.QueryRow(`SELECT id FROM users WHERE (lower(email)=lower(?) OR rex_user_id=?) AND disabled=0 AND deleted_at IS NULL LIMIT 1`, key, key).Scan(&id) == nil {
				return id
			}
		}
		return ""
	}
	for id := range props {
		var raw string
		if err := db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
			return err
		}
		if owner := resolve(raw); owner != "" {
			if _, err := db.Exec(`UPDATE properties SET owner_id=? WHERE id=?`, owner, id); err != nil {
				return err
			}
			delete(props, id)
		}
	}
	for id := range contacts {
		var owner, raw string
		_ = db.QueryRow(`SELECT p.owner_id FROM property_owners o JOIN properties p ON p.id=o.property_id WHERE o.contact_id=? AND p.owner_id<>'' ORDER BY p.id LIMIT 1`, id).Scan(&owner)
		if owner == "" {
			if err := db.QueryRow(`SELECT data FROM contacts WHERE id=?`, id).Scan(&raw); err != nil {
				return err
			}
			owner = resolve(raw)
		}
		if owner != "" {
			if _, err := db.Exec(`UPDATE contacts SET owner_id=? WHERE id=?`, owner, id); err != nil {
				return err
			}
			delete(contacts, id)
		}
	}
	return nil
}
func rawOwner(raw string) any {
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	return imap(m["rex"])["system_owner_user"]
}
