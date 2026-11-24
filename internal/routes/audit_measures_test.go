package routes

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
	"github.com/stjudewashere/seonaut/internal/services"
)

func renderFR(t *testing.T, tpl string, data interface{}) string {
	t.Helper()
	tr, err := services.NewTranslator("../../translations", "fr")
	if err != nil {
		t.Fatal(err)
	}
	r, err := services.NewRenderer(&services.RendererConfig{TemplatesFolder: "../../web/templates"}, tr)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	r.RenderTemplate(&b, tpl, &PageView{Lang: "fr", Theme: "dark", PageTitle: "T", Data: data}, "fr")
	return b.String()
}

func lotAResult() *audit.Result {
	score, lcp, cls, tbt := 46, 4006.0, 0.02, 640.0
	copy := 5.0
	return &audit.Result{
		GeneratedAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC),
		Site:        audit.SiteFacts{PagesCrawled: 40},
		Scores:      audit.Scores{Technical: 8, Copywriting: &copy},
		Measured:    true,
		Speed: &audit.SpeedReport{URL: "https://ex.com/",
			Mobile: &audit.SpeedRun{FieldScope: "site",
				Field: &audit.FieldData{LCP: &audit.FieldMetric{P75: 1823, Rating: "good"}, INP: &audit.FieldMetric{P75: 134, Rating: "good"}, CLS: &audit.FieldMetric{P75: 0.31, Rating: "poor"}},
				Lab:   &audit.LabData{Score: &score, LCPms: &lcp, CLS: &cls, TBTms: &tbt, Opportunities: []audit.Opportunity{{Title: "Réduisez le JavaScript [inutilisé](https://x)", SavingMs: 900}}}},
			Desktop: &audit.SpeedRun{Error: "HTTP 500"}},
		Structured: &audit.StructuredReport{PagesRead: 38, PagesWith: 20, Unreadable: 1, UnreadableExamples: []string{"https://ex.com/x"}, OtherTypes: []string{"WebSite"},
			Families: []audit.SDFamily{
				{Key: "product", Pages: 12, ErrorPages: 3, Problems: []audit.SDProblem{
					{Type: "Offer", Field: "price", Kind: "missing", Severity: "error", Pages: 3, Examples: []string{"https://ex.com/p1"}},
					{Type: "Product", Field: "gtin", Kind: "missing", Severity: "warning", Pages: 12, Examples: []string{"https://ex.com/p2"}},
					{Type: "BreadcrumbList", Field: "itemListElement", Kind: "few_items", Severity: "warning", Pages: 1, Examples: []string{"https://ex.com/p3"}},
					{Type: "Plumber", Field: "aggregateRating|offers|review", Kind: "invalid", Severity: "error", Pages: 1, Examples: []string{"https://ex.com/p4"}}}},
				{Key: "breadcrumb"}, {Key: "organization", Pages: 38}, {Key: "review"}, {Key: "faq", Pages: 1}}},
		Linking: &audit.LinkingReport{Pages: 38, WeakCount: 9, Deep: 2,
			Top:          []audit.LinkedPage{{URL: "https://ex.com/", Score: 100}},
			Weak:         []audit.LinkedPage{{URL: "https://ex.com/p9", Score: 1, Product: true}, {URL: "https://ex.com/z", Inlinks: 1}},
			DeepExamples: []audit.LinkedPage{{URL: "https://ex.com/deep", Depth: 5}}},
		Duplicates: []audit.DuplicateGroup{{Pages: []string{"https://ex.com/a", "https://ex.com/b"}, Similarity: 93}},
		Competing:  []audit.CompetingPair{{A: "https://ex.com/c", B: "https://ex.com/d", Reason: "titles", TitleA: "T1", TitleB: "T2", Similarity: 90}, {A: "https://ex.com/e", B: "https://ex.com/f", Reason: "query", Query: "pompe piscine"}},
		AI: &audit.AIReport{
			Summary:        "SUMMARY",
			Criteria:       []audit.Criterion{{Group: "copywriting", Question: "value", Rating: "partial", Verdict: "V"}},
			QueriesChecked: true,
			TargetQueries: []audit.TargetQuery{
				{Page: "https://ex.com/", Query: "pompe piscine", Alignment: "good", Google: audit.QueryConfirmed, GoogleRelated: []string{"pompe piscine prix"}},
				{Page: "https://ex.com/a", Query: "électrolyseur bretagne", Alignment: "weak", Google: audit.QueryVariant, GoogleVariant: "électrolyseur piscine"}},
			DroppedQueries: []audit.TargetQuery{{Page: "https://ex.com/b", Query: "zzz inconnu", Google: audit.QueryUnknown, GoogleRelated: []string{"zzz connu"}}},
		},
		Priorities: []audit.Priority{{ID: "m:speed", Source: "technical", Level: 2, Title: "AUDIT_PRIO_SPEED"}},
	}
}

// Every key the new sections use exists in French, the sections come in the
// order of the study (searches, rich results, speed, links, competing pages,
// then the text), and a failed measurement shows no figure.
func TestLotASectionsRenderInFrench(t *testing.T) {
	a := &audit.Audit{URL: "https://ex.com/", Result: lotAResult()}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	if i := strings.Index(out, "AUDIT_"); i >= 0 {
		t.Fatalf("untranslated key: %s", out[i:i+60])
	}
	order := []string{"La recherche que chaque page", "Ce que Google peut afficher", "La vitesse vécue", "Les liens entre vos pages", "Les pages qui se font concurrence", "La façon dont la page d&#39;accueil"}
	last := -1
	for _, s := range order {
		i := strings.Index(out, s)
		if i < 0 || i < last {
			t.Errorf("section %q missing or out of order (%d after %d)", s, i, last)
		}
		last = i
	}
	for _, s := range []string{
		"1,8 s", "134 ms", "0,31", "bon jusqu&#39;à 2,5 s", "Note de la simulation", "46", "Réduisez le JavaScript inutilisé", "gain possible : 0,9 s",
		"Offre : il manque le prix", "(conseillé par Google)", "Fil d&#39;Ariane : une seule étape", "Commerce ou entreprise locale : format incorrect pour le prix, la note ou un avis",
		"Recherches écartées (1)", "zzz inconnu", "Google propose plutôt : « zzz connu »", "4,0 s", "Proposée par Google", "« électrolyseur piscine »", "« pompe piscine prix »",
		"Pages peu reliées : 9", "/p9 (fiche produit)", "5 clics", "texte commun à 93 %", "même recherche visée", "Paires : 2",
		"sur l&#39;ensemble du site", "Vitesse vécue par vos visiteurs sur téléphone",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	if strings.Contains(out, "Sur ordinateur") {
		t.Error("the failed desktop measurement must show no line and no figure")
	}
	if strings.Contains(out, "https://x") {
		t.Error("Markdown links of Lighthouse titles must be reduced to their text")
	}
}

// A report saved before 1.7.0 has none of the new data: no new section shows.
func TestOldReportShowsNoLotASection(t *testing.T) {
	res := lotAResult()
	res.Speed, res.Structured, res.Linking, res.Duplicates, res.Competing, res.Measured = nil, nil, nil, nil, nil, false
	res.AI.QueriesChecked, res.AI.DroppedQueries = false, nil
	res.AI.TargetQueries = []audit.TargetQuery{{Page: "https://ex.com/", Query: "q", Alignment: "good"}}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(&audit.Audit{URL: "https://ex.com/", Result: res}, "fr", true)})
	for _, s := range []string{"Ce que Google peut afficher", "La vitesse vécue", "Les liens entre vos pages", "Les pages qui se font concurrence", "Proposée par Google", "Recherches écartées"} {
		if strings.Contains(out, s) {
			t.Errorf("old report shows %q", s)
		}
	}
	if !strings.Contains(out, "Aucun volume de recherche") {
		t.Error("old reports keep the former help text of the searches")
	}
}

func TestSpeedViewFailedHasNoFigure(t *testing.T) {
	v := newSpeedRunView(&audit.SpeedRun{Error: "x"}, "fr")
	if !v.Failed || v.Field != nil || v.Lab != nil {
		t.Errorf("%+v", v)
	}
}
