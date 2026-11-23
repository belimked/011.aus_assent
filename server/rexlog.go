package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

type rexLog struct {
	At, AccountID, Direction, Kind, Service, Method, Path, Caller string
	Attempt                                                       int
	RequestJSON                                                   string
	HTTPStatus                                                    int
	OK                                                            bool
	Error                                                         string
	DurationMS                                                    int64
	ResultSummary, ResponseExcerpt                                string
	Entity, LocalID, RexID                                        string
}

func rexKind(service, method string) string {
	if strings.EqualFold(service, "Authentication") || strings.EqualFold(service, "AccountSettings") && strings.EqualFold(method, "read") {
		return "auth"
	}
	m := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(method, "-", ""), "_", ""))
	for _, prefix := range []string{"search", "read", "get", "describe", "find", "getcurrent"} {
		if strings.HasPrefix(m, prefix) {
			return "read"
		}
	}
	return "write"
}

var bearerPattern = regexp.MustCompile(`(?i)bearer\s+[^\s"',}]+`)

func rexRedact(v any, secrets ...string) string {
	var x any
	b, _ := json.Marshal(v)
	if json.Unmarshal(b, &x) != nil {
		x = string(b)
	}
	var mask func(any) any
	mask = func(v any) any {
		switch y := v.(type) {
		case map[string]any:
			for k, z := range y {
				lower := strings.ToLower(k)
				if strings.Contains(lower, "password") || strings.Contains(lower, "token") || lower == "authorization" || lower == "secret" {
					y[k] = "[REDACTED]"
				} else {
					y[k] = mask(z)
				}
			}
			return y
		case []any:
			for i, z := range y {
				y[i] = mask(z)
			}
			return y
		case string:
			y = bearerPattern.ReplaceAllString(y, "Bearer [REDACTED]")
			for _, s := range secrets {
				if s != "" {
					y = strings.ReplaceAll(y, s, "[REDACTED]")
				}
			}
			return y
		default:
			return v
		}
	}
	b, _ = json.Marshal(mask(x))
	return string(b)
}
func rexSummary(raw []byte, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %T", err)
	}
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return "invalid response"
	}
	result, ok := v["result"].(map[string]any)
	if !ok {
		return "response received"
	}
	parts := []string{}
	for _, key := range []string{"total", "id"} {
		if value, ok := result[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", key, value))
		}
	}
	for _, key := range []string{"rows", "items", "data", "results"} {
		if rows, ok := result[key].([]any); ok {
			parts = append(parts, fmt.Sprintf("rows=%d", len(rows)))
			break
		}
	}
	return strings.Join(parts, ", ")
}
func rexDBLogger(db *sql.DB) func(rexLog) {
	return func(l rexLog) {
		if _, err := db.Exec(`INSERT INTO rex_request_log(at,account_id,direction,kind,service,method,path,caller,attempt,request_json,http_status,ok,error,duration_ms,result_summary,response_excerpt,entity,local_id,rex_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, l.At, l.AccountID, l.Direction, l.Kind, l.Service, l.Method, l.Path, l.Caller, l.Attempt, l.RequestJSON, l.HTTPStatus, l.OK, l.Error, l.DurationMS, l.ResultSummary, l.ResponseExcerpt, l.Entity, l.LocalID, l.RexID); err != nil {
			log.Printf("Rex request log insert failed: %v", err)
		}
	}
}
func rexLogAt() string { return time.Now().UTC().Format(layout) }
