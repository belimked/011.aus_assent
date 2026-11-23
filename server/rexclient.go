package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type rexRateError struct{ delay time.Duration }

func (e rexRateError) Error() string { return "Rex rate limited" }

type rexEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}
type rexClient struct {
	BaseURL, AppID, Token       string
	HTTP                        *http.Client
	Interval                    time.Duration
	Retries                     int
	mu                          sync.Mutex
	next                        time.Time
	AccountID, Caller, Password string
	Log                         func(rexLog)
	ReadOnly                    bool
	VerifiedAccount             string // Never inferred from the requested login account.
	Entity, LocalID, RexID      string
}

func rexPath(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r == '_' || r == ' ' {
			b.WriteByte('-')
			continue
		}
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (i+1 < len(s) && unicode.IsUpper(prev) && unicode.IsLower(rune(s[i+1]))) {
					b.WriteByte('-')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func newRexClient(base, appID string) *rexClient {
	return &rexClient{BaseURL: strings.TrimRight(base, "/"), AppID: appID, HTTP: &http.Client{Timeout: 15 * time.Second}, Interval: time.Second, Retries: 2}
}
func (c *rexClient) wait(ctx context.Context) error {
	c.mu.Lock()
	d := time.Until(c.next)
	if d < 0 {
		d = 0
	}
	c.next = time.Now().Add(d + c.Interval)
	c.mu.Unlock()
	if d == 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *rexClient) callPath(ctx context.Context, service, method string, body any, camel bool) (json.RawMessage, rexEnvelope, error) {
	var e rexEnvelope
	data, err := json.Marshal(body)
	if err != nil {
		return nil, e, err
	}
	if !camel {
		service = rexPath(service)
		method = rexPath(method)
	}
	url := c.BaseURL + "/v1/rex/" + service + "/" + method
	path := "/v1/rex/" + service + "/" + method
	// Log each transport attempt, including rate limits and malformed responses.
	record := func(attempt, status int, started time.Time, raw []byte, callErr error) {
		if c.Log == nil {
			return
		}
		var response any
		if json.Unmarshal(raw, &response) != nil {
			response = string(raw)
		}
		if rexKind(service, method) == "auth" {
			response = map[string]any{"result": "[REDACTED]"}
		}
		excerpt := rexRedact(response, c.Password, c.Token)
		if len(excerpt) > 4096 {
			excerpt = excerpt[:4096]
		}
		reason := ""
		if callErr != nil {
			reason = rexRedact(callErr.Error(), c.Password, c.Token)
		}
		c.Log(rexLog{Entity: c.Entity, LocalID: c.LocalID, RexID: c.RexID, At: rexLogAt(), AccountID: c.AccountID, Direction: "out", Kind: rexKind(service, method), Service: service, Method: method, Path: path, Caller: c.Caller, Attempt: attempt + 1, RequestJSON: rexRedact(body, c.Password, c.Token), HTTPStatus: status, OK: callErr == nil && status < 400 && status > 0, Error: reason, DurationMS: time.Since(started).Milliseconds(), ResultSummary: rexSummary(raw, callErr), ResponseExcerpt: excerpt})
	}
	if rexKind(service, method) == "write" && !c.ReadOnly {
		account := c.VerifiedAccount
		if account == "" || account != c.AccountID || !rexAccountAllowed(account) {
			err := fmt.Errorf("blocked: account %s not in write allowlist", account)
			record(0, 0, time.Now(), nil, err)
			return nil, e, err
		}
	}
	if c.ReadOnly && rexKind(service, method) == "write" {
		err := fmt.Errorf("Rex write blocked by read-only mode")
		record(0, 0, time.Now(), nil, err)
		return nil, e, err
	}
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if err = c.wait(ctx); err != nil {
			return nil, e, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			return nil, e, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-App-Identifier", c.AppID)
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		started := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			record(attempt, 0, started, nil, err)
			return nil, e, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			record(attempt, resp.StatusCode, started, raw, readErr)
			return nil, e, readErr
		}
		var callErr error
		if resp.StatusCode == 429 {
			callErr = rexRateError{time.Second}
		} else if json.Unmarshal(raw, &e) != nil {
			callErr = fmt.Errorf("Rex HTTP %d: invalid envelope", resp.StatusCode)
		} else if resp.StatusCode >= 400 {
			callErr = fmt.Errorf("Rex HTTP %d: %s", resp.StatusCode, safeRexError(e.Error))
		} else if len(e.Error) > 0 && string(e.Error) != "null" {
			callErr = fmt.Errorf("Rex: %s", safeRexError(e.Error))
		}
		record(attempt, resp.StatusCode, started, raw, callErr)
		if resp.StatusCode == 429 && attempt < c.Retries {
			delay := time.Duration(attempt+1) * time.Second
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				delay = time.Duration(n) * time.Second
			}
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			if delay < 0 {
				delay = 0
			}
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, e, ctx.Err()
			case <-t.C:
			}
			continue
		}
		if resp.StatusCode == 429 {
			delay := time.Second
			if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n > 0 {
				delay = time.Duration(n) * time.Second
			}
			return raw, e, rexRateError{delay}
		}
		if json.Unmarshal(raw, &e) != nil {
			return raw, e, fmt.Errorf("Rex HTTP %d: invalid envelope", resp.StatusCode)
		}
		if resp.StatusCode >= 400 {
			return raw, e, fmt.Errorf("Rex HTTP %d: %s", resp.StatusCode, safeRexError(e.Error))
		}
		if len(e.Error) > 0 && string(e.Error) != "null" {
			return raw, e, fmt.Errorf("Rex: %s", safeRexError(e.Error))
		}
		return raw, e, nil
	}
	return nil, e, fmt.Errorf("Rex retries exhausted")
}
func safeRexError(raw json.RawMessage) string { // Do not echo credentials embedded in server errors.
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return "error (see redacted raw file)"
	}
	switch x := v.(type) {
	case string:
		if len(x) > 300 {
			x = x[:300]
		}
		return x
	case map[string]any:
		if msg, ok := x["message"].(string); ok && msg != "" {
			if len(msg) > 300 {
				msg = msg[:300]
			}
			if t, ok := x["type"].(string); ok && t != "" {
				return t + ": " + msg
			}
			return msg
		}
		return "error (see redacted raw file)"
	default:
		return "error (see redacted raw file)"
	}
}
func rexAccountAllowed(account string) bool {
	if account == "" || account == "758" && os.Getenv("REX_ALLOW_PRODUCTION_WRITE") != "1" {
		return false
	}
	list, configured := os.LookupEnv("REX_WRITE_ALLOWED_ACCOUNTS")
	if !configured {
		list = "2057"
	}
	if strings.TrimSpace(list) == "" {
		return false
	}
	for _, id := range strings.Split(list, ",") {
		if strings.TrimSpace(id) == account {
			return true
		}
	}
	return false
}
func (c *rexClient) Call(ctx context.Context, service, method string, body any) (json.RawMessage, rexEnvelope, error) {
	return c.callPath(ctx, service, method, body, false)
}
func (c *rexClient) Login(ctx context.Context, email, password, account string) (json.RawMessage, error) {
	b := map[string]any{"email": email, "password": password, "token_lifetime": 86400}
	if account != "" {
		b["account_id"] = account
	}
	raw, e, err := c.Call(ctx, "Authentication", "login", b)
	if err != nil {
		return raw, fmt.Errorf("Rex login failed")
	}
	var token string
	if json.Unmarshal(e.Result, &token) != nil || token == "" {
		return raw, fmt.Errorf("Rex login token missing")
	}
	c.Token = token
	c.VerifiedAccount = ""
	// Authentication::login returns a token, not proof of the active account.
	// UserProfile is a read-only endpoint; fail closed unless it explicitly supplies the active account.
	_, profile, err := c.Call(ctx, "UserProfile", "getCurrentAccount", map[string]any{})
	if err == nil {
		var current map[string]any
		if json.Unmarshal(profile.Result, &current) == nil {
			c.VerifiedAccount = istring(current["account_id"])
			if c.VerifiedAccount == "" {
				c.VerifiedAccount = istring(current["id"])
			}
		}
	}
	if c.Log != nil {
		c.Log(rexLog{At: rexLogAt(), AccountID: c.VerifiedAccount, Direction: "out", Kind: "auth", Service: "UserProfile", Method: "verifiedAccount", Caller: c.Caller, OK: c.VerifiedAccount != "" && c.VerifiedAccount == c.AccountID})
	}
	return raw, nil
}
