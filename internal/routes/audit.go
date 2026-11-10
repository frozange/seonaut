package routes

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/stjudewashere/seonaut/internal/audit"
)

type auditHandler struct {
	*userHandler
}

// auditFormView is the data of the public audit form.
type auditFormView struct {
	URL          string
	Email        string
	ErrorMessage string
}

func (h *auditHandler) lang() string { return h.Config.Audit.Language }

func (h *auditHandler) renderAudit(w http.ResponseWriter, tpl, title string, status int, data interface{}) {
	pv := &PageView{Lang: h.lang(), Theme: "dark", PageTitle: title, Data: data}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	h.Renderer.RenderTemplate(w, tpl, pv, h.lang())
}

// formHandler shows the free audit form.
func (h *auditHandler) formHandler(w http.ResponseWriter, r *http.Request) {
	h.renderAudit(w, "audit_form", "AUDIT_PAGE_TITLE", http.StatusOK, &auditFormView{URL: r.URL.Query().Get("url")})
}

// requestHandler queues an audit: per-IP limit, Turnstile, network guard on the
// site, then the queue. The visitor lands on the report page, which waits.
func (h *auditHandler) requestHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/audit", http.StatusSeeOther)
		return
	}
	v := &auditFormView{URL: strings.TrimSpace(r.FormValue("url")), Email: strings.TrimSpace(r.FormValue("email"))}
	fail := func(status int, key string) {
		v.ErrorMessage = key
		h.renderAudit(w, "audit_form", "AUDIT_PAGE_TITLE", status, v)
	}

	ip := h.ClientIP.IP(r)
	if !h.AuditIPLimiter.Allow(ip) {
		fail(http.StatusTooManyRequests, "ERROR_TOO_MANY_ATTEMPTS")
		return
	}
	if err := h.Turnstile.Verify(r.Context(), r.FormValue("cf-turnstile-response"), ip); err != nil {
		fail(http.StatusForbidden, "ERROR_HUMAN_CHECK")
		return
	}
	u, err := audit.NormalizeURL(v.URL)
	if err != nil {
		fail(http.StatusOK, "AUDIT_ERROR_URL")
		return
	}
	if err := h.NetGuard.CheckURL(r.Context(), u); err != nil {
		fail(http.StatusOK, "AUDIT_ERROR_URL")
		return
	}

	a, err := h.AuditService.Request(u.String(), v.Email, ip)
	switch {
	case errors.Is(err, audit.ErrInvalidEmail):
		fail(http.StatusOK, "ERROR_INVALID_EMAIL")
		return
	case errors.Is(err, audit.ErrLimit):
		fail(http.StatusTooManyRequests, "AUDIT_ERROR_LIMIT")
		return
	case err != nil:
		log.Printf("audit request: %v", err)
		fail(http.StatusServiceUnavailable, "AUDIT_ERROR_UNAVAILABLE")
		return
	}
	http.Redirect(w, r, "/rapport/"+url.PathEscape(a.Token), http.StatusSeeOther)
}

// reportHandler shows the report, or a waiting page that refreshes itself.
func (h *auditHandler) reportHandler(w http.ResponseWriter, r *http.Request) {
	a, err := h.AuditService.Store.ByToken(r.PathValue("token"))
	if err != nil {
		h.renderAudit(w, "audit_status", "AUDIT_PAGE_TITLE", http.StatusNotFound, &auditStatusView{NotFound: true})
		return
	}
	if a.Status != audit.StatusDone || a.Result == nil {
		h.renderAudit(w, "audit_status", "AUDIT_PAGE_TITLE", http.StatusOK, &auditStatusView{
			Host: hostOf(a.URL), Status: string(a.Status), Failed: a.Status == audit.StatusFailed,
		})
		return
	}
	h.renderAudit(w, "audit_report", "AUDIT_REPORT_TITLE", http.StatusOK, newReportView(a, h.lang()))
}

type auditStatusView struct {
	Host     string
	Status   string
	Failed   bool
	NotFound bool
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host
}

// reportView prepares the result for the template: formatted scores and the
// issues split by priority.
type reportView struct {
	Host        string
	URL         string
	Date        string
	Result      *audit.Result
	Scores      []scoreCard
	Critical    []audit.IssueSummary
	Alerts      []audit.IssueSummary
	Warnings    []audit.IssueSummary
	HasAI       bool
	ContactURL  string
	SampleNote  bool
	Copywriting []audit.Criterion
	Content     []audit.Criterion
}

type scoreCard struct {
	Key   string // translation key of the label
	Value string // "7,5" or "–"
	Class string // good, mid, low, none
	Pct   int    // width of the bar
}

func formatScore(v *float64, lang string) scoreCard {
	if v == nil {
		return scoreCard{Value: "–", Class: "none"}
	}
	s := strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", *v), "0"), ".")
	if lang == "fr" {
		s = strings.ReplaceAll(s, ".", ",")
	}
	c := "low"
	switch {
	case *v >= 7.5:
		c = "good"
	case *v >= 5:
		c = "mid"
	}
	return scoreCard{Value: s, Class: c, Pct: int(*v * 10)}
}

func newReportView(a *audit.Audit, lang string) *reportView {
	res := a.Result
	v := &reportView{
		Host:   hostOf(a.URL),
		URL:    a.URL,
		Date:   res.GeneratedAt.Format("02/01/2006"),
		Result: res,
		HasAI:  res.AI != nil,
	}
	if lang == "fr" {
		v.ContactURL = "https://lafactory.com/fr/contact/"
	} else {
		v.ContactURL = "https://lafactory.com/contact/"
	}
	tech := res.Scores.Technical
	for _, s := range []struct {
		key string
		val *float64
	}{{"AUDIT_SCORE_TECHNICAL", &tech}, {"AUDIT_SCORE_COPYWRITING", res.Scores.Copywriting}, {"AUDIT_SCORE_CONTENT", res.Scores.Content}} {
		c := formatScore(s.val, lang)
		c.Key = s.key
		v.Scores = append(v.Scores, c)
	}
	for _, is := range res.Issues {
		switch is.Priority {
		case 1:
			v.Critical = append(v.Critical, is)
		case 2:
			v.Alerts = append(v.Alerts, is)
		default:
			v.Warnings = append(v.Warnings, is)
		}
	}
	if res.AI != nil {
		for _, c := range res.AI.Criteria {
			if c.Group == "copywriting" {
				v.Copywriting = append(v.Copywriting, c)
			} else {
				v.Content = append(v.Content, c)
			}
		}
	}
	v.SampleNote = res.Site.LimitReached
	return v
}
