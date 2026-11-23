package main

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"
)

const gzipMinBytes = 1024

// Only API responses are wrapped. Static assets, uploaded files, audio, CSV,
// and streaming responses are passed through unchanged. Buffering lets us avoid
// gzip overhead on small JSON and preserve the handler's status and headers.
type gzipAPIWriter struct {
	http.ResponseWriter
	body    bytes.Buffer
	status  int
	started bool
}

func (w *gzipAPIWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *gzipAPIWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
func (w *gzipAPIWriter) plain() {
	if w.started {
		return
	}
	w.started = true
	w.ResponseWriter.WriteHeader(w.statusCode())
	if w.body.Len() > 0 {
		_, _ = w.body.WriteTo(w.ResponseWriter)
	}
}
func (w *gzipAPIWriter) Write(p []byte) (int, error) {
	if w.started {
		return w.ResponseWriter.Write(p)
	}
	if !strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "application/json") {
		w.plain()
		return w.ResponseWriter.Write(p)
	}
	return w.body.Write(p)
}

// A handler using Flush is streaming; do not delay or compress it.
func (w *gzipAPIWriter) Flush() {
	w.plain()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *gzipAPIWriter) finish() {
	if w.started {
		return
	}
	if w.body.Len() < gzipMinBytes || !strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "application/json") || w.Header().Get("Content-Encoding") != "" || w.statusCode() == http.StatusNoContent || w.statusCode() == http.StatusNotModified {
		w.plain()
		return
	}
	w.started = true
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(w.statusCode())
	gz := gzip.NewWriter(w.ResponseWriter)
	_, _ = w.body.WriteTo(gz)
	_ = gz.Close()
}

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(fields[0]), "gzip") {
			continue
		}
		for _, option := range fields[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(option), "=")
			if strings.EqualFold(key, "q") && ok {
				q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || q <= 0 {
					return false
				}
			}
		}
		return true
	}
	return false
}

func gzipAPI(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		next.ServeHTTP(w, r)
		return
	}
	// All API variants share the cache key; even sub-threshold responses vary.
	w.Header().Add("Vary", "Accept-Encoding")
	if !acceptsGzip(r.Header.Get("Accept-Encoding")) || r.Method == http.MethodHead {
		next.ServeHTTP(w, r)
		return
	}
	wrapped := &gzipAPIWriter{ResponseWriter: w}
	next.ServeHTTP(wrapped, r)
	wrapped.finish()
}
