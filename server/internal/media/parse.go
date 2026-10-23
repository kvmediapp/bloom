package media

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	episodeRe = regexp.MustCompile(`(?i)\bS(\d{1,2})E(\d{1,3})(?:-?E\d{1,3})*`)
	yearRe    = regexp.MustCompile(`\s*\((\d{4})\)`)
	tagStart  = regexp.MustCompile(`\s*[\[{]`)
)

// Names holds what can be read from a TRaSH-style path without any web lookup.
type Names struct {
	Kind    string // "movie" or "episode"
	Title   string // movie title or episode title
	Year    int
	Show    string
	Season  int
	Episode int
}

// cleanName drops {tmdb-..}/[Bluray-1080p ...] tags and a (year).
func cleanName(s string) (string, int) {
	if loc := tagStart.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	year := 0
	if m := yearRe.FindStringSubmatch(s); m != nil {
		year, _ = strconv.Atoi(m[1])
		s = yearRe.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s), year
}

// ParsePath reads names from a path relative to the media directory.
func ParsePath(rel string) Names {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	base := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
	if m := episodeRe.FindStringSubmatchIndex(base); m != nil {
		n := Names{Kind: "episode"}
		n.Season, _ = strconv.Atoi(base[m[2]:m[3]])
		n.Episode, _ = strconv.Atoi(base[m[4]:m[5]])
		show := strings.TrimRight(strings.TrimSpace(base[:m[0]]), "- ")
		if len(parts) >= 3 { // shows/<Show>/Season 01/file
			show = parts[1]
		}
		n.Show, n.Year = cleanName(show)
		rest := strings.TrimLeft(base[m[1]:], " -")
		n.Title, _ = cleanName(rest)
		if n.Title == "" {
			n.Title = "Episode " + strconv.Itoa(n.Episode)
		}
		return n
	}
	n := Names{Kind: "movie"}
	n.Title, n.Year = cleanName(base)
	if n.Title == "" {
		n.Title = base
	}
	return n
}

// enrich fills the display fields on an item from its path.
func enrich(it *Item) {
	n := ParsePath(it.Path)
	it.Kind, it.Year = n.Kind, n.Year
	it.Show, it.Season, it.Episode = n.Show, n.Season, n.Episode
	it.Title = n.Title
}
