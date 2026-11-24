package main

import (
	"encoding/base64"
	"net/http"
	"os"
	"time"
)

func (a *app) imageAI(w http.ResponseWriter, r *http.Request, p principal) {
	started := time.Now()
	meta := map[string]any{"latencyMs": 0, "inputTokens": 0, "outputTokens": 0}
	defer func() { meta["latencyMs"] = time.Since(started).Milliseconds(); a.auditCall(p, "ai_image", meta) }()
	key := os.Getenv("AXLEO_BEDROCK_API_KEY")
	model := env("AXLEO_BEDROCK_MODEL", "au.anthropic.claude-haiku-4-5-20251001-v1:0")
	if key == "" || !auBedrockModel(model) {
		problem(w, 503, "ai not configured")
		return
	}
	data, mime, code := readImage(r, 5<<20)
	if code != 0 {
		problem(w, code, "invalid image")
		return
	}
	ctx := r.FormValue("ctx")
	if ctx != "card" && ctx != "note" && ctx != "enq" {
		problem(w, 400, "invalid ctx")
		return
	}
	lang := r.FormValue("lang")
	meta["lang"] = lang
	format := "jpeg"
	if mime == "image/png" {
		format = "png"
	}
	prompt := "Extract real estate contact facts visible in the image. Context: card=business card, note=handwritten or printed note, enq=chat/SMS screenshot of an enquiry. For screenshots, the enquirer is the person asking, not the agent. Put referral source, agency and any other useful facts into notes. Never invent information; unknown fields must be null." + audRule + " priceMin/priceMax are whole Australian dollars. In notes, write money as Australian dollars ($ in English, 澳元 in Chinese). Return only the record_contact tool input."
	content := []any{
		map[string]any{"image": map[string]any{"format": format, "source": map[string]any{"bytes": base64.StdEncoding.EncodeToString(data)}}},
		map[string]any{"text": "Context: " + ctx + "; language: " + lang},
	}
	a.converseContact(w, r, key, model, prompt, content, started, meta)
}
