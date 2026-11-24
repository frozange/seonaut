package audit

import (
	"fmt"
	"sort"
	"strings"
)

// Structured data families, in the order of the report. A root item belongs
// to the family of its type; nested items (offers, ratings) count for it.
var sdFamilies = []string{"product", "breadcrumb", "organization", "review", "faq"}

func sdFamily(rootType string) string {
	switch {
	case rootType == "Product":
		return "product"
	case rootType == "BreadcrumbList":
		return "breadcrumb"
	case organizationTypes[rootType] || localBusinessTypes[rootType]:
		return "organization"
	case rootType == "AggregateRating" || rootType == "Review":
		return "review"
	case rootType == "FAQPage":
		return "faq"
	}
	return ""
}

// StructuredReport sums up the structured data of the indexable pages.
type StructuredReport struct {
	PagesRead          int        `json:"pages_read"`
	PagesWith          int        `json:"pages_with"`
	Families           []SDFamily `json:"families"`
	OtherTypes         []string   `json:"other_types,omitempty"`
	Unreadable         int        `json:"unreadable,omitempty"`
	UnreadableExamples []string   `json:"unreadable_examples,omitempty"`
}

// SDFamily is one kind of rich result.
type SDFamily struct {
	Key          string      `json:"key"`
	Pages        int         `json:"pages"`
	ErrorPages   int         `json:"error_pages"`
	WarningPages int         `json:"warning_pages"`
	Problems     []SDProblem `json:"problems,omitempty"`
	Examples     []string    `json:"examples,omitempty"`
}

// SDProblem is one field problem and the pages it touches.
type SDProblem struct {
	Type     string   `json:"type"`
	Field    string   `json:"field"`
	Kind     string   `json:"kind"`
	Severity string   `json:"severity"`
	Pages    int      `json:"pages"`
	Examples []string `json:"examples,omitempty"`
}

// BuildStructured aggregates the page items by family.
func BuildStructured(pages []*PageSignal) *StructuredReport {
	rep := &StructuredReport{}
	fam := map[string]*SDFamily{}
	probs := map[string]map[SDIssue]*SDProblem{}
	others := map[string]int{}
	for _, k := range sdFamilies {
		fam[k] = &SDFamily{Key: k}
		probs[k] = map[SDIssue]*SDProblem{}
	}
	for _, p := range pages {
		if !p.Indexable() {
			continue
		}
		rep.PagesRead++
		if p.BadJSONLD > 0 {
			rep.Unreadable++
			if len(rep.UnreadableExamples) < 5 {
				rep.UnreadableExamples = append(rep.UnreadableExamples, p.URL)
			}
		}
		if len(p.Structured) > 0 {
			rep.PagesWith++
		}
		type state struct{ seen, err, warn bool }
		pageFam := map[string]*state{}
		pageIssue := map[string]map[SDIssue]bool{}
		for _, it := range p.Structured {
			k := sdFamily(it.Type)
			if k == "" {
				others[it.Type]++
				continue
			}
			st := pageFam[k]
			if st == nil {
				st = &state{}
				pageFam[k] = st
				pageIssue[k] = map[SDIssue]bool{}
			}
			st.seen = true
			for _, is := range it.Issues {
				if is.Severity == "error" {
					st.err = true
				} else {
					st.warn = true
				}
				pageIssue[k][is] = true
			}
		}
		for k, st := range pageFam {
			f := fam[k]
			f.Pages++
			if len(f.Examples) < 3 {
				f.Examples = append(f.Examples, p.URL)
			}
			if st.err {
				f.ErrorPages++
			} else if st.warn {
				f.WarningPages++
			}
			for is := range pageIssue[k] {
				pr := probs[k][is]
				if pr == nil {
					pr = &SDProblem{Type: is.Type, Field: is.Field, Kind: is.Kind, Severity: is.Severity}
					probs[k][is] = pr
				}
				pr.Pages++
				if len(pr.Examples) < 3 {
					pr.Examples = append(pr.Examples, p.URL)
				}
			}
		}
	}
	for _, k := range sdFamilies {
		f := fam[k]
		for _, pr := range probs[k] {
			f.Problems = append(f.Problems, *pr)
		}
		sort.Slice(f.Problems, func(i, j int) bool {
			a, b := f.Problems[i], f.Problems[j]
			if a.Severity != b.Severity {
				return a.Severity == "error"
			}
			if a.Pages != b.Pages {
				return a.Pages > b.Pages
			}
			return a.Type+a.Field < b.Type+b.Field
		})
		if len(f.Problems) > 8 {
			f.Problems = f.Problems[:8]
		}
		rep.Families = append(rep.Families, *f)
	}
	for t := range others {
		rep.OtherTypes = append(rep.OtherTypes, t)
	}
	sort.Strings(rep.OtherTypes)
	if len(rep.OtherTypes) > 12 {
		rep.OtherTypes = rep.OtherTypes[:12]
	}
	return rep
}

// Family returns one family of the report, nil when absent.
func (r *StructuredReport) Family(key string) *SDFamily {
	if r == nil {
		return nil
	}
	for i := range r.Families {
		if r.Families[i].Key == key {
			return &r.Families[i]
		}
	}
	return nil
}

// MeasureEvidence adds the measurements of this module to the evidence pack,
// so the AI may cite them, and returns the id of each entry by topic.
func MeasureEvidence(res *Result, evidence []EvidenceEntry) ([]EvidenceEntry, map[string]string) {
	ids := map[string]string{}
	add := func(topic, label string, data interface{}) {
		id := fmt.Sprintf("E%d", len(evidence)+1)
		evidence = append(evidence, EvidenceEntry{ID: id, Label: label, Data: data})
		ids[topic] = id
	}
	if s := res.Speed; s != nil {
		add("speed", "Speed of the home page (PageSpeed Insights: field data are real Chrome visits over 28 days, lab data one simulated visit)", speedEvidence(s))
	}
	if s := res.Structured; s != nil {
		var fams []map[string]interface{}
		for _, f := range s.Families {
			if f.Pages == 0 {
				continue
			}
			var problems []string
			for _, p := range f.Problems {
				problems = append(problems, fmt.Sprintf("%s %s %s (%s, %d pages)", p.Type, p.Field, p.Kind, p.Severity, p.Pages))
			}
			fams = append(fams, map[string]interface{}{"rich_result": f.Key, "pages": f.Pages, "pages_with_errors": f.ErrorPages,
				"pages_with_missing_recommended_fields_only": f.WarningPages, "problems": problems})
		}
		add("structured", "Structured data for Google rich results, on indexable pages", map[string]interface{}{
			"indexable_pages_read": s.PagesRead, "pages_with_structured_data": s.PagesWith, "families": fams,
			"other_types": s.OtherTypes, "pages_with_unreadable_json_ld": s.Unreadable,
		})
	}
	if l := res.Linking; l != nil {
		top := []string{}
		for _, p := range l.Top {
			top = append(top, fmt.Sprintf("%s (ease of access %d/100, linked from %d pages)", p.URL, p.Score, p.Inlinks))
		}
		weak := []string{}
		for _, p := range l.Weak {
			weak = append(weak, fmt.Sprintf("%s (linked from %d pages)", p.URL, p.Inlinks))
		}
		add("linking", "Internal links: how easily visitors and Google's crawler reach each page through the site's own links (LaFactory measure on a 0 to 100 scale, not a score Google uses)", map[string]interface{}{
			"indexable_pages": l.Pages, "pages_easiest_to_reach": top,
			"pages_linked_from_one_page_or_none": l.WeakCount, "pages_no_other_page_links_to": l.Orphans, "examples_weakly_linked": weak,
		})
	}
	if p := res.Popularity; p != nil {
		add("popularity", "Sites that link to the audited site, from the Common Crawl domain graph (a sample of the web; the rank is a LaFactory benchmark, Google publishes no link score)", popularityEvidence(p))
	}
	if len(res.Duplicates) > 0 {
		var dups [][]string
		for _, g := range res.Duplicates {
			dups = append(dups, g.Pages)
		}
		add("duplicates", "Groups of pages whose main text is a near identical copy (menus and footer aside, translations excluded)", map[string]interface{}{
			"near_duplicate_groups": dups,
		})
	}
	return evidence, ids
}

func speedEvidence(s *SpeedReport) map[string]interface{} {
	out := map[string]interface{}{"url": s.URL}
	for name, run := range map[string]*SpeedRun{"mobile": s.Mobile, "desktop": s.Desktop} {
		if run == nil {
			continue
		}
		if run.Error != "" {
			out[name] = "measurement failed, no figure"
			continue
		}
		m := map[string]interface{}{}
		if f := run.Field; f != nil {
			fd := map[string]interface{}{"scope": run.FieldScope, "overall": f.Overall()}
			if f.LCP != nil {
				fd["largest_contentful_paint_seconds_p75"] = round1(f.LCP.P75 / 1000)
				fd["lcp_rating"] = f.LCP.Rating
			}
			if f.INP != nil {
				fd["interaction_to_next_paint_ms_p75"] = f.INP.P75
				fd["inp_rating"] = f.INP.Rating
			}
			if f.CLS != nil {
				fd["cumulative_layout_shift_p75"] = f.CLS.P75
				fd["cls_rating"] = f.CLS.Rating
			}
			m["field_real_visits"] = fd
		} else {
			m["field_real_visits"] = "not enough real visits for Google to publish field data"
		}
		if l := run.Lab; l != nil {
			lab := map[string]interface{}{}
			if l.Score != nil {
				lab["lighthouse_score_out_of_100"] = *l.Score
			}
			if l.LCPms != nil {
				lab["largest_contentful_paint_seconds"] = round1(*l.LCPms / 1000)
			}
			var adv []string
			for _, o := range l.Opportunities {
				adv = append(adv, o.Title)
			}
			lab["main_advice"] = adv
			m["lab_simulation"] = lab
		}
		out[name] = m
	}
	return out
}

// MeasuredPriorities turns the measurements into priorities the action plan
// ranks with the others. Speed counts on field data only: the lab is a
// simulation, not what visitors live.
func MeasuredPriorities(res *Result, ev map[string]string) []Priority {
	var out []Priority
	evid := func(topic string) []string {
		if id, ok := ev[topic]; ok {
			return []string{id}
		}
		return nil
	}
	if s := res.Speed; s != nil && s.Mobile != nil && s.Mobile.Field != nil {
		switch s.Mobile.Field.Overall() {
		case "poor":
			out = append(out, Priority{ID: "m:speed", Source: "technical", Level: 2, Title: "AUDIT_PRIO_SPEED", Evidence: evid("speed")})
		case "average":
			out = append(out, Priority{ID: "m:speed", Source: "technical", Level: 3, Title: "AUDIT_PRIO_SPEED", Evidence: evid("speed")})
		}
	}
	if f := res.Structured.Family("product"); f != nil && f.ErrorPages > 0 {
		out = append(out, Priority{ID: "m:structured", Source: "technical", Level: 2, Title: "AUDIT_PRIO_STRUCTURED", Pages: f.ErrorPages, Evidence: evid("structured")})
	}
	if n := duplicatePages(res.Duplicates); n > 0 {
		out = append(out, Priority{ID: "m:duplicates", Source: "technical", Level: 3, Title: "AUDIT_PRIO_DUPLICATES", Pages: n, Evidence: evid("duplicates")})
	}
	// Google finds a page through any crawlable link: only a page no other page
	// links to (found through the sitemap at best) makes a priority.
	if l := res.Linking; l != nil && l.Orphans > 0 {
		out = append(out, Priority{ID: "m:linking", Source: "technical", Level: 3, Title: "AUDIT_PRIO_LINKING", Pages: l.Orphans, Evidence: evid("linking")})
	}
	return out
}

func duplicatePages(g []DuplicateGroup) int {
	n := 0
	for _, x := range g {
		n += len(x.Pages)
	}
	return n
}

// keyURLs is the set of key page addresses.
func keyURLs(pages []PageFacts) map[string]bool {
	m := map[string]bool{}
	for _, p := range pages {
		m[normalizeURL(p.URL)] = true
	}
	return m
}

// pageLang returns the declared language of the home page.
func pageLang(pages []PageFacts) string {
	if len(pages) == 0 {
		return ""
	}
	return strings.TrimSpace(pages[0].Lang)
}

// RefreshMeasuredPriorities drops the measured priorities (m:...) of a saved
// report that the current rules no longer make, keeping the AI text of the
// others. Reports saved before the orphan count read it from their examples.
func RefreshMeasuredPriorities(res *Result) {
	if l := res.Linking; l != nil && l.Orphans == 0 {
		for _, p := range l.Weak {
			if p.Inlinks == 0 {
				l.Orphans++
			}
		}
	}
	still := map[string]bool{}
	for _, p := range MeasuredPriorities(res, nil) {
		still[p.ID] = true
	}
	var kept []Priority
	for _, p := range res.Priorities {
		if strings.HasPrefix(p.ID, "m:") && !still[p.ID] {
			continue
		}
		kept = append(kept, p)
	}
	res.Priorities = kept
}
