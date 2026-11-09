package security

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTurnstileEndpoint is Cloudflare's siteverify API.
const DefaultTurnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrTurnstileFailed is returned when the token is missing, invalid or expired.
var ErrTurnstileFailed = errors.New("turnstile verification failed")

// Turnstile verifies Cloudflare Turnstile tokens on the server side.
type Turnstile struct {
	SiteKey  string
	secret   string
	endpoint string
	client   *http.Client
}

// NewTurnstile returns a verifier. When siteKey or secret is empty the verifier is
// disabled and Enabled reports false.
func NewTurnstile(siteKey, secret string) *Turnstile {
	return &Turnstile{
		SiteKey:  siteKey,
		secret:   secret,
		endpoint: DefaultTurnstileEndpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether both keys are configured.
func (t *Turnstile) Enabled() bool {
	return t != nil && t.SiteKey != "" && t.secret != ""
}

// Verify checks a token. It fails closed: a network error, a malformed reply or
// an unexpected status refuses the request.
func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) error {
	if !t.Enabled() {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 2048 {
		return ErrTurnstileFailed
	}

	form := url.Values{"secret": {t.secret}, "response": {token}}
	if remoteIP != "" && remoteIP != "unknown" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrTurnstileFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return ErrTurnstileFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrTurnstileFailed
	}

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<16)).Decode(&out); err != nil {
		return ErrTurnstileFailed
	}
	if !out.Success {
		return ErrTurnstileFailed
	}
	return nil
}
