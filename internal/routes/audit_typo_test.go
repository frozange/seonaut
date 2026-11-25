package routes

import "testing"

func TestFrenchSpacing(t *testing.T) {
	in := `<style>.a :where(b) { color: red; }</style><p title="a : b">Titre : la page ; 3 % ? « oui » !</p><script>if (a ? b : c) {}</script>`
	want := "<style>.a :where(b) { color: red; }</style><p title=\"a : b\">Titre : la page ; 3 % ? « oui » !</p><script>if (a ? b : c) {}</script>"
	if got := string(frenchSpacing([]byte(in))); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
