package main

// ownerRexSyncAll maps property id → contact id → the latest Rex status of that
// "Owner of" relation job, so the phone can show each owner's state (UAT 3 问题 3)
// instead of only the property's most recent job. Loaded once per pull: one
// query, fully read and closed before the per-item reads (single connection).
func ownerRexSyncAll(db querier) (map[string]map[string]any, error) {
	rows, err := db.Query(`SELECT COALESCE(json_extract(payload,'$.relation.propertyId'),''),COALESCE(json_extract(payload,'$.contactId'),''),status,COALESCE(last_error,''),created_at FROM rex_outbox WHERE entity='rex_contact_relation' AND json_extract(payload,'$.relation.type')='Owner of' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]any{}
	for rows.Next() {
		var pid, cid, status, reason, at string
		if err = rows.Scan(&pid, &cid, &status, &reason, &at); err != nil {
			return nil, err
		}
		if pid == "" || cid == "" {
			continue
		}
		if out[pid] == nil {
			out[pid] = map[string]any{}
		}
		out[pid][cid] = map[string]any{"status": status, "reason": reason, "at": at} // later jobs win
	}
	return out, rows.Err()
}
