package hls_test

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kvmediapp/bloom/server/internal/api"
	"github.com/kvmediapp/bloom/server/internal/hls"
	"github.com/kvmediapp/bloom/server/internal/media"
)

type source struct {
	name string
	args []string // ffmpeg args between the lavfi inputs and the output path
	pre  []string // extra input args (for example a timestamp offset)
}

var sources = []source{
	{name: "irregular gops h264 b-frames", args: []string{
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "2",
		"-force_key_frames", "0,3,9,10,19,27,40,41,43,60,61,75,76,100,112,113,125,139",
		"-x264-params", "scenecut=0:keyint=infinite", "-c:a", "aac"}},
	{name: "regular gops hevc", args: []string{
		"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "keyint=72:min-keyint=72:scenecut=0:log-level=error", "-c:a", "ac3"}},
	{name: "long gops and a start offset", pre: []string{"-itsoffset", "1.5"}, args: []string{
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "3",
		"-force_key_frames", "0,14,15,40,70,71,100",
		"-x264-params", "scenecut=0:keyint=infinite", "-c:a", "aac"}},
}

func buildSource(t *testing.T, s source, path string) {
	t.Helper()
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	args = append(args, s.pre...)
	args = append(args, "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=140")
	args = append(args, s.pre...)
	args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration=140")
	args = append(args, s.args...)
	args = append(args, path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Skipf("cannot build %s: %v: %s", s.name, err, out)
	}
}

var extinf = regexp.MustCompile(`#EXTINF:([0-9.]+)`)

// TestPlanMatchesFFmpeg checks that the segments we predict from the keyframe
// index are the ones ffmpeg's HLS muxer really writes.
func TestPlanMatchesFFmpeg(t *testing.T) {
	needFFmpeg(t)
	for _, s := range sources {
		t.Run(s.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "in.mkv")
			buildSource(t, s, src)

			p, err := media.ProbeFile(context.Background(), src)
			if err != nil {
				t.Fatal(err)
			}
			plan := media.Decide(p)
			if plan.Mode == media.ModeUnsupported {
				t.Fatalf("unsupported: %v", plan.Reasons)
			}
			out := filepath.Join(dir, "out")
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("ffmpeg", hls.BuildArgs(src, plan)...)
			cmd.Dir = out
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg: %v: %s", err, b)
			}
			raw, err := os.ReadFile(filepath.Join(out, "ffmpeg.m3u8"))
			if err != nil {
				t.Fatal(err)
			}
			var actual []float64
			for _, m := range extinf.FindAllStringSubmatch(string(raw), -1) {
				d, _ := strconv.ParseFloat(m[1], 64)
				actual = append(actual, d)
			}

			kf, end, err := hls.Keyframes(context.Background(), src)
			if err != nil {
				t.Fatal(err)
			}
			predicted := hls.PlanSegments(kf, end, hls.SegmentTarget)
			if len(predicted) != len(actual) {
				t.Fatalf("segment count: predicted %d, ffmpeg wrote %d\npredicted: %v\nactual: %v", len(predicted), len(actual), predicted, actual)
			}
			for i := range actual {
				// Every cut point must match. The final segment's length is
				// only an estimate (the muxer measures it from both streams),
				// and players use the timestamps inside the segment anyway.
				tol := 0.1
				if i == len(actual)-1 {
					tol = 1.0
				}
				if math.Abs(predicted[i].Dur-actual[i]) > tol {
					t.Errorf("segment %d: predicted %.3fs, ffmpeg %.3fs", i, predicted[i].Dur, actual[i])
				}
			}
			files, _ := filepath.Glob(filepath.Join(out, "seg_*.m4s"))
			if len(files) != len(predicted) {
				t.Errorf("ffmpeg wrote %d files, plan has %d", len(files), len(predicted))
			}
		})
	}
}

// TestSeekToEndImmediately asks for the last segment before ffmpeg could have
// produced it. The server must wait for it and answer 200.
func TestSeekToEndImmediately(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	mediaDir, cacheDir := filepath.Join(dir, "media"), filepath.Join(dir, "cache")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildSource(t, sources[1], filepath.Join(mediaDir, "Seek Film.mkv"))

	lib := media.NewLibrary(mediaDir)
	if err := lib.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	it := lib.Items()[0]
	srv := httptest.NewServer((&api.Server{Lib: lib, HLS: hls.NewManager(mediaDir, cacheDir)}).Handler())
	defer srv.Close()

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(srv.URL + "/stream/" + it.ID + "/index.m3u8")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("playlist: %v %v", err, resp)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	pl := string(body)
	if !strings.Contains(pl, "#EXT-X-PLAYLIST-TYPE:VOD") || !strings.Contains(pl, "#EXT-X-ENDLIST") {
		t.Fatalf("playlist is not a complete VOD playlist:\n%s", pl)
	}
	segs := regexp.MustCompile(`seg_\d{5}\.m4s`).FindAllString(pl, -1)
	if len(segs) < 5 {
		t.Fatalf("expected a multi-segment playlist, got %d", len(segs))
	}
	for _, name := range []string{segs[len(segs)-1], "init.mp4", segs[len(segs)/2]} {
		r, err := client.Get(srv.URL + "/stream/" + it.ID + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		n, _ := io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 200 || n == 0 {
			t.Errorf("%s: status %d, %d bytes", name, r.StatusCode, n)
		}
	}
}
