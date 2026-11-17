package main

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// rexWebhook receives Rex AdminWebhooks deliveries. Rex does not sign payloads, so the
// URL carries a random secret (AXLEO_REX_WEBHOOK_TOKEN). Phase 1 only acknowledges and
// logs ids/events; queueing a refresh is done by the connector later. Rex requires a
// response within 10 seconds, so nothing slow happens here.
func (a *app) rexWebhook(w http.ResponseWriter, r *http.Request) {
	want := os.Getenv("AXLEO_REX_WEBHOOK_TOKEN")
	got := r.PathValue("token")
	if len(want) < 32 || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		problem(w, 404, "not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		problem(w, 400, "bad body")
		return
	}
	log.Printf("rex webhook: %s", rexWebhookSummary(body))
	w.WriteHeader(http.StatusAccepted)
}

// rexWebhookSummary keeps event names and record ids only, never field values.
func rexWebhookSummary(body []byte) string {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return "unparsed bytes=" + strconv.Itoa(len(body))
	}
	var out []string
	var walk func(any, string)
	walk = func(x any, k string) {
		switch t := x.(type) {
		case map[string]any:
			for key, val := range t {
				walk(val, key)
			}
		case []any:
			for _, val := range t {
				walk(val, k)
			}
		case string:
			if k == "event" || k == "type" || k == "service" || k == "id" || strings.HasSuffix(k, "_id") || k == "timestamp" {
				out = append(out, k+"="+t)
			}
		case float64:
			if k == "id" || strings.HasSuffix(k, "_id") || k == "timestamp" {
				out = append(out, k+"="+strconv.FormatFloat(t, 'f', -1, 64))
			}
		}
	}
	walk(v, "")
	if len(out) > 60 {
		out = append(out[:60], "...")
	}
	return "bytes=" + strconv.Itoa(len(body)) + " " + strings.Join(out, " ")
}
