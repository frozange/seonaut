package routes

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/stjudewashere/seonaut/internal/audit"
)

// RelayAuthHeader carries the relay secret of the main site.
const RelayAuthHeader = "X-LF-Relay-Auth"

// minRelaySecret is the shortest relay secret that arms the route.
const minRelaySecret = 32

// relayRequest is what the main site sends for one of its signed-in members.
type relayRequest struct {
	URL   string `json:"url"`
	Email string `json:"email"`
	IP    string `json:"ip"`
	// Owner marks an administrator of the main site: the audit is the owner's
	// and uses none of the form's quotas, as with auditctl.
	Owner bool `json:"owner"`
}

// relayResponse answers the main site. Error is a translation key of the form.
type relayResponse struct {
	Status    string `json:"status,omitempty"`
	Slug      string `json:"slug,omitempty"`
	ReportURL string `json:"report_url,omitempty"`
	Error     string `json:"error,omitempty"`
}

// relayAllowed tells whether the request carries the configured relay secret.
// A secret shorter than minRelaySecret never arms the route.
func relayAllowed(secret, got string) bool {
	if len(secret) < minRelaySecret {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(secret)) == 1
}

// relayHandler queues an audit for a member of the main site. The member signed
// in there, which stands in for Turnstile; the per-IP limit (on the member's
// address), the network guard and the quotas of the form still apply.
func (h *auditHandler) relayHandler(w http.ResponseWriter, r *http.Request) {
	if !relayAllowed(h.Config.Audit.RelaySecret, r.Header.Get(RelayAuthHeader)) {
		http.NotFound(w, r)
		return
	}
	reply := func(status int, res relayResponse) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(res)
	}
	fail := func(status int, key string) { reply(status, relayResponse{Error: key}) }

	var in relayRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in); err != nil {
		fail(http.StatusBadRequest, "GENERIC_ERROR")
		return
	}

	ip := strings.TrimSpace(in.IP)
	if in.Owner {
		ip = audit.OwnerIP
	} else {
		if ip == "" || ip == audit.OwnerIP {
			fail(http.StatusBadRequest, "GENERIC_ERROR")
			return
		}
		if !h.AuditIPLimiter.Allow(ip) {
			fail(http.StatusTooManyRequests, "ERROR_TOO_MANY_ATTEMPTS")
			return
		}
	}

	u, err := audit.NormalizeURL(strings.TrimSpace(in.URL))
	if err != nil {
		fail(http.StatusUnprocessableEntity, "AUDIT_ERROR_URL")
		return
	}
	if err := h.NetGuard.CheckURL(r.Context(), u); err != nil {
		fail(http.StatusUnprocessableEntity, "AUDIT_ERROR_URL")
		return
	}

	a, err := h.AuditService.Request(u.String(), in.Email, ip)
	switch {
	case errors.Is(err, audit.ErrInvalidEmail):
		fail(http.StatusUnprocessableEntity, "ERROR_INVALID_EMAIL")
		return
	case errors.Is(err, audit.ErrLimit):
		fail(http.StatusTooManyRequests, "AUDIT_ERROR_LIMIT")
		return
	case err != nil:
		log.Printf("audit relay: %v", err)
		fail(http.StatusServiceUnavailable, "AUDIT_ERROR_UNAVAILABLE")
		return
	}
	reply(http.StatusOK, relayResponse{
		Status:    string(a.Status),
		Slug:      audit.Slug(a.URL),
		ReportURL: h.AuditService.ReportURL(a),
	})
}
