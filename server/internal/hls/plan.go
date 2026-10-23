package hls

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Segment is one planned HLS segment.
type Segment struct {
	Start float64 // seconds, in the source's own timeline
	Dur   float64
}

// SegmentTarget is the nominal segment length, matching -hls_time.
const SegmentTarget = 6.0

// Keyframes reads the presentation times of every video keyframe in a file,
// plus the time the video ends. It demuxes the whole file, so it takes a few
// seconds to a minute on large files; callers cache the result.
func Keyframes(ctx context.Context, path string) (kf []float64, end float64, err error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pts_time,duration_time,flags", "-of", "csv=p=0", path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		f := strings.Split(strings.TrimSpace(sc.Text()), ",")
		if len(f) < 2 {
			continue
		}
		pts, perr := strconv.ParseFloat(f[0], 64)
		if perr != nil {
			continue
		}
		if len(f) >= 3 {
			if d, derr := strconv.ParseFloat(f[1], 64); derr == nil && pts+d > end {
				end = pts + d
			}
		}
		if pts > end {
			end = pts
		}
		if strings.Contains(f[len(f)-1], "K") {
			kf = append(kf, pts)
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, 0, fmt.Errorf("ffprobe: %w", err)
	}
	if len(kf) == 0 {
		return nil, 0, fmt.Errorf("no video keyframes found")
	}
	sort.Float64s(kf)
	return kf, end, nil
}

// PlanSegments predicts where ffmpeg's HLS muxer will cut a stream-copied
// video. The muxer ends segment n at the first keyframe at or after
// n*target seconds from the first keyframe (and after the previous cut), so
// the plan can be computed before ffmpeg has produced anything.
func PlanSegments(kf []float64, end, target float64) []Segment {
	if len(kf) == 0 {
		return nil
	}
	first := kf[0]
	starts := []float64{first}
	i, n := 0, 1
	for {
		j := i + 1
		for j < len(kf) && kf[j]-first < float64(n)*target {
			j++
		}
		if j >= len(kf) {
			break
		}
		starts = append(starts, kf[j])
		i, n = j, n+1
	}
	segs := make([]Segment, len(starts))
	for k, s := range starts {
		next := end
		if k+1 < len(starts) {
			next = starts[k+1]
		}
		d := next - s
		if d <= 0 {
			d = 0.04
		}
		segs[k] = Segment{Start: s, Dur: d}
	}
	return segs
}

// VODPlaylist renders a complete, seekable playlist for the planned segments.
func VODPlaylist(segs []Segment) string {
	maxDur := 0.0
	for _, s := range segs {
		if s.Dur > maxDur {
			maxDur = s.Dur
		}
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", int(maxDur)+1)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	b.WriteString("#EXT-X-MAP:URI=\"init.mp4\"\n")
	for k, s := range segs {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\nseg_%05d.m4s\n", s.Dur, k)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
