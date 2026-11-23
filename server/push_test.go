package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPNs(t *testing.T) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	der, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "key.p8")
	if e = os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); e != nil {
		t.Fatal(e)
	}
	jwt, e := apnsJWT("key", "team", file)
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatal(jwt)
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil || len(sig) != 64 {
		t.Fatalf("invalid JOSE signature: %v", e)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.ProtoMajor != 1 && r.ProtoMajor != 2 {
			t.Fatal("invalid protocol")
		}
		if r.URL.Path != "/3/device/abc" || r.Header.Get("apns-topic") != "topic" || r.Header.Get("apns-push-type") != "alert" || r.Header.Get("authorization") != "bearer "+jwt {
			t.Errorf("unexpected request: %v", r)
		}
		var payload map[string]any
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil || payload["todoId"] != "todo" {
			t.Errorf("payload: %v %v", payload, e)
		}
		if calls == 1 {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(410)
			_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
		}
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		remove, e := sendAPNs(context.Background(), server.Client(), server.URL, "abc", "topic", jwt, "New to-do", "Call", "todo", "contact")
		if e != nil || remove != (i == 1) {
			t.Fatalf("send %d: remove=%v err=%v", i, remove, e)
		}
	}
}

func TestAPNsBadToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"reason":"BadDeviceToken"}`))
	}))
	defer server.Close()
	remove, e := sendAPNs(context.Background(), server.Client(), server.URL, "abc", "topic", "jwt", "title", "body", "", "")
	if !remove || e != nil {
		t.Fatalf("remove=%v err=%v", remove, e)
	}
}

func TestDeviceRegistration(t *testing.T) {
	a, tokens := setup(t)
	payload := map[string]any{"platform": "ios", "token": strings.Repeat("ab", 32)}
	if w := request(a, "POST", "/api/v1/devices", "", payload); w.Code != 401 {
		t.Fatalf("anonymous registration: %d", w.Code)
	}
	if w := request(a, "POST", "/api/v1/devices", tokens["mary"], payload); w.Code != 200 {
		t.Fatalf("registration: %d %s", w.Code, w.Body)
	}
	if w := request(a, "POST", "/api/v1/devices", tokens["john"], payload); w.Code != 200 {
		t.Fatalf("token transfer: %d %s", w.Code, w.Body)
	}
	if w := request(a, "DELETE", "/api/v1/devices", tokens["mary"], payload); w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
	var count int
	_ = a.db.QueryRow(`SELECT count(*) FROM device_tokens`).Scan(&count)
	if count != 1 {
		t.Fatalf("other user deleted token: %d", count)
	}
	if w := request(a, "DELETE", "/api/v1/devices", tokens["john"], payload); w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM device_tokens`).Scan(&count)
	if count != 0 {
		t.Fatalf("token remains: %d", count)
	}
}
