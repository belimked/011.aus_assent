package main

import (
	"encoding/json"
	"log"
	"time"
)

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

func (a *app) auditCall(p principal, action string, metadata map[string]any) {
	diff, _ := json.Marshal(metadata)
	if _, err := a.db.Exec(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, p.id, p.device, action, action, "", string(diff), now()); err != nil {
		log.Printf("%s audit failed: %v", action, err)
	}
}
