// Package media probes video files and decides how each one will be served.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Stream is the subset of ffprobe's stream output Bloom cares about.
type Stream struct {
	Index         int               `json:"index"`
	CodecType     string            `json:"codec_type"`
	CodecName     string            `json:"codec_name"`
	Profile       string            `json:"profile"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	PixFmt        string            `json:"pix_fmt"`
	ColorTransfer string            `json:"color_transfer"`
	Channels      int               `json:"channels"`
	Tags          map[string]string `json:"tags"`
	Disposition   map[string]int    `json:"disposition"`
	SideData      []map[string]any  `json:"side_data_list"`
}

// Probe is the parsed result of `ffprobe -show_format -show_streams`.
type Probe struct {
	Streams []Stream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
}

// ProbeFile runs ffprobe on path.
func ProbeFile(ctx context.Context, path string) (*Probe, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	var p Probe
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, fmt.Errorf("parse ffprobe output for %s: %w", path, err)
	}
	return &p, nil
}

func (p *Probe) DurationSec() float64 {
	f, _ := strconv.ParseFloat(p.Format.Duration, 64)
	return f
}

func (p *Probe) SizeBytes() int64 {
	n, _ := strconv.ParseInt(p.Format.Size, 10, 64)
	return n
}

// FirstVideo returns the first real video stream (cover art is skipped).
func (p *Probe) FirstVideo() *Stream {
	for i := range p.Streams {
		s := &p.Streams[i]
		if s.CodecType == "video" && s.Disposition["attached_pic"] == 0 {
			return s
		}
	}
	return nil
}

// AudioStreams returns all audio streams in file order.
func (p *Probe) AudioStreams() []*Stream {
	var out []*Stream
	for i := range p.Streams {
		if p.Streams[i].CodecType == "audio" {
			out = append(out, &p.Streams[i])
		}
	}
	return out
}

// DolbyVisionProfile returns the Dolby Vision profile number, or 0 when the
// stream carries no Dolby Vision configuration.
func (s *Stream) DolbyVisionProfile() int {
	for _, sd := range s.SideData {
		t, _ := sd["side_data_type"].(string)
		if !strings.Contains(strings.ToUpper(t), "DOVI") {
			continue
		}
		if v, ok := sd["dv_profile"].(float64); ok {
			return int(v)
		}
	}
	return 0
}

// HDRLabel is a short human label for the stream's dynamic range.
func (s *Stream) HDRLabel() string {
	if dv := s.DolbyVisionProfile(); dv > 0 {
		return fmt.Sprintf("Dolby Vision P%d", dv)
	}
	switch s.ColorTransfer {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	return "SDR"
}
