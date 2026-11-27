package routes

import (
	"strings"
	"testing"
)

func TestRelayAllowed(t *testing.T) {
	secret := strings.Repeat("s", minRelaySecret)
	cases := []struct {
		name, secret, got string
		want              bool
	}{
		{"right secret", secret, secret, true},
		{"wrong secret", secret, strings.Repeat("x", minRelaySecret), false},
		{"no header", secret, "", false},
		{"short secret arms nothing", "short", "short", false},
		{"not configured", "", "", false},
	}
	for _, c := range cases {
		if got := relayAllowed(c.secret, c.got); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
