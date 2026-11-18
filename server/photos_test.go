package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func multipartRequest(a *app, path, token string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="image.png"`)
	h.Set("Content-Type", "image/png")
	f, _ := mw.CreatePart(h)
	_, _ = f.Write(data)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}

func TestPhotos(t *testing.T) {
	a, tokens := setup(t)
	dir := t.TempDir()
	t.Setenv("AXLEO_PHOTO_DIR", dir)
	image := append([]byte("\x89PNG\r\n\x1a\n"), []byte("fixture")...)
	fields := map[string]string{"id": "test_photo_123"}
	w := multipartRequest(a, "/api/v1/photos", tokens["mary"], image, fields)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	row := obj(t, w)
	if row["id"] != fields["id"] || row["mime"] != "image/png" || row["size"] != float64(len(image)) || row["createdAt"] == "" {
		t.Fatal(row)
	}
	saved, err := os.ReadFile(filepath.Join(dir, fields["id"]+".png"))
	if err != nil || !bytes.Equal(saved, image) {
		t.Fatal(err, saved)
	}
	w = request(a, "GET", "/api/v1/photos/"+fields["id"], tokens["mary"], nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "private, max-age=86400" || !bytes.Equal(w.Body.Bytes(), image) {
		t.Fatal(w.Code, w.Body)
	}
	w = multipartRequest(a, "/api/v1/photos", tokens["mary"], append([]byte("\x89PNG\r\n\x1a\n"), []byte("different")...), fields)
	if w.Code != 200 || obj(t, w)["createdAt"] != row["createdAt"] {
		t.Fatal(w.Code, w.Body)
	}
	saved, _ = os.ReadFile(filepath.Join(dir, fields["id"]+".png"))
	if !bytes.Equal(saved, image) {
		t.Fatal("idempotent upload overwrote file")
	}
	if w = request(a, "GET", "/api/v1/photos/"+fields["id"], tokens["john"], nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = request(a, "GET", "/api/v1/photos/"+fields["id"], tokens["admi"], nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = multipartRequest(a, "/api/v1/photos", tokens["mary"], []byte("not an image"), nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = multipartRequest(a, "/api/v1/photos", tokens["mary"], make([]byte, photoMaxSize+1), nil); w.Code != 413 && w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = multipartRequest(a, "/api/v1/photos", tokens["mary"], image, map[string]string{"propertyId": "missing"}); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestAIImage(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("AXLEO_BEDROCK_API_KEY", "secret")
	t.Setenv("AXLEO_BEDROCK_MODEL", "au.anthropic.test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		content := payload["messages"].([]any)[0].(map[string]any)["content"].([]any)
		image := content[0].(map[string]any)["image"].(map[string]any)
		if image["format"] != "png" || image["source"].(map[string]any)["bytes"] == "" {
			t.Error("missing image block", image)
		}
		choice := payload["toolConfig"].(map[string]any)["toolChoice"].(map[string]any)["tool"].(map[string]any)
		if choice["name"] != "record_contact" {
			t.Error(choice)
		}
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"toolUse":{"name":"record_contact","input":{"contactName":"Mary","confidence":{"contactName":0.8}}}}]}}}`))
	}))
	defer server.Close()
	bedrockBaseURL = server.URL
	t.Cleanup(func() { bedrockBaseURL = "" })
	image := append([]byte("\x89PNG\r\n\x1a\n"), []byte("fixture")...)
	w := multipartRequest(a, "/api/v1/ai/image", tokens["mary"], image, map[string]string{"ctx": "enq"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	result := obj(t, w)
	if result["model"] != "au.anthropic.test" || result["latencyMs"] == nil {
		t.Fatal(result)
	}
	fields := result["fields"].(map[string]any)
	conf := result["confidence"].(map[string]any)
	if len(fields) != len(contactKeys) || len(conf) != len(contactKeys) || fields["contactName"] != "Mary" || conf["contactName"] != 0.8 {
		t.Fatal(result)
	}
	if w = multipartRequest(a, "/api/v1/ai/image", tokens["mary"], image, map[string]string{"ctx": "bad"}); w.Code != 400 {
		t.Fatal(w.Code)
	}
	t.Setenv("AXLEO_BEDROCK_API_KEY", "")
	if w = multipartRequest(a, "/api/v1/ai/image", tokens["mary"], image, map[string]string{"ctx": "enq"}); w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body)
	}
}
