package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

const (
	sandboxAgentProperties  = 200
	sandboxImportProperties = 1734
	sandboxAgentContacts    = 200
	sandboxImportContacts   = 3938
)

func sandboxUsers(db *sql.DB) (map[string]string, error) {
	ids := map[string]string{}
	for _, name := range []string{"mary", "john", "import", "admin"} {
		email := name + "@axleo.local"
		var id string
		if err := db.QueryRow(`SELECT id FROM users WHERE email=? AND disabled=0 AND deleted_at IS NULL`, email).Scan(&id); err != nil {
			return nil, fmt.Errorf("sandbox-test requires %s (create the account before importing): %w", email, err)
		}
		ids[name] = id
	}
	return ids, nil
}

// Existing assignments are immutable on replay. Only newly linked rows consume remaining capacity.
func runSandboxImport(ctx context.Context, db *sql.DB, c *rexClient, account string, dry bool, limit int, ids map[string]string) error {
	c.ReadOnly, c.Caller, c.AccountID = true, "import", account
	props, contacts := []importRow{}, []importRow{}
	if err := importPages(ctx, c, "Properties", limit, func(r importRow) error { props = append(props, r); return nil }); err != nil {
		return err
	}
	if err := importPages(ctx, c, "Contacts", limit, func(r importRow) error { contacts = append(contacts, r); return nil }); err != nil {
		return err
	}
	newProps, newContacts := map[string]bool{}, map[string]bool{}
	stats := map[string]*importStats{"properties": {}, "contacts": {}}
	save := func(kind string, rows []importRow, added map[string]bool) error {
		for _, r := range rows {
			var local string
			err := db.QueryRow(`SELECT local_id FROM rex_links WHERE entity=? AND account_id=? AND rex_id=?`, kind, account, iv(r, "id")).Scan(&local)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if errors.Is(err, sql.ErrNoRows) {
				added["rex-"+kind+"-"+account+"-"+iv(r, "id")] = true
			}
			if err := importSave(db, kind, account, r, ids["admin"], dry, stats[kind]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := save("properties", props, newProps); err != nil {
		return err
	}
	if err := save("contacts", contacts, newContacts); err != nil {
		return err
	}
	if !dry {
		for _, r := range props {
			if err := saveImportRelations(db, account, r); err != nil {
				return err
			}
		}
		if err := assignSandbox(db, ids, newProps, newContacts); err != nil {
			return err
		}
	}
	for _, kind := range []string{"properties", "contacts"} {
		s := stats[kind]
		fmt.Printf("%s: added=%d updated=%d skipped=%d dry-run=%t\n", kind, s.Added, s.Updated, s.Skipped, dry)
	}
	return nil
}

func assignSandbox(db *sql.DB, ids map[string]string, newProps, newContacts map[string]bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Same ordering as the reference SQL: relation first, then squared planar distance, then id.
	rows, err := tx.Query(`SELECT p.id, p.owner_id FROM properties p
 WHERE json_extract(p.data,'$.source')='rex_import' AND p.deleted_at IS NULL
 ORDER BY EXISTS(SELECT 1 FROM property_owners o WHERE o.property_id=p.id) DESC,
 CASE WHEN CAST(json_extract(p.data,'$.lat') AS REAL)<>0 THEN
 (CAST(json_extract(p.data,'$.lat') AS REAL)+27.5794)*(CAST(json_extract(p.data,'$.lat') AS REAL)+27.5794)+
 ((CAST(json_extract(p.data,'$.lng') AS REAL)-153.0606)*0.887)*((CAST(json_extract(p.data,'$.lng') AS REAL)-153.0606)*0.887)
 ELSE 1e9 END, p.id`)
	if err != nil {
		return err
	}
	type property struct{ id, owner string }
	ranked := []property{}
	for rows.Next() {
		var p property
		var owner sql.NullString
		if err = rows.Scan(&p.id, &owner); err != nil {
			break
		}
		p.owner = owner.String
		ranked = append(ranked, p)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	rank := map[string]int{}
	counts := map[string]int{}
	for i, p := range ranked {
		rank[p.id] = i + 1
		if !newProps[p.id] {
			counts[p.owner]++
		}
	}
	changed := false
	updateProperty := func(id, owner string) error {
		result, e := tx.Exec(`UPDATE properties SET owner_id=?,updated_at=?,version=version+1 WHERE id=? AND owner_id IS NOT ?`, owner, now(), id, owner)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		changed = changed || n > 0
		return e
	}
	// The first 400 slots alternate; when replaying, count already assigned slots rather than moving old rows.
	agentSlots := counts[ids["mary"]] + counts[ids["john"]]
	for _, p := range ranked {
		if !newProps[p.id] {
			continue
		}
		owner := ids["admin"]
		if agentSlots < 2*sandboxAgentProperties {
			candidate := "mary"
			if agentSlots%2 == 1 {
				candidate = "john"
			}
			if counts[ids[candidate]] < sandboxAgentProperties {
				owner = ids[candidate]
				counts[owner]++
				agentSlots++
			}
		} else if counts[ids["import"]] < sandboxImportProperties {
			owner = ids["import"]
			counts[owner]++
		}
		if err := updateProperty(p.id, owner); err != nil {
			return err
		}
	}
	type contact struct {
		id, owner, lu string
		rank          int
	}
	cr, err := tx.Query(`SELECT c.id, c.owner_id, o.property_id FROM contacts c LEFT JOIN property_owners o ON o.contact_id=c.id
 WHERE json_extract(c.data,'$.source')='rex_import' AND c.deleted_at IS NULL ORDER BY c.id`)
	if err != nil {
		return err
	}
	cm := map[string]*contact{}
	for cr.Next() {
		var id string
		var owner, pid sql.NullString
		if err = cr.Scan(&id, &owner, &pid); err != nil {
			break
		}
		v := cm[id]
		if v == nil {
			v = &contact{id: id, owner: owner.String}
			cm[id] = v
		}
		if r := rank[pid.String]; r > 0 && (v.rank == 0 || r < v.rank) {
			v.rank = r
			v.lu = ranked[r-1].owner
		}
	}
	if err == nil {
		err = cr.Err()
	}
	cr.Close()
	if err != nil {
		return err
	}
	linked, unlinked := []*contact{}, []*contact{}
	contactCounts := map[string]int{}
	for _, v := range cm {
		if !newContacts[v.id] {
			contactCounts[v.owner]++
			continue
		}
		if v.rank > 0 {
			linked = append(linked, v)
		} else {
			unlinked = append(unlinked, v)
		}
	}
	sort.Slice(linked, func(i, j int) bool {
		if linked[i].rank == linked[j].rank {
			return linked[i].id < linked[j].id
		}
		return linked[i].rank < linked[j].rank
	})
	sort.Slice(unlinked, func(i, j int) bool { return unlinked[i].id < unlinked[j].id })
	assigned := map[string]string{}
	for _, name := range []string{"mary", "john"} {
		owner := ids[name]
		for _, v := range linked {
			if v.lu == owner && assigned[v.id] == "" && contactCounts[owner] < sandboxAgentContacts {
				assigned[v.id] = owner
				contactCounts[owner]++
			}
		}
		for _, v := range unlinked {
			if assigned[v.id] == "" && contactCounts[owner] < sandboxAgentContacts {
				assigned[v.id] = owner
				contactCounts[owner]++
			}
		}
	}
	owner := ids["import"]
	for _, group := range [][]*contact{linked, unlinked} {
		for _, v := range group {
			if assigned[v.id] == "" && contactCounts[owner] < sandboxImportContacts {
				assigned[v.id] = owner
				contactCounts[owner]++
			}
		}
	}
	for id := range newContacts {
		target := assigned[id]
		if target == "" {
			target = ids["admin"]
		}
		result, e := tx.Exec(`UPDATE contacts SET owner_id=?,data=json_set(data,'$.owner',?),updated_at=?,version=version+1 WHERE id=? AND owner_id IS NOT ?`, target, target, now(), id, target)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		changed = changed || n > 0
	}
	if changed {
		for _, id := range ids {
			if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id); err != nil {
				return err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if changed {
		fmt.Println("Assignment changed: Mary, John, import and admin sessions revoked; mobile users must log in again for a full sync.")
	}
	return nil
}
