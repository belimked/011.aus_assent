package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (a *app) adminRexEvents(w http.ResponseWriter, r *http.Request) {
	q := ` FROM rex_sync_events WHERE 1=1`
	args := []any{}
	for _, f := range []struct{ key, col string }{{"entity", "entity"}, {"local_id", "local_id"}, {"rex_id", "rex_id"}, {"direction", "direction"}} {
		if v := r.URL.Query().Get(f.key); v != "" {
			if f.key == "direction" && v != "app_to_rex" && v != "rex_to_app" {
				problem(w, 400, "invalid direction")
				return
			}
			q += " AND " + f.col + "=?"
			args = append(args, v)
		}
	}
	var total int
	if err := a.db.QueryRow(`SELECT count(*)`+q, args...).Scan(&total); err != nil {
		problem(w, 500, "database error")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	rows, err := a.db.Query(`SELECT id,at,direction,entity,local_id,rex_id,changes,result,error`+q+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id int64
		var at, direction, entity, local, rex, raw, result, reason string
		if rows.Scan(&id, &at, &direction, &entity, &local, &rex, &raw, &result, &reason) != nil {
			problem(w, 500, "database error")
			return
		}
		changes := map[string]any{}
		_ = json.Unmarshal([]byte(raw), &changes)
		items = append(items, map[string]any{"id": id, "at": at, "direction": direction, "entity": entity, "localId": local, "rexId": rex, "changes": changes, "result": result, "error": reason})
	}
	if rows.Err() != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"items": items, "total": total})
}
