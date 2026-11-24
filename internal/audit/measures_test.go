package audit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/models"
)

func page(u string, links ...string) *PageSignal {
	return &PageSignal{URL: u, Status: 200, Links: links}
}

func TestLinkScoresHomeFirstAndRedirectsFollowed(t *testing.T) {
	h := "https://ex.com/"
	pages := []*PageSignal{
		page(h, h+"a", h+"b", h+"old"),
		page(h+"a", h, h+"b"),
		page(h+"b", h),
		{URL: h + "old", Status: 301, Redirect: h + "c"},
		page(h+"c", h),
		page(h + "lonely"),
	}
	g := BuildLinkGraph(pages)
	if len(g.URLs) != 5 {
		t.Fatalf("redirects are not nodes: %v", g.URLs)
	}
	scores := LinkScores(g)
	at := func(u string) int { return scores[g.Index[u]] }
	if at(h) != 100 || at(h+"lonely") != 0 {
		t.Errorf("home %d, page nobody links %d", at(h), at(h+"lonely"))
	}
	if len(g.Incoming[g.Index[h+"c"]]) != 1 {
		t.Error("the link to the redirect must count for its target")
	}
	if !(at(h+"b") > at(h+"c")) {
		t.Errorf("b has two inlinks, c one: %d vs %d", at(h+"b"), at(h+"c"))
	}
}

func TestLinkScoresCycleScoresEqually(t *testing.T) {
	h := "https://ex.com/"
	g := BuildLinkGraph([]*PageSignal{page(h, h+"a"), page(h+"a", h+"b"), page(h+"b", h)})
	for i, s := range LinkScores(g) {
		if s != 100 {
			t.Errorf("node %d: %d, a cycle has no spread", i, s)
		}
	}
}

func TestBuildLinkingWeakAndDeep(t *testing.T) {
	h := "https://ex.com/"
	deep := page(h+"d", h)
	deep.Depth = 5
	prod := page(h+"p", h)
	prod.Structured = []StructuredItem{{Type: "Product"}}
	key := page(h+"k", h)
	noidx := page(h+"n", h)
	noidx.Noindex = true
	r := BuildLinking([]*PageSignal{page(h, h+"k", h+"p", h+"d", h+"n", h+"x"), key, prod, deep, noidx, page(h+"x", h, h+"k")}, h, map[string]bool{h + "k": true})
	if r.Pages != 5 {
		t.Errorf("noindex page scored: %d", r.Pages)
	}
	if r.WeakCount != 3 || r.Weak[0].URL != h+"p" || !r.Weak[0].Product {
		t.Errorf("weak pages, product first (key page k has two links): %+v", r.Weak)
	}
	if r.Deep != 1 || r.DeepExamples[0].URL != h+"d" {
		t.Errorf("deep %+v", r.DeepExamples)
	}
	if r.Top[0].URL != h {
		t.Errorf("top %+v", r.Top)
	}
}

func words(n int, seed string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s%d ", seed, i)
	}
	return b.String()
}

func textPage(u, lang, text string) *PageSignal {
	return &PageSignal{URL: u, Status: 200, Lang: lang, Words: len(strings.Fields(text)), Shingles: Shingles(text)}
}

func TestSimilarityEstimates(t *testing.T) {
	a := Shingles(words(400, "w"))
	if s := Similarity(a, a); s != 1 {
		t.Errorf("same text: %v", s)
	}
	if s := Similarity(a, Shingles(words(400, "z"))); s != 0 {
		t.Errorf("different texts: %v", s)
	}
	half := Similarity(Shingles(words(400, "w")), Shingles(words(200, "w")+words(200, "z")))
	if half < 0.2 || half > 0.5 {
		t.Errorf("half shared text should read about a third: %v", half)
	}
}

func TestNearDuplicatesSkipTranslationsAndCanonicals(t *testing.T) {
	body := words(300, "piscine")
	pages := []*PageSignal{
		textPage("https://ex.com/a", "fr", body),
		textPage("https://ex.com/b", "fr", body+" fin"),
		textPage("https://ex.com/en/a", "en", body),
		textPage("https://ex.com/c", "fr", words(300, "autre")),
		textPage("https://ex.com/short", "fr", "trop court pour compter ici vraiment"),
	}
	canon := textPage("https://ex.com/a?ref=1", "fr", body)
	canon.Canonical = "https://ex.com/a"
	pages = append(pages, canon)
	g := NearDuplicates(pages)
	if len(g) != 1 || len(g[0].Pages) != 2 || g[0].Pages[0] != "https://ex.com/a" || g[0].Pages[1] != "https://ex.com/b" {
		t.Fatalf("groups %+v", g)
	}
	if g[0].Similarity < 90 {
		t.Errorf("similarity %d", g[0].Similarity)
	}
}

func TestMatchSuggestions(t *testing.T) {
	sugg := []string{"électrolyseur piscine sel", "électrolyseur piscine prix", "pompe piscine"}
	if r := matchSuggestions("Electrolyseurs piscine", sugg); r.Status != QueryConfirmed || len(r.Related) != 2 {
		t.Errorf("confirmed %+v", r)
	}
	if r := matchSuggestions("électrolyseur piscine bretagne", sugg); r.Status != QueryVariant || r.Variant != "électrolyseur piscine sel" {
		t.Errorf("variant %+v", r)
	}
	if r := matchSuggestions("installateur photovoltaïque lyon", sugg); r.Status != QueryUnknown {
		t.Errorf("unknown %+v", r)
	}
}

func TestCheckQueriesAgainstSuggest(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("hl") != "fr" || r.URL.Query().Get("client") != "firefox" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		q := r.URL.Query().Get("q")
		switch q {
		case "traitement eau piscine":
			fmt.Fprint(w, `["traitement eau piscine",["traitement eau piscine sel","traitement eau piscine"]]`)
		case "piscine au sel entretien hiver", "piscine au sel entretien":
			fmt.Fprint(w, `["x",["piscine au sel entretien hiver facile"]]`)
		case "étanchéité parking":
			fmt.Fprint(w, `["x",["étanchéité parking souterrain","étanchéité parking prix","étanchéité toiture"]]`)
		default:
			fmt.Fprint(w, `["x",[]]`)
		}
	}))
	defer srv.Close()
	old := SuggestEndpoint
	SuggestEndpoint = srv.URL
	defer func() { SuggestEndpoint = old }()

	s := &Suggester{Lang: "fr", Country: "fr"}
	kept, dropped := s.CheckQueries(context.Background(), []TargetQuery{
		{Page: "https://ex.com/", Query: "traitement eau piscine"},
		{Page: "https://ex.com/a", Query: "piscine au sel entretien hiver"},
		{Page: "https://ex.com/b", Query: "zzz inconnu du tout"},
		{Page: "https://ex.com/c", Query: "étanchéité parking sous-sol cuvelage"},
	})
	if len(kept) != 2 || kept[0].Google != QueryConfirmed || kept[1].Google != QueryConfirmed {
		t.Errorf("kept %+v", kept)
	}
	if len(dropped) != 2 || dropped[0].Google != QueryUnknown || len(dropped[0].GoogleRelated) != 0 {
		t.Errorf("dropped %+v", dropped)
	}
	if alt := dropped[1].GoogleRelated; len(alt) != 2 || alt[0] != "étanchéité parking souterrain" {
		t.Errorf("what Google proposes instead: %v", alt)
	}

	SuggestEndpoint = "http://127.0.0.1:1"
	kept, dropped = (&Suggester{Lang: "fr"}).CheckQueries(context.Background(), []TargetQuery{{Query: "q"}})
	if len(kept) != 1 || kept[0].Google != QueryUnchecked || len(dropped) != 0 {
		t.Errorf("Suggest down: keep and say unchecked: %+v %+v", kept, dropped)
	}
}

func TestSuggestLocale(t *testing.T) {
	for _, c := range []struct{ lang, host, l, c string }{
		{"fr-FR", "sel-tech.fr", "fr", "fr"}, {"fr", "ex.be", "fr", "be"}, {"", "ex.com", "fr", "fr"}, {"en-GB", "ex.com", "en", "gb"},
	} {
		if l, co := SuggestLocale(c.lang, c.host, "fr"); l != c.l || co != c.c {
			t.Errorf("%s %s: %s %s", c.lang, c.host, l, co)
		}
	}
}

const psiField = `{"loadingExperience":{"id":"https://ex.com/","metrics":{
 "LARGEST_CONTENTFUL_PAINT_MS":{"percentile":1823},"INTERACTION_TO_NEXT_PAINT":{"percentile":134},
 "CUMULATIVE_LAYOUT_SHIFT_SCORE":{"percentile":31}}},
 "originLoadingExperience":{"metrics":{"LARGEST_CONTENTFUL_PAINT_MS":{"percentile":3000}}},
 "lighthouseResult":{"categories":{"performance":{"score":0.46}},"audits":{
  "largest-contentful-paint":{"numericValue":5120.5},"cumulative-layout-shift":{"numericValue":0.02},
  "total-blocking-time":{"numericValue":640},"server-response-time":{"numericValue":180},
  "unused-javascript":{"title":"Réduisez le JavaScript inutilisé","score":0.2,"details":{"overallSavingsMs":900}},
  "render-blocking-insight":{"title":"Requêtes de blocage du rendu","score":0,"metricSavings":{"LCP":300}},
  "small-win":{"title":"x","score":0.5,"details":{"overallSavingsMs":100}}}}}`

func TestParsePSIFieldAndLab(t *testing.T) {
	run, err := ParsePSI([]byte(psiField))
	if err != nil {
		t.Fatal(err)
	}
	if run.FieldScope != "page" || run.Field.LCP.Rating != "good" || run.Field.INP.Rating != "good" {
		t.Errorf("field %+v", run.Field)
	}
	if run.Field.CLS.P75 != 0.31 || run.Field.CLS.Rating != "poor" || run.Field.Overall() != "poor" {
		t.Errorf("CLS comes times 100: %+v", run.Field.CLS)
	}
	if *run.Lab.Score != 46 || len(run.Lab.Opportunities) != 2 || run.Lab.Opportunities[0].SavingMs != 900 {
		t.Errorf("lab %+v", run.Lab)
	}
}

func TestParsePSIOriginFallbackAndNoField(t *testing.T) {
	run, err := ParsePSI([]byte(`{"loadingExperience":{"origin_fallback":true,"metrics":{"LARGEST_CONTENTFUL_PAINT_MS":{"percentile":3000}}},
	 "originLoadingExperience":{"metrics":{"LARGEST_CONTENTFUL_PAINT_MS":{"percentile":4100}}},"lighthouseResult":{"categories":{"performance":{"score":0.9}},"audits":{}}}`))
	if err != nil || run.FieldScope != "site" || run.Field.LCP.P75 != 4100 || run.Field.LCP.Rating != "poor" {
		t.Errorf("origin fallback %+v %v", run, err)
	}
	run, err = ParsePSI([]byte(`{"loadingExperience":{"initial_url":"https://sel-tech.fr/"},"lighthouseResult":{"categories":{"performance":{"score":0.71}},"audits":{"largest-contentful-paint":{"numericValue":3900}}}}`))
	if err != nil || run.Field != nil || *run.Lab.Score != 71 {
		t.Errorf("no field data: %+v %v", run, err)
	}
	if _, err := ParsePSI([]byte(`{"kind":"x"}`)); err == nil {
		t.Error("an answer without any measurement is an error, not zeros")
	}
}

func TestSpeedRunFailureRecordsNoFigure(t *testing.T) {
	var gotKey, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotQuery = r.Header.Get("X-Goog-Api-Key"), r.URL.RawQuery
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"Lighthouse returned error: NO_FCP"}}`)
	}))
	defer srv.Close()
	old := PSIEndpoint
	PSIEndpoint = srv.URL
	defer func() { PSIEndpoint = old }()

	st := &SpeedTester{Key: "SECRET-KEY", Retries: 2, Backoff: time.Millisecond}
	run := st.Run(context.Background(), "https://ex.com/", "mobile")
	if run.Error == "" || run.Field != nil || run.Lab != nil {
		t.Errorf("failed call: %+v", run)
	}
	if !strings.Contains(run.Error, "NO_FCP") || strings.Contains(run.Error, "SECRET-KEY") {
		t.Errorf("error %q", run.Error)
	}
	if gotKey != "SECRET-KEY" || strings.Contains(gotQuery, "SECRET") {
		t.Error("the key travels in the header, never in the address")
	}
}

func TestReadEnvKey(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("A=1\nexport PSI_API_KEY=\"abc\"\nB=\n"), 0o600)
	if k, err := ReadEnvKey(p, "PSI_API_KEY"); err != nil || k != "abc" {
		t.Errorf("%q %v", k, err)
	}
	if _, err := ReadEnvKey(p, "B"); err == nil {
		t.Error("empty value is an error")
	}
	if _, err := ReadEnvKey(p, "C"); err == nil || strings.Contains(err.Error(), "abc") {
		t.Errorf("missing name: %v", err)
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCollectorWatchesOnlyAuditProjects(t *testing.T) {
	c := NewCollector(2)
	c.Watch(7)
	doc := parseDoc(t, `<html lang="fr"><body><header>Menu</header><main><p>`+words(80, "mot")+`</p></main></body></html>`)
	mk := func(u string) *models.PageReport {
		r := &models.PageReport{URL: u, StatusCode: 200, MediaType: "text/html", Title: "T"}
		r.ParsedURL = mustParse(t, u)
		r.Links = []models.Link{{URL: u + "#top", ParsedURL: mustParse(t, u+"#top")}, {URL: "https://ex.com/b", ParsedURL: mustParse(t, "https://ex.com/b")}, {URL: "https://ex.com/n", ParsedURL: mustParse(t, "https://ex.com/n"), NoFollow: true}}
		return r
	}
	c.ObservePage(8, mk("https://ex.com/x"), doc)
	c.ObservePage(7, mk("https://ex.com/a"), doc)
	c.ObservePage(7, &models.PageReport{URL: "https://ex.com/old", StatusCode: 301, RedirectURL: "/a", ParsedURL: mustParse(t, "https://ex.com/old")}, nil)
	c.ObservePage(7, mk("https://ex.com/c"), doc)
	got := c.Take(7)
	if len(got) != 2 {
		t.Fatalf("cap of 2 pages: %d", len(got))
	}
	a := got[0]
	if a.Words != 80 || len(a.Shingles) == 0 || len(a.Links) != 1 || a.Links[0] != "https://ex.com/b" {
		t.Errorf("page signal %+v", a)
	}
	if got[1].Redirect != "https://ex.com/a" {
		t.Errorf("redirect resolved: %+v", got[1])
	}
	if c.Take(8) != nil || c.Take(7) != nil {
		t.Error("unwatched or taken projects hold nothing")
	}
}

func TestMeasuredPriorities(t *testing.T) {
	res := &Result{
		Speed:      &SpeedReport{Mobile: &SpeedRun{Field: &FieldData{LCP: &FieldMetric{P75: 5000, Rating: "poor"}}}},
		Structured: &StructuredReport{Families: []SDFamily{{Key: "product", Pages: 3, ErrorPages: 2}}},
		Duplicates: []DuplicateGroup{{Pages: []string{"a", "b"}}},
		Linking:    &LinkingReport{Pages: 10, WeakCount: 4, Orphans: 1},
	}
	ev, ids := MeasureEvidence(res, []EvidenceEntry{{ID: "E1"}})
	if len(ev) != 5 || ids["speed"] != "E2" || ids["duplicates"] == "" {
		t.Errorf("evidence %v %v", len(ev), ids)
	}
	p := MeasuredPriorities(res, ids)
	got := map[string]int{}
	for _, x := range p {
		got[x.ID] = x.Level
	}
	want := map[string]int{"m:speed": 2, "m:structured": 2, "m:duplicates": 3, "m:linking": 3}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: level %d, want %d", k, got[k], v)
		}
	}
	res.Speed.Mobile.Field = nil
	res.Speed.Mobile.Lab = &LabData{}
	for _, x := range MeasuredPriorities(res, ids) {
		if x.ID == "m:speed" {
			t.Error("lab data alone never makes a speed priority")
		}
	}
}

// One crawlable link is enough for Google to find a page: pages linked once
// are information, only pages nobody links to make a priority.
func TestLinkingPriorityOnlyForOrphans(t *testing.T) {
	res := &Result{Linking: &LinkingReport{Pages: 38, WeakCount: 4, Weak: []LinkedPage{{Inlinks: 1}, {Inlinks: 1}}},
		Priorities: []Priority{{ID: "t:X"}, {ID: "m:linking"}, {ID: "e:intent"}}}
	for _, p := range MeasuredPriorities(res, nil) {
		if p.ID == "m:linking" {
			t.Error("pages linked once made a priority")
		}
	}
	RefreshMeasuredPriorities(res)
	if len(res.Priorities) != 2 || res.Priorities[1].ID != "e:intent" {
		t.Errorf("saved report keeps a stale priority: %+v", res.Priorities)
	}
	res.Linking.Weak = append(res.Linking.Weak, LinkedPage{Inlinks: 0})
	res.Linking.Orphans = 0
	res.Priorities = []Priority{{ID: "m:linking"}}
	RefreshMeasuredPriorities(res)
	if res.Linking.Orphans != 1 || len(res.Priorities) != 1 {
		t.Errorf("orphan read from the examples: %+v %+v", res.Linking, res.Priorities)
	}
}
