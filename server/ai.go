package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

var bedrockBaseURL = "" // test-only override
var contactKeys = []string{"contactName", "firstName", "lastName", "phone", "email", "address", "suburb", "intent", "priceMin", "priceMax", "timeframe", "notes", "language"}

func contactSchema() map[string]any {
	props, confidence := map[string]any{}, map[string]any{}
	for _, key := range contactKeys {
		field := map[string]any{"type": []string{"string", "null"}}
		if key == "priceMin" || key == "priceMax" {
			field["type"] = []string{"number", "null"}
		}
		if key == "intent" {
			field["enum"] = []any{"buy", "sell", "rent", "appraisal", "other", nil}
		}
		props[key] = field
		confidence[key] = map[string]any{"type": "number", "minimum": 0, "maximum": 1}
	}
	props["confidence"] = map[string]any{"type": "object", "properties": confidence, "required": contactKeys, "additionalProperties": false}
	required := append(append([]string{}, contactKeys...), "confidence")
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func (a *app) extractAI(w http.ResponseWriter, r *http.Request, p principal) {
	started := time.Now()
	meta := map[string]any{"latencyMs": 0, "inputTokens": 0, "outputTokens": 0}
	defer func() { meta["latencyMs"] = time.Since(started).Milliseconds(); a.auditCall(p, "ai_extract", meta) }()
	key := os.Getenv("AXLEO_BEDROCK_API_KEY")
	model := env("AXLEO_BEDROCK_MODEL", "au.anthropic.claude-haiku-4-5-20251001-v1:0")
	if key == "" || !strings.HasPrefix(model, "au.") {
		problem(w, 503, "ai not configured")
		return
	}
	m, err := body(r)
	if err != nil {
		problem(w, 400, "invalid JSON")
		return
	}
	text, ok := m["text"].(string)
	if !ok || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 4000 {
		problem(w, 400, "invalid text")
		return
	}
	ctx, _ := m["ctx"].(string)
	if ctx != "" && ctx != "dk" && ctx != "oh" && ctx != "enq" && ctx != "note" {
		problem(w, 400, "invalid ctx")
		return
	}
	lang, _ := m["lang"].(string)
	meta["lang"] = lang
	location, _ := time.LoadLocation("Australia/Brisbane")
	today := time.Now().In(location).Format("2006-01-02")
	prompt := "Extract real estate contact facts in English, Mandarin or Cantonese. Today in Australia/Brisbane is " + today + ". Context: dk=door knock owner/tenant, oh=open home attendee/buyer, enq=enquiry, note=free note. Australian mobile phones use 04xx xxx xxx. Never invent information; unknown fields must be null. For Chinese names, put the whole name in firstName if unsure how to split. Phone: output exactly the digits spoken; if the transcript has a mis-recognised extra or missing digit (an Australian mobile is 10 digits starting 04), keep it as heard and give phone confidence below 0.5. intent: decide from the speaker's words only (sell/may sell/thinking of selling = sell; looking to buy = buy); the context is a hint, not evidence; English words inside Chinese speech count (e.g. 'maybe sell'). notes: only extra facts the speaker said that no other field holds, in the speaker's language; null if none; never describe the context or the task. Return only the record_contact tool input."
	a.converseContact(w, r, key, model, prompt, []any{map[string]any{"text": "Context: " + ctx + "; language: " + lang + "\n" + text}}, started, meta)
}

// converseContact shares the forced tool call and result normalization across text and image extraction.
func (a *app) converseContact(w http.ResponseWriter, r *http.Request, key, model, prompt string, content []any, started time.Time, meta map[string]any) {
	requestBody := map[string]any{
		"system":          []any{map[string]any{"text": prompt}},
		"messages":        []any{map[string]any{"role": "user", "content": content}},
		"inferenceConfig": map[string]any{"maxTokens": 1024, "temperature": 0},
		"toolConfig":      map[string]any{"tools": []any{map[string]any{"toolSpec": map[string]any{"name": "record_contact", "description": "Extract a lead", "inputSchema": map[string]any{"json": contactSchema()}}}}, "toolChoice": map[string]any{"tool": map[string]any{"name": "record_contact"}}},
	}
	payload, _ := json.Marshal(requestBody)
	base := bedrockBaseURL
	if base == "" {
		base = "https://bedrock-runtime." + env("AXLEO_BEDROCK_REGION", "ap-southeast-2") + ".amazonaws.com"
	}
	req, err := http.NewRequestWithContext(r.Context(), "POST", base+"/model/"+url.PathEscape(model)+"/converse", strings.NewReader(string(payload)))
	if err != nil {
		problem(w, 502, "ai upstream error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := speechHTTPClient.Do(req)
	if err != nil {
		problem(w, 502, "ai upstream error")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		w.Header().Set("Retry-After", resp.Header.Get("Retry-After"))
		problem(w, 429, "ai rate limited")
		return
	}
	if resp.StatusCode != 200 {
		problem(w, 502, "ai upstream error")
		return
	}
	var result struct {
		Output struct {
			Message struct {
				Content []struct {
					ToolUse *struct {
						Name  string          `json:"name"`
						Input json.RawMessage `json:"input"`
					} `json:"toolUse"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result)
	if err != nil {
		problem(w, 502, "ai upstream error")
		return
	}
	meta["inputTokens"], meta["outputTokens"] = result.Usage.InputTokens, result.Usage.OutputTokens
	log.Printf("ai_extract inputTokens=%d outputTokens=%d latencyMs=%d", result.Usage.InputTokens, result.Usage.OutputTokens, time.Since(started).Milliseconds())
	for _, item := range result.Output.Message.Content {
		if item.ToolUse == nil || item.ToolUse.Name != "record_contact" {
			continue
		}
		var input map[string]any
		if json.Unmarshal(item.ToolUse.Input, &input) != nil {
			break
		}
		fields := map[string]any{}
		conf := map[string]any{}
		rawConf, _ := input["confidence"].(map[string]any)
		for _, k := range contactKeys {
			value := input[k]
			if k == "priceMin" || k == "priceMax" {
				if _, ok := value.(float64); !ok {
					value = nil
				}
			} else if s, ok := value.(string); !ok || strings.TrimSpace(s) == "" {
				value = nil
			}
			if k == "intent" && value != nil {
				switch value {
				case "buy", "sell", "rent", "appraisal", "other":
				default:
					value = nil
				}
			}
			fields[k] = value
			score, ok := rawConf[k].(float64)
			if !ok || score < 0 || score > 1 {
				score = 0
			}
			conf[k] = score
		}
		respond(w, 200, map[string]any{"fields": fields, "confidence": conf, "model": model, "latencyMs": time.Since(started).Milliseconds()})
		return
	}
	problem(w, 502, "ai upstream error")
}
