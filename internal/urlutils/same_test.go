package urlutils

import "testing"

func TestSameURL(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://sel-tech.fr/etancheite-dun-parking-de-5000m%c2%b2/", "https://sel-tech.fr/etancheite-dun-parking-de-5000m²/", true},
		{"https://sel-tech.fr/x-5000m%c2%b2/", "https://sel-tech.fr/x-5000m%C2%B2/", true},
		{"https://Sel-Tech.fr/a/", "https://sel-tech.fr/a/", true},
		{"https://sel-tech.fr/a/#top", "https://sel-tech.fr/a/", true},
		{"https://sel-tech.fr", "https://sel-tech.fr/", true},
		{"https://sel-tech.fr/a/", "https://sel-tech.fr/b/", false},
		{"https://sel-tech.fr/a/", "https://sel-tech.fr/a", false},
		{"https://sel-tech.fr/A/", "https://sel-tech.fr/a/", false},
		{"http://sel-tech.fr/a/", "https://sel-tech.fr/a/", false},
		{"https://sel-tech.fr/a/?p=1", "https://sel-tech.fr/a/?p=2", false},
		{"https://sel-tech.fr/a%2Fb/", "https://sel-tech.fr/a/b/", false},
		{"https://sel-tech.fr/a%2fb/", "https://sel-tech.fr/a%2Fb/", true},
		{"https://sel-tech.fr/%7Euser/", "https://sel-tech.fr/~user/", true},
	}
	for _, c := range cases {
		if got := SameURL(c.a, c.b); got != c.want {
			t.Errorf("SameURL(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
