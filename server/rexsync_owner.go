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
		return rexLocalUser(db, imap(rawOwner(raw)), os.Getenv("REX_DEFAULT_OWNER_EMAIL"))
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
// rexLocalUser maps a Rex user reference to an active app user: REX_USER_MAP
// (Rex id or email:local email), then the same email or users.rex_user_id,
// then fallback (an email; empty for no fallback). Returns "" when unmatched.
func rexLocalUser(db *sql.DB, user importRow, fallback string) string {
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
	candidates = append(candidates, fallback)
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
func rawOwner(raw string) any {
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	return imap(m["rex"])["system_owner_user"]
}
