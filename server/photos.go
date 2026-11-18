package main

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var photoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,64}$`)

const photoMaxSize = 8 << 20

func photoDir() string {
	return env("AXLEO_PHOTO_DIR", filepath.Join(filepath.Dir(env("AXLEO_DB", "./axleo.db")), "photos"))
}

// readImage enforces the actual image signature, not the multipart Content-Type.
func readImage(r *http.Request, max int64) ([]byte, string, int) {
	r.Body = http.MaxBytesReader(nil, r.Body, max+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, "", 413
		}
		return nil, "", 400
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", 400
	}
	defer f.Close()
	if header.Header.Get("Content-Type") != "image/jpeg" && header.Header.Get("Content-Type") != "image/png" {
		return nil, "", 400
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, "", 400
	}
	if int64(len(data)) > max {
		return nil, "", 413
	}
	mime := ""
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}) {
		mime = "image/jpeg"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		mime = "image/png"
	}
	if mime == "" || mime != header.Header.Get("Content-Type") {
		return nil, "", 400
	}
	return data, mime, 0
}

type photo struct {
	ID         string `json:"id"`
	PropertyID string `json:"propertyId"`
	MIME       string `json:"mime"`
	Size       int64  `json:"size"`
	CreatedAt  string `json:"createdAt"`
	Creator    string `json:"-"`
}

func (a *app) findPhoto(id string) (photo, error) {
	var row photo
	var property sql.NullString
	err := a.db.QueryRow(`SELECT id,property_id,created_by,mime,size,created_at FROM photos WHERE id=?`, id).Scan(&row.ID, &property, &row.Creator, &row.MIME, &row.Size, &row.CreatedAt)
	row.PropertyID = property.String
	return row, err
}

func (a *app) uploadPhoto(w http.ResponseWriter, r *http.Request, p principal) {
	// Audit only metadata; never log image content.
	meta := map[string]any{"size": 0, "propertyId": ""}
	defer a.auditCall(p, "photo_upload", meta)
	data, mime, code := readImage(r, photoMaxSize)
	if code != 0 {
		problem(w, code, "invalid image")
		return
	}
	id := r.FormValue("id")
	if id == "" {
		id = ulid()
	} else if !photoIDPattern.MatchString(id) {
		problem(w, 400, "invalid id")
		return
	}
	propertyID := r.FormValue("propertyId")
	meta["size"], meta["propertyId"] = len(data), propertyID
	if existing, err := a.findPhoto(id); err == nil {
		if existing.Creator != p.id {
			problem(w, 404, "not found")
			return
		}
		respond(w, 200, existing)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		problem(w, 500, "database error")
		return
	}
	if propertyID != "" {
		if _, err := a.get("properties", propertyID, p, false); err != nil {
			problem(w, 404, "not found")
			return
		}
	}
	dir := photoDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		problem(w, 500, "storage error")
		return
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		problem(w, 500, "storage error")
		return
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		problem(w, 500, "storage error")
		return
	}
	if err = tmp.Close(); err != nil {
		problem(w, 500, "storage error")
		return
	}
	ext := ".jpg"
	if mime == "image/png" {
		ext = ".png"
	}
	row := photo{ID: id, PropertyID: propertyID, Creator: p.id, MIME: mime, Size: int64(len(data)), CreatedAt: now()}
	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	var property any
	if propertyID != "" {
		property = propertyID
	}
	_, err = tx.Exec(`INSERT INTO photos(id,property_id,created_by,mime,size,created_at) VALUES(?,?,?,?,?,?)`, id, property, p.id, mime, len(data), row.CreatedAt)
	if err != nil {
		// A competing upload may have claimed the same ID. Do not overwrite its file.
		tx.Rollback()
		if existing, lookupErr := a.findPhoto(id); lookupErr == nil && existing.Creator == p.id {
			respond(w, 200, existing)
			return
		}
		problem(w, 404, "not found")
		return
	}
	dest := filepath.Join(dir, id+ext)
	if err = os.Rename(tmp.Name(), dest); err != nil {
		problem(w, 500, "storage error")
		return
	}
	if err = tx.Commit(); err != nil {
		os.Remove(dest)
		problem(w, 500, "database error")
		return
	}
	respond(w, 201, row)
}

func (a *app) servePhoto(w http.ResponseWriter, r *http.Request, p principal) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/photos/")
	if !photoIDPattern.MatchString(id) {
		problem(w, 404, "not found")
		return
	}
	row, err := a.findPhoto(id)
	if err != nil {
		problem(w, 404, "not found")
		return
	}
	if row.Creator != p.id && p.role != "admin" {
		if row.PropertyID == "" {
			problem(w, 404, "not found")
			return
		}
		if _, err := a.get("properties", row.PropertyID, p, false); err != nil {
			problem(w, 404, "not found")
			return
		}
	}
	ext := ".jpg"
	if row.MIME == "image/png" {
		ext = ".png"
	}
	f, err := os.Open(filepath.Join(photoDir(), id+ext))
	if err != nil {
		problem(w, 404, "not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", row.MIME)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, id+ext, time.Time{}, f)
}
