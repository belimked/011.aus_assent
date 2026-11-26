package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Contact merge (docs/contact-merge.md). No approval step, no undo: the caller
// previews the computed result, then executes it with the preview hash. Every
// rule lives here so the preview and the write can never disagree.

// querier is satisfied by *sql.DB (preview) and *sql.Tx (execute). The pool has a
// single connection: every Rows is closed before the next statement runs.
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

// mergeLinkWait bounds how long a rex_merge job waits for a contact that is still
// being created in Rex before it proceeds with the links that exist.
const mergeLinkWait = 10 * time.Minute

var errMergeStale = errors.New("merge preview is out of date")

type mergeContact struct {
	ID            string         `json:"id"`
	Owner         string         `json:"ownerId"`
	Created       string         `json:"created_at"`
	Updated       string         `json:"updated_at"`
	Version       int            `json:"version"`
	Data          map[string]any `json:"data"`
	Rex           string         `json:"rexId"`
	CreatePending bool           `json:"rexCreatePending"`
	Notes         []mergeNoteRow `json:"contactNotes"`
	Props         []mergePropRow `json:"propertyOwners"`
}
type mergeNoteRow struct {
	ID      string `json:"id"`
	UserID  string `json:"userId"`
	D       string `json:"d"`
	A       string `json:"a"`
	T       string `json:"t"`
	Created string `json:"created_at"`
}
type mergePropRow struct {
	PropertyID string `json:"propertyId"`
	Relation   string `json:"relation"`
	Updated    string `json:"updated_at"`
}

type mergeField struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	From   string `json:"from"`
	A      any    `json:"a"`
	B      any    `json:"b"`
	Reason string `json:"reason"` // newer | only | same
}
type mergeConsent struct {
	Channel string `json:"channel"`
	Value   any    `json:"value"`
	A       any    `json:"a"`
	B       any    `json:"b"`
}
type mergeNoteOut struct {
	D    string `json:"d"`
	A    string `json:"a"`
	T    string `json:"t"`
	At   string `json:"at,omitempty"`
	From string `json:"from"`
	Log  bool   `json:"log,omitempty"`
}
type mergeReplaced struct {
	Relation string `json:"relation"`
	Label    string `json:"label"`
	From     string `json:"from"`
}
type mergeRel struct {
	PropertyID string          `json:"propertyId"`
	Address    string          `json:"address"`
	Relation   string          `json:"relation"` // normalised kind: owner, tenant, past_owner, ...
	Label      string          `json:"label"`
	Raw        string          `json:"-"` // stored property_owners.relation spelling
	At         string          `json:"at"`
	From       string          `json:"from"`
	Replaced   []mergeReplaced `json:"replaced,omitempty"`
	Changed    bool            `json:"changed"` // new for the primary or type changed
}
type mergeSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	Created   string `json:"createdAt"`
	Updated   string `json:"updatedAt"`
	RexLinked bool   `json:"rexLinked"`
}
type mergePlan struct {
	PrimaryID     string         `json:"primaryId"`
	SecondaryID   string         `json:"secondaryId"`
	PrimaryReason string         `json:"primaryReason"` // rex | older
	Contacts      []mergeSummary `json:"contacts"`      // [primary, secondary]
	Fields        []mergeField   `json:"fields"`
	DNC           map[string]any `json:"dnc"`
	Consent       []mergeConsent `json:"consent"`
	Notes         []mergeNoteOut `json:"notes"` // newest first, merge log first
	Tags          []string       `json:"tags"`
	Roles         []string       `json:"roles"`
	Properties    []mergeRel     `json:"properties"`
	Relations     []any          `json:"relations"`
	LogNote       string         `json:"logNote"`
	PreviewHash   string         `json:"previewHash"`
	Rex           string         `json:"rex"` // both | primary | secondary | none | pending
	data          map[string]any
	logD, logA    string
	primary       *mergeContact
	secondary     *mergeContact
}

func loadMergeContact(q querier, id string) (*mergeContact, error) {
	c := &mergeContact{ID: id}
	var raw string
	if err := q.QueryRow(`SELECT data,version,updated_at,created_at,COALESCE(owner_id,'') FROM contacts WHERE id=? AND deleted_at IS NULL`, id).Scan(&raw, &c.Version, &c.Updated, &c.Created, &c.Owner); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &c.Data); err != nil || c.Data == nil {
		c.Data = map[string]any{}
	}
	_ = q.QueryRow(`SELECT rex_id FROM rex_links WHERE entity='contacts' AND local_id=?`, id).Scan(&c.Rex)
	if c.Rex == "" {
		c.Rex = str(c.Data, "rexId")
	}
	var pending int
	_ = q.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='contacts' AND entity_id=? AND op='POST' AND status IN ('pending','waiting_auth') AND COALESCE(json_extract(payload,'$.t'),'')=''`, id).Scan(&pending)
	c.CreatePending = pending > 0
	rows, err := q.Query(`SELECT id,COALESCE(user_id,''),COALESCE(d,''),COALESCE(a,''),COALESCE(t,''),created_at FROM contact_notes WHERE contact_id=? ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n mergeNoteRow
		if err = rows.Scan(&n.ID, &n.UserID, &n.D, &n.A, &n.T, &n.Created); err != nil {
			break
		}
		c.Notes = append(c.Notes, n)
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(`SELECT property_id,relation,updated_at FROM property_owners WHERE contact_id=? ORDER BY updated_at,property_id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p mergePropRow
		if err = rows.Scan(&p.PropertyID, &p.Relation, &p.Updated); err != nil {
			break
		}
		c.Props = append(c.Props, p)
	}
	rows.Close()
	return c, err
}

func mergeTime(s string) time.Time {
	for _, f := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

// fieldTime: a field-level time in data.fieldTimes wins over the record's updated_at.
func (c *mergeContact) fieldTime(key string) time.Time {
	if t := mergeTime(str(imap(c.Data["fieldTimes"]), key)); !t.IsZero() {
		return t
	}
	return mergeTime(c.Updated)
}

func mergeEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

func contactLabel(d map[string]any) string {
	name := strings.TrimSpace(str(d, "first") + " " + str(d, "last"))
	contact := str(d, "mobile")
	if contact == "" {
		contact = str(d, "email")
	}
	switch {
	case name != "" && contact != "":
		return name + " (" + contact + ")"
	case name != "":
		return name
	case contact != "":
		return contact
	}
	return str(d, "id")
}

// relKind normalises property relation spellings: "Owner of", "owner", "Previous tenant of".
func relKind(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	k = strings.TrimSuffix(k, " of")
	k = strings.TrimPrefix(strings.Replace(k, "previous ", "past ", 1), " ")
	k = strings.Join(strings.Fields(k), "_")
	if k == "" {
		return "owner"
	}
	return k
}
func relLabel(kind string) string {
	switch kind {
	case "owner":
		return "Owner"
	case "tenant":
		return "Tenant"
	case "past_owner":
		return "Previous owner"
	case "past_tenant":
		return "Previous tenant"
	}
	s := strings.ReplaceAll(kind, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var mergeIdentityKeys = map[string]bool{"id": true, "ownerId": true, "owner": true, "version": true, "updated_at": true, "deleted_at": true, "mobileNorm": true, "rexId": true, "source": true, "fieldTimes": true, "updated": true,
	"notes": true, "tags": true, "roles": true, "rels": true, "flags": true, "consent": true, "optedOut": true, "possibleDuplicateOf": true}

// mergedDuplicateFlags is the primary's possibleDuplicateOf (问题02, rexdedupe.go)
// after the merge: both lists combined, minus the merged pair itself. Flags that
// name a third contact stay.
func mergedDuplicateFlags(p, s *mergeContact) []string {
	out := []string{}
	seen := map[string]bool{p.ID: true, s.ID: true}
	for _, c := range []*mergeContact{p, s} {
		for _, id := range stringsFrom(c.Data["possibleDuplicateOf"]) {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// clearMergedDuplicateFlags removes the secondary's flags (its flags to third
// contacts were carried over to the primary by mergedDuplicateFlags) and points
// third contacts that flagged the secondary at the primary instead (the
// secondary's record now lives there). Runs inside the merge transaction.
func clearMergedDuplicateFlags(tx *sql.Tx, primary, secondary, stamp string) error {
	rows, err := tx.Query(`SELECT id,data FROM contacts WHERE id<>? AND (deleted_at IS NULL OR id=?) AND instr(data,'"possibleDuplicateOf"')>0 AND instr(data,?)>0`, primary, secondary, `"`+secondary+`"`)
	if err != nil {
		return err
	}
	type upd struct{ id, data string }
	updates := []upd{}
	for rows.Next() {
		var id, raw string
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var data map[string]any
		if json.Unmarshal([]byte(raw), &data) != nil || data == nil {
			continue
		}
		old := stringsFrom(data["possibleDuplicateOf"])
		list := []string{}
		seen := map[string]bool{id: true}
		for _, x := range old {
			if id == secondary {
				break // carried over to the primary
			}
			if x == secondary {
				x = primary
			}
			if x != "" && !seen[x] {
				seen[x] = true
				list = append(list, x)
			}
		}
		if ijson(list) == ijson(old) {
			continue
		}
		if len(list) == 0 {
			delete(data, "possibleDuplicateOf")
		} else {
			data["possibleDuplicateOf"] = list
		}
		updates = append(updates, upd{id, ijson(data)})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, u := range updates {
		if _, err = tx.Exec(`UPDATE contacts SET data=?,updated_at=?,version=version+1 WHERE id=?`, u.data, stamp, u.id); err != nil {
			return err
		}
	}
	return nil
}

// computeMerge is pure: the same two loaded contacts always give the same plan
// (apart from the merge log note, which carries the actor and the date).
func computeMerge(a, b *mergeContact, addresses map[string]string, actor string, at time.Time) *mergePlan {
	p, s, reason := a, b, "older"
	if (a.Rex != "") != (b.Rex != "") {
		reason = "rex"
		if b.Rex != "" {
			p, s = b, a
		}
	} else {
		ta, tb := mergeTime(a.Created), mergeTime(b.Created)
		if tb.Before(ta) || (tb.Equal(ta) && b.ID < a.ID) {
			p, s = b, a
		}
	}
	plan := &mergePlan{PrimaryID: p.ID, SecondaryID: s.ID, PrimaryReason: reason, primary: p, secondary: s, Fields: []mergeField{}, Consent: []mergeConsent{}, Notes: []mergeNoteOut{}, Properties: []mergeRel{}, Relations: []any{}}
	for _, c := range []*mergeContact{p, s} {
		plan.Contacts = append(plan.Contacts, mergeSummary{ID: c.ID, Name: strings.TrimSpace(str(c.Data, "first") + " " + str(c.Data, "last")), Mobile: str(c.Data, "mobile"), Email: str(c.Data, "email"), Created: c.Created, Updated: c.Updated, RexLinked: c.Rex != ""})
	}
	switch {
	case p.Rex != "" && s.Rex != "":
		plan.Rex = "both"
	case p.Rex != "":
		plan.Rex = "primary"
	case s.Rex != "":
		plan.Rex = "secondary"
	case p.CreatePending || s.CreatePending:
		plan.Rex = "pending"
	default:
		plan.Rex = "none"
	}
	// newer reports which side the timeline picks for key; a tie keeps the primary.
	newer := func(key string) (*mergeContact, *mergeContact) {
		if s.fieldTime(key).After(p.fieldTime(key)) {
			return s, p
		}
		return p, s
	}
	data := map[string]any{}
	for k, v := range p.Data {
		data[k] = v
	}
	if str(data, "source") == "" && str(s.Data, "source") != "" {
		data["source"] = s.Data["source"]
	}
	keys := map[string]bool{}
	for _, c := range []*mergeContact{p, s} {
		for k := range c.Data {
			if !mergeIdentityKeys[k] {
				keys[k] = true
			}
		}
	}
	ordered := []string{}
	for k := range keys {
		ordered = append(ordered, k)
	}
	rank := map[string]int{"first": 1, "last": 2, "mobile": 3, "email": 4, "address": 5, "reason": 6}
	sort.Slice(ordered, func(i, j int) bool {
		ri, rj := rank[ordered[i]], rank[ordered[j]]
		if ri == 0 {
			ri = 100
		}
		if rj == 0 {
			rj = 100
		}
		if ri != rj {
			return ri < rj
		}
		return ordered[i] < ordered[j]
	})
	for _, k := range ordered {
		pv, sv := p.Data[k], s.Data[k]
		if pl, ok := pv.([]any); ok || isList(sv) {
			// Unknown lists (history, voiceAudit, ...) are combined like notes.
			sl, _ := sv.([]any)
			data[k] = unionJSON(pl, sl)
			continue
		}
		win, lose := newer(k)
		value, from, why := win.Data[k], win.ID, "newer"
		if mergeEmpty(value) {
			value, from, why = lose.Data[k], lose.ID, "only"
		} else if mergeEmpty(lose.Data[k]) {
			why = "only"
		}
		if ijson(pv) == ijson(sv) {
			why = "same"
		}
		if mergeEmpty(value) {
			continue
		}
		data[k] = value
		switch value.(type) {
		case string, float64, bool:
			plan.Fields = append(plan.Fields, mergeField{Key: k, Value: value, From: from, A: pv, B: sv, Reason: why})
		}
	}
	// Compliance exceptions: DNC if either is DNC; per channel the stricter consent.
	pf, sf := imap(p.Data["flags"]), imap(s.Data["flags"])
	flags := map[string]any{}
	win, lose := newer("flags")
	for k, v := range lose.flagsOrEmpty() {
		flags[k] = v
	}
	for k, v := range win.flagsOrEmpty() {
		flags[k] = v
	}
	dnc := pf["dnc"] == true || sf["dnc"] == true
	if dnc || pf["dnc"] != nil || sf["dnc"] != nil {
		flags["dnc"] = dnc
	}
	data["flags"] = flags
	plan.DNC = map[string]any{"value": dnc, "a": pf["dnc"] == true, "b": sf["dnc"] == true}
	pc, sc := imap(p.Data["consent"]), imap(s.Data["consent"])
	consent := map[string]any{}
	channels := map[string]bool{}
	for k := range pc {
		channels[k] = true
	}
	for k := range sc {
		channels[k] = true
	}
	chList := []string{}
	for k := range channels {
		chList = append(chList, k)
	}
	sort.Strings(chList)
	refused := func(v any) bool { return v == float64(0) || v == false || v == "0" }
	for _, k := range chList {
		pv, sv := pc[k], sc[k]
		var v any
		switch {
		case refused(pv):
			v = pv
		case refused(sv):
			v = sv
		case pv != nil:
			v = pv
		default:
			v = sv
		}
		consent[k] = v
		plan.Consent = append(plan.Consent, mergeConsent{Channel: k, Value: v, A: pv, B: sv})
	}
	if len(consent) > 0 || p.Data["consent"] != nil || s.Data["consent"] != nil {
		data["consent"] = consent
	}
	if v, ok := consent["sms"]; ok {
		data["optedOut"] = refused(v)
	} else if p.Data["optedOut"] == true || s.Data["optedOut"] == true {
		data["optedOut"] = true
	}
	// Tags: union ignoring case and surrounding spaces; the newer record's spelling wins.
	tw, tl := newer("tags")
	plan.Tags = []string{}
	seenTag := map[string]bool{}
	for _, c := range []*mergeContact{tw, tl} {
		for _, t := range stringsFrom(c.Data["tags"]) {
			t = strings.TrimSpace(t)
			key := strings.ToLower(t)
			if t == "" || seenTag[key] {
				continue
			}
			seenTag[key] = true
			plan.Tags = append(plan.Tags, t)
		}
	}
	data["tags"] = plan.Tags
	plan.Roles = []string{}
	seenRole := map[string]bool{}
	for _, c := range []*mergeContact{p, s} {
		for _, r := range stringsFrom(c.Data["roles"]) {
			r = strings.TrimSpace(r)
			if r == "" || seenRole[strings.ToLower(r)] {
				continue
			}
			seenRole[strings.ToLower(r)] = true
			plan.Roles = append(plan.Roles, r)
		}
	}
	data["roles"] = plan.Roles
	plan.Notes = mergeNotes(p, s)
	notes := []any{}
	for _, n := range plan.Notes {
		notes = append(notes, map[string]any{"d": n.D, "a": n.A, "t": n.T})
	}
	data["notes"] = notes
	plan.Properties = mergeProperties(p, s, addresses)
	// rels JSON: contact-to-contact entries are combined; property entries follow the
	// resolved property relation so rels and property_owners never disagree.
	final := map[string]mergeRel{}
	for _, r := range plan.Properties {
		final[r.PropertyID] = r
	}
	rels := []any{}
	seenRel := map[string]bool{}
	for _, c := range []*mergeContact{p, s} {
		list, _ := c.Data["rels"].([]any)
		for _, v := range list {
			r := imap(v)
			if r == nil {
				continue
			}
			copyRel := map[string]any{}
			for k, x := range r {
				copyRel[k] = x
			}
			if pid := str(r, "propertyId"); pid != "" {
				f, ok := final[pid]
				if !ok {
					continue
				}
				copyRel["type"] = relLabel(f.Relation) + " of"
			}
			if cid := str(r, "contactId"); cid == s.ID {
				copyRel["contactId"] = p.ID
			}
			if str(copyRel, "contactId") == p.ID {
				continue // a relation to itself after the merge
			}
			key := ijson(copyRel)
			if seenRel[key] {
				continue
			}
			seenRel[key] = true
			rels = append(rels, copyRel)
		}
	}
	plan.Relations = rels
	if len(rels) > 0 || p.Data["rels"] != nil || s.Data["rels"] != nil {
		data["rels"] = rels
	}
	if dups := mergedDuplicateFlags(p, s); len(dups) > 0 {
		data["possibleDuplicateOf"] = dups
	} else {
		delete(data, "possibleDuplicateOf")
	}
	data["id"] = p.ID
	delete(data, "rexId")
	if p.Rex != "" {
		data["rexId"] = p.Rex
	}
	data["mobileNorm"] = normAUMobile(str(data, "mobile"))
	plan.data = data
	plan.PreviewHash = plan.hash()
	// Dated parts are added after hashing: a preview stays valid across the day.
	plan.logD, plan.logA = at.In(brisbane).Format("2006-01-02"), actor
	plan.LogNote = mergeLogNote(plan, s, actor, plan.logD)
	data["updated"] = map[string]any{"by": actor, "d": plan.logD}
	data["notes"] = append([]any{map[string]any{"d": plan.logD, "a": actor, "t": plan.LogNote}}, notes...)
	plan.Notes = append([]mergeNoteOut{{D: plan.logD, A: actor, T: plan.LogNote, At: at.UTC().Format(layout), From: p.ID, Log: true}}, plan.Notes...)
	return plan
}

func (c *mergeContact) flagsOrEmpty() map[string]any {
	if m := imap(c.Data["flags"]); m != nil {
		return m
	}
	return map[string]any{}
}
func isList(v any) bool { _, ok := v.([]any); return ok }
func unionJSON(a, b []any) []any {
	out := []any{}
	seen := map[string]bool{}
	for _, v := range append(append([]any{}, a...), b...) {
		k := ijson(v)
		if !seen[k] {
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func (plan *mergePlan) hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%d:%s|%s:%d:%s|", plan.primary.ID, plan.primary.Version, plan.primary.Updated, plan.secondary.ID, plan.secondary.Version, plan.secondary.Updated)
	view := *plan
	view.LogNote, view.PreviewHash = "", ""
	b, _ := json.Marshal(view)
	h.Write(b)
	h.Write([]byte(ijson(plan.data)))
	h.Write([]byte(ijson([]any{plan.primary.Notes, plan.primary.Props, plan.secondary.Notes, plan.secondary.Props})))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// mergeNotes combines data.notes and contact_notes of both, drops exact duplicates
// and orders newest first (the app's notes[0] is the latest).
func mergeNotes(p, s *mergeContact) []mergeNoteOut {
	type entry struct {
		n   mergeNoteOut
		at  time.Time
		seq int
	}
	entries := []*entry{}
	index := map[string]*entry{}
	add := func(d, a, t, at, from string) {
		key := strings.TrimSpace(d) + "\x00" + strings.TrimSpace(a) + "\x00" + strings.TrimSpace(t)
		when := mergeTime(at)
		if when.IsZero() {
			when = mergeTime(d)
		}
		if e, ok := index[key]; ok {
			if e.n.From != from {
				e.n.From = "both"
			}
			// A contact_notes row carries the precise time of a dated note.
			if !when.IsZero() && (e.at.IsZero() || (len(at) > len(e.n.At) && sameDay(when, e.at))) {
				e.at, e.n.At = when, at
			}
			return
		}
		e := &entry{n: mergeNoteOut{D: d, A: a, T: t, At: at, From: from}, at: when, seq: len(entries)}
		entries = append(entries, e)
		index[key] = e
	}
	for _, c := range []*mergeContact{p, s} {
		list, _ := c.Data["notes"].([]any)
		for _, v := range list {
			n := imap(v)
			if n == nil || strings.TrimSpace(str(n, "t")) == "" {
				continue
			}
			add(str(n, "d"), str(n, "a"), str(n, "t"), str(n, "at"), c.ID)
		}
		for _, n := range c.Notes {
			if strings.TrimSpace(n.T) == "" {
				continue
			}
			add(n.D, n.A, n.T, n.Created, c.ID)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].at.Equal(entries[j].at) {
			return entries[i].at.After(entries[j].at)
		}
		return entries[i].seq < entries[j].seq
	})
	out := []mergeNoteOut{}
	for _, e := range entries {
		out = append(out, e.n)
	}
	return out
}
func sameDay(a, b time.Time) bool {
	return a.UTC().Format("2006-01-02") == b.UTC().Format("2006-01-02")
}

// mergeProperties resolves property relations by time. Sources: property_owners
// (time = updated_at) and rels entries with a propertyId (time = the contact's
// "rels" time). Same property + same type: one relation; same property + different
// types: the most recent wins and the others are reported as replaced.
func mergeProperties(p, s *mergeContact, addresses map[string]string) []mergeRel {
	type cand struct {
		kind, raw, from string
		at              time.Time
		atRaw           string
		order           int
	}
	groups := map[string][]cand{}
	pids := []string{}
	add := func(pid string, c cand) {
		if _, ok := groups[pid]; !ok {
			pids = append(pids, pid)
		}
		c.order = len(groups[pid])
		groups[pid] = append(groups[pid], c)
	}
	for _, c := range []*mergeContact{p, s} {
		for _, r := range c.Props {
			at := mergeTime(r.Updated)
			if at.IsZero() {
				at = mergeTime(c.Updated)
			}
			add(r.PropertyID, cand{kind: relKind(r.Relation), raw: r.Relation, from: c.ID, at: at, atRaw: at.UTC().Format(layout)})
		}
		list, _ := c.Data["rels"].([]any)
		for _, v := range list {
			r := imap(v)
			if pid := str(r, "propertyId"); pid != "" {
				at := c.fieldTime("rels")
				add(pid, cand{kind: relKind(str(r, "type")), raw: relKind(str(r, "type")), from: c.ID, at: at, atRaw: at.UTC().Format(layout)})
			}
		}
	}
	out := []mergeRel{}
	for _, pid := range pids {
		list := groups[pid]
		best := list[0]
		for _, c := range list[1:] {
			if c.at.After(best.at) || (c.at.Equal(best.at) && best.from != p.ID && c.from == p.ID) {
				best = c
			}
		}
		r := mergeRel{PropertyID: pid, Address: addresses[pid], Relation: best.kind, Label: relLabel(best.kind), Raw: best.raw, At: best.atRaw, From: best.from}
		seen := map[string]bool{best.kind: true}
		primaryHad := ""
		for _, c := range list {
			if c.from == p.ID && primaryHad == "" {
				primaryHad = c.kind
			}
			if !seen[c.kind] {
				seen[c.kind] = true
				r.Replaced = append(r.Replaced, mergeReplaced{Relation: c.kind, Label: relLabel(c.kind), From: c.from})
			}
		}
		r.Changed = primaryHad != best.kind
		out = append(out, r)
	}
	// Non-conflicting relations are appended in time order.
	sort.SliceStable(out, func(i, j int) bool { return mergeTime(out[i].At).Before(mergeTime(out[j].At)) })
	return out
}

func mergeLogNote(plan *mergePlan, s *mergeContact, actor, day string) string {
	other := contactLabel(s.Data)
	en := []string{fmt.Sprintf("Merged with %s by %s on %s.", other, actor, day)}
	zh := []string{fmt.Sprintf("由 %s 于 %s 与 %s 合并。", actor, day, other)}
	for _, r := range plan.Properties {
		where := r.Address
		if where == "" {
			where = r.PropertyID
		}
		for _, x := range r.Replaced {
			en = append(en, fmt.Sprintf("Relation to %s changed from %s to %s by merge.", where, x.Label, r.Label))
			zh = append(zh, fmt.Sprintf("与 %s 的关系因合并由 %s 改为 %s。", where, x.Label, r.Label))
		}
	}
	return strings.Join(en, " ") + "\n" + strings.Join(zh, "")
}

func loadMergePlan(q querier, aID, bID, actor string, at time.Time) (*mergePlan, error) {
	a, err := loadMergeContact(q, aID)
	if err != nil {
		return nil, err
	}
	b, err := loadMergeContact(q, bID)
	if err != nil {
		return nil, err
	}
	addresses := map[string]string{}
	for _, c := range []*mergeContact{a, b} {
		ids := []string{}
		for _, r := range c.Props {
			ids = append(ids, r.PropertyID)
		}
		list, _ := c.Data["rels"].([]any)
		for _, v := range list {
			if pid := str(imap(v), "propertyId"); pid != "" {
				ids = append(ids, pid)
			}
		}
		for _, pid := range ids {
			if _, ok := addresses[pid]; ok {
				continue
			}
			var no, street, suburb string
			_ = q.QueryRow(`SELECT COALESCE(no,''),COALESCE(street,''),COALESCE(suburb,'') FROM properties WHERE id=?`, pid).Scan(&no, &street, &suburb)
			addresses[pid] = strings.TrimSpace(strings.TrimSpace(no+" "+street) + map[bool]string{true: ", " + suburb, false: ""}[suburb != "" && street != ""])
		}
	}
	return computeMerge(a, b, addresses, actor, at), nil
}

// executeMerge applies the plan inside tx. hash=="" skips the preview check
// (legacy approvals decided by an admin). Only tx is used: never a.db here.
func (a *app) executeMerge(tx *sql.Tx, p principal, aID, bID, hash string) (*mergePlan, error) {
	at := time.Now().UTC()
	actor := p.name
	if actor == "" {
		actor = "Agent"
	}
	plan, err := loadMergePlan(tx, aID, bID, actor, at)
	if err != nil {
		return nil, err
	}
	if hash != "" && hash != plan.PreviewHash {
		return plan, errMergeStale
	}
	stamp := at.Format(layout)
	primary, secondary := plan.PrimaryID, plan.SecondaryID
	data := plan.data
	b, _ := json.Marshal(data)
	res, err := tx.Exec(`UPDATE contacts SET data=?,first=?,last=?,mobile=?,email=?,updated_at=?,version=version+1,roles=?,tags=?,flags=?,consent=? WHERE id=? AND version=? AND deleted_at IS NULL`, append([]any{string(b), str(data, "first"), str(data, "last"), str(data, "mobile"), str(data, "email"), stamp}, append(contactColumns(data), primary, plan.primary.Version)...)...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return plan, errMergeStale
	}
	// Properties of either contact change their ownerIds on the phones.
	if _, err = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1 WHERE id IN (SELECT property_id FROM property_owners WHERE contact_id IN (?,?))`, stamp, primary, secondary); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`DELETE FROM property_owners WHERE contact_id IN (?,?)`, primary, secondary); err != nil {
		return nil, err
	}
	for _, r := range plan.Properties {
		var exists int
		if err = tx.QueryRow(`SELECT count(*) FROM properties WHERE id=?`, r.PropertyID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			continue // rels entry for a property this server does not hold
		}
		raw := r.Raw
		if raw == "" {
			raw = r.Relation
		}
		if _, err = tx.Exec(`INSERT INTO property_owners(property_id,contact_id,relation,updated_at) VALUES(?,?,?,?)`, r.PropertyID, primary, raw, r.At); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`UPDATE properties SET updated_at=?,version=version+1 WHERE id=?`, stamp, r.PropertyID); err != nil {
			return nil, err
		}
	}
	for _, table := range []string{"contact_notes", "todos", "leads", "ofi_attendees", "referrals"} {
		if _, err = tx.Exec(`UPDATE `+table+` SET contact_id=? WHERE contact_id=?`, primary, secondary); err != nil {
			return nil, err
		}
	}
	noteID := random()[:26]
	if _, err = tx.Exec(`INSERT INTO contact_notes(id,contact_id,user_id,d,a,t,created_at) VALUES(?,?,?,?,?,?,?)`, noteID, primary, nullable(p.id), plan.logD, plan.logA, plan.LogNote, stamp); err != nil {
		return nil, err
	}
	ref := map[string]any{"kind": "merge", "primaryId": primary, "secondaryId": secondary}
	for _, table := range []string{"todos", "leads", "ofi_attendees", "referrals"} {
		rows, qerr := tx.Query(`SELECT id FROM `+table+` WHERE instr(data,?)>0`, `"`+secondary+`"`)
		if qerr != nil {
			return nil, qerr
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		if _, err = tx.Exec(`UPDATE `+table+` SET data=replace(data,?,?),updated_at=?,version=version+1 WHERE instr(data,?)>0`, `"`+secondary+`"`, `"`+primary+`"`, stamp, `"`+secondary+`"`); err != nil {
			return nil, err
		}
		for _, id := range ids {
			if err = a.record(tx, p, "merge-reference", table, id, ref); err != nil {
				return nil, err
			}
		}
	}
	// Other contacts' relationship entries pointing at the merged-away record.
	if _, err = tx.Exec(`UPDATE contacts SET data=replace(data,?,?),updated_at=?,version=version+1 WHERE id NOT IN (?,?) AND deleted_at IS NULL AND instr(data,?)>0`, `"contactId":"`+secondary+`"`, `"contactId":"`+primary+`"`, stamp, primary, secondary, `"contactId":"`+secondary+`"`); err != nil {
		return nil, err
	}
	if err = clearMergedDuplicateFlags(tx, primary, secondary, stamp); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE contacts SET deleted_at=?,updated_at=?,version=version+1 WHERE id=?`, stamp, stamp, secondary); err != nil {
		return nil, err
	}
	// Audit only (no undo): both records exactly as they were before the merge.
	audit := map[string]any{"kind": "merge", "primaryId": primary, "secondaryId": secondary, "previewHash": plan.PreviewHash, "result": plan, "before": map[string]any{"primary": plan.primary, "secondary": plan.secondary}, "noteId": noteID}
	for _, row := range [][2]string{{"merge", primary}, {"merged-into", secondary}} {
		if _, err = tx.Exec(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, p.id, p.device, row[0], "contacts", row[1], ijson(audit), stamp); err != nil {
			return nil, err
		}
	}
	if plan.Rex != "none" {
		rels := []any{}
		for _, r := range plan.Properties {
			if !r.Changed && len(r.Replaced) == 0 {
				continue
			}
			replaces := []string{}
			for _, x := range r.Replaced {
				replaces = append(replaces, x.Relation)
			}
			rels = append(rels, map[string]any{"propertyId": r.PropertyID, "kind": r.Relation, "replaces": replaces})
		}
		extra := map[string]any{"deadline": at.Add(mergeLinkWait).Format(layout), "dnc": plan.DNC["value"], "relations": rels, "note": map[string]any{"id": noteID, "contactId": primary, "d": plan.logD, "a": plan.logA, "t": plan.LogNote}}
		if err = enqueueRexMerge(tx, "merge:"+primary+":"+secondary, map[string]any{"primaryId": primary, "secondaryId": secondary, "rexPlan": extra}); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// mergeRoute: POST /contacts/merge-preview {aId,bId}; POST /contacts/{id}/merge {secondaryId,previewHash}.
func (a *app) mergeRoute(w http.ResponseWriter, r *http.Request, p principal, aID, bID string, preview bool, hash string) {
	if aID == "" || bID == "" || aID == bID {
		problem(w, 400, "two different contacts required")
		return
	}
	// Both must be visible to the caller: agents merge their own, admins any.
	if !a.validRequest(map[string]any{"kind": "merge", "primaryId": aID, "secondaryId": bID}, p) {
		problem(w, 404, "not found")
		return
	}
	if preview {
		plan, err := loadMergePlan(a.db, aID, bID, p.name, time.Now().UTC())
		if err != nil {
			problem(w, 404, "not found")
			return
		}
		respond(w, 200, plan)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	if hash == "" {
		hash = "required"
	}
	plan, err := a.executeMerge(tx, p, aID, bID, hash)
	if errors.Is(err, errMergeStale) {
		_ = tx.Rollback()
		respond(w, 409, map[string]any{"error": "preview changed; review the merge again", "preview": plan})
		return
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, 409, "conflict")
		return
	}
	out := map[string]any{"ok": true, "primaryId": plan.PrimaryID, "secondaryId": plan.SecondaryID}
	if c, e := a.get("contacts", plan.PrimaryID, principal{id: p.id, role: "admin"}, false); e == nil {
		out["contact"] = c
	}
	respond(w, 200, out)
}
