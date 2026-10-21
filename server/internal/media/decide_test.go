package media

import "testing"

func stream(typ, codec string, extra func(*Stream)) Stream {
	s := Stream{CodecType: typ, CodecName: codec, Disposition: map[string]int{}}
	if extra != nil {
		extra(&s)
	}
	return s
}

func TestDecide(t *testing.T) {
	hevc := func(s *Stream) { s.PixFmt = "yuv420p10le"; s.Index = 0 }
	audio := func(idx, ch int) func(*Stream) {
		return func(s *Stream) { s.Index = idx; s.Channels = ch }
	}

	tests := []struct {
		name       string
		streams    []Stream
		wantMode   string
		wantAudio  string
		wantTagHVC bool
	}{
		{"hevc + eac3 is a plain remux", []Stream{stream("video", "hevc", hevc), stream("audio", "eac3", audio(1, 6))}, ModeRemux, "copy", true},
		{"hevc + dts converts audio to eac3", []Stream{stream("video", "hevc", hevc), stream("audio", "dts", audio(1, 6))}, ModeRemuxAudio, "eac3", true},
		{"h264 + flac stereo converts audio to aac", []Stream{stream("video", "h264", func(s *Stream) { s.PixFmt = "yuv420p" }), stream("audio", "flac", audio(1, 2))}, ModeRemuxAudio, "aac", false},
		{"10-bit h264 is unsupported", []Stream{stream("video", "h264", func(s *Stream) { s.PixFmt = "yuv420p10le" }), stream("audio", "aac", audio(1, 2))}, ModeUnsupported, "none", false},
		{"av1 is unsupported", []Stream{stream("video", "av1", nil), stream("audio", "aac", audio(1, 2))}, ModeUnsupported, "none", false},
		{"no audio still remuxes", []Stream{stream("video", "hevc", hevc)}, ModeRemux, "none", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := Decide(&Probe{Streams: tc.streams})
			if plan.Mode != tc.wantMode {
				t.Errorf("mode = %q, want %q (reasons: %v)", plan.Mode, tc.wantMode, plan.Reasons)
			}
			if plan.Audio != tc.wantAudio {
				t.Errorf("audio = %q, want %q", plan.Audio, tc.wantAudio)
			}
			if plan.TagHVC1 != tc.wantTagHVC {
				t.Errorf("TagHVC1 = %v, want %v", plan.TagHVC1, tc.wantTagHVC)
			}
		})
	}
}

func TestDecidePrefersDefaultAudio(t *testing.T) {
	commentary := stream("audio", "aac", func(s *Stream) { s.Index = 1; s.Channels = 2 })
	main := stream("audio", "eac3", func(s *Stream) { s.Index = 2; s.Channels = 6; s.Disposition = map[string]int{"default": 1} })
	plan := Decide(&Probe{Streams: []Stream{stream("video", "hevc", nil), commentary, main}})
	if plan.AudioIndex != 2 {
		t.Errorf("AudioIndex = %d, want 2 (the default track)", plan.AudioIndex)
	}
}

func TestDolbyVisionProfile7IsFlagged(t *testing.T) {
	v := stream("video", "hevc", func(s *Stream) {
		s.SideData = []map[string]any{{"side_data_type": "DOVI configuration record", "dv_profile": float64(7)}}
	})
	plan := Decide(&Probe{Streams: []Stream{v}})
	if len(plan.Reasons) == 0 {
		t.Fatal("expected a warning for Dolby Vision profile 7")
	}
	if got := v.HDRLabel(); got != "Dolby Vision P7" {
		t.Errorf("HDRLabel = %q", got)
	}
}
