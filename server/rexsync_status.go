package main

func attachRexSync(db querier, kind, id string, item map[string]any) {
	var status, reason, at string
	query := `SELECT status,COALESCE(last_error,''),created_at FROM rex_outbox WHERE entity=? AND entity_id=? ORDER BY id DESC LIMIT 1`
	if kind == "properties" {
		query = `SELECT status,COALESCE(last_error,''),created_at FROM rex_outbox WHERE (entity='rex_contact_relation' AND json_extract(payload,'$.relation.propertyId')=?) OR (entity='properties' AND entity_id=?) ORDER BY CASE WHEN entity='rex_contact_relation' THEN 0 ELSE 1 END,id DESC LIMIT 1`
		_ = db.QueryRow(query, id, id).Scan(&status, &reason, &at)
	} else {
		_ = db.QueryRow(query, kind, id).Scan(&status, &reason, &at)
	}
	if status != "" {
		item["rexSync"] = map[string]any{"status": status, "reason": reason, "at": at}
	} else {
		var linked int
		_ = db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity=? AND local_id=?`, kind, id).Scan(&linked)
		if linked > 0 {
			item["rexSync"] = map[string]any{"status": "sent", "at": ""}
		}
	}
}
