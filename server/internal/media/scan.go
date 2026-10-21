package media

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".mov": true, ".ts": true, ".avi": true,
}

// Item is one playable file in the library.
type Item struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"` // relative to the media directory
	Title       string  `json:"title"`
	DurationSec float64 `json:"durationSec"`
	SizeBytes   int64   `json:"sizeBytes"`
	Resolution  string  `json:"resolution"`
	HDR         string  `json:"hdr"`
	Plan        Plan    `json:"plan"`
}

// Library holds the scanned items.
type Library struct {
	MediaDir string

	mu    sync.RWMutex
	items []Item
	byID  map[string]Item
}

func NewLibrary(mediaDir string) *Library {
	return &Library{MediaDir: mediaDir, byID: map[string]Item{}}
}

func (l *Library) Items() []Item {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Item, len(l.items))
	copy(out, l.items)
	return out
}

func (l *Library) Get(id string) (Item, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	it, ok := l.byID[id]
	return it, ok
}

func (l *Library) set(items []Item) {
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title) })
	m := make(map[string]Item, len(items))
	for _, it := range items {
		m[it.ID] = it
	}
	l.mu.Lock()
	l.items, l.byID = items, m
	l.mu.Unlock()
}

// Scan walks the media directory and probes every video file.
func (l *Library) Scan(ctx context.Context) error {
	var paths []string
	err := filepath.WalkDir(l.MediaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && path != l.MediaDir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && videoExts[strings.ToLower(filepath.Ext(path))] {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", l.MediaDir, err)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		items []Item
		sem   = make(chan struct{}, 4)
	)
	for _, path := range paths {
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := ProbeFile(ctx, path)
			if err != nil {
				log.Printf("skip %s: %v", path, err)
				return
			}
			rel, _ := filepath.Rel(l.MediaDir, path)
			it := Item{
				ID:          idFor(rel),
				Path:        rel,
				Title:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
				DurationSec: p.DurationSec(),
				SizeBytes:   p.SizeBytes(),
				HDR:         "SDR",
				Plan:        Decide(p),
			}
			if v := p.FirstVideo(); v != nil {
				it.Resolution = fmt.Sprintf("%dx%d", v.Width, v.Height)
				it.HDR = v.HDRLabel()
			}
			mu.Lock()
			items = append(items, it)
			mu.Unlock()
		}(path)
	}
	wg.Wait()
	l.set(items)
	log.Printf("scan complete: %d files found, %d playable entries", len(paths), len(items))
	return nil
}

func idFor(rel string) string {
	sum := sha1.Sum([]byte(rel))
	return hex.EncodeToString(sum[:])[:12]
}

// Save writes the scanned items to a JSON file so restarts are instant.
func (l *Library) Save(path string) error {
	b, err := json.MarshalIndent(l.Items(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads a file written by Save. A missing file is not an error.
func (l *Library) Load(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var items []Item
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	l.set(items)
	return nil
}
