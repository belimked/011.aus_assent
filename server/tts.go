package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var azureTTSBaseURL = "" // test-only override
var ttsHTTPClient = &http.Client{Timeout: 15 * time.Second}

func (a *app) synthesize(w http.ResponseWriter, r *http.Request, p principal) {
	started := time.Now()
	meta := map[string]any{"latencyMs": 0, "chars": 0, "status": 503}
	defer func() { meta["latencyMs"] = time.Since(started).Milliseconds(); a.auditCall(p, "tts", meta) }()
	fail := func(code int, msg string) { meta["status"] = code; problem(w, code, msg) }
	if os.Getenv("AXLEO_AZURE_SPEECH_KEY") == "" {
		fail(503, "tts not configured")
		return
	}
	m, err := body(r)
	if err != nil {
		fail(400, "invalid JSON")
		return
	}
	text, ok := m["text"].(string)
	text = strings.TrimSpace(text)
	lang := str(m, "lang")
	voices := map[string][2]string{
		"en":  {"en-AU", "en-AU-NatashaNeural"},
		"zh":  {"zh-CN", "zh-CN-XiaoxiaoNeural"},
		"yue": {"zh-HK", "zh-HK-HiuMaanNeural"},
	}
	voice, valid := voices[lang]
	if !ok || !utf8.ValidString(text) || utf8.RuneCountInString(text) < 1 || utf8.RuneCountInString(text) > 500 || !valid {
		fail(400, "invalid text or lang")
		return
	}
	meta["lang"], meta["chars"] = lang, utf8.RuneCountInString(text)
	var escaped bytes.Buffer
	if xml.EscapeText(&escaped, []byte(text)) != nil {
		fail(400, "invalid text")
		return
	}
	ssml := `<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="` + voice[0] + `"><voice name="` + voice[1] + `">` + escaped.String() + `</voice></speak>`
	base := azureTTSBaseURL
	if base == "" {
		base = "https://" + env("AXLEO_AZURE_SPEECH_REGION", "australiaeast") + ".tts.speech.microsoft.com"
	}
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(r.Context(), "POST", base+"/cognitiveservices/v1", strings.NewReader(ssml))
		if err != nil {
			fail(502, "tts upstream error")
			return
		}
		req.Header.Set("Ocp-Apim-Subscription-Key", os.Getenv("AXLEO_AZURE_SPEECH_KEY"))
		req.Header.Set("Content-Type", "application/ssml+xml")
		req.Header.Set("X-Microsoft-OutputFormat", "audio-24khz-48kbitrate-mono-mp3")
		req.Header.Set("User-Agent", "axleo")
		resp, err := ttsHTTPClient.Do(req)
		if err != nil {
			fail(502, "tts upstream error")
			return
		}
		if resp.StatusCode == 429 {
			retry := resp.Header.Get("Retry-After")
			resp.Body.Close()
			if attempt == 0 {
				wait := time.Second
				if n, err := strconv.Atoi(retry); err == nil {
					wait = time.Duration(n) * time.Second
				} else if date, err := http.ParseTime(retry); err == nil {
					wait = time.Until(date)
				}
				if wait < 0 {
					wait = 0
				}
				if wait > 3*time.Second {
					wait = 3 * time.Second
				}
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-r.Context().Done():
					timer.Stop()
					fail(502, "tts upstream error")
					return
				}
				continue
			}
			w.Header().Set("Retry-After", retry)
			fail(429, "tts rate limited")
			return
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			fail(502, "tts upstream error")
			return
		}
		mp3, err := io.ReadAll(io.LimitReader(resp.Body, (10<<20)+1))
		resp.Body.Close()
		if err != nil || len(mp3) == 0 || len(mp3) > 10<<20 {
			fail(502, "tts upstream error")
			return
		}
		meta["status"] = 200
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(200)
		_, _ = w.Write(mp3)
		return
	}
}
