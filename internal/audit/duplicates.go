package audit

import (
	"hash/fnv"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	shingleWords   = 5    // words per shingle
	sketchSize     = 256  // bottom-k sketch kept per page
	nearDupMin     = 0.85 // estimated Jaccard similarity of the main texts
	minDupWords    = 60   // shorter texts are left out: an empty template is not content
	maxComparisons = 3000 // pages compared at most
)

// Shingles returns the bottom-k sketch of the 5-word shingles of a text: the
// sketchSize smallest 64-bit hashes, sorted. Two sketches estimate the Jaccard
// similarity of the full shingle sets.
func Shingles(text string) []uint64 {
	words := Tokens(text)
	if len(words) < shingleWords {
		return nil
	}
	set := map[uint64]bool{}
	for i := 0; i+shingleWords <= len(words); i++ {
		h := fnv.New64a()
		h.Write([]byte(strings.Join(words[i:i+shingleWords], " ")))
		set[h.Sum64()] = true
	}
	out := make([]uint64, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) > sketchSize {
		out = out[:sketchSize]
	}
	return out
}

// Similarity estimates the Jaccard similarity of two shingle sets from their
// sketches: among the k smallest hashes of the union, the share both hold.
func Similarity(a, b []uint64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	k := sketchSize
	if len(a) < k {
		k = len(a)
	}
	if len(b) < k {
		k = len(b)
	}
	i, j, union, both := 0, 0, 0, 0
	for union < k && (i < len(a) || j < len(b)) {
		switch {
		case j >= len(b) || (i < len(a) && a[i] < b[j]):
			i++
		case i >= len(a) || b[j] < a[i]:
			j++
		default:
			both++
			i++
			j++
		}
		union++
	}
	if union == 0 {
		return 0
	}
	return float64(both) / float64(union)
}

// Tokens lowercases a text, removes accents and keeps runs of letters and digits.
func Tokens(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// DuplicateGroup is a set of pages whose main texts are near identical.
type DuplicateGroup struct {
	Pages      []string `json:"pages"`
	Similarity int      `json:"similarity"` // percent, lowest pair kept in the group
}

var paginated = regexp.MustCompile(`(?i)(/page/\d+/?$)|([?&](page|paged|p)=\d+)`)

// comparable keeps indexable pages that are not a pagination of a listing.
func comparable(pages []*PageSignal) []*PageSignal {
	var out []*PageSignal
	seen := map[string]bool{}
	for _, p := range pages {
		if !p.Indexable() || seen[p.URL] || paginated.MatchString(p.URL) {
			continue
		}
		seen[p.URL] = true
		out = append(out, p)
		if len(out) >= maxComparisons {
			break
		}
	}
	return out
}

// translations reports two pages that are language versions of each other:
// declared languages differ, or one lists the other in its hreflang links.
func translations(a, b *PageSignal) bool {
	la, lb := primaryLang(a.Lang), primaryLang(b.Lang)
	if la != "" && lb != "" && la != lb {
		return true
	}
	for _, h := range a.Hreflangs {
		if h == b.URL {
			return true
		}
	}
	for _, h := range b.Hreflangs {
		if h == a.URL {
			return true
		}
	}
	return false
}

func primaryLang(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	if i := strings.IndexAny(l, "-_"); i >= 0 {
		l = l[:i]
	}
	return l
}

// NearDuplicates groups the pages whose main texts (menus, header and footer
// left out) are near identical. Translations are never grouped together.
func NearDuplicates(pages []*PageSignal) []DuplicateGroup {
	var cand []*PageSignal
	for _, p := range comparable(pages) {
		if p.Words >= minDupWords && len(p.Shingles) >= 10 {
			cand = append(cand, p)
		}
	}
	parent := make([]int, len(cand))
	low := make([]float64, len(cand))
	for i := range parent {
		parent[i], low[i] = i, 1
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range cand {
		for j := i + 1; j < len(cand); j++ {
			if translations(cand[i], cand[j]) {
				continue
			}
			s := Similarity(cand[i].Shingles, cand[j].Shingles)
			if s < nearDupMin {
				continue
			}
			ri, rj := find(i), find(j)
			m := math.Min(s, math.Min(low[ri], low[rj]))
			if ri != rj {
				parent[rj] = ri
			}
			low[ri] = m
		}
	}
	groups := map[int][]string{}
	for i := range cand {
		r := find(i)
		groups[r] = append(groups[r], cand[i].URL)
	}
	var out []DuplicateGroup
	for r, g := range groups {
		if len(g) < 2 {
			continue
		}
		sort.Strings(g)
		out = append(out, DuplicateGroup{Pages: g, Similarity: int(math.Floor(low[r] * 100))})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Pages) != len(out[j].Pages) {
			return len(out[i].Pages) > len(out[j].Pages)
		}
		return out[i].Pages[0] < out[j].Pages[0]
	})
	return out
}

// QueryKey normalises a search so that two pages aimed at the same words in
// another order, case or accent compare equal.
func QueryKey(q string) string {
	t := ContentTokens(q)
	sort.Strings(t)
	return strings.Join(t, " ")
}
