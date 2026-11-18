package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func audioRequest(a *app, token string, audio []byte) *httptest.ResponseRecorder {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	f, _ := mw.CreateFormFile("audio", "test.wav")
	_, _ = f.Write(audio)
	_ = mw.WriteField("lang", "en")
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/v1/stt/transcribe", &b)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}
func testWAV(seconds int) []byte {
	b := make([]byte, 44+seconds*32000)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 16000)
	binary.LittleEndian.PutUint32(b[28:], 32000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(len(b)-44))
	return b
}
func TestSTTEndpoints(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "")
	if w := audioRequest(a, tokens["mary"], testWAV(1)); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_AZURE_SPEECH_KEY", "secret")
	attempts := 0
	azure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "secret" {
			t.Error("missing auth")
		}
		_ = r.ParseMultipartForm(1 << 20)
		var def map[string]any
		_ = json.Unmarshal([]byte(r.FormValue("definition")), &def)
		if def["locales"].([]any)[0] != "en-AU" {
			t.Error(def)
		}
		if attempts == 1 {
			if def["phraseList"] == nil {
				t.Error("missing phraseList")
			}
			http.Error(w, "phraseList rejected secret", 400)
			return
		}
		if def["phraseList"] != nil {
			t.Error("retry kept phraseList")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"combinedPhrases":[{"text":"hello"},{"text":"world"}]}`))
	}))
	defer azure.Close()
	azureFastBaseURL = azure.URL
	t.Cleanup(func() { azureFastBaseURL = "" })
	w := audioRequest(a, tokens["mary"], testWAV(1))
	if w.Code != 200 || obj(t, w)["text"] != "hello world" || attempts != 2 {
		t.Fatalf("%d %s attempts=%d", w.Code, w.Body, attempts)
	}
	if w = audioRequest(a, tokens["mary"], testWAV(121)); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = audioRequest(a, tokens["mary"], make([]byte, 10<<20)); w.Code != 400 {
		t.Fatal(w.Code)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret upstream failure", 500) }))
	defer failed.Close()
	azureFastBaseURL = failed.URL
	if w = audioRequest(a, tokens["mary"], testWAV(1)); w.Code != 502 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body)
	}
	var diff string
	_ = a.db.QueryRow(`SELECT diff FROM audit_log WHERE action='stt' ORDER BY id DESC LIMIT 1`).Scan(&diff)
	if strings.Contains(diff, "hello") || strings.Contains(diff, "secret") {
		t.Fatal(diff)
	}
}
func TestAIEndpoints(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_BEDROCK_API_KEY", "")
	data := map[string]any{"text": "Fictional buyer Mary 0412 345 678", "ctx": "oh"}
	if w := request(a, "POST", "/api/v1/ai/extract", tokens["mary"], data); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_API_KEY", "secret")
	t.Setenv("AXLEO_BEDROCK_MODEL", "global.anthropic.bad")
	if w := request(a, "POST", "/api/v1/ai/extract", tokens["mary"], data); w.Code != 503 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_MODEL", "au.anthropic.test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || !strings.Contains(r.URL.Path, "/model/au.anthropic.test/converse") {
			t.Error("request auth/path")
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		config := payload["toolConfig"].(map[string]any)
		if config["toolChoice"].(map[string]any)["tool"].(map[string]any)["name"] != "record_contact" {
			t.Error("tool not forced")
		}
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"toolUse":{"name":"record_contact","input":{"contactName":"Mary","firstName":"Mary","phone":"0412 345 678","confidence":{"contactName":0.9}}}}]}},"usage":{"inputTokens":12,"outputTokens":3}}`))
	}))
	defer server.Close()
	bedrockBaseURL = server.URL
	t.Cleanup(func() { bedrockBaseURL = "" })
	w := request(a, "POST", "/api/v1/ai/extract", tokens["mary"], data)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	result := obj(t, w)
	if result["fields"].(map[string]any)["lastName"] != nil || result["confidence"].(map[string]any)["phone"] != float64(0) {
		t.Fatal(result)
	}
	if w = request(a, "POST", "/api/v1/ai/extract", tokens["mary"], map[string]any{"text": strings.Repeat("x", 4001)}); w.Code != 400 {
		t.Fatal(w.Code)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret upstream failure", 500) }))
	defer failed.Close()
	bedrockBaseURL = failed.URL
	w = request(a, "POST", "/api/v1/ai/extract", tokens["mary"], data)
	if w.Code != 502 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body)
	}
	var diff string
	_ = a.db.QueryRow(`SELECT diff FROM audit_log WHERE action='ai_extract' ORDER BY id DESC LIMIT 1`).Scan(&diff)
	if strings.Contains(diff, "Mary") {
		t.Fatal(diff)
	}
}
