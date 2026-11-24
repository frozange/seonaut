package audit

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stjudewashere/seonaut/internal/webgraph"
)

// Domains of the test graphs, sorted reversed as Common Crawl publishes them.
var popVertices = "0\tcom.big\t1\n1\tcom.blog\t1\n2\tcom.dir\t1\n3\tfr.other\t1\n4\tfr.rival\t1\n5\tfr.sel-tech\t1\n6\tfr.shop\t1\n7\torg.wiki\t1\n"

var popRanks = "#harmonicc_pos\t#harmonicc_val\t#pr_pos\t#pr_val\t#host_rev\t#n_hosts\n" +
	"1\t9\t1\t0.1\torg.wiki\t1\n2\t8\t2\t0.1\tcom.big\t1\n3\t7\t3\t0.1\tfr.sel-tech\t1\n4\t6\t4\t0.1\tfr.rival\t1\n5\t5\t5\t0.1\tcom.blog\t1\n6\t4\t6\t0.1\tcom.dir\t1\n"

func buildIndex(t *testing.T, release, edges string) *webgraph.Index {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "dl")
	os.MkdirAll(src, 0o755)
	for name, text := range map[string]string{"vertices": popVertices, "edges": edges, "ranks": popRanks} {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		w.Write([]byte(text))
		w.Close()
		os.WriteFile(filepath.Join(src, name+".txt.gz"), b.Bytes(), 0o644)
	}
	if _, err := (&webgraph.Builder{}).Build(release, src, filepath.Join(dir, "index")); err != nil {
		t.Fatal(err)
	}
	ix, err := webgraph.Open(filepath.Join(dir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ix.Close)
	return ix
}

func popIndexes(t *testing.T) (cur, prev *webgraph.Index) {
	// This quarter: big, blog and wiki link to sel-tech; dir links to both
	// competitors, wiki too, shop and big to rival only.
	cur = buildIndex(t, "cc-main-2026-aug-sep-oct", "0\t4\n0\t5\n1\t5\n2\t3\n2\t4\n6\t4\n7\t3\n7\t4\n7\t5\n")
	// Last quarter: big, shop and wiki linked to sel-tech.
	prev = buildIndex(t, "cc-main-2026-may-jun-jul", "0\t5\n6\t5\n7\t5\n")
	return cur, prev
}

func refNames(l []RefDomain) string {
	var out []string
	for _, r := range l {
		out = append(out, r.Domain)
	}
	return strings.Join(out, ",")
}

func TestBuildPopularity(t *testing.T) {
	cur, prev := popIndexes(t)
	p := BuildPopularity(cur, prev, "WWW.sel-tech.fr:443", []string{"rival.fr", "https://www.other.fr/page", "sel-tech.fr", "", "absent.fr", "rival.fr"})
	if p.Release != "cc-main-2026-aug-sep-oct" || p.Previous != "cc-main-2026-may-jun-jul" || p.Domains != 8 || p.ReleaseEnd.Month() != 10 {
		t.Fatalf("release = %+v", p)
	}
	if s := p.Site; s.Domain != "sel-tech.fr" || !s.Found || s.Referring != 3 || s.Rank != 3 || !s.PrevFound || s.PrevReferring != 3 {
		t.Fatalf("site = %+v", s)
	}
	if got := refNames(p.Top); got != "wiki.org,big.com,blog.com" {
		t.Fatalf("top = %s", got)
	}
	if !p.Compared || p.GainedCount != 1 || refNames(p.Gained) != "blog.com" || p.LostCount != 1 || refNames(p.Lost) != "shop.fr" {
		t.Fatalf("gained %v lost %v", p.Gained, p.Lost)
	}
	if len(p.Competitors) != 3 || p.Competitors[0].Domain != "rival.fr" || p.Competitors[0].Referring != 4 ||
		p.Competitors[1].Domain != "other.fr" || p.Competitors[1].Referring != 2 || p.Competitors[2].Found {
		t.Fatalf("competitors = %+v", p.Competitors)
	}
	// dir.com cites both competitors and not the site; wiki.org cites the site
	// too; shop.fr and big.com cite one competitor only.
	if p.GapMin != 2 || p.GapCount != 1 || len(p.Gap) != 1 || p.Gap[0].Domain != "dir.com" || p.Gap[0].Rank != 6 || strings.Join(p.Gap[0].Cites, ",") != "rival.fr,other.fr" {
		t.Fatalf("gap = %+v (%d, min %d)", p.Gap, p.GapCount, p.GapMin)
	}
	ev := popularityEvidence(p)
	if !strings.Contains(ev["source"].(string), "sample") || ev["sites_linking_to_competitors_not_to_the_site"] != 1 {
		t.Fatalf("evidence = %v", ev)
	}

	// One competitor: a site citing it is enough.
	one := BuildPopularity(cur, nil, "sel-tech.fr", []string{"rival.fr"})
	if one.GapMin != 1 || one.GapCount != 2 || refNames([]RefDomain{{Domain: one.Gap[0].Domain}, {Domain: one.Gap[1].Domain}}) != "dir.com,shop.fr" {
		t.Fatalf("one competitor: %+v", one.Gap)
	}
	if one.Compared || one.Previous != "" {
		t.Fatal("compared without a previous quarter")
	}
}

func TestPopularityAbsentIsNeverZero(t *testing.T) {
	cur, prev := popIndexes(t)
	p := BuildPopularity(cur, prev, "nouveau-site.fr", []string{"rival.fr"})
	if p.Site.Found || p.Site.Referring != 0 || p.Top != nil || p.Compared || p.GapMin != 0 {
		t.Fatalf("absent site = %+v", p)
	}
	ev := popularityEvidence(p)
	if !strings.Contains(ev["site"].(string), "not in the sample yet") {
		t.Fatalf("evidence = %v", ev)
	}
	if BuildPopularity(nil, nil, "sel-tech.fr", nil) != nil {
		t.Fatal("report without an index")
	}
	// New in the sample: no comparison.
	empty := buildIndex(t, "cc-main-2026-may-jun-jul", "")
	q := BuildPopularity(cur, empty, "sel-tech.fr", nil)
	if !q.Compared || q.GainedCount != 3 || q.LostCount != 0 {
		t.Fatalf("site with no referrer last quarter: %+v", q)
	}
}

func TestRegistrableDomain(t *testing.T) {
	for in, want := range map[string]string{
		"www.sel-tech.fr": "sel-tech.fr", "Shop.Example.co.uk.": "example.co.uk", "sel-tech.fr:8080": "sel-tech.fr",
		"127.0.0.1": "", "": "", "localhost": "localhost",
	} {
		if got := RegistrableDomain(in); got != want {
			t.Errorf("RegistrableDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNeedsPopularity(t *testing.T) {
	cur := &webgraph.Release{Name: "cc-main-2026-aug-sep-oct"}
	prev := &webgraph.Release{Name: "cc-main-2026-may-jun-jul"}
	cases := []struct {
		p         *PopularityReport
		cur, prev *webgraph.Release
		want      bool
	}{
		{nil, nil, nil, false},
		{nil, cur, nil, true},
		{&PopularityReport{Release: cur.Name}, cur, nil, false},
		{&PopularityReport{Release: cur.Name}, cur, prev, true},
		{&PopularityReport{Release: cur.Name, Previous: prev.Name}, cur, prev, false},
		{&PopularityReport{Release: "cc-main-2026-may-jun-jul"}, cur, prev, false}, // an older release stays
	}
	for i, c := range cases {
		if got := NeedsPopularity(c.p, c.cur, c.prev); got != c.want {
			t.Errorf("case %d: %v", i, got)
		}
	}
}

func TestPopularityEvidenceEntry(t *testing.T) {
	cur, prev := popIndexes(t)
	res := &Result{Popularity: BuildPopularity(cur, prev, "sel-tech.fr", nil)}
	ev, ids := MeasureEvidence(res, nil)
	if ids["popularity"] == "" || len(ev) != 1 || !strings.Contains(ev[0].Label, "Google publishes no link score") {
		t.Fatalf("evidence = %+v", ev)
	}
	// Popularity never makes a priority: Google documents no link threshold.
	for _, p := range MeasuredPriorities(res, ids) {
		t.Fatalf("priority from popularity: %+v", p)
	}
}
