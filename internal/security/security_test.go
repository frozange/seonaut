package security

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func request(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIPIgnoresHeaderFromUntrustedPeer(t *testing.T) {
	res, _ := NewClientIPResolver([]string{"127.0.0.1"}, "CF-Connecting-IP")
	got := res.IP(request("203.0.113.9:5555", map[string]string{"CF-Connecting-IP": "1.2.3.4"}))
	if got != "203.0.113.9" {
		t.Errorf("spoofed header was honoured: %s", got)
	}
}

func TestClientIPHonoursHeaderFromTrustedProxy(t *testing.T) {
	res, _ := NewClientIPResolver([]string{"127.0.0.1/32"}, "CF-Connecting-IP")
	got := res.IP(request("127.0.0.1:5555", map[string]string{"CF-Connecting-IP": "49.228.1.2"}))
	if got != "49.228.1.2" {
		t.Errorf("got %s", got)
	}
}

func TestClientIPForwardedForTakesRightMostUntrusted(t *testing.T) {
	res, _ := NewClientIPResolver([]string{"127.0.0.1", "10.0.0.0/8"}, "X-Forwarded-For")
	got := res.IP(request("127.0.0.1:80", map[string]string{"X-Forwarded-For": "6.6.6.6, 49.228.1.2, 10.1.1.1"}))
	if got != "49.228.1.2" {
		t.Errorf("got %s, the left-most entry is client controlled", got)
	}
}

func TestClientIPGarbageHeaderFallsBackToSocket(t *testing.T) {
	res, _ := NewClientIPResolver([]string{"127.0.0.1"}, "CF-Connecting-IP")
	got := res.IP(request("127.0.0.1:80", map[string]string{"CF-Connecting-IP": "not-an-ip"}))
	if got != "127.0.0.1" {
		t.Errorf("got %s", got)
	}
}

func TestClientIPInvalidTrustedEntry(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"nope"}, "X"); err == nil {
		t.Error("invalid trusted proxy must fail")
	}
}

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	l := NewRateLimiter(3, time.Hour)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("event %d refused", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("fourth event allowed")
	}
	if !l.Allow("b") {
		t.Fatal("keys must be independent")
	}

	now = now.Add(61 * time.Minute)
	if !l.Allow("a") {
		t.Fatal("window did not slide")
	}
	if l.Remaining("a") != 2 {
		t.Errorf("remaining = %d", l.Remaining("a"))
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	l := NewRateLimiter(0, time.Hour)
	for i := 0; i < 100; i++ {
		if !l.Allow("a") {
			t.Fatal("disabled limiter refused")
		}
	}
	var nilLimiter *RateLimiter
	if !nilLimiter.Allow("a") {
		t.Fatal("nil limiter refused")
	}
}

func turnstileServer(t *testing.T, status int, body string, seen *string) *Turnstile {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if seen != nil {
			*seen = string(b)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	ts := NewTurnstile("site", "secret")
	ts.endpoint = srv.URL
	return ts
}

func TestTurnstileSuccess(t *testing.T) {
	var seen string
	ts := turnstileServer(t, 200, `{"success":true}`, &seen)
	if err := ts.Verify(context.Background(), "tok", "49.228.1.2"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "remoteip=49.228.1.2") || !strings.Contains(seen, "secret=secret") {
		t.Errorf("unexpected request body %q", seen)
	}
}

func TestTurnstileFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		token  string
	}{
		{"rejected", 200, `{"success":false}`, "tok"},
		{"server error", 500, `{"success":true}`, "tok"},
		{"garbage", 200, `not json`, "tok"},
		{"empty token", 200, `{"success":true}`, ""},
	}
	for _, c := range cases {
		ts := turnstileServer(t, c.status, c.body, nil)
		if err := ts.Verify(context.Background(), c.token, ""); err == nil {
			t.Errorf("%s: verification passed", c.name)
		}
	}

	ts := NewTurnstile("site", "secret")
	ts.endpoint = "http://127.0.0.1:1"
	if err := ts.Verify(context.Background(), "tok", ""); err == nil {
		t.Error("unreachable endpoint: verification passed")
	}
}

func TestTurnstileDisabled(t *testing.T) {
	ts := NewTurnstile("", "")
	if ts.Enabled() {
		t.Fatal("must be disabled without keys")
	}
	if err := ts.Verify(context.Background(), "", ""); err != nil {
		t.Fatal("disabled verifier must pass")
	}
}

func TestHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("app")) })
	h := Headers(inner, HeadersOptions{NoIndex: true, HTTPS: true})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/signin", nil))
	if rec.Header().Get("X-Robots-Tag") == "" {
		t.Error("missing X-Robots-Tag")
	}
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("missing HSTS")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if !strings.Contains(rec.Body.String(), "Disallow: /\n") {
		t.Errorf("robots.txt = %q", rec.Body.String())
	}

	open := Headers(inner, HeadersOptions{})
	rec = httptest.NewRecorder()
	open.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if rec.Header().Get("X-Robots-Tag") != "" || rec.Body.String() != "app" {
		t.Error("NoIndex off must leave the app untouched")
	}
}

func TestRequireSecretHeader(t *testing.T) {
	var seen string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Origin-Auth")
		w.Write([]byte("app"))
	})
	h := RequireSecretHeader(inner, "X-Origin-Auth", "s3cret")

	for _, v := range []string{"", "wrong", "s3cre", "s3cret2"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if v != "" {
			req.Header.Set("X-Origin-Auth", v)
		}
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("value %q: status %d", v, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Origin-Auth", "s3cret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "app" {
		t.Fatalf("valid secret refused: %d", rec.Code)
	}
	if seen != "" {
		t.Error("secret header was passed on to the application")
	}

	if RequireSecretHeader(inner, "X-Origin-Auth", "") == nil {
		t.Fatal("nil handler")
	}
	rec = httptest.NewRecorder()
	RequireSecretHeader(inner, "X-Origin-Auth", "").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Error("an empty secret must disable the check")
	}
}
