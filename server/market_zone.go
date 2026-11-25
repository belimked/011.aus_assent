package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

func createMarketZone(db *sql.DB, email string) error {
	var id, team string
	if err := db.QueryRow(`SELECT id,team_id FROM users WHERE email=? AND deleted_at IS NULL AND disabled=0`, email).Scan(&id, &team); err != nil {
		return err
	}
	const name = "Pricefinder nine suburbs"
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM zones WHERE owner_id=? AND name=? AND deleted_at IS NULL`, id, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		fmt.Println("zone already exists for", email)
		return nil
	}
	data := map[string]any{"id": ulid(), "name": name, "type": "suburbs", "suburbs": []string{"Sunnybank", "Sunnybank Hills", "Calamvale", "Parkinson", "Macgregor", "Robertson", "Runcorn", "Kuraby", "Eight Mile Plains"}, "ownerId": id}
	b, _ := json.Marshal(data)
	stamp := now()
	_, err := db.Exec(`INSERT INTO zones(id,team_id,name,geometry,data,owner_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, data["id"], team, name, "", string(b), id, stamp, stamp)
	return err
}
