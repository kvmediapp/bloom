package media

import "testing"

func TestParsePath(t *testing.T) {
	cases := []struct {
		rel  string
		want Names
	}{
		{"movies/Heat (1995)/Heat (1995) {imdb-tt0113277} [Bluray-1080p][DTS 5.1][x264]-GRP.mkv",
			Names{Kind: "movie", Title: "Heat", Year: 1995}},
		{"shows/The Bear (2022)/Season 02/The Bear (2022) - S02E03 - Sundae [WEBDL-1080p][EAC3 5.1][h264]-GRP.mkv",
			Names{Kind: "episode", Show: "The Bear", Year: 2022, Season: 2, Episode: 3, Title: "Sundae"}},
		{"shows/Show/Season 01/Show - S01E01-E02 - Pilot.mkv",
			Names{Kind: "episode", Show: "Show", Season: 1, Episode: 1, Title: "Pilot"}},
		{"movies/Odd.mkv", Names{Kind: "movie", Title: "Odd"}},
	}
	for _, c := range cases {
		if got := ParsePath(c.rel); got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.rel, got, c.want)
		}
	}
}
