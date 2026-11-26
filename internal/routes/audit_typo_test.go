package routes

import "testing"

func TestFrenchSpacing(t *testing.T) {
	in := `<style>.a :where(b) { color: red; }</style><p title="a : b">Titre : la page ; 3 % ? « oui » !</p><script>if (a ? b : c) {}</script>`
	want := "<style>.a :where(b) { color: red; }</style><p title=\"a : b\">Titre : la page ; 3 % ? « oui » !</p><script>if (a ? b : c) {}</script>"
	if got := string(frenchSpacing([]byte(in))); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestGoogleWords(t *testing.T) {
	for in, want := range map[string]string{
		"Le plan du site que vous transmettez à Google (sitemap) contient 15 adresses": "Le sitemap que vous transmettez à Google contient 15 adresses",
		"Le plan du site transmis à Google (le sitemap, la liste des pages) contient":  "Le sitemap transmis à Google contient",
		"Retirez-les du plan du site.":                                 "Retirez-les du sitemap.",
		"Le titre Google porte ces mots.":                              "L&#39;élément &lt;title&gt; porte ces mots.",
		"reprise dans le titre Google, le titre de la page":            "reprise dans l&#39;élément &lt;title&gt;, le titre de la page",
		"Les titres Google sont clairs.":                               "Les éléments &lt;title&gt; sont clairs.",
		"Le mot PME apparaît dans la description affichée par Google.": "Le mot PME apparaît dans la meta description.",
		"sa description Google":                                        "sa meta description",
	} {
		if got := googleWords(in); got != want {
			t.Errorf("%q\n got  %q\n want %q", in, got, want)
		}
	}
}
