package audit

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestExclusion(t *testing.T) {
	dir := t.TempDir()
	list := "# blog\nhttps://www.ex.com/an-article/\n/other-article\n/tag/*\n\n"
	if err := os.WriteFile(filepath.Join(dir, "ex-com.txt"), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
	ex, err := LoadExclusion(dir, "https://ex.com/")
	if err != nil || ex.Len() != 3 {
		t.Fatalf("load: %v %d", err, ex.Len())
	}
	for raw, want := range map[string]bool{
		"https://ex.com/an-article/":           true,
		"https://ex.com/an-article":            true,
		"https://www.ex.com/an-article/?x=1#c": true,
		"https://ex.com/other-article/":        true,
		"https://ex.com/tag/seo/":              true,
		"https://ex.com/tag":                   true,
		"https://ex.com/":                      false,
		"https://ex.com/an-article/sub/":       false,
		"https://ex.com/services/":             false,
		"https://other.com/an-article/":        false,
		"https://ex.com/fr/an-article/":        false,
	} {
		u, _ := url.Parse(raw)
		if got := ex.Match(u); got != want {
			t.Errorf("Match(%s) = %v, want %v", raw, got, want)
		}
	}
	if ex, err := LoadExclusion(dir, "https://none.com/"); err != nil || ex != nil || ex.Match(&url.URL{Host: "none.com", Path: "/x"}) {
		t.Errorf("a site with no list must exclude nothing: %v %v", ex, err)
	}
	if ex, err := LoadExclusion("", "https://ex.com/"); err != nil || ex != nil {
		t.Error("no directory must exclude nothing")
	}
}
