package audit

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/webgraph"
	"golang.org/x/net/publicsuffix"
)

// Popularity: the sites that link to the audited site, read from the domain
// graph Common Crawl publishes every quarter (see internal/webgraph). It is a
// sample of the web, so the counts are lower than those of paid link indexes,
// and a domain absent from it is "not in the sample yet", never 0. The rank is
// Common Crawl's harmonic centrality, shown as a LaFactory benchmark: Google
// documents that it uses links between pages (ranking systems guide, "link
// analysis systems and PageRank") but publishes no score.

const (
	popTop          = 10
	popGapShown     = 15
	popCompareLimit = 300000  // referrers compared by name with the previous quarter
	popGapLimit     = 2000000 // referrers of a competitor read for the gap
)

// PopularityReport is stored with the audit result.
type PopularityReport struct {
	Release     string      `json:"release"`
	ReleaseEnd  time.Time   `json:"release_end"`
	Previous    string      `json:"previous,omitempty"`
	PreviousEnd time.Time   `json:"previous_end,omitempty"`
	Domains     uint32      `json:"domains"` // domains in the sample
	Site        DomainPop   `json:"site"`
	Top         []RefDomain `json:"top,omitempty"`
	Compared    bool        `json:"compared"` // gained and lost were measured
	Gained      []RefDomain `json:"gained,omitempty"`
	GainedCount int         `json:"gained_count"`
	Lost        []RefDomain `json:"lost,omitempty"`
	LostCount   int         `json:"lost_count"`
	Competitors []DomainPop `json:"competitors,omitempty"`
	Gap         []GapDomain `json:"gap,omitempty"`
	GapCount    int         `json:"gap_count"`
	GapMin      int         `json:"gap_min,omitempty"` // competitors a site must link to, to be listed
}

// DomainPop is what the sample says about one domain.
type DomainPop struct {
	Domain        string `json:"domain"`
	Found         bool   `json:"found"`
	Referring     int    `json:"referring"`
	Rank          uint32 `json:"rank,omitempty"` // 0: unranked
	PrevFound     bool   `json:"prev_found,omitempty"`
	PrevReferring int    `json:"prev_referring,omitempty"`
}

// RefDomain is a site that links to the audited one.
type RefDomain struct {
	Domain string `json:"domain"`
	Rank   uint32 `json:"rank,omitempty"`
}

// GapDomain links to competitors and not to the audited site.
type GapDomain struct {
	Domain string   `json:"domain"`
	Rank   uint32   `json:"rank,omitempty"`
	Cites  []string `json:"cites"`
}

// RegistrableDomain is the domain the graph aggregates a host under:
// "www.shop.co.uk" gives "shop.co.uk".
func RegistrableDomain(host string) string {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return d
	}
	return strings.TrimPrefix(host, "www.")
}

// releaseEnd reads the last month of a release from its name.
func releaseEnd(name string) time.Time {
	r, _ := webgraph.ParseRelease(name)
	return r.EndMonth
}

// BuildPopularity measures the site and its competitors in the current
// sample, and the change since the previous quarter when prev is not nil.
func BuildPopularity(cur, prev *webgraph.Index, host string, competitors []string) *PopularityReport {
	if cur == nil {
		return nil
	}
	rep := &PopularityReport{Release: cur.Meta.Release, ReleaseEnd: releaseEnd(cur.Meta.Release), Domains: cur.Meta.Nodes}
	if prev != nil {
		rep.Previous, rep.PreviousEnd = prev.Meta.Release, releaseEnd(prev.Meta.Release)
	}
	site := RegistrableDomain(host)
	rep.Site = measureDomain(cur, prev, site)
	siteRefs := map[uint32]bool{}
	if id, ok := cur.Lookup(site); ok {
		refs := cur.Referrers(id)
		for _, r := range refs {
			siteRefs[r] = true
		}
		for _, r := range cur.TopOf(refs, popTop) {
			rep.Top = append(rep.Top, RefDomain{Domain: r.Domain, Rank: r.Rank})
		}
		if prev != nil {
			rep.compare(cur, prev, id, site)
		}
	}

	seen := map[string]bool{site: true}
	var found []string
	compIDs := map[string]uint32{}
	for _, c := range competitors {
		d := RegistrableDomain(hostOf(c))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		p := measureDomain(cur, prev, d)
		rep.Competitors = append(rep.Competitors, p)
		if p.Found && p.Referring > 0 && p.Referring <= popGapLimit {
			id, _ := cur.Lookup(d)
			compIDs[d] = id
			found = append(found, d)
		}
	}
	if len(found) > 0 && rep.Site.Found {
		rep.gap(cur, site, found, compIDs, siteRefs)
	}
	return rep
}

func hostOf(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

func measureDomain(cur, prev *webgraph.Index, d string) DomainPop {
	p := DomainPop{Domain: d}
	if d == "" {
		return p
	}
	if id, ok := cur.Lookup(d); ok {
		p.Found, p.Referring, p.Rank = true, cur.InDegree(id), cur.Rank(id)
	}
	if prev != nil {
		if id, ok := prev.Lookup(d); ok {
			p.PrevFound, p.PrevReferring = true, prev.InDegree(id)
		}
	}
	return p
}

// compare lists the referring domains gained and lost since the previous
// quarter, matched by name: domain ids change from one release to the next.
func (rep *PopularityReport) compare(cur, prev *webgraph.Index, id uint32, site string) {
	pid, ok := prev.Lookup(site)
	if !ok {
		return // new in the sample: nothing to compare with
	}
	now, before := cur.Referrers(id), prev.Referrers(pid)
	if len(now) > popCompareLimit || len(before) > popCompareLimit {
		return
	}
	nowNames := make(map[string]uint32, len(now))
	for _, r := range now {
		nowNames[cur.Domain(r)] = r
	}
	beforeNames := make(map[string]uint32, len(before))
	for _, r := range before {
		beforeNames[prev.Domain(r)] = r
	}
	var gained, lost []uint32
	for name, r := range nowNames {
		if _, ok := beforeNames[name]; !ok {
			gained = append(gained, r)
		}
	}
	var lostPrev []uint32
	for name, r := range beforeNames {
		if _, ok := nowNames[name]; ok {
			continue
		}
		// A lost site still in the sample takes its current rank.
		if cid, ok := cur.Lookup(name); ok {
			lost = append(lost, cid)
		} else {
			lostPrev = append(lostPrev, r)
		}
	}
	rep.Compared = true
	rep.GainedCount, rep.LostCount = len(gained), len(lost)+len(lostPrev)
	for _, r := range cur.TopOf(gained, popTop) {
		rep.Gained = append(rep.Gained, RefDomain{Domain: r.Domain, Rank: r.Rank})
	}
	var lostAll []RefDomain
	for _, r := range cur.TopOf(lost, popTop) {
		lostAll = append(lostAll, RefDomain{Domain: r.Domain, Rank: r.Rank})
	}
	for _, r := range prev.TopOf(lostPrev, popTop) {
		lostAll = append(lostAll, RefDomain{Domain: r.Domain}) // its rank belonged to another sample
	}
	sortRefs(lostAll)
	if len(lostAll) > popTop {
		lostAll = lostAll[:popTop]
	}
	rep.Lost = lostAll
}

func sortRefs(l []RefDomain) {
	key := func(r RefDomain) uint64 {
		if r.Rank > 0 {
			return uint64(r.Rank)
		}
		return 1 << 40
	}
	sort.SliceStable(l, func(i, j int) bool {
		if key(l[i]) != key(l[j]) {
			return key(l[i]) < key(l[j])
		}
		return l[i].Domain < l[j].Domain
	})
}

// gap lists the sites that link to competitors and not to the site: to at
// least two of them when two or more are known, else to the one.
func (rep *PopularityReport) gap(cur *webgraph.Index, site string, comps []string, ids map[string]uint32, siteRefs map[uint32]bool) {
	rep.GapMin = 1
	if len(comps) >= 2 {
		rep.GapMin = 2
	}
	siteID, _ := cur.Lookup(site)
	exclude := map[uint32]bool{siteID: true}
	for _, d := range comps {
		exclude[ids[d]] = true
	}
	cites := map[uint32][]string{}
	for _, d := range comps {
		for _, r := range cur.Referrers(ids[d]) {
			if siteRefs[r] || exclude[r] {
				continue
			}
			cites[r] = append(cites[r], d)
		}
	}
	var list []uint32
	for r, c := range cites {
		if len(c) >= rep.GapMin {
			list = append(list, r)
		}
	}
	rep.GapCount = len(list)
	// More competitors cited first, then the most central.
	sort.Slice(list, func(i, j int) bool {
		a, b := len(cites[list[i]]), len(cites[list[j]])
		if a != b {
			return a > b
		}
		ra, rb := rankKey(cur.Rank(list[i])), rankKey(cur.Rank(list[j]))
		if ra != rb {
			return ra < rb
		}
		return list[i] < list[j]
	})
	if len(list) > popGapShown {
		list = list[:popGapShown]
	}
	for _, r := range list {
		rep.Gap = append(rep.Gap, GapDomain{Domain: cur.Domain(r), Rank: cur.Rank(r), Cites: cites[r]})
	}
}

func rankKey(r uint32) uint64 {
	if r == 0 {
		return 1 << 40
	}
	return uint64(r)
}

// popularityEvidence is the entry of the evidence pack.
func popularityEvidence(p *PopularityReport) map[string]interface{} {
	out := map[string]interface{}{
		"source":  "Common Crawl domain graph " + p.Release + ": a sample of the web over three months, lower counts than paid link indexes",
		"domain":  p.Site.Domain,
		"domains": p.Domains,
	}
	if !p.Site.Found {
		out["site"] = "not in the sample yet (this says nothing about the links the site has)"
		return out
	}
	site := map[string]interface{}{"sites_linking_to_it": p.Site.Referring}
	if p.Site.Rank > 0 {
		site["harmonic_centrality_rank"] = fmt.Sprintf("%d of %d (LaFactory benchmark from Common Crawl, not a Google score)", p.Site.Rank, p.Domains)
	}
	var top []string
	for _, r := range p.Top {
		top = append(top, r.Domain)
	}
	site["most_central_sites_linking"] = top
	if p.Compared {
		site["since_previous_quarter"] = map[string]int{"gained": p.GainedCount, "lost_or_not_seen_again": p.LostCount}
	}
	out["site"] = site
	if len(p.Competitors) > 0 {
		var comps []string
		for _, c := range p.Competitors {
			if c.Found {
				comps = append(comps, fmt.Sprintf("%s: %d sites linking", c.Domain, c.Referring))
			} else {
				comps = append(comps, c.Domain+": not in the sample yet")
			}
		}
		out["competitors"] = comps
		out["sites_linking_to_competitors_not_to_the_site"] = p.GapCount
	}
	return out
}
