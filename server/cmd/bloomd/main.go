// Command bloomd is the Bloom media server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kvmediapp/bloom/server/internal/api"
	"github.com/kvmediapp/bloom/server/internal/hls"
	"github.com/kvmediapp/bloom/server/internal/media"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	mediaDir := env("BLOOM_MEDIA_DIR", "/media")
	cacheDir := env("BLOOM_CACHE_DIR", "/cache")
	addr := env("BLOOM_ADDR", ":8484")
	token := os.Getenv("BLOOM_TOKEN")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatalf("create cache dir: %v", err)
	}

	lib := media.NewLibrary(mediaDir)
	cacheFile := filepath.Join(cacheDir, "library.json")
	if err := lib.Load(cacheFile); err != nil {
		log.Printf("load library cache: %v", err)
	}
	if len(lib.Items()) == 0 {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := lib.Scan(ctx); err != nil {
				log.Printf("initial scan: %v", err)
				return
			}
			if err := lib.Save(cacheFile); err != nil {
				log.Printf("save library: %v", err)
			}
		}()
	}

	srv := &api.Server{
		Lib:       lib,
		HLS:       hls.NewManager(mediaDir, cacheDir),
		CacheFile: cacheFile,
		Token:     token,
	}
	if token == "" {
		log.Printf("warning: BLOOM_TOKEN is not set; anyone on your network can use this server")
	}
	log.Printf("bloomd listening on %s (media %s, cache %s)", addr, mediaDir, cacheDir)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}
