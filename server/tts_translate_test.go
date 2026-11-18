package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTTS(t *testing.T) {
	a, tokens := setup(t)
	data := map[string]any{"text": "  <&>  ", "lang": "en"}
	if w := request(a, "POST", "/api/v1/tts", "", data); w.Code != 401 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "")
	if w := request(a, "POST", "/api/v1/tts", tokens["mary"], data); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "test-key")
	for _, d := range []map[string]any{{"text": " ", "lang": "en"}, {"text": strings.Repeat("界", 501), "lang": "zh"}, {"text": "hi", "lang": "xx"}, {"text": 3, "lang": "en"}} {
		if w := request(a, "POST", "/api/v1/tts", tokens["mary"], d); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	attempts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		b, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/cognitiveservices/v1" || r.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" || r.Header.Get("Content-Type") != "application/ssml+xml" || r.Header.Get("X-Microsoft-OutputFormat") != "audio-24khz-48kbitrate-mono-mp3" || r.Header.Get("User-Agent") != "axleo" || !strings.Contains(string(b), "&lt;&amp;&gt;") || !strings.Contains(string(b), "en-AU-NatashaNeural") {
			t.Errorf("bad upstream request (body omitted)")
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte("ID3fake-mp3"))
	}))
	defer upstream.Close()
	azureTTSBaseURL = upstream.URL
	t.Cleanup(func() { azureTTSBaseURL = "" })
	w := request(a, "POST", "/api/v1/tts", tokens["mary"], data)
	if w.Code != 200 || w.Header().Get("Content-Type") != "audio/mpeg" || w.Body.String() != "ID3fake-mp3" || attempts != 2 {
		t.Fatal(w.Code, attempts)
	}
	var diff string
	_ = a.db.QueryRow(`SELECT diff FROM audit_log WHERE action='tts' ORDER BY id DESC LIMIT 1`).Scan(&diff)
	if strings.Contains(diff, "<&>") || !strings.Contains(diff, `"status":200`) || !strings.Contains(diff, `"chars":3`) {
		t.Fatal(diff)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer failed.Close()
	azureTTSBaseURL = failed.URL
	if w = request(a, "POST", "/api/v1/tts", tokens["mary"], data); w.Code != 502 {
		t.Fatal(w.Code)
	}
}

func TestTranslate(t *testing.T) {
	a, tokens := setup(t)
	data := map[string]any{"text": "  Fictional <&> address 12 Test St $123  ", "from": "en", "to": "yue"}
	if w := request(a, "POST", "/api/v1/ai/translate", "", data); w.Code != 401 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_API_KEY", "")
	if w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], data); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_API_KEY", "test-key")
	t.Setenv("AXLEO_BEDROCK_MODEL", "global.bad")
	if w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], data); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_MODEL", "au.test")
	for _, d := range []map[string]any{{"text": " ", "from": "en", "to": "zh"}, {"text": strings.Repeat("界", 1001), "from": "en", "to": "zh"}, {"text": "hi", "from": "bad", "to": "zh"}, {"text": "hi", "from": "en", "to": "bad"}} {
		if w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], d); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	w := request(a, "POST", "/api/v1/ai/translate", tokens["mary"], map[string]any{"text": "  unchanged  ", "from": "en", "to": "en"})
	if w.Code != 200 || obj(t, w)["text"] != "unchanged" {
		t.Fatal(w.Code, w.Body)
	}
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if r.Header.Get("Authorization") != "Bearer test-key" || r.URL.Path != "/model/au.test/converse" || payload["toolConfig"] != nil || payload["inferenceConfig"].(map[string]any)["temperature"] != float64(0) {
			t.Error("bad upstream request")
		}
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"text":"譯文"}]}},"usage":{"inputTokens":12,"outputTokens":3}}`))
	}))
	defer upstream.Close()
	bedrockBaseURL = upstream.URL
	t.Cleanup(func() { bedrockBaseURL = "" })
	w = request(a, "POST", "/api/v1/ai/translate", tokens["mary"], data)
	if w.Code != 200 || obj(t, w)["text"] != "譯文" || calls != 1 {
		t.Fatal(w.Code, w.Body)
	}
	var diff string
	_ = a.db.QueryRow(`SELECT diff FROM audit_log WHERE action='ai_translate' ORDER BY id DESC LIMIT 1`).Scan(&diff)
	if strings.Contains(diff, "Fictional") || strings.Contains(diff, "譯文") || !strings.Contains(diff, `"inputTokens":12`) {
		t.Fatal(diff)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer failed.Close()
	bedrockBaseURL = failed.URL
	if w = request(a, "POST", "/api/v1/ai/translate", tokens["mary"], data); w.Code != 502 {
		t.Fatal(w.Code)
	}
}
