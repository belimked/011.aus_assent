package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a *app) devices(w http.ResponseWriter, r *http.Request, p principal) {
	m, err := body(r)
	if err != nil || str(m, "platform") != "ios" || len(str(m, "token")) < 16 || len(str(m, "token")) > 256 || strings.Trim(str(m, "token"), "0123456789abcdefABCDEF") != "" {
		problem(w, 400, "invalid device token")
		return
	}
	token := str(m, "token")
	if r.Method == "POST" {
		_, err = a.db.Exec(`INSERT INTO device_tokens(user_id,platform,token,updated_at) VALUES(?,?,?,?) ON CONFLICT(token) DO UPDATE SET user_id=excluded.user_id,platform=excluded.platform,updated_at=excluded.updated_at`, p.id, "ios", token, now())
	} else if r.Method == "DELETE" {
		_, err = a.db.Exec(`DELETE FROM device_tokens WHERE user_id=? AND platform='ios' AND token=?`, p.id, token)
	} else {
		problem(w, 405, "method not allowed")
		return
	}
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

// APNs uses the native device token, never Expo's relay. A missing configuration
// deliberately leaves the local-only installation functional.
func apnsJWT(keyID, teamID, keyFile string) (string, error) {
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return "", fmt.Errorf("invalid APNs key")
	}
	private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	key, ok := private.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("APNs key must be EC")
	}
	encode := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": keyID})
	claims, _ := json.Marshal(map[string]any{"iss": teamID, "iat": time.Now().Unix()})
	unsigned := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	// APNs expects JOSE's fixed-width R || S, not ASN.1 DER.
	var parsed struct{ R, S *big.Int }
	if _, err = asn1.Unmarshal(sig, &parsed); err != nil {
		return "", err
	}
	signature := append(parsed.R.FillBytes(make([]byte, 32)), parsed.S.FillBytes(make([]byte, 32))...)
	return unsigned + "." + encode(signature), nil
}

func sendAPNs(ctx context.Context, client *http.Client, endpoint, token, topic, jwt, title, body, id, contactID string) (bool, error) {
	payload, _ := json.Marshal(map[string]any{"aps": map[string]any{"alert": map[string]string{"title": title, "body": body}, "sound": "default"}, "todoId": id, "contactId": contactID})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/3/device/"+token, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("apns-topic", topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return false, nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(raw, &result)
	if resp.StatusCode == 410 || result.Reason == "BadDeviceToken" {
		return true, nil
	}
	return false, fmt.Errorf("APNs %d: %s", resp.StatusCode, result.Reason)
}

func (a *app) notifyUser(userID, title, body, id, contactID string) {
	keyID, keyFile, teamID, topic := os.Getenv("AXLEO_APNS_KEY_ID"), os.Getenv("AXLEO_APNS_KEY_FILE"), os.Getenv("AXLEO_APNS_TEAM_ID"), os.Getenv("AXLEO_APNS_TOPIC")
	if keyID == "" || keyFile == "" || teamID == "" || topic == "" {
		log.Printf("APNs disabled: notification for user %s", userID)
		return
	}
	jwt, err := apnsJWT(keyID, teamID, keyFile)
	if err != nil {
		log.Printf("APNs key: %v", err)
		return
	}
	rows, err := a.db.Query(`SELECT token FROM device_tokens WHERE user_id=? AND platform='ios'`, userID)
	if err != nil {
		log.Printf("APNs tokens: %v", err)
		return
	}
	tokens := []string{}
	for rows.Next() {
		var token string
		if rows.Scan(&token) == nil {
			tokens = append(tokens, token)
		}
	}
	rows.Close()
	endpoint := "https://api.push.apple.com"
	if os.Getenv("AXLEO_APNS_ENV") == "sandbox" {
		endpoint = "https://api.sandbox.push.apple.com"
	}
	client := &http.Client{Transport: &http.Transport{ForceAttemptHTTP2: true}, Timeout: 10 * time.Second}
	for _, token := range tokens {
		remove, err := sendAPNs(context.Background(), client, endpoint, token, topic, jwt, title, body, id, contactID)
		if err != nil {
			log.Printf("APNs send: %v", err)
		}
		if remove {
			_, _ = a.db.Exec(`DELETE FROM device_tokens WHERE token=?`, token)
		}
	}
}

// Invoked only after a committed mutation; sync op replays never reach this hook.
func (a *app) notifyMutation(kind, method, id string, m map[string]any, previousAssignee, actor string) {
	if kind == "todos" && method != "DELETE" {
		target := str(m, "assigneeId")
		if target != "" && target != actor && target != previousAssignee {
			go a.notifyUser(target, "New to-do", str(m, "t"), id, str(m, "cid"))
		}
	}
	if kind == "referrals" && method == "POST" {
		partner := str(m, "to")
		var target string
		// A partner not present as an active app user receives no push.
		if a.db.QueryRow(`SELECT id FROM users WHERE (id=? OR name=? OR ? LIKE name || ' (%)') AND disabled=0 AND deleted_at IS NULL LIMIT 1`, partner, partner, partner).Scan(&target) == nil && target != actor {
			go a.notifyUser(target, "New referral", str(m, "type"), "", str(m, "contactId"))
		}
	}
}

// Approval and direct reassignments share the same post-commit notification rule.
func (a *app) previousApprovalAssignee(m map[string]any) string {
	if str(m, "kind") != "reassign" || str(m, "entity") != "todos" {
		return ""
	}
	id := str(m, "recordId")
	if id == "" {
		id = str(m, "id")
	}
	var previous string
	_ = a.db.QueryRow(`SELECT assignee_id FROM todos WHERE id=?`, id).Scan(&previous)
	return previous
}
func (a *app) notifyApprovedTodo(m map[string]any, previous, actor string) {
	if str(m, "kind") != "reassign" || str(m, "entity") != "todos" {
		return
	}
	id := str(m, "recordId")
	if id == "" {
		id = str(m, "id")
	}
	var text, contactID string
	_ = a.db.QueryRow(`SELECT COALESCE(text,''),COALESCE(contact_id,'') FROM todos WHERE id=?`, id).Scan(&text, &contactID)
	a.notifyMutation("todos", "PATCH", id, map[string]any{"assigneeId": str(m, "targetId"), "t": text, "cid": contactID}, previous, actor)
}
