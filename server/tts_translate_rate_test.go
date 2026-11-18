package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewEndpointRateLimits(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_BEDROCK_API_KEY", "test-key")
	t.Setenv("AXLEO_BEDROCK_MODEL", "au.test")
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "test-key")
	translation := map[string]any{"text": "hello", "from": "en", "to": "en"}
	for i := 0; i < 30; i++ {
		if w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], translation); w.Code != 200 {
			t.Fatal(i, w.Code)
		}
	}
	if w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], translation); w.Code != 429 {
		t.Fatal(w.Code)
	}
	// TTS and the old STT/extract bucket must remain available.
	if w := request(a, "POST", "/api/v1/tts", tokens["mary"], map[string]any{"text": ""}); w.Code != 400 {
		t.Fatal(w.Code)
	}
	for i := 1; i < 60; i++ {
		if w := request(a, "POST", "/api/v1/tts", tokens["mary"], map[string]any{"text": ""}); w.Code != 400 {
			t.Fatal(i, w.Code)
		}
	}
	if w := request(a, "POST", "/api/v1/tts", tokens["mary"], map[string]any{"text": ""}); w.Code != 429 {
		t.Fatal(w.Code)
	}
	if w := request(a, "POST", "/api/v1/ai/extract", tokens["mary"], map[string]any{"text": ""}); w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestTTSUpstreamRateLimit(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "test-key")
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
	}))
	defer upstream.Close()
	azureTTSBaseURL = upstream.URL
	t.Cleanup(func() { azureTTSBaseURL = "" })
	if w := request(a, "POST", "/api/v1/tts", tokens["mary"], map[string]any{"text": "hello", "lang": "en"}); w.Code != 429 || calls != 2 {
		t.Fatal(w.Code, calls)
	}
}
