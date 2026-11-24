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
	competeMin     = 0.85 // weighted title, description, H1 and length similarity
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

// CompetingPair is two pages that answer the same search.
type CompetingPair struct {
	A          string `json:"a"`
	B          string `json:"b"`
	Reason     string `json:"reason"` // "titles" or "query"
	Similarity int    `json:"similarity,omitempty"`
	TitleA     string `json:"title_a,omitempty"`
	TitleB     string `json:"title_b,omitempty"`
	Query      string `json:"query,omitempty"`
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

// Weights of LibreCrawl's near-duplicate score (issue_detector.py
// _duplication_similarity, MIT): Dice similarity of the words of the title,
// the description and the H1, plus the ratio of the word counts.
const (
	wTitle = 0.35
	wDesc  = 0.35
	wH1    = 0.20
	wWords = 0.10
)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, t := range Tokens(s) {
		m[t] = true
	}
	return m
}

func dice(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := 0
	for t := range a {
		if b[t] {
			n++
		}
	}
	return 2 * float64(n) / float64(len(a)+len(b))
}

func lengthSimilarity(a, b int) float64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	return math.Min(float64(a), float64(b)) / math.Max(float64(a), float64(b))
}

// CompetingPages finds pairs of pages whose title, description and H1 say the
// same thing: Google sees two candidates for one search and may show neither
// well. Pairs already reported as near duplicates and translations are left out.
func CompetingPages(pages []*PageSignal, dups []DuplicateGroup) []CompetingPair {
	inDup := map[string]int{}
	for i, g := range dups {
		for _, u := range g.Pages {
			inDup[u] = i + 1
		}
	}
	type fields struct{ title, desc, h1 map[string]bool }
	cand := comparable(pages)
	f := make([]fields, len(cand))
	for i, p := range cand {
		f[i] = fields{wordSet(p.Title), wordSet(p.Description), wordSet(p.H1)}
	}
	var out []CompetingPair
	for i := range cand {
		if len(f[i].title) == 0 && len(f[i].desc) == 0 {
			continue
		}
		for j := i + 1; j < len(cand); j++ {
			a, b := cand[i], cand[j]
			if (inDup[a.URL] != 0 && inDup[a.URL] == inDup[b.URL]) || translations(a, b) {
				continue
			}
			s := dice(f[i].title, f[j].title) * wTitle
			if s+wDesc+wH1+wWords < competeMin {
				continue
			}
			s += dice(f[i].desc, f[j].desc)*wDesc + dice(f[i].h1, f[j].h1)*wH1 + lengthSimilarity(a.Words, b.Words)*wWords
			if s >= competeMin {
				out = append(out, CompetingPair{A: a.URL, B: b.URL, Reason: "titles", Similarity: int(math.Floor(s * 100)), TitleA: a.Title, TitleB: b.Title})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	return out
}

// QueryKey normalises a search so that two pages aimed at the same words in
// another order, case or accent compare equal.
func QueryKey(q string) string {
	t := ContentTokens(q)
	sort.Strings(t)
	return strings.Join(t, " ")
}

// SameQueryPairs pairs the key pages to which the AI gave the same target search.
func SameQueryPairs(queries []TargetQuery, pages []*PageSignal) []CompetingPair {
	byURL := map[string]*PageSignal{}
	for _, p := range pages {
		byURL[p.URL] = p
	}
	byKey := map[string][]TargetQuery{}
	var keys []string
	for _, q := range queries {
		k := QueryKey(q.Query)
		if k == "" {
			continue
		}
		if _, ok := byKey[k]; !ok {
			keys = append(keys, k)
		}
		byKey[k] = append(byKey[k], q)
	}
	var out []CompetingPair
	for _, k := range keys {
		qs := byKey[k]
		for i := 0; i < len(qs); i++ {
			for j := i + 1; j < len(qs); j++ {
				a, b := normalizeURL(qs[i].Page), normalizeURL(qs[j].Page)
				if a == b {
					continue
				}
				if pa, pb := byURL[a], byURL[b]; pa != nil && pb != nil && translations(pa, pb) {
					continue
				}
				out = append(out, CompetingPair{A: a, B: b, Reason: "query", Query: qs[i].Query})
			}
		}
	}
	return out
}
