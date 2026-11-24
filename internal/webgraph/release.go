// Package webgraph turns the domain-level web graph Common Crawl publishes
// every quarter (vertices, edges, harmonic centrality ranks) into a compact
// on-disk index that answers, in milliseconds, which domains link to a given
// domain and how central each of them is.
//
// Data: https://commoncrawl.org/web-graphs (free to use, see the Common Crawl
// terms of use). Method: commoncrawl/cc-webgraph (Apache-2.0), whose output
// files are read as published; no code of it is copied here.
package webgraph

import (
	"fmt"
	"strings"
	"time"
)

// BaseURL is where Common Crawl publishes the graphs; tests point it to a
// local server.
var BaseURL = "https://data.commoncrawl.org/projects/hyperlinkgraph"

var monthNames = []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}

// Release is one published graph, covering three consecutive months of crawls.
type Release struct {
	Name     string    // e.g. cc-main-2026-aug-sep-oct
	EndMonth time.Time // first day of the last month covered
}

// ReleaseEnding returns the names Common Crawl uses for a graph whose last
// month is end: "cc-main-2026-aug-sep-oct", and across a new year both
// "cc-main-2025-26-nov-dec-jan" (the form in use) and "cc-main-2025-nov-dec-jan".
func ReleaseEnding(end time.Time) []Release {
	end = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, -2, 0)
	var months []string
	for m := start; !m.After(end); m = m.AddDate(0, 1, 0) {
		months = append(months, monthNames[m.Month()-1])
	}
	span := strings.Join(months, "-")
	if start.Year() == end.Year() {
		return []Release{{Name: fmt.Sprintf("cc-main-%d-%s", end.Year(), span), EndMonth: end}}
	}
	return []Release{
		{Name: fmt.Sprintf("cc-main-%d-%02d-%s", start.Year(), end.Year()%100, span), EndMonth: end},
		{Name: fmt.Sprintf("cc-main-%d-%s", start.Year(), span), EndMonth: end},
	}
}

// ParseRelease reads a release name back. ok is false for a name this package
// would not produce.
func ParseRelease(name string) (Release, bool) {
	rest, found := strings.CutPrefix(name, "cc-main-")
	if !found {
		return Release{}, false
	}
	parts := strings.Split(rest, "-")
	if len(parts) < 4 {
		return Release{}, false
	}
	last := parts[len(parts)-1]
	mi := -1
	for i, m := range monthNames {
		if m == last {
			mi = i
		}
	}
	var year int
	if _, err := fmt.Sscanf(parts[0], "%d", &year); err != nil || mi < 0 || year < 2000 {
		return Release{}, false
	}
	first := parts[len(parts)-3]
	if monthIndex(first) > mi {
		year++ // the window crosses a new year: the name carries the start year
	}
	end := time.Date(year, time.Month(mi+1), 1, 0, 0, 0, 0, time.UTC)
	for _, r := range ReleaseEnding(end) {
		if r.Name == name {
			return r, true
		}
	}
	return Release{}, false
}

func monthIndex(m string) int {
	for i, n := range monthNames {
		if n == m {
			return i
		}
	}
	return -1
}

// Candidates lists the releases that may have been published after the one
// ending in after (zero: none known yet), newest first, up to now. Common
// Crawl publishes a graph a few weeks after its last month.
func Candidates(after, now time.Time) []Release {
	last := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	first := last.AddDate(0, -8, 0)
	if !after.IsZero() {
		first = after.AddDate(0, 1, 0)
	}
	var out []Release
	for m := last; !m.Before(first); m = m.AddDate(0, -1, 0) {
		out = append(out, ReleaseEnding(m)...)
	}
	return out
}

// PreviousCandidates lists the releases that may precede r without
// overlapping it, newest first: the quarter gained and lost domains are
// measured against.
func PreviousCandidates(r Release) []Release {
	var out []Release
	for k := 3; k <= 5; k++ {
		out = append(out, ReleaseEnding(r.EndMonth.AddDate(0, -k, 0))...)
	}
	return out
}

// Files are the three files of a release this package reads.
var Files = []string{"vertices", "edges", "ranks"}

// FileURL is the address of one file of a release.
func FileURL(release, file string) string {
	return fmt.Sprintf("%s/%s/domain/%s-domain-%s.txt.gz", strings.TrimRight(BaseURL, "/"), release, release, file)
}
