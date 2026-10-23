// Package hls turns library files into Apple-compatible HLS (fragmented MP4).
//
// One ffmpeg process per title is started on first request. Video is copied
// without re-encoding; only incompatible audio is converted. While ffmpeg
// runs, we publish a complete VOD playlist built from the file's keyframe
// index, so the player can seek anywhere at once. A request for a segment
// ffmpeg has not produced yet waits for it.
package hls

import (
	"bytes"
	"context"
	"encoding/json"
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
	ready chan struct{} // closed once index.m3u8 is written (or the job failed)
	done  chan struct{} // closed when ffmpeg has exited
	err   error
}

const doneMarker = ".done.v2"

type keyIndex struct {
	Keyframes []float64 `json:"keyframes"`
	End       float64   `json:"end"`
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
	j := &job{ready: make(chan struct{}), done: make(chan struct{})}
	if _, err := os.Stat(filepath.Join(dir, doneMarker)); err == nil {
		close(j.ready)
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

	fail := func(err error) {
		j.err = err
		log.Printf("remux failed for %s: %v", it.Title, err)
		m.mu.Lock()
		delete(m.jobs, it.ID) // allow a retry on the next request
		m.mu.Unlock()
		select {
		case <-j.ready:
		default:
			close(j.ready)
		}
		close(j.done)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		fail(fmt.Errorf("start ffmpeg: %w", err))
		return
	}

	// While ffmpeg runs, build the full playlist from the keyframe index.
	segs, err := m.plan(it, src)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		fail(fmt.Errorf("keyframe index: %w", err))
		return
	}
	if err := writeAtomic(filepath.Join(dir, "index.m3u8"), []byte(VODPlaylist(segs))); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		fail(err)
		return
	}
	log.Printf("playlist ready for %s: %d segments after %s", it.Title, len(segs), time.Since(start).Round(time.Millisecond))
	close(j.ready)

	if err := cmd.Wait(); err != nil {
		fail(fmt.Errorf("ffmpeg: %w: %s", err, tail(stderr.String(), 800)))
		return
	}
	_ = os.WriteFile(filepath.Join(dir, doneMarker), nil, 0o644)
	log.Printf("remux finished for %s in %s", it.Title, time.Since(start).Round(time.Millisecond))
	close(j.done)
}

// plan returns the segment plan, using a cached keyframe index when present.
func (m *Manager) plan(it media.Item, src string) ([]Segment, error) {
	cache := filepath.Join(m.CacheDir, "index", it.ID+".json")
	var idx keyIndex
	if b, err := os.ReadFile(cache); err == nil && json.Unmarshal(b, &idx) == nil && len(idx.Keyframes) > 0 {
		return PlanSegments(idx.Keyframes, idx.End, SegmentTarget), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	kf, end, err := Keyframes(ctx, src)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(keyIndex{Keyframes: kf, End: end}); err == nil {
		_ = os.MkdirAll(filepath.Dir(cache), 0o755)
		_ = writeAtomic(cache, b)
	}
	return PlanSegments(kf, end, SegmentTarget), nil
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
		"ffmpeg.m3u8",
	)
	return args
}

// Status reports "idle" (nothing started), "preparing" (the playlist is still
// being built) or "ready" (the playlist exists and playback can begin).
func (m *Manager) Status(id string) string {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return "idle"
	}
	select {
	case <-j.ready:
		if j.err != nil {
			return "idle"
		}
		return "ready"
	default:
		return "preparing"
	}
}

// Active counts remux jobs whose ffmpeg is still running.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, j := range m.jobs {
		select {
		case <-j.done:
		default:
			n++
		}
	}
	return n
}

// WaitPlaylist blocks until the full playlist has been written.
func (m *Manager) WaitPlaylist(ctx context.Context, id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return errors.New("no remux job for this title")
	}
	select {
	case <-j.ready:
		return j.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ErrNoSuchFile means the file will never exist (ffmpeg finished without it).
var ErrNoSuchFile = errors.New("no such segment")

// WaitFile blocks until a segment or init file exists. ffmpeg writes files
// under a temporary name and renames them when complete, so existence means
// the file is whole. The init segment is served only after the first media
// segment exists, because ffmpeg finishes it just before that.
func (m *Manager) WaitFile(ctx context.Context, id, name string) error {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return errors.New("no remux job for this title")
	}
	dir := m.Dir(id)
	gate := name
	if name == "init.mp4" {
		gate = "seg_00000.m4s"
	}
	exists := func() bool { _, err := os.Stat(filepath.Join(dir, gate)); return err == nil }
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if exists() {
			return nil
		}
		select {
		case <-j.done:
			if exists() {
				return nil
			}
			if j.err != nil {
				return j.err
			}
			return ErrNoSuchFile
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// Finished reports whether a title's remux has completed.
func (m *Manager) Finished(id string) bool {
	_, err := os.Stat(filepath.Join(m.Dir(id), doneMarker))
	return err == nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
