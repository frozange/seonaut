package audit

import (
	"math"
	"sort"
)

// Link Score: PageRank over the internal link graph on a 0 to 100 log scale,
// ported from LibreCrawl (src/core/crawl_analysis.py _link_score, MIT, see
// THIRD_PARTY_NOTICES). The home page usually reads 100; a page few others
// link to reads close to 0.
const (
	linkDamping    = 0.85
	linkIterations = 40
)

// LinkGraph is the internal graph of the crawl: HTML pages answered 200, and
// the links between them once redirects are followed.
type LinkGraph struct {
	URLs     []string
	Index    map[string]int
	Incoming [][]int // sources linking to each page
	Outdeg   []int
}

// BuildLinkGraph keeps the pages answered 200 as nodes; a link to a redirect
// counts for the page it ends on (at most five hops), like the link equity
// Google passes through redirects.
func BuildLinkGraph(pages []*PageSignal) *LinkGraph {
	g := &LinkGraph{Index: map[string]int{}}
	redirect := map[string]string{}
	for _, p := range pages {
		if p.Redirect != "" {
			redirect[p.URL] = p.Redirect
			continue
		}
		if p.Status != 200 {
			continue
		}
		if _, dup := g.Index[p.URL]; !dup {
			g.Index[p.URL] = len(g.URLs)
			g.URLs = append(g.URLs, p.URL)
		}
	}
	resolve := func(u string) (int, bool) {
		for hop := 0; hop < 5; hop++ {
			if i, ok := g.Index[u]; ok {
				return i, true
			}
			next, ok := redirect[u]
			if !ok {
				return 0, false
			}
			u = next
		}
		return 0, false
	}
	n := len(g.URLs)
	g.Incoming = make([][]int, n)
	g.Outdeg = make([]int, n)
	for _, p := range pages {
		s, ok := g.Index[p.URL]
		if !ok || p.Redirect != "" || p.Status != 200 {
			continue
		}
		seen := map[int]bool{}
		for _, l := range p.Links {
			d, ok := resolve(l)
			if !ok || d == s || seen[d] {
				continue
			}
			seen[d] = true
			g.Incoming[d] = append(g.Incoming[d], s)
			g.Outdeg[s]++
		}
	}
	return g
}

// LinkScores returns the Link Score of every node, indexed like g.URLs.
func LinkScores(g *LinkGraph) []int {
	n := len(g.URLs)
	scores := make([]int, n)
	if n == 0 {
		return scores
	}
	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1 / float64(n)
	}
	teleport := (1 - linkDamping) / float64(n)
	var dangling []int
	hasLinks := false
	for i, d := range g.Outdeg {
		if d == 0 {
			dangling = append(dangling, i)
		} else {
			hasLinks = true
		}
	}
	share := make([]float64, n)
	next := make([]float64, n)
	for it := 0; it < linkIterations; it++ {
		for i := range rank {
			share[i] = 0
			if g.Outdeg[i] > 0 {
				share[i] = rank[i] * linkDamping / float64(g.Outdeg[i])
			}
		}
		var dang float64
		for _, i := range dangling {
			dang += rank[i]
		}
		base := teleport + linkDamping*dang/float64(n)
		var delta float64
		for d := range next {
			v := base
			for _, s := range g.Incoming[d] {
				v += share[s]
			}
			next[d] = v
			delta += math.Abs(v - rank[d])
		}
		rank, next = next, rank
		if delta < 1e-9*float64(n) {
			break
		}
	}
	low, high := rank[0], rank[0]
	for _, v := range rank {
		low, high = math.Min(low, v), math.Max(high, v)
	}
	// Ranks equal up to float noise (a cycle, a fully linked site) score equally.
	if high <= low*(1+1e-6) {
		for i := range scores {
			if hasLinks || n == 1 {
				scores[i] = 100
			}
		}
		return scores
	}
	span := math.Log(high / low)
	for i, v := range rank {
		s := int(math.Round(100 * math.Log(v/low) / span))
		scores[i] = int(math.Max(0, math.Min(100, float64(s))))
	}
	return scores
}

// LinkedPage is one page of the linking section.
type LinkedPage struct {
	URL     string `json:"url"`
	Score   int    `json:"score"`
	Inlinks int    `json:"inlinks"` // other pages linking to it
	Depth   int    `json:"depth"`
	Product bool   `json:"product,omitempty"`
}

// LinkingReport is the internal linking section of the report.
type LinkingReport struct {
	Pages        int          `json:"pages"` // indexable pages scored
	Top          []LinkedPage `json:"top"`
	Weak         []LinkedPage `json:"weak"`       // examples, product and key pages first
	WeakCount    int          `json:"weak_count"` // indexable pages linked from one page or none
	Deep         int          `json:"deep"`       // indexable pages four clicks or more from the home page
	DeepExamples []LinkedPage `json:"deep_examples,omitempty"`
}

// BuildLinking scores the pages and picks what the report shows. important
// lists the pages to show first among the weak ones (key pages). Pages found
// only in the sitemap carry depth 0 like the home page, so the home page is
// told apart by its address.
func BuildLinking(pages []*PageSignal, home string, important map[string]bool) *LinkingReport {
	g := BuildLinkGraph(pages)
	if len(g.URLs) == 0 {
		return nil
	}
	scores := LinkScores(g)
	byURL := map[string]*PageSignal{}
	for _, p := range pages {
		if _, ok := byURL[p.URL]; !ok && p.Redirect == "" {
			byURL[p.URL] = p
		}
	}
	var all []LinkedPage
	for i, u := range g.URLs {
		p := byURL[u]
		if p == nil || !p.Indexable() {
			continue
		}
		all = append(all, LinkedPage{URL: u, Score: scores[i], Inlinks: len(g.Incoming[i]), Depth: p.Depth, Product: hasType(p, "Product")})
	}
	if len(all) == 0 {
		return nil
	}
	rep := &LinkingReport{Pages: len(all)}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	for i := 0; i < len(all) && i < 5; i++ {
		rep.Top = append(rep.Top, all[i])
	}
	var weak, deep []LinkedPage
	for _, p := range all {
		if p.Inlinks <= 1 && p.URL != home {
			weak = append(weak, p)
		}
		if p.Depth >= 4 {
			deep = append(deep, p)
		}
	}
	rep.WeakCount, rep.Deep = len(weak), len(deep)
	rank := func(p LinkedPage) int {
		switch {
		case important[p.URL]:
			return 0
		case p.Product:
			return 1
		}
		return 2
	}
	sort.SliceStable(weak, func(i, j int) bool {
		if rank(weak[i]) != rank(weak[j]) {
			return rank(weak[i]) < rank(weak[j])
		}
		return weak[i].Score < weak[j].Score
	})
	sort.SliceStable(deep, func(i, j int) bool { return deep[i].Depth > deep[j].Depth })
	rep.Weak = firstPages(weak, 8)
	rep.DeepExamples = firstPages(deep, 5)
	return rep
}

func firstPages(p []LinkedPage, n int) []LinkedPage {
	if len(p) > n {
		return p[:n]
	}
	return p
}

func hasType(p *PageSignal, t string) bool {
	for _, it := range p.Structured {
		if it.Type == t {
			return true
		}
	}
	return false
}
