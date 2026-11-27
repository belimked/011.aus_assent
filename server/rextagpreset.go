package main

// UAT3: a tag added in the app also joins Rex's preset tag list (AdminTags),
// so it shows up in Rex's tag picker. Writing the tag to the contact
// (Contacts::update related.contact_tags, rexworker.go) never depends on this:
// any failure here is recorded on the outbox row and the contact write goes on.
//
// AdminTags::search is verified in sandbox (2026-11-17, rexprobe "tags"): criteria
// {name:"tag", value}, row fields id, object_type, tag, cached_count.
// NOT sandbox-verified (2026-11-27): AdminTags::create and its data shape
// {tag, object_type:"contact"} (docs only list create(data, return_id)); whether the
// search's tag criterion is case-insensitive; the exact object_type value. Check
// with `axleo rex-probe --only TP` (read-only) after deploying.
//
// REX_TAG_PRESET=0 turns this off (contact tags are then written as before).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const rexTagPresetTTL = time.Hour

func rexTagPresetEnabled() bool { return os.Getenv("REX_TAG_PRESET") != "0" }

func rexTagPresetSearchBody(tag string) map[string]any {
	return map[string]any{"criteria": []any{map[string]any{"name": "tag", "value": tag}}, "limit": 20}
}
func rexTagPresetCreateBody(tag string) map[string]any {
	return map[string]any{"data": map[string]any{"tag": tag, "object_type": "contact"}, "return_id": true}
}

// Preset tag names known per Rex account, lower-cased, with an expiry. Only
// positive results are cached: a missing or failed lookup is retried next job.
type rexTagPresetCache struct {
	mu    sync.Mutex
	known map[string]map[string]time.Time
}

var rexTagPresets = &rexTagPresetCache{known: map[string]map[string]time.Time{}}

func rexTagKey(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }
func (c *rexTagPresetCache) has(account, tag string, at time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp, ok := c.known[account][rexTagKey(tag)]
	return ok && at.Before(exp)
}
func (c *rexTagPresetCache) add(account, tag string, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.known[account] == nil {
		c.known[account] = map[string]time.Time{}
	}
	c.known[account][rexTagKey(tag)] = at.Add(rexTagPresetTTL)
}
func (c *rexTagPresetCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.known = map[string]map[string]time.Time{}
}

// rexTagPresetRows accepts a search result as {rows:[...]} or a bare list.
func rexTagPresetRows(raw json.RawMessage) []map[string]any {
	var v any
	_ = json.Unmarshal(raw, &v)
	if m, ok := v.(map[string]any); ok {
		for _, k := range []string{"rows", "items", "data", "results"} {
			if a, ok := m[k].([]any); ok {
				v = a
				break
			}
		}
	}
	a, _ := v.([]any)
	return sliceMaps(a)
}

// Contact tags only; object_type may come back as "contact" or {id:"contact"} (unverified).
func rexTagPresetIsContact(row map[string]any) bool {
	t := row["object_type"]
	if m, ok := t.(map[string]any); ok {
		t = m["id"]
	}
	s, _ := t.(string)
	return s == "" || strings.EqualFold(s, "contact")
}

func rexTagPresetReason(env rexEnvelope, err error) string {
	var rate rexRateError
	switch {
	case errors.As(err, &rate):
		return "Rex rate limited"
	case len(env.Error) > 0 && string(env.Error) != "null":
		return safeRexError(env.Error)
	case err != nil:
		return err.Error()
	}
	return "unknown error"
}

// ensureRexTagPreset makes sure tag is in the account's AdminTags list (lookup,
// then create when missing, case-insensitive). It never fails the job: it
// returns a short note for the outbox row ("" when the tag was already known).
// Called with no DB rows or transaction open (single write connection).
func (w *rexWorker) ensureRexTagPreset(ctx context.Context, tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" || !rexTagPresetEnabled() || rexTagPresets.has(w.account, tag, time.Now()) {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	fail := func(step string, env rexEnvelope, err error) string {
		note := "Rex tag list " + step + " failed (tag still written to contact): " + rexTagPresetReason(env, err)
		log.Printf("rex worker: tag preset %q account %s: %s", tag, w.account, note)
		return note
	}
	_, env, err := w.client.Call(ctx, "AdminTags", "search", rexTagPresetSearchBody(tag))
	if err != nil {
		return fail("lookup", env, err)
	}
	found := false
	for _, row := range rexTagPresetRows(env.Result) {
		name := strings.TrimSpace(str(row, "tag"))
		if name == "" || !rexTagPresetIsContact(row) {
			continue
		}
		rexTagPresets.add(w.account, name, time.Now())
		found = found || strings.EqualFold(name, tag)
	}
	if found {
		return ""
	}
	// UNVERIFIED in sandbox: AdminTags::create body. A lost response may still have
	// created it; the next lookup then finds it, so no duplicate is created.
	_, env, err = w.client.Call(ctx, "AdminTags", "create", rexTagPresetCreateBody(tag))
	if err != nil {
		return fail("create", env, err)
	}
	rexTagPresets.add(w.account, tag, time.Now())
	return "added to Rex tag list"
}

// probeTagPreset (rex-probe TP, read-only): lists a few AdminTags rows, then runs
// the worker's lookup with the first contact tag upper-cased to show whether the
// tag criterion is case-insensitive. Never calls AdminTags::create.
func (p *probeRun) probeTagPreset(call func(string) (json.RawMessage, error)) (string, string) {
	rows := func(raw json.RawMessage) []map[string]any {
		var env rexEnvelope
		_ = json.Unmarshal(raw, &env)
		return rexTagPresetRows(env.Result)
	}
	list, e1 := call("tagList")
	_, e2 := call("tagFields")
	first, hasTag, objectType := "", false, "无"
	for _, row := range rows(list) {
		if _, ok := row["tag"].(string); ok {
			hasTag = true
		}
		if row["object_type"] != nil && objectType == "无" {
			objectType = fmt.Sprintf("%T", row["object_type"])
		}
		if name := strings.TrimSpace(str(row, "tag")); name != "" && first == "" && rexTagPresetIsContact(row) {
			first = name
		}
	}
	caseless, e3 := "未检查（列表里没有联系人标签）", error(nil)
	if first != "" {
		p.vars["presetTag"] = strings.ToUpper(first)
		var raw json.RawMessage
		raw, e3 = call("tagLookup")
		found := false
		for _, row := range rows(raw) {
			found = found || strings.EqualFold(str(row, "tag"), first)
		}
		caseless = fmt.Sprint(found)
	}
	verdict := "UNKNOWN"
	if e1 == nil && e3 == nil && hasTag {
		verdict = "PASS"
	}
	return verdict, fmt.Sprintf("AdminTags 行有 tag 字段：%v；object_type 类型：%s；按大写名查找能找到原标签（大小写不敏感）：%s；AdminTags::create 未调用、未在沙箱验证；错误：%s", hasTag, objectType, caseless, probeError(errors.Join(e1, e2, e3), p.c))
}
