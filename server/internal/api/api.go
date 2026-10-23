// Package api exposes the library and HLS streams over HTTP.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kvmediapp/bloom/server/internal/hls"
	"github.com/kvmediapp/bloom/server/internal/media"
)

var streamFile = regexp.MustCompile(`^(index\.m3u8|init\.mp4|seg_\d{5}\.m4s)$`)

// Server wires the library and the remux manager to HTTP routes.
type Server struct {
	Lib       *media.Library
	HLS       *hls.Manager
	CacheFile string // where the library is saved after a scan
	Token     string // optional bearer token; empty disables auth
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": len(s.Lib.Items())})
	})
	mux.HandleFunc("GET /api/items", s.items)
	mux.HandleFunc("POST /api/scan", s.scan)
	mux.HandleFunc("GET /stream/{id}/{file}", s.stream)
	return logRequests(s.auth(mux))
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Token != "" && r.URL.Path != "/health" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) items(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Lib.Items())
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if err := s.Lib.Scan(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.CacheFile != "" {
		if err := s.Lib.Save(s.CacheFile); err != nil {
			log.Printf("save library: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": len(s.Lib.Items())})
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	id, file := r.PathValue("id"), r.PathValue("file")
	it, ok := s.Lib.Get(id)
	if !ok || !streamFile.MatchString(file) {
		http.NotFound(w, r)
		return
	}
	if err := s.HLS.Ensure(it); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, hls.ErrUnsupported) {
			status = http.StatusUnprocessableEntity
		}
		http.Error(w, err.Error(), status)
		return
	}

	if file == "index.m3u8" {
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := s.HLS.WaitPlaylist(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}

	path := filepath.Join(s.HLS.Dir(id), file)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(file, ".m3u8"):
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		if !s.HLS.Finished(id) {
			w.Header().Set("Cache-Control", "no-cache")
		}
	default:
		w.Header().Set("Content-Type", "video/mp4")
	}
	http.ServeFile(w, r, path)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests logs every stream request with its status and duration, so a
// player stall can be matched to the exact segment that failed.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/stream/") {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s) range=%q", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond), r.Header.Get("Range"))
	})
}
