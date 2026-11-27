package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// UAT 3 问题 3: adding an owner to a property that is not in Rex creates the
// property there first (Properties::findPossibleDuplicate → Properties::create),
// then the Owner relation is written as usual. Only rex_contact_relation
// "Owner of" jobs reach this path: browsing, tags and door-knock notes never
// create a Rex property. See docs/rex-integration.md "App 新建 Rex 房产".
//
// The worker processes one job at a time, so two owner jobs for the same
// property run one after the other; the second sees the rex_links row the first
// wrote. rex_property_creates is the guard for what rex_links cannot show: a
// create that was sent but whose result is unknown.

const propertyCreateUnknown = "Rex property create outcome unknown; check Rex for this address, then run `axleo rex-property-backfill --retry-unknown`"

func rexPropertyCreateEnabled() bool { return os.Getenv("REX_PROPERTY_CREATE") != "0" }

type rexAddress struct {
	Unit, No, Street, Suburb, State, Postcode string
	Lat, Lng                                  float64
	HasLL                                     bool
}

// fields are the adr_* values shared by findPossibleDuplicate and create.
func (a rexAddress) fields() map[string]any {
	m := map[string]any{"adr_street_number": a.No, "adr_street_name": a.Street, "adr_suburb_or_town": a.Suburb, "adr_state_or_region": a.State, "adr_postcode": a.Postcode}
	if a.Unit != "" {
		m["adr_unit_number"] = a.Unit
	}
	return m
}

func (a rexAddress) String() string {
	no := a.No
	if a.Unit != "" {
		no = a.Unit + "/" + no
	}
	return strings.TrimSpace(no + " " + a.Street + ", " + a.Suburb)
}

// localRexAddress reads the address Rex needs. reason is non-empty, and written
// for the agent, when the address cannot be sent.
func localRexAddress(db *sql.DB, id string) (addr rexAddress, reason string, err error) {
	var raw string
	err = db.QueryRow(`SELECT COALESCE(no,''),COALESCE(street,''),COALESCE(suburb,''),data FROM properties WHERE id=? AND deleted_at IS NULL`, id).Scan(&addr.No, &addr.Street, &addr.Suburb, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return addr, "Property was deleted in the app, so it was not added to Rex", nil
	}
	if err != nil {
		return addr, "", err
	}
	var d map[string]any
	_ = json.Unmarshal([]byte(raw), &d)
	for _, k := range []string{"no", "street", "suburb"} {
		v := strings.TrimSpace(str(d, k))
		switch {
		case k == "no" && addr.No == "":
			addr.No = v
		case k == "street" && addr.Street == "":
			addr.Street = v
		case k == "suburb" && addr.Suburb == "":
			addr.Suburb = v
		}
	}
	addr.No, addr.Street, addr.Suburb = strings.TrimSpace(addr.No), strings.TrimSpace(addr.Street), strings.TrimSpace(addr.Suburb)
	// Rex matches unit and street number separately: "3/11" is unit 3 at 11.
	if i := strings.Index(addr.No, "/"); i >= 0 {
		addr.Unit, addr.No = strings.TrimSpace(addr.No[:i]), strings.TrimSpace(addr.No[i+1:])
	}
	if u := strings.TrimSpace(istring(d["unit"])); u != "" {
		addr.Unit = u
	}
	addr.State = strings.TrimSpace(istring(d["state"]))
	if addr.State == "" {
		addr.State = env("REX_PROPERTY_STATE", "QLD")
	}
	addr.Postcode = strings.TrimSpace(istring(d["postcode"]))
	if addr.Postcode == "" {
		addr.Postcode = seedPostcodes[key(addr.Suburb)]
	}
	addr.Lat, addr.Lng, addr.HasLL = propertyLL(d)
	switch {
	case addr.No == "" && addr.Street == "":
		return addr, "Address has no street number or street name, so it was not added to Rex. Add the property in Rex by hand", nil
	case addr.No == "":
		return addr, "Address has no street number, so it was not added to Rex. Add the property in Rex by hand", nil
	case addr.Street == "":
		return addr, "Address has no street name, so it was not added to Rex. Add the property in Rex by hand", nil
	case addr.Suburb == "":
		return addr, "Address has no suburb, so it was not added to Rex. Add the property in Rex by hand", nil
	case addr.Postcode == "":
		return addr, "Postcode for " + addr.Suburb + " is unknown, so the property was not added to Rex. Add it in Rex by hand", nil
	}
	return addr, "", nil
}

func (w *rexWorker) createMarker(pid string) (state, rid string) {
	_ = w.db.QueryRow(`SELECT state,COALESCE(rex_id,'') FROM rex_property_creates WHERE property_id=? AND account_id=?`, pid, w.account).Scan(&state, &rid)
	return
}

func (w *rexWorker) setCreateMarker(pid, state, rid, reason string) error {
	stamp := now()
	_, err := w.db.Exec(`INSERT INTO rex_property_creates(property_id,account_id,state,rex_id,reason,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(property_id,account_id) DO UPDATE SET state=excluded.state,rex_id=excluded.rex_id,reason=excluded.reason,updated_at=excluded.updated_at`, pid, w.account, state, sql.NullString{String: rid, Valid: rid != ""}, reason, stamp, stamp)
	return err
}

// linkRexProperty records pid ↔ rid exactly as an import would (rex_links is
// what sync, Open in Rex and the poller key on) and bumps the property so the
// phone receives rexId on its next pull.
func (w *rexWorker) linkRexProperty(pid, rid, state, how string) error {
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	if _, err = tx.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties',?,?,'Properties',?,?)`, pid, rid, stamp, w.account); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO rex_property_creates(property_id,account_id,state,rex_id,reason,created_at,updated_at) VALUES(?,?,?,?,'',?,?) ON CONFLICT(property_id,account_id) DO UPDATE SET state=excluded.state,rex_id=excluded.rex_id,reason='',updated_at=excluded.updated_at`, pid, w.account, state, rid, stamp, stamp); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1 WHERE id=?`, stamp, pid); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, stamp, "app_to_rex", "properties", pid, rid, ijson(map[string]any{"linked": how}), "ok"); err != nil {
		return err
	}
	return tx.Commit()
}

// rexCall logs in when needed and retries once after a TokenException.
func (w *rexWorker) rexCall(ctx context.Context, service, method string, body any) (json.RawMessage, json.RawMessage, error) {
	if w.client.Token == "" {
		if _, e := w.client.Login(ctx, w.email, w.password, w.account); e != nil {
			return nil, nil, errRexLogin
		}
	}
	_, env, e := w.client.Call(ctx, service, method, body)
	if e != nil && strings.Contains(string(env.Error), "TokenException") {
		w.client.Token = ""
		if _, loginErr := w.client.Login(ctx, w.email, w.password, w.account); loginErr != nil {
			return nil, nil, errRexLogin
		}
		_, env, e = w.client.Call(ctx, service, method, body)
	}
	return env.Result, env.Error, e
}

// errRexLogin is shared with rexmerge_worker.go.

// ensureRexProperty returns the Rex id for local property pid, linking an
// existing Rex property with the same address or creating one. done=true means
// the job was already finished, retried or deferred and the caller must return err.
func (w *rexWorker) ensureRexProperty(ctx context.Context, j rexJob, pid string, finish finishFunc) (rid string, done bool, err error) {
	if !rexPropertyCreateEnabled() {
		return "", true, finish("skipped", "not in Rex", "", "")
	}
	var otherAccount int
	_ = w.db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='properties' AND local_id=?`, pid).Scan(&otherAccount)
	if otherAccount > 0 { // rex_links is keyed by (entity,local_id): one Rex account per property
		return "", true, finish("skipped", "property linked to another Rex account", "", "")
	}
	switch state, _ := w.createMarker(pid); state {
	case "sending", "unknown":
		return "", true, finish("failed", propertyCreateUnknown, "", "")
	}
	addr, reason, err := localRexAddress(w.db, pid)
	if err != nil {
		return "", true, err
	}
	if reason != "" {
		return "", true, finish("skipped", reason, "", "")
	}
	w.client.Entity, w.client.LocalID, w.client.RexID = "properties", pid, ""
	lookup := addr.fields()
	lookup["property_category_id"] = nil // try residential and land formats
	result, rexErr, e := w.rexCall(ctx, "Properties", "findPossibleDuplicate", lookup)
	if e != nil {
		var rate rexRateError
		switch {
		case errors.Is(e, errRexLogin):
			return "", true, w.retry(j, "Rex login failed", 0)
		case errors.As(e, &rate):
			return "", true, w.retry(j, "Rex rate limited", rate.delay)
		case len(rexErr) > 0 && string(rexErr) != "null":
			return "", true, finish("failed", "Rex rejected the address: "+safeRexError(rexErr), "", "")
		}
		// A read: unknown whether Rex has the address, so wait rather than create blindly.
		return "", true, w.retry(j, "Rex property duplicate check failed; will retry", 0)
	}
	if found := probeID(result); found != "" && found != "false" && found != "0" {
		var other string
		e = w.db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='properties' AND account_id=? AND rex_id=?`, w.account, found).Scan(&other)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return "", true, e
		}
		if other != "" && other != pid {
			// Two app properties for one address. Never point both at one Rex record:
			// the agent adds the owner on the app property already in Rex.
			otherAddr, _, _ := localRexAddress(w.db, other)
			label := otherAddr.String()
			if label == "" || label == "," {
				label = other
			}
			return "", true, finish("skipped", "Rex already has this address, linked to app property "+label+". Add the owner there", "", "")
		}
		if other == "" {
			if e = w.linkRexProperty(pid, found, "linked", "existing Rex property (same address)"); e != nil {
				return "", true, e
			}
		}
		return found, false, nil
	}
	// Persist the attempt before sending: a crash or lost response leaves "sending"
	// and blocks a second create.
	if e = w.setCreateMarker(pid, "sending", "", ""); e != nil {
		return "", true, e
	}
	data := addr.fields()
	data["property_category"] = rexRef(env("REX_PROPERTY_CATEGORY", "residential"))
	if addr.HasLL {
		data["adr_latitude"], data["adr_longitude"] = addr.Lat, addr.Lng
	}
	// system_owner_user is not sent, as with Contacts::create: Rex assigns the
	// integration user; the app's own allocation (properties.owner_id) is unchanged.
	result, rexErr, e = w.rexCall(ctx, "Properties", "create", map[string]any{"data": data, "return_id": true})
	if e != nil {
		var rate rexRateError
		switch {
		case errors.Is(e, errRexLogin), errors.As(e, &rate), len(rexErr) > 0 && string(rexErr) != "null":
			// Not sent (login failed) or Rex answered: nothing was created.
			if _, de := w.db.Exec(`DELETE FROM rex_property_creates WHERE property_id=? AND account_id=?`, pid, w.account); de != nil {
				return "", true, de
			}
			if errors.Is(e, errRexLogin) {
				return "", true, w.retry(j, "Rex login failed", 0)
			}
			if errors.As(e, &rate) {
				return "", true, w.retry(j, "Rex rate limited", rate.delay)
			}
			return "", true, finish("failed", "Rex rejected the new property: "+safeRexError(rexErr), "", "")
		}
		if me := w.setCreateMarker(pid, "unknown", "", e.Error()); me != nil {
			return "", true, me
		}
		return "", true, finish("failed", propertyCreateUnknown, "", "")
	}
	created := probeID(result)
	if created == "" || created == "false" {
		if me := w.setCreateMarker(pid, "unknown", "", "Rex returned no id"); me != nil {
			return "", true, me
		}
		return "", true, finish("failed", propertyCreateUnknown, "", "")
	}
	if e = w.linkRexProperty(pid, created, "created", "created in Rex by AxleoCollect"); e != nil {
		return "", true, e
	}
	return created, false, nil
}

// ---- Historical "not in Rex" Owner relations (manual command) ----

type propertyBackfill struct {
	Requeued, Unknown, AlreadyLinked int
	Jobs                             []string
}

// backfillPropertyOwners puts skipped Owner relations whose property was not in
// Rex back to pending, so the worker runs the create path above. Only the latest
// job per contact/property pair, only if the owner link still exists locally.
// retryUnknown also clears sending/unknown markers (after an operator checked
// Rex): the next attempt starts with findPossibleDuplicate, so a property the
// lost create did make is linked, not duplicated.
func backfillPropertyOwners(db *sql.DB, account string, dry, retryUnknown bool) (propertyBackfill, error) {
	var out propertyBackfill
	type job struct {
		id               int64
		status, pid, cid string
	}
	// Read everything first: the pool has one connection (single_conn_guard_test.go).
	rows, err := db.Query(`SELECT o.id,o.status,COALESCE(json_extract(o.payload,'$.relation.propertyId'),''),COALESCE(json_extract(o.payload,'$.contactId'),'') FROM rex_outbox o
 WHERE o.entity='rex_contact_relation' AND json_extract(o.payload,'$.relation.type')='Owner of'
 AND o.id=(SELECT max(x.id) FROM rex_outbox x WHERE x.entity='rex_contact_relation' AND x.entity_id=o.entity_id)
 AND ((o.status='skipped' AND o.last_error='not in Rex') OR (?=1 AND o.status='failed' AND o.last_error=?))
 AND EXISTS(SELECT 1 FROM property_owners po WHERE po.property_id=json_extract(o.payload,'$.relation.propertyId') AND po.contact_id=json_extract(o.payload,'$.contactId'))
 ORDER BY o.id`, map[bool]int{true: 1}[retryUnknown], propertyCreateUnknown)
	if err != nil {
		return out, err
	}
	jobs := []job{}
	for rows.Next() {
		var j job
		if err = rows.Scan(&j.id, &j.status, &j.pid, &j.cid); err != nil {
			rows.Close()
			return out, err
		}
		jobs = append(jobs, j)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	tx, err := db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	for _, j := range jobs {
		var linked int
		if err = tx.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='properties' AND local_id=? AND account_id=?`, j.pid, account).Scan(&linked); err != nil {
			return out, err
		}
		if linked > 0 {
			out.AlreadyLinked++ // linked since (import or another owner): the relation is still due
		}
		var state string
		_ = tx.QueryRow(`SELECT state FROM rex_property_creates WHERE property_id=? AND account_id=?`, j.pid, account).Scan(&state)
		if state == "sending" || state == "unknown" {
			if !retryUnknown {
				out.Unknown++
				continue
			}
			if _, err = tx.Exec(`DELETE FROM rex_property_creates WHERE property_id=? AND account_id=?`, j.pid, account); err != nil {
				return out, err
			}
		}
		if _, err = tx.Exec(`UPDATE rex_outbox SET status='pending',tries=0,last_error='requeued: create Rex property',next_attempt_at=NULL WHERE id=?`, j.id); err != nil {
			return out, err
		}
		out.Requeued++
		out.Jobs = append(out.Jobs, j.cid+":"+j.pid)
	}
	if dry {
		return out, nil
	}
	return out, tx.Commit()
}

func rexPropertyBackfillCLI(db *sql.DB, args []string) error {
	dry, retryUnknown := false, false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dry = true
		case "--retry-unknown":
			retryUnknown = true
		default:
			return errors.New("usage: axleo rex-property-backfill [--dry-run] [--retry-unknown]")
		}
	}
	account := os.Getenv("REX_WRITE_ACCOUNT_ID")
	if account == "" {
		return errors.New("REX_WRITE_ACCOUNT_ID required")
	}
	r, err := backfillPropertyOwners(db, account, dry, retryUnknown)
	if err != nil {
		return err
	}
	for _, j := range r.Jobs {
		fmt.Println("requeue", j)
	}
	fmt.Printf("requeued=%d alreadyLinked=%d blockedUnknown=%d dry=%v\n", r.Requeued, r.AlreadyLinked, r.Unknown, dry)
	return nil
}
