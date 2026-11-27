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
				Lab:   &audit.LabData{Score: &score, LCPms: &lcp, CLS: &cls, TBTms: &tbt, Opportunities: []audit.Opportunity{{Title: "Réduisez le JavaScript [inutilisé](https://x)", SavingMs: 900}, {ID: "render-blocking-insight", Title: "Requêtes de blocage du rendu", SavingMs: 800}}}},
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
	order := []string{"La recherche que chaque page", "Ce que Google peut afficher", "La vitesse de votre site", "Les liens entre vos pages", "Les pages en double", "La façon dont la page d&#39;accueil"}
	last := -1
	for _, s := range order {
		i := strings.Index(out, s)
		if i < 0 || i < last {
			t.Errorf("section %q missing or out of order (%d after %d)", s, i, last)
		}
		last = i
	}
	for _, s := range []string{
		"1,8\u00a0s", "134\u00a0ms", "Indice PageSpeed", "46", "Réduisez le JavaScript inutilisé", "jusqu&#39;à 0,9\u00a0s à gagner", "Fichiers de style et scripts qui retardent l&#39;affichage de la page",
		"Offre : il manque le prix", "(conseillé par Google)", "Fil d&#39;Ariane : une seule étape", "Commerce ou entreprise locale : format incorrect pour le prix, la note ou un avis",
		"Recherches écartées (1)", "zzz inconnu", "Google propose plutôt : « zzz connu »", "4,0\u00a0s", "Proposée par Google", "« électrolyseur piscine »", "« pompe piscine prix »",
		"Pages peu reliées : 9", "/p9 (fiche produit)", "texte commun à 93 %", "Groupes de pages en double : 1",
		"Core Web Vitals sur mobile", "Vitesse mobile", "Vitesse bureau", "Largest Contentful Paint (LCP), visites réelles",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	if !strings.Contains(out, "Mesure indisponible ce jour-là.") {
		t.Error("the failed desktop measurement must say so")
	}
	// Two cards, "Vitesse mobile" and "Vitesse bureau", without explanations
	// (Francis, 2026-11-27: "vitesse mobile / bureau point final").
	for _, s := range []string{"Données de laboratoire", "Données sur le terrain", "Lighthouse", "28 jours"} {
		if strings.Contains(out, s) {
			t.Errorf("explanation shown: %q", s)
		}
	}
	// No CLS tile (Francis, 2026-11-27: "je ne veux pas le cls").
	for _, s := range []string{"0,31", "Cumulative Layout Shift", "Total Blocking Time", "640\u00a0ms"} {
		if strings.Contains(out, s) {
			t.Errorf("CLS shown: %q", s)
		}
	}
	for _, s := range []string{"concurrence", "même recherche visée", "4 clics"} {
		if strings.Contains(out, s) {
			t.Errorf("pages are never presented as competing: %q", s)
		}
	}
	if strings.Contains(out, "https://x") {
		t.Error("Markdown links of Lighthouse titles must be reduced to their text")
	}
}

// A report saved before 1.7.0 has none of the new data: no new section shows.
func TestOldReportShowsNoLotASection(t *testing.T) {
	res := lotAResult()
	res.Speed, res.Structured, res.Linking, res.Duplicates, res.Measured = nil, nil, nil, nil, false
	res.AI.QueriesChecked, res.AI.DroppedQueries = false, nil
	res.AI.TargetQueries = []audit.TargetQuery{{Page: "https://ex.com/", Query: "q", Alignment: "good"}}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(&audit.Audit{URL: "https://ex.com/", Result: res}, "fr", true)})
	for _, s := range []string{"Ce que Google peut afficher", "La vitesse de votre site", "Les liens entre vos pages", "Les pages en double", "Proposée par Google", "Recherches écartées"} {
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

// The desktop measurement gets the same cards and tiles as mobile, not a
// one-line sentence (Francis, 2026-11-27, sel-tech.fr report).
func TestDesktopSpeedShowsItsOwnTiles(t *testing.T) {
	res := lotAResult()
	dScore, dLCP, dCLS, dTBT := 100, 768.0, 0.004, 0.0
	res.Speed.Desktop = &audit.SpeedRun{Lab: &audit.LabData{Score: &dScore, LCPms: &dLCP, CLS: &dCLS, TBTms: &dTBT}}
	a := &audit.Audit{URL: "https://ex.com/", Status: audit.StatusDone, Result: res}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	for _, s := range []string{"Vitesse mobile", "Vitesse bureau", "100<small>/100</small>", "0,8 s"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	for _, s := range []string{"Cumulative Layout Shift", "Total Blocking Time"} {
		if strings.Contains(out, s) {
			t.Errorf("tile shown: %q", s)
		}
	}
}

// The speed card names the LCP element and what to change on it, and lists the
// files behind each advice (sel-tech.fr, 2026-11-27: the Crillon card image).
func TestSpeedCardShowsTheSolutions(t *testing.T) {
	res := lotAResult()
	res.Speed.Mobile.Lab.LCP = &audit.LCPElement{Label: "Piscine du Crillon", Source: "https://sel-tech.fr/wp-content/uploads/2026/11/crillon-carte.webp",
		Phases: []audit.LCPPhase{{ID: "elementRenderDelay", Ms: 1076}}, NoFetchPriority: true}
	res.Speed.Mobile.Lab.Opportunities[0].Files = []string{"https://sel-tech.fr/wp-content/uploads/2026/11/molitor-cadre-1215x476.webp"}
	a := &audit.Audit{URL: "https://ex.com/", Status: audit.StatusDone, Result: res}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	for _, s := range []string{"Piscine du Crillon", "crillon-carte.webp", "fetchpriority=", "molitor-cadre-1215x476.webp"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	// The time split is measured on an unthrottled visit and contradicts the
	// simulated LCP tile (0,4 s of parts for 4,1 s): not shown.
	if strings.Contains(out, "Attente avant l&#39;affichage") {
		t.Error("LCP time split shown")
	}
}

// The site's rank stays in the backlinks section; only the jargon around it
// went (Francis, 2026-11-27: "j'ai demandé à retirer le charabia, pas le
// classement du site").
func TestBacklinksShowTheSiteRank(t *testing.T) {
	res := lotAResult()
	res.Popularity = &audit.PopularityReport{Domains: 134979706, Site: audit.DomainPop{Domain: "ex.com", Found: true, Referring: 6, Rank: 45611747}}
	a := &audit.Audit{URL: "https://ex.com/", Status: audit.StatusDone, Result: res}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	if !strings.Contains(out, "Votre site occupe le rang") {
		t.Error("site rank missing")
	}
	if strings.Contains(out, "centralité") {
		t.Error("jargon shown")
	}
}

// Review of 2026-11-27 (sel-tech.fr, audit 34): no priority verdict repeated in
// its criterion card, no level label when all priorities share one, no
// announced 0-100 link score that is not shown, readable addresses.
func TestReportHasNoRepeatsNorEmptyLabels(t *testing.T) {
	res := lotAResult()
	verdict := "La page piscine part des soucis du client."
	res.AI = &audit.AIReport{Criteria: []audit.Criterion{{ID: "customer", Group: "copywriting", Question: "customer", Rating: "partial", Verdict: verdict, Detail: "Analyse."}}}
	res.Priorities = []audit.Priority{{ID: "e:customer", Level: 3, Source: "editorial", Title: verdict}}
	res.Issues = append(res.Issues, audit.IssueSummary{Type: "ERROR_EMPTY_DESCRIPTION", Priority: 2, Pages: 1, Examples: []string{"https://ex.com/parking-5000m%c2%b2/"}})
	a := &audit.Audit{URL: "https://ex.com/", Status: audit.StatusDone, Result: res}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	if n := strings.Count(out, verdict); n != 1 {
		t.Errorf("verdict shown %d times, want once", n)
	}
	if strings.Contains(out, "Le texte parle-t-il du client") || !strings.Contains(out, "Analyse.") {
		t.Error("a criterion shown as a priority keeps its own card, or its analysis is lost")
	}
	if strings.Contains(out, "Pour finir") {
		t.Error("level label shown though every priority has the same level")
	}
	if strings.Contains(out, "de 0 à 100") {
		t.Error("a link score is announced but not shown")
	}
	if !strings.Contains(out, "parking-5000m²") || strings.Contains(out, "%c2%b2") {
		t.Error("example address not decoded")
	}
}
