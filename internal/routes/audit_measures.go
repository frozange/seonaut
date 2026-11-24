package routes

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/stjudewashere/seonaut/internal/audit"
)

// View of the measurements added in 1.7.0: speed, structured data, internal
// links and near duplicate pages. Figures are formatted here so the
// template only places them; a measurement that failed carries no figure.

type speedView struct {
	Mobile  *speedRunView
	Desktop *speedRunView
}

type speedRunView struct {
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
	Saving string
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
	return &speedView{Mobile: newSpeedRunView(s.Mobile, lang), Desktop: newSpeedRunView(s.Desktop, lang)}
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
		if f.CLS != nil {
			v.Field = append(v.Field, metricView{Key: "AUDIT_SPEED_CLS", Value: decimal(f.CLS.P75, 2, lang), Threshold: decimal(0.1, 2, lang), Class: ratingClass(f.CLS.Rating)})
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
		if l.CLS != nil {
			lv.Metrics = append(lv.Metrics, metricView{Key: "AUDIT_SPEED_CLS", Value: decimal(*l.CLS, 2, lang), Threshold: decimal(0.1, 2, lang), Class: ratingClass(audit.RateMetric("CLS", *l.CLS))})
		}
		for _, o := range l.Opportunities {
			lv.Opportunities = append(lv.Opportunities, opportunityView{Title: stripMarkdownLinks(o.Title), Saving: seconds(float64(o.SavingMs), lang)})
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
	v := &linkingView{Pages: l.Pages, Top: linkRows(l.Top), Weak: linkRows(l.Weak), WeakCount: l.WeakCount}
	switch {
	case l.WeakCount*10 >= l.Pages:
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
