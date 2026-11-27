package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/security"
	"github.com/stjudewashere/seonaut/internal/services"
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

// With one audit a day (Francis, 2026-11-27), a mistyped address must not
// spend the member's IP allowance: the address is checked first.
func TestRelayTypoKeepsTheIPAllowance(t *testing.T) {
	secret := strings.Repeat("s", minRelaySecret)
	cfg := &config.Config{Audit: &config.AuditConfig{RelaySecret: secret}}
	h := &auditHandler{&userHandler{&services.Container{Config: cfg, AuditIPLimiter: security.NewRateLimiter(1, time.Hour)}}}
	for i := 0; i < 2; i++ {
		body := `{"url":"ftp://example.com","email":"a@example.com","ip":"203.0.113.9"}`
		r := httptest.NewRequest(http.MethodPost, "/audit/relay", strings.NewReader(body))
		r.Header.Set(RelayAuthHeader, secret)
		w := httptest.NewRecorder()
		h.relayHandler(w, r)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d: status %d, want 422 (bad address), body %s", i+1, w.Code, w.Body.String())
		}
	}
	if !h.AuditIPLimiter.Allow("203.0.113.9") {
		t.Error("the IP allowance was spent by mistyped addresses")
	}
}
