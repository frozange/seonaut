package routes

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stjudewashere/seonaut/internal/audit"
)

// The report of sel-tech.fr (audit 34) said the same facts in the summary, the
// strengths, the priorities and the cards: once each after dedupeAI.
func TestDedupeOnTheSelTechReport(t *testing.T) {
	b, err := os.ReadFile("testdata/result_seltech_35.json")
	if err != nil {
		t.Fatal(err)
	}
	var res audit.Result
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	a := &audit.Audit{URL: "https://sel-tech.fr/", Status: audit.StatusDone, Result: &res}
	out := renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})
	if strings.Contains(out, `<p class="lfr-conclusion">`) {
		t.Error("closing recap shown")
	}
	if n := strings.Count(out, "carreaux qui se décollent"); n > 2 {
		t.Errorf("\"carreaux qui se décollent\" said %d times", n)
	}
	if n := strings.Count(out, "26 chantiers") + strings.Count(out, "26 réalisations"); n > 2 {
		t.Errorf("the 26 projects said %d times", n)
	}
	t.Logf("strengths kept: %d of %d", len(newReportView(a, "fr", true).Result.AI.Strengths), len(res.AI.Strengths))
}

func TestDumpSelTechText(t *testing.T) {
	if os.Getenv("DUMP") == "" {
		t.Skip()
	}
	b, _ := os.ReadFile("testdata/result_seltech_35.json")
	var res audit.Result
	_ = json.Unmarshal(b, &res)
	a := &audit.Audit{URL: "https://sel-tech.fr/", Status: audit.StatusDone, Result: &res}
	_ = os.WriteFile(os.Getenv("DUMP"), []byte(renderFR(t, "audit_fragment", &fragmentView{Report: newReportView(a, "fr", true)})), 0o644)
}
