package media

import (
	"fmt"
	"strings"
)

// Playback modes, cheapest first.
const (
	ModeRemux       = "remux"       // repackage video and audio without re-encoding
	ModeRemuxAudio  = "remux+audio" // copy video, convert the audio track
	ModeUnsupported = "unsupported" // needs a video transcode (not built yet)
)

// Plan says how a file will be turned into Apple-compatible HLS.
type Plan struct {
	Mode       string   `json:"mode"`
	Video      string   `json:"video"` // "copy" or "none"
	Audio      string   `json:"audio"` // "copy", "eac3", "aac" or "none"
	Reasons    []string `json:"reasons"`
	AudioIndex int      `json:"-"` // absolute stream index, -1 when none
	TagHVC1    bool     `json:"-"`
}

// Decide picks the cheapest way to play a probed file on Apple devices.
func Decide(p *Probe) Plan {
	plan := Plan{Video: "copy", Audio: "none", AudioIndex: -1, Reasons: []string{}}

	v := p.FirstVideo()
	if v == nil {
		plan.Mode, plan.Video = ModeUnsupported, "none"
		plan.Reasons = append(plan.Reasons, "no video stream")
		return plan
	}

	switch v.CodecName {
	case "hevc":
		plan.TagHVC1 = true
		if dv := v.DolbyVisionProfile(); dv == 7 {
			plan.Reasons = append(plan.Reasons,
				"Dolby Vision profile 7 needs conversion to profile 8.1 (Phase 2); it may not play correctly until then")
		}
	case "h264":
		if strings.Contains(v.PixFmt, "10") {
			plan.Mode, plan.Video = ModeUnsupported, "none"
			plan.Reasons = append(plan.Reasons, "10-bit H.264 is not supported by Apple hardware and needs a video transcode")
			return plan
		}
	default:
		plan.Mode, plan.Video = ModeUnsupported, "none"
		plan.Reasons = append(plan.Reasons, fmt.Sprintf("%s video needs a transcode", v.CodecName))
		return plan
	}

	plan.Mode = ModeRemux
	audio := p.AudioStreams()
	if len(audio) == 0 {
		return plan
	}

	// Prefer the track flagged default, else the first one.
	a := audio[0]
	for _, s := range audio {
		if s.Disposition["default"] == 1 {
			a = s
			break
		}
	}
	plan.AudioIndex = a.Index

	switch a.CodecName {
	case "aac", "ac3", "eac3":
		plan.Audio = "copy"
	default:
		plan.Mode = ModeRemuxAudio
		if a.Channels > 2 {
			plan.Audio = "eac3"
		} else {
			plan.Audio = "aac"
		}
		plan.Reasons = append(plan.Reasons,
			fmt.Sprintf("%s audio is converted to %s; lossless and object-based detail (such as Atmos) is lost", a.CodecName, plan.Audio))
	}
	return plan
}
