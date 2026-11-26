package audit

import (
	"strings"
	"testing"
)

func TestStyleRulesFormat(t *testing.T) {
	in := &Interpreter{Language: "French", Banned: []string{"x"}}
	got := in.style()
	if strings.Contains(got, "%!") {
		t.Errorf("bad format verbs: %q", got[strings.Index(got, "%!"):][:40])
	}
	if !strings.Contains(got, "One finding, one place") || !strings.Contains(got, "natural French") {
		t.Error("no-repetition rules missing")
	}
}
