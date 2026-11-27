package routes

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
)

// View of the measurements added in 1.7.0: speed, structured data, internal
// links and near duplicate pages. Figures are formatted here so the
// template only places them; a measurement that failed carries no figure.

type speedView struct {
	Mobile  *speedRunView
	Desktop *speedRunView
}

// The report shows no CLS tile (Francis, 2026-11-27: "je ne veux pas le cls");
// Google's overall field rating still includes it.
type speedRunView struct {
	Device     string // translation key suffix: "" for mobile, "_DESKTOP"
	Failed     bool
	HasField   bool
	FieldScope string // page or site
	Overall    string // good, mid, low
	Field      []metricView
	Lab        *labView
}

type metricView struct {
	Key       string // translation key of the label
	Value     string
	Threshold string
	Class     string // good, mid, low
}

type labView struct {
	Score         string
	ScoreClass    string
	Metrics       []metricView
	Opportunities []opportunityView
}

type opportunityView struct {
	Title  string
	Key    string // plain wording of a known Lighthouse advice, when there is one
	Saving string
}

// psiAdvice gives the plain wording of the Lighthouse advice a business owner
// meets most. Google's own titles ("Requêtes de blocage du rendu") are kept
// only for the others.
var psiAdvice = map[string]string{
	"render-blocking-resources": "AUDIT_PSI_RENDER_BLOCKING",
	"render-blocking-insight":   "AUDIT_PSI_RENDER_BLOCKING",
	"unused-javascript":         "AUDIT_PSI_UNUSED_JS",
	"unused-css-rules":          "AUDIT_PSI_UNUSED_CSS",
	"modern-image-formats":      "AUDIT_PSI_IMAGES",
	"uses-optimized-images":     "AUDIT_PSI_IMAGES",
	"uses-responsive-images":    "AUDIT_PSI_IMAGES",
	"image-delivery-insight":    "AUDIT_PSI_IMAGES",
	"offscreen-images":          "AUDIT_PSI_OFFSCREEN",
	"server-response-time":      "AUDIT_PSI_SERVER",
	"document-latency-insight":  "AUDIT_PSI_SERVER",
	"uses-text-compression":     "AUDIT_PSI_COMPRESSION",
}

func ratingClass(r string) string {
	switch r {
	case "good":
		return "good"
	case "average":
		return "mid"
	}
	return "low"
}

// decimal keeps its digits: "4,0 s" next to "bon jusqu'à 2,5 s" reads as a
// measure, "4 s" as a rounding.
func decimal(v float64, digits int, lang string) string {
	s := fmt.Sprintf("%.*f", digits, v)
	if lang == "fr" {
		s = strings.ReplaceAll(s, ".", ",")
	}
	return s
}

// A no-break space keeps a figure and its unit on one line.
func seconds(ms float64, lang string) string { return decimal(ms/1000, 1, lang) + "\u00a0s" }

func millis(ms float64) string { return fmt.Sprintf("%.0f\u00a0ms", ms) }

func newSpeedView(s *audit.SpeedReport, lang string) *speedView {
	if s == nil {
		return nil
	}
	v := &speedView{Mobile: newSpeedRunView(s.Mobile, lang), Desktop: newSpeedRunView(s.Desktop, lang)}
	if v.Desktop != nil {
		v.Desktop.Device = "_DESKTOP"
	}
	return v
}

func newSpeedRunView(r *audit.SpeedRun, lang string) *speedRunView {
	if r == nil {
		return nil
	}
	if r.Error != "" {
		return &speedRunView{Failed: true}
	}
	v := &speedRunView{}
	if f := r.Field; f != nil {
		v.HasField, v.FieldScope, v.Overall = true, r.FieldScope, ratingClass(f.Overall())
		if f.LCP != nil {
			v.Field = append(v.Field, metricView{Key: "AUDIT_SPEED_LCP", Value: seconds(f.LCP.P75, lang), Threshold: seconds(2500, lang), Class: ratingClass(f.LCP.Rating)})
		}
		if f.INP != nil {
			v.Field = append(v.Field, metricView{Key: "AUDIT_SPEED_INP", Value: millis(f.INP.P75), Threshold: millis(200), Class: ratingClass(f.INP.Rating)})
		}
	}
	if l := r.Lab; l != nil {
		lv := &labView{}
		if l.Score != nil {
			lv.Score = fmt.Sprint(*l.Score)
			switch {
			case *l.Score >= 90:
				lv.ScoreClass = "good"
			case *l.Score >= 50:
				lv.ScoreClass = "mid"
			default:
				lv.ScoreClass = "low"
			}
		}
		if l.LCPms != nil {
			lv.Metrics = append(lv.Metrics, metricView{Key: "AUDIT_SPEED_LCP", Value: seconds(*l.LCPms, lang), Threshold: seconds(2500, lang), Class: ratingClass(audit.RateMetric("LCP", *l.LCPms))})
		}
		if l.TBTms != nil {
			c := "good"
			if *l.TBTms > 600 {
				c = "low"
			} else if *l.TBTms > 200 {
				c = "mid"
			}
			lv.Metrics = append(lv.Metrics, metricView{Key: "AUDIT_SPEED_TBT", Value: millis(*l.TBTms), Threshold: millis(200), Class: c})
		}
		for _, o := range l.Opportunities {
			lv.Opportunities = append(lv.Opportunities, opportunityView{Title: stripMarkdownLinks(o.Title), Key: psiAdvice[o.ID], Saving: seconds(float64(o.SavingMs), lang)})
		}
		v.Lab = lv
	}
	return v
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// stripMarkdownLinks keeps the text of the Markdown links Lighthouse puts in titles.
func stripMarkdownLinks(s string) string { return mdLink.ReplaceAllString(s, "$1") }

type sdView struct {
	PagesRead          int
	PagesWith          int
	Families           []sdFamilyView
	Unreadable         int
	UnreadableExamples []string
	OtherTypes         string
}

type sdFamilyView struct {
	Key          string
	Class        string // good, mid, low, none
	Pages        int
	ErrorPages   int
	WarningPages int
	Problems     []sdProblemView
	Examples     []string
}

type sdProblemView struct {
	Type     string // translation suffix of the type
	Field    string // translation suffix of the field
	Kind     string // translation suffix of the problem
	Error    bool
	Pages    int
	Examples []string
}

var keySafe = regexp.MustCompile(`[^A-Za-z0-9]+`)

// sdDisplayType groups the LocalBusiness subtypes under one label.
func sdDisplayType(t string) string {
	switch t {
	case "Product", "Offer", "AggregateOffer", "AggregateRating", "Rating", "Review", "Brand", "Person",
		"BreadcrumbList", "ListItem", "OfferShippingDetails", "MerchantReturnPolicy", "Organization",
		"PostalAddress", "FAQPage", "Question":
		return t
	}
	return "LocalBusiness"
}

func newSDView(r *audit.StructuredReport) *sdView {
	if r == nil || r.PagesRead == 0 {
		return nil
	}
	v := &sdView{PagesRead: r.PagesRead, PagesWith: r.PagesWith, Unreadable: r.Unreadable, OtherTypes: strings.Join(r.OtherTypes, ", ")}
	for _, u := range r.UnreadableExamples {
		v.UnreadableExamples = append(v.UnreadableExamples, pagePath(u))
	}
	for _, f := range r.Families {
		// Reviews and questions are shown when the site has them; the three
		// others are what any shop or local business can expect.
		if f.Pages == 0 && (f.Key == "review" || f.Key == "faq") {
			continue
		}
		fv := sdFamilyView{Key: f.Key, Pages: f.Pages, ErrorPages: f.ErrorPages, WarningPages: f.WarningPages}
		switch {
		case f.Pages == 0:
			fv.Class = "none"
		case f.ErrorPages > 0:
			fv.Class = "low"
		case f.WarningPages > 0:
			fv.Class = "mid"
		default:
			fv.Class = "good"
		}
		for _, u := range f.Examples {
			fv.Examples = append(fv.Examples, pagePath(u))
		}
		for _, p := range f.Problems {
			pv := sdProblemView{Type: sdDisplayType(p.Type), Field: keySafe.ReplaceAllString(p.Field, "_"), Kind: p.Kind, Error: p.Severity == "error", Pages: p.Pages}
			for _, u := range p.Examples {
				pv.Examples = append(pv.Examples, pagePath(u))
			}
			fv.Problems = append(fv.Problems, pv)
		}
		v.Families = append(v.Families, fv)
	}
	return v
}

type linkingView struct {
	Pages     int
	Class     string
	Top       []linkRow
	Weak      []linkRow
	WeakCount int
	Orphans   int
}

type linkRow struct {
	Path    string
	Score   int
	Bar     int
	Inlinks int
	Depth   int
	Product bool
}

func linkRows(list []audit.LinkedPage) []linkRow {
	var out []linkRow
	for _, p := range list {
		out = append(out, linkRow{Path: pagePath(p.URL), Score: p.Score, Bar: int(math.Max(3, float64(p.Score))), Inlinks: p.Inlinks, Depth: p.Depth, Product: p.Product})
	}
	return out
}

func newLinkingView(l *audit.LinkingReport) *linkingView {
	if l == nil {
		return nil
	}
	v := &linkingView{Pages: l.Pages, Top: linkRows(l.Top), Weak: linkRows(l.Weak), WeakCount: l.WeakCount, Orphans: l.Orphans}
	switch {
	case l.Orphans > 0:
		v.Class = "mid"
	default:
		v.Class = "good"
	}
	return v
}

type dupView struct {
	Groups []dupGroupView
	Pages  int // indexable pages compared
}

type dupGroupView struct {
	Paths      []string
	Similarity int
}

func newDupView(res *audit.Result) *dupView {
	if !res.Measured {
		return nil
	}
	v := &dupView{}
	if res.Structured != nil {
		v.Pages = res.Structured.PagesRead
	}
	for _, g := range res.Duplicates {
		gv := dupGroupView{Similarity: g.Similarity}
		for _, u := range g.Pages {
			gv.Paths = append(gv.Paths, pagePath(u))
		}
		v.Groups = append(v.Groups, gv)
	}
	return v
}

// count groups the thousands: "134 979 706" in French (narrow no-break
// space), "134,979,706" in English.
func count(n uint64, lang string) string {
	s := strconv.FormatUint(n, 10)
	sep := ","
	if lang == "fr" {
		sep = " "
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	return b.String()
}

type popView struct {
	Start, End         time.Time
	StartLayout        string // month alone, or with its year across a new year
	PrevStart, PrevEnd time.Time
	PrevStartLayout    string
	HasPrevious        bool
	Domains            string
	Site               popDomainView
	Top                []popRefView
	Compared           bool
	NewInSample        bool // absent from the previous quarter's sample
	Gained, Lost       []popRefView
	GainedCount        string
	LostCount          string
	GainedN, LostN     int
	Competitors        []popDomainView // the site first
	Gap                []popGapView
	GapCount           string
	GapN, GapMin       int
	HasCompetitors     bool
}

type popDomainView struct {
	Domain     string
	You        bool
	Found      bool
	Referring  string
	ReferringN int
	Rank       string
	TopPct     string // "2" for "among the top 2 %", empty past half the sample
}

type popRefView struct {
	Domain string
	Rank   string
}

type popGapView struct {
	Domain string
	Rank   string
	Cites  []string
}

func periodStart(end time.Time) (time.Time, string) {
	start := end.AddDate(0, -2, 0)
	if start.Year() != end.Year() {
		return start, "January 2006"
	}
	return start, "January"
}

// topPct is the share of the sample ranked at or above rank, rounded up.
func topPct(rank, total uint32, lang string) string {
	if rank == 0 || total == 0 {
		return ""
	}
	pct := float64(rank) * 100 / float64(total)
	switch {
	case pct > 50:
		return ""
	case pct < 1:
		return decimal(math.Max(0.1, math.Ceil(pct*10)/10), 1, lang)
	}
	return fmt.Sprint(math.Ceil(pct))
}

func newPopDomainView(d audit.DomainPop, total uint32, lang string, you bool) popDomainView {
	v := popDomainView{Domain: d.Domain, You: you, Found: d.Found}
	if d.Found {
		v.Referring, v.ReferringN = count(uint64(d.Referring), lang), d.Referring
		if d.Rank > 0 {
			v.Rank, v.TopPct = count(uint64(d.Rank), lang), topPct(d.Rank, total, lang)
		}
	}
	return v
}

func popRefs(l []audit.RefDomain, lang string) []popRefView {
	var out []popRefView
	for _, r := range l {
		v := popRefView{Domain: r.Domain}
		if r.Rank > 0 {
			v.Rank = count(uint64(r.Rank), lang)
		}
		out = append(out, v)
	}
	return out
}

func newPopView(p *audit.PopularityReport, lang string) *popView {
	if p == nil {
		return nil
	}
	v := &popView{End: p.ReleaseEnd, Domains: count(uint64(p.Domains), lang),
		Site: newPopDomainView(p.Site, p.Domains, lang, true), Top: popRefs(p.Top, lang),
		Compared: p.Compared, Gained: popRefs(p.Gained, lang), Lost: popRefs(p.Lost, lang),
		GainedN: p.GainedCount, LostN: p.LostCount,
		GainedCount: count(uint64(p.GainedCount), lang), LostCount: count(uint64(p.LostCount), lang)}
	v.Start, v.StartLayout = periodStart(p.ReleaseEnd)
	if p.Previous != "" {
		v.HasPrevious, v.PrevEnd = true, p.PreviousEnd
		v.PrevStart, v.PrevStartLayout = periodStart(p.PreviousEnd)
		v.NewInSample = p.Site.Found && !p.Site.PrevFound
	}
	if len(p.Competitors) > 0 {
		v.HasCompetitors = true
		v.Competitors = append(v.Competitors, v.Site)
		for _, c := range p.Competitors {
			v.Competitors = append(v.Competitors, newPopDomainView(c, p.Domains, lang, false))
		}
		v.GapN, v.GapMin, v.GapCount = p.GapCount, p.GapMin, count(uint64(p.GapCount), lang)
		for _, g := range p.Gap {
			gv := popGapView{Domain: g.Domain, Cites: g.Cites}
			if g.Rank > 0 {
				gv.Rank = count(uint64(g.Rank), lang)
			}
			v.Gap = append(v.Gap, gv)
		}
	}
	return v
}
