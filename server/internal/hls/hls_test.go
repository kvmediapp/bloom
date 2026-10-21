package hls_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kvmediapp/bloom/server/internal/api"
	"github.com/kvmediapp/bloom/server/internal/hls"
	"github.com/kvmediapp/bloom/server/internal/media"
)

func needFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// makeSource builds a short HEVC test file with the given audio codec.
func makeSource(t *testing.T, path, audioCodec string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24:duration=14",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=14",
		"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "keyint=24:log-level=error",
		"-c:a", audioCodec, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create test source (%s): %v: %s", audioCodec, err, out)
	}
}

func probeCodecs(t *testing.T, playlist string) (video, tag, audio string) {
	t.Helper()
	p, err := media.ProbeFile(context.Background(), playlist)
	if err != nil {
		t.Fatal(err)
	}
	v := p.FirstVideo()
	if v == nil {
		t.Fatal("output has no video")
	}
	video = v.CodecName
	raw, _ := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_tag_string", "-of", "default=nw=1:nk=1", playlist).Output()
	if fields := strings.Fields(string(raw)); len(fields) > 0 {
		tag = fields[0] // ffprobe may repeat the line for HLS variants
	}
	if a := p.AudioStreams(); len(a) > 0 {
		audio = a[0].CodecName
	}
	return
}

func TestEndToEndRemux(t *testing.T) {
	needFFmpeg(t)

	for _, tc := range []struct {
		name, audioIn, audioOut string
	}{
		{"ac3 audio is copied", "ac3", "ac3"},
		{"flac audio is converted to aac", "flac", "aac"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mediaDir, cacheDir := filepath.Join(dir, "media"), filepath.Join(dir, "cache")
			if err := os.MkdirAll(mediaDir, 0o755); err != nil {
				t.Fatal(err)
			}
			makeSource(t, filepath.Join(mediaDir, "Test Film.mkv"), tc.audioIn)

			lib := media.NewLibrary(mediaDir)
			if err := lib.Scan(context.Background()); err != nil {
				t.Fatal(err)
			}
			items := lib.Items()
			if len(items) != 1 {
				t.Fatalf("got %d items, want 1", len(items))
			}
			it := items[0]
			if it.Plan.Mode == media.ModeUnsupported {
				t.Fatalf("plan unsupported: %v", it.Plan.Reasons)
			}

			srv := httptest.NewServer((&api.Server{Lib: lib, HLS: hls.NewManager(mediaDir, cacheDir)}).Handler())
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream/"+it.ID+"/index.m3u8", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("playlist status %d", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
				t.Errorf("content type %q", ct)
			}

			// Wait for the remux to finish, then check what was produced.
			deadline := time.Now().Add(60 * time.Second)
			for !(hls.NewManager(mediaDir, cacheDir)).Finished(it.ID) && time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
			}
			outDir := filepath.Join(cacheDir, "hls", it.ID)
			for _, f := range []string{"init.mp4", "seg_00000.m4s", "index.m3u8"} {
				if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
					t.Fatalf("missing %s: %v", f, err)
				}
			}
			pl, _ := os.ReadFile(filepath.Join(outDir, "index.m3u8"))
			if !strings.Contains(string(pl), "#EXT-X-ENDLIST") {
				t.Error("finished playlist has no ENDLIST")
			}

			video, tag, audio := probeCodecs(t, filepath.Join(outDir, "index.m3u8"))
			if video != "hevc" || tag != "hvc1" {
				t.Errorf("video = %s/%s, want hevc/hvc1", video, tag)
			}
			if audio != tc.audioOut {
				t.Errorf("audio = %s, want %s", audio, tc.audioOut)
			}

			// The library listing is valid JSON with our fields.
			listResp, err := http.Get(srv.URL + "/api/items")
			if err != nil {
				t.Fatal(err)
			}
			defer listResp.Body.Close()
			var listed []media.Item
			if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil || len(listed) != 1 {
				t.Fatalf("items listing: %v (%d items)", err, len(listed))
			}
		})
	}
}

func TestStreamRejectsOddFileNames(t *testing.T) {
	lib := media.NewLibrary(t.TempDir())
	srv := httptest.NewServer((&api.Server{Lib: lib, HLS: hls.NewManager(t.TempDir(), t.TempDir())}).Handler())
	defer srv.Close()
	for _, path := range []string{"/stream/abc/..%2Fsecret", "/stream/abc/evil.sh", "/stream/missing/index.m3u8"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestAuthToken(t *testing.T) {
	lib := media.NewLibrary(t.TempDir())
	srv := httptest.NewServer((&api.Server{Lib: lib, HLS: hls.NewManager(t.TempDir(), t.TempDir()), Token: "s3cret"}).Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/items")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: status %d, want 401", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/items", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("good token: status %d, want 200", resp.StatusCode)
	}
	resp, _ = http.Get(srv.URL + "/health")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health should not need a token, got %d", resp.StatusCode)
	}
}
