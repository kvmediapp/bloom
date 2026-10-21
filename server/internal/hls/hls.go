// Package hls turns library files into Apple-compatible HLS (fragmented MP4).
//
// Phase 0 runs one ffmpeg process per title, started on first request. Video
// is copied without re-encoding; only incompatible audio is converted. The
// playlist is an EVENT playlist, so playback can begin as soon as the first
// segments exist and the playlist is finalized when ffmpeg finishes.
package hls

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kvmediapp/bloom/server/internal/media"
)

var ErrUnsupported = errors.New("file needs a video transcode, which is not built yet")

type job struct {
	done chan struct{}
	err  error
}

// Manager starts and tracks remux jobs.
type Manager struct {
	MediaDir string
	CacheDir string

	mu   sync.Mutex
	jobs map[string]*job
}

func NewManager(mediaDir, cacheDir string) *Manager {
	return &Manager{MediaDir: mediaDir, CacheDir: cacheDir, jobs: map[string]*job{}}
}

// Dir is where one title's playlist and segments live.
func (m *Manager) Dir(id string) string { return filepath.Join(m.CacheDir, "hls", id) }

// Ensure starts a remux for the item if one is not already running or finished.
func (m *Manager) Ensure(it media.Item) error {
	if it.Plan.Mode == media.ModeUnsupported {
		return fmt.Errorf("%w: %s", ErrUnsupported, strings.Join(it.Plan.Reasons, "; "))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[it.ID]; ok {
		return nil
	}
	dir := m.Dir(it.ID)
	j := &job{done: make(chan struct{})}
	if _, err := os.Stat(filepath.Join(dir, ".done")); err == nil {
		close(j.done)
		m.jobs[it.ID] = j
		return nil
	}
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m.jobs[it.ID] = j
	go m.run(it, dir, j)
	return nil
}

func (m *Manager) run(it media.Item, dir string, j *job) {
	src := filepath.Join(m.MediaDir, it.Path)
	cmd := exec.Command("ffmpeg", BuildArgs(src, it.Plan)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	if err != nil {
		j.err = fmt.Errorf("ffmpeg: %w: %s", err, tail(stderr.String(), 800))
		log.Printf("remux failed for %s: %v", it.Title, j.err)
		m.mu.Lock()
		delete(m.jobs, it.ID) // allow a retry on the next request
		m.mu.Unlock()
	} else {
		_ = os.WriteFile(filepath.Join(dir, ".done"), nil, 0o644)
		log.Printf("remux finished for %s in %s", it.Title, time.Since(start).Round(time.Millisecond))
	}
	close(j.done)
}

// BuildArgs returns the ffmpeg arguments for a plan. Exported for tests.
func BuildArgs(src string, p media.Plan) []string {
	args := []string{"-hide_banner", "-loglevel", "warning", "-nostdin", "-y", "-i", src, "-map", "0:v:0"}
	if p.AudioIndex >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(p.AudioIndex))
	}
	args = append(args, "-sn", "-dn", "-c:v", "copy")
	if p.TagHVC1 {
		args = append(args, "-tag:v", "hvc1")
	}
	switch p.Audio {
	case "copy":
		args = append(args, "-c:a", "copy")
	case "eac3":
		args = append(args, "-c:a", "eac3", "-b:a", "640k")
	case "aac":
		args = append(args, "-c:a", "aac", "-b:a", "256k")
	}
	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "event",
		"-hls_segment_type", "fmp4",
		"-hls_flags", "independent_segments+temp_file",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", "seg_%05d.m4s",
		"index.m3u8",
	)
	return args
}

// WaitPlaylist blocks until the playlist has two segments (enough to start
// playback) or the remux has finished.
func (m *Manager) WaitPlaylist(ctx context.Context, id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return errors.New("no remux job for this title")
	}
	playlist := filepath.Join(m.Dir(id), "index.m3u8")
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-j.done:
			return j.err
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if b, err := os.ReadFile(playlist); err == nil && bytes.Count(b, []byte("#EXTINF")) >= 2 {
				return nil
			}
		}
	}
}

// Finished reports whether a title's remux has completed.
func (m *Manager) Finished(id string) bool {
	_, err := os.Stat(filepath.Join(m.Dir(id), ".done"))
	return err == nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
