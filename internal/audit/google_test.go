package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSplitByGoogle(t *testing.T) {
	in := []IssueSummary{
		{Type: "ERROR_40x", Priority: 1, Pages: 2},
		{Type: "ERROR_NOT_VALID_HEADINGS", Priority: 2, Pages: 3},
		{Type: "ERROR_DUPLICATED_TITLE", Priority: 2, Pages: 4},
		{Type: "ERROR_EXTERNAL_WITHOUT_NOFOLLOW", Priority: 3, Pages: 9},
	}
	search, hygiene := SplitByGoogle(in)
	if len(search) != 2 || search[0].Type != "ERROR_40x" || search[1].Type != "ERROR_DUPLICATED_TITLE" {
		t.Errorf("search = %+v", search)
	}
	if len(hygiene) != 2 || hygiene[0].Type != "ERROR_NOT_VALID_HEADINGS" || hygiene[1].Type != "ERROR_EXTERNAL_WITHOUT_NOFOLLOW" {
		t.Errorf("hygiene = %+v", hygiene)
	}
	if got := TechnicalScore(search, 10); got != 8.9 { // 10 - 1.2*0.6 - 0.6*0.7
		t.Errorf("hygiene issues must not weigh on the score")
	}
}

func TestHygieneTypesExist(t *testing.T) {
	// Every listed type must be a real issue type, or the split silently misses it.
	known := map[string]bool{}
	for _, k := range knownIssueTypes(t) {
		known[k] = true
	}
	for k := range outsideGoogleRanking {
		if !known[k] {
			t.Errorf("%s is not an issue type", k)
		}
	}
}

func knownIssueTypes(t *testing.T) []string {
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations not found: %v", err)
	}
	re := regexp.MustCompile(`\(\s*\d+\s*,\s*['"]([A-Z_0-9]+)['"]\s*,\s*\d\s*\)`)
	var out []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			out = append(out, m[1])
		}
	}
	return out
}
