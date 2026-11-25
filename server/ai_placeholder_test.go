package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAICleanString(t *testing.T) {
	for in, want := range map[string]string{
		"<UNKNOWN>": "", "<UNKNOWN><UNKNOWN>.<UNKNOWN>": "", " UNKNOWN ": "", "unknown": "", "N/A": "", "n/a": "",
		"null": "", "None": "", "-": "", "?": "", "<name>": "", "< unknown >": "", "<UNKNOWN> <UNKNOWN>": "",
		"Sawley": "Sawley", "Tom <UNKNOWN>": "Tom", "0412 345 678": "0412 345 678", "+61 412 345 678": "+61 412 345 678",
		"Nonesuch": "Nonesuch", "a@b.com": "a@b.com",
	} {
		if got := aiCleanString(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

// Placeholder strings from the model must reach the phone as null, and the
// prompt must ask for null instead of placeholders.
func TestAIExtractDropsPlaceholders(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_BEDROCK_API_KEY", "secret")
	t.Setenv("AXLEO_BEDROCK_MODEL", "au.anthropic.test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		prompt := payload["system"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(prompt, "<UNKNOWN>") || !strings.Contains(prompt, "null") {
			t.Error("prompt must forbid placeholders")
		}
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"toolUse":{"name":"record_contact","input":{"contactName":"<UNKNOWN>","firstName":"Sawley","lastName":"<UNKNOWN>","phone":"<UNKNOWN><UNKNOWN>.<UNKNOWN>","email":"N/A","address":"unknown","notes":"Keen to sell next month","timeframe":"-","confidence":{}}}}]}},"usage":{"inputTokens":1,"outputTokens":1}}`))
	}))
	defer server.Close()
	bedrockBaseURL = server.URL
	t.Cleanup(func() { bedrockBaseURL = "" })
	w := request(a, "POST", "/api/v1/ai/extract", tokens["mary"], map[string]any{"text": "Owner is Sawley, mobile is 67076507. Keep to sale next month.", "ctx": "dk"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	fields := obj(t, w)["fields"].(map[string]any)
	for _, k := range []string{"contactName", "lastName", "phone", "email", "address", "timeframe"} {
		if fields[k] != nil {
			t.Fatalf("%s should be null: %v", k, fields[k])
		}
	}
	if fields["firstName"] != "Sawley" || fields["notes"] != "Keen to sell next month" {
		t.Fatal(fields)
	}
}
