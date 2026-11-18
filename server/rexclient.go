package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	BaseURL, AppID, Token string
	HTTP                  *http.Client
	Interval              time.Duration
	Retries               int
	mu                    sync.Mutex
	next                  time.Time
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
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, e, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, e, readErr
		}
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
	return raw, nil
}
