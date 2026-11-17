package main

import (
	"strings"
	"testing"
)

func TestRexWebhook(t *testing.T) {
	a, _ := setup(t)
	tok := strings.Repeat("a", 40)
	t.Setenv("AXLEO_REX_WEBHOOK_TOKEN", tok)
	body := map[string]any{"events": []any{map[string]any{"event": "contacts.updated", "context": map[string]any{"record_id": 123, "name": "secret"}}}}
	if w := request(a, "POST", "/api/v1/rex/webhook/"+tok, "", body); w.Code != 202 {
		t.Fatalf("good token: %d %s", w.Code, w.Body)
	}
	if w := request(a, "POST", "/api/v1/rex/webhook/"+strings.Repeat("b", 40), "", body); w.Code != 404 {
		t.Fatalf("bad token: %d", w.Code)
	}
	t.Setenv("AXLEO_REX_WEBHOOK_TOKEN", "")
	if w := request(a, "POST", "/api/v1/rex/webhook/", "", body); w.Code == 202 {
		t.Fatal("unset token must not accept")
	}
	s := rexWebhookSummary([]byte(`{"events":[{"event":"contacts.updated","context":{"record_id":67628506,"name":"secret"}}]}`))
	if !strings.Contains(s, "record_id=67628506") || !strings.Contains(s, "event=contacts.updated") || strings.Contains(s, "secret") {
		t.Fatalf("summary: %s", s)
	}
}
