package main

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// audRule makes every AI step treat money as Australian dollars (the app is used by Brisbane agents).
const audRule = " Currency: all money is Australian dollars (AUD) unless the speaker explicitly names another currency; never convert between currencies or change an amount. In Chinese speech, 元/块/刀 and bare 万 amounts mean Australian dollars; 万 = 10,000 and 亿 = 100,000,000 (120万 = 1.2 million)."

func (a *app) allowCall(p principal) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calls == nil {
		a.calls = make(map[string][]time.Time)
	}
	cutoff := time.Now().Add(-time.Minute)
	recent := a.calls[p.id][:0]
	for _, t := range a.calls[p.id] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 30 {
		a.calls[p.id] = recent
		return false
	}
	a.calls[p.id] = append(recent, time.Now())
	return true
}

// allowEndpointCall keeps new speech/translation limits independent of the legacy STT/extract bucket.
func (a *app) allowEndpointCall(p principal, endpoint string, limit int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calls == nil {
		a.calls = make(map[string][]time.Time)
	}
	key := endpoint + ":" + p.id
	now := time.Now()
	recent := a.calls[key][:0]
	for _, t := range a.calls[key] {
		if t.After(now.Add(-time.Minute)) {
			recent = append(recent, t)
		}
	}
	a.calls[key] = recent
	if len(recent) >= limit {
		return false
	}
	a.calls[key] = append(recent, now)
	return true
}

func (a *app) auditCall(p principal, action string, metadata map[string]any) {
	diff, _ := json.Marshal(metadata)
	if _, err := a.db.Exec(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, p.id, p.device, action, action, "", string(diff), now()); err != nil {
		log.Printf("%s audit failed: %v", action, err)
	}
}

// nullRule forbids the placeholder strings models sometimes emit for unknown facts.
const nullRule = " If a fact is not clearly stated, set that field to null (JSON null). Never output placeholder text such as <UNKNOWN>, UNKNOWN, unknown, N/A, none, null as a string, '-', '?', or any <...> token. A single surname or first name is still a name; a phone number is only what was actually spoken."

var aiPlaceholderTag = regexp.MustCompile(`<[^<>]*>`)
var aiPlaceholderWords = map[string]bool{"unknown": true, "n/a": true, "na": true, "null": true, "nil": true, "none": true, "-": true, "--": true, "?": true, "??": true, "not provided": true, "not stated": true, "not mentioned": true, "undefined": true}

// aiCleanString drops placeholder text a model used for "no value": listed
// words (case-insensitive) and any <...> token, e.g. "<UNKNOWN><UNKNOWN>.<UNKNOWN>".
// It returns "" when nothing real remains.
func aiCleanString(s string) string {
	t := strings.TrimSpace(s)
	if strings.Contains(t, "<") {
		t = strings.Join(strings.Fields(aiPlaceholderTag.ReplaceAllString(t, " ")), " ")
		t = strings.TrimFunc(t, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
	}
	if aiPlaceholderWords[strings.ToLower(t)] {
		return ""
	}
	return t
}
