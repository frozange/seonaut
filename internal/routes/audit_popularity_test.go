package routes

import (
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
)

func popResult() *audit.PopularityReport {
	return &audit.PopularityReport{
		Release: "cc-main-2026-aug-sep-oct", ReleaseEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Previous: "cc-main-2026-may-jun-jul", PreviousEnd: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Domains: 134979706,
		Site:    audit.DomainPop{Domain: "ex.com", Found: true, Referring: 1234, Rank: 2345678, PrevFound: true, PrevReferring: 1200},
		Top:     []audit.RefDomain{{Domain: "wikipedia.org", Rank: 12}, {Domain: "petit-blog.fr"}},
		Compared: true, GainedCount: 40, LostCount: 6,
		Gained: []audit.RefDomain{{Domain: "nouveau.fr", Rank: 99000}},
		Lost:   []audit.RefDomain{{Domain: "parti.fr"}},
		Competitors: []audit.DomainPop{
			{Domain: "rival.fr", Found: true, Referring: 1, Rank: 3000000},
			{Domain: "absent.fr"},
		},
		GapMin: 1, GapCount: 3,
		Gap: []audit.GapDomain{{Domain: "annuaire.fr", Rank: 5000, Cites: []string{"rival.fr"}}},
	}
}

func renderPop(t *testing.T, p *audit.PopularityReport) string {
	t.Helper()
	res := lotAResult()
	res.Popularity = p
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(&audit.Audit{URL: "https://ex.com/", Result: res}, "fr", true)})
	if i := strings.Index(out, "AUDIT_"); i >= 0 {
		t.Fatalf("untranslated key: %s", out[i:i+60])
	}
	return out
}

func TestPopularityRendersInFrench(t *testing.T) {
	out := renderPop(t, popResult())
	for _, s := range []string{
		"Popularité : les sites qui vous citent", "août à octobre 2026", "<b>1 234</b> sites vous citent",
		"rang 2 345 678 sur 134 979 706", "des 2 % de sites les mieux reliés de cet échantillon", "repère de LaFactory", "ne publie aucune note de liens",
		"wikipedia.org</span><span class=\"lfr-small lfr-num\">12</span>", "<span>Site</span><span>Rang</span>", "petit-blog.fr", "Nouveaux depuis le trimestre précédent : 40", "nouveau.fr", "Liens non retrouvés ce trimestre : 6", "parti.fr",
		"plus bas que dans les outils payants",
		"Vous face aux concurrents", "<b>ex.com</b> (vous)", "1 site le cite", "absent.fr", "Pas encore dans l&#39;échantillon",
		"Sites qui citent vos concurrents et pas vous : 3", "annuaire.fr", "cite rival.fr", "aucun lien de « toxique »",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	// Ranked lists name the column once, not the word on every line.
	i := strings.Index(out, "Les sites les mieux reliés")
	lists := out[i : i+strings.Index(out[i:], "Vous face aux concurrents")]
	for _, s := range []string{"rang 12", "rang 99", "rang 5"} {
		if strings.Contains(lists, s) || strings.Contains(out, "annuaire.fr<br>") && strings.Contains(out, ">rang 5") {
			t.Errorf("rank word repeated in a list: %q", s)
		}
	}
	// The section sits between speed and internal links.
	if a, b, c := strings.Index(out, "La vitesse vécue"), strings.Index(out, "Popularité"), strings.Index(out, "Les liens entre vos pages"); !(a < b && b < c) {
		t.Errorf("popularity out of place: %d %d %d", a, b, c)
	}
	for _, s := range []string{"Domain Authority", "toxiques", "Spam", "autorité de domaine"} {
		if strings.Contains(out, s) {
			t.Errorf("invented metric shown: %q", s)
		}
	}
}

func TestPopularityAbsentSiteShowsNoZero(t *testing.T) {
	p := &audit.PopularityReport{Release: "cc-main-2026-aug-sep-oct", ReleaseEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Domains: 134979706, Site: audit.DomainPop{Domain: "nouveau-site.fr"}}
	out := renderPop(t, p)
	i := strings.Index(out, "Popularité")
	sec := out[i : i+strings.Index(out[i:], "</section>")]
	if !strings.Contains(sec, "Pas encore dans l&#39;échantillon") || !strings.Contains(sec, "nouveau-site.fr n&#39;apparaît pas encore") {
		t.Fatalf("absent site: %s", sec)
	}
	for _, s := range []string{">0<", " 0 site", "rang", "Vous face aux concurrents", "Nouveaux depuis"} {
		if strings.Contains(sec, s) || (s == "Vous face aux concurrents" && strings.Contains(out, s)) {
			t.Errorf("absent site shows %q", s)
		}
	}
}

func TestPopularityVariants(t *testing.T) {
	// No previous quarter yet, a site nobody cites, no competitor.
	p := &audit.PopularityReport{Release: "cc-main-2025-26-nov-dec-jan", ReleaseEnd: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Domains: 100, Site: audit.DomainPop{Domain: "ex.com", Found: true}}
	out := renderPop(t, p)
	for _, s := range []string{"novembre 2025 à janvier 2026", "aucun des sites explorés ce trimestre ne lui fait encore de lien", "apparaîtra avec la prochaine publication"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	if strings.Contains(out, "Vous face aux concurrents") || strings.Contains(out, "sites vous citent") {
		t.Error("competitors or a zero count shown")
	}
	// New in the sample this quarter.
	p.Previous, p.PreviousEnd, p.Site.Referring, p.Site.Rank = "cc-main-2025-aug-sep-oct", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), 1, 90
	out = renderPop(t, p)
	if !strings.Contains(out, "<b>1</b> site vous cite") || !strings.Contains(out, "n&#39;était pas dans l&#39;échantillon du trimestre précédent") {
		t.Error("new in the sample")
	}
	if strings.Contains(out, "les mieux reliés de cet échantillon") {
		t.Error("a rank in the second half of the sample shows a share")
	}
	// No report part when there is no index.
	if out := renderPop(t, nil); strings.Contains(out, "Popularité") {
		t.Error("section without data")
	}
}

func TestCountAndShare(t *testing.T) {
	for n, want := range map[uint64]string{0: "0", 12: "12", 1234: "1 234", 134979706: "134 979 706"} {
		if got := count(n, "fr"); got != want {
			t.Errorf("count(%d) = %q", n, got)
		}
	}
	if count(1234567, "en") != "1,234,567" {
		t.Error(count(1234567, "en"))
	}
	for _, c := range []struct {
		rank, total uint32
		want        string
	}{{1, 1000, "0,1"}, {3, 1000, "0,3"}, {15, 1000, "2"}, {499, 1000, "50"}, {501, 1000, ""}, {0, 1000, ""}} {
		if got := topPct(c.rank, c.total, "fr"); got != c.want {
			t.Errorf("topPct(%d, %d) = %q, want %q", c.rank, c.total, got, c.want)
		}
	}
}
