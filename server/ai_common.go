package main

import (
	"encoding/json"
	"log"
	"time"
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
