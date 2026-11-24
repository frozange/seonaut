package routes

import (
	"errors"
	"fmt"
	"log"
	"math"
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
	http.Redirect(w, r, h.AuditService.ReportURL(a), http.StatusSeeOther)
}

// reportHandler shows the report, or a waiting page that refreshes itself. When
// the report lives on the main site (report_url), the visitor is sent there:
// links already mailed keep working. The token or the site slug both find it.
func (h *auditHandler) reportHandler(w http.ResponseWriter, r *http.Request) {
	a, err := h.AuditService.Store.ByKey(r.PathValue("token"))
	if err != nil {
		h.renderAudit(w, "audit_status", "AUDIT_PAGE_TITLE", http.StatusNotFound, &auditStatusView{NotFound: true})
		return
	}
	if h.AuditService.ReportOnSite() {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, h.AuditService.ReportURL(a), http.StatusFound)
		return
	}
	if a.Status != audit.StatusDone || a.Result == nil {
		h.renderAudit(w, "audit_status", "AUDIT_PAGE_TITLE", http.StatusOK, newStatusView(a))
		return
	}
	h.renderAudit(w, "audit_report", "AUDIT_REPORT_TITLE", http.StatusOK, newReportView(a, h.lang(), false))
}

// fragmentHandler returns the report, or its waiting state, as an HTML fragment
// for the page of the main site that frames it: no html, head or body, styles
// scoped under .lfr, every crawled or AI text escaped by html/template. Headers
// tell the framing page the state, the audited host and the canonical slug, so
// a page asked by an old token can redirect to the slug address.
func (h *auditHandler) fragmentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	a, err := h.AuditService.Store.ByKey(r.PathValue("token"))
	if err != nil {
		w.Header().Set("X-LF-Audit-Status", "notfound")
		h.renderAudit(w, "audit_fragment", "AUDIT_PAGE_TITLE", http.StatusNotFound, &fragmentView{Status: &auditStatusView{NotFound: true}})
		return
	}
	w.Header().Set("X-LF-Audit-Host", hostOf(a.URL))
	w.Header().Set("X-LF-Audit-Slug", audit.Slug(a.URL))
	if a.Status != audit.StatusDone || a.Result == nil {
		w.Header().Set("X-LF-Audit-Status", string(a.Status))
		h.renderAudit(w, "audit_fragment", "AUDIT_PAGE_TITLE", http.StatusOK, &fragmentView{Status: newStatusView(a)})
		return
	}
	w.Header().Set("X-LF-Audit-Status", string(audit.StatusDone))
	h.renderAudit(w, "audit_fragment", "AUDIT_REPORT_TITLE", http.StatusOK, &fragmentView{Report: newReportView(a, h.lang(), true)})
}

// fragmentView carries either a report or a waiting state.
type fragmentView struct {
	Report *reportView
	Status *auditStatusView
}

func newStatusView(a *audit.Audit) *auditStatusView {
	return &auditStatusView{Host: hostOf(a.URL), Status: string(a.Status), Failed: a.Status == audit.StatusFailed}
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

// reportView prepares the result for the template: formatted scores, the
// figures the charts draw, and the issues split by priority.
type reportView struct {
	Host        string
	URL         string
	Date        string
	Result      *audit.Result
	Embed       bool // framed by a page of the main site: no title, no call to action
	Scores      []scoreCard
	Critical    []issueRow
	Alerts      []issueRow
	Warnings    []issueRow
	Hygiene     []issueRow
	IssueTypes  int // critical, alert and warning types found
	HasAI       bool
	ContactURL  string
	SampleNote  bool
	Copywriting []audit.Criterion
	Content     []audit.Criterion
	CopyMix     ratingMix
	ContentMix  ratingMix
	Levels      []levelCount
	Queries     []queryRow
	Dropped     []queryRow // target searches Google Suggest does not know
	Checked     bool       // target searches were checked against Google Suggest
	Speed       *speedView
	Structured  *sdView
	Linking     *linkingView
	Dups        *dupView
}

type scoreCard struct {
	Key   string // translation key of the label
	Value string // "7,5" or "–"
	Class string // good, mid, low, none
	Pct   int    // width of the bar
	Arc   string // length of the gauge arc, on a circle of radius 52
}

// ratingMix counts the criteria of a group by rating, for the stacked bar.
type ratingMix struct {
	Good, Partial, Poor, Total int
}

// levelCount is the number of priorities of one level.
type levelCount struct {
	Level int
	Count int
}

// issueRow is an issue type with the share of read pages it touches.
type issueRow struct {
	audit.IssueSummary
	Pct int // bar length: pages concerned out of pages read, at least visible
}

// queryRow is a target query with a short page path and its alignment step.
type queryRow struct {
	audit.TargetQuery
	Path        string
	Step        int    // 3 good, 2 partial, 1 weak
	GoogleClass string // good, mid, none
}

const gaugeCircumference = 2 * math.Pi * 52

func formatScore(v *float64, lang string) scoreCard {
	if v == nil {
		return scoreCard{Class: "none", Arc: "0"}
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
	pct := int(math.Round(*v * 10))
	return scoreCard{Value: s, Class: c, Pct: pct, Arc: fmt.Sprintf("%.1f", gaugeCircumference*float64(pct)/100)}
}

func newIssueRows(list []audit.IssueSummary, read int) []issueRow {
	rows := make([]issueRow, 0, len(list))
	for _, is := range list {
		pct := 100
		if read > 0 {
			pct = is.Pages * 100 / read
		}
		if pct < 3 {
			pct = 3
		}
		if pct > 100 {
			pct = 100
		}
		rows = append(rows, issueRow{IssueSummary: is, Pct: pct})
	}
	return rows
}

func newRatingMix(list []audit.Criterion) ratingMix {
	var m ratingMix
	for _, c := range list {
		switch c.Rating {
		case "good":
			m.Good++
		case "partial":
			m.Partial++
		default:
			m.Poor++
		}
	}
	m.Total = m.Good + m.Partial + m.Poor
	return m
}

// pagePath shortens a page address to its path for display.
func pagePath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	return p
}

func newQueryRow(q audit.TargetQuery) queryRow {
	step := 1
	switch q.Alignment {
	case "good":
		step = 3
	case "partial":
		step = 2
	}
	gc := "none"
	switch q.Google {
	case audit.QueryConfirmed:
		gc = "good"
	case audit.QueryVariant:
		gc = "mid"
	case audit.QueryUnknown:
		gc = "low"
	}
	return queryRow{TargetQuery: q, Path: pagePath(q.Page), Step: step, GoogleClass: gc}
}

func newReportView(a *audit.Audit, lang string, embed bool) *reportView {
	res := a.Result
	v := &reportView{
		Host:   hostOf(a.URL),
		URL:    a.URL,
		Date:   res.GeneratedAt.Format("02/01/2006"),
		Result: res,
		Embed:  embed,
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
	var critical, alerts, warnings []audit.IssueSummary
	for _, is := range res.Issues {
		switch is.Priority {
		case 1:
			critical = append(critical, is)
		case 2:
			alerts = append(alerts, is)
		default:
			warnings = append(warnings, is)
		}
	}
	read := res.Site.PagesCrawled
	v.Critical = newIssueRows(critical, read)
	v.Alerts = newIssueRows(alerts, read)
	v.Warnings = newIssueRows(warnings, read)
	v.Hygiene = newIssueRows(res.Hygiene, read)
	v.IssueTypes = len(res.Issues)

	counts := map[int]int{}
	for _, p := range res.Priorities {
		counts[p.Level]++
	}
	for _, l := range []int{1, 2, 3} {
		if counts[l] > 0 {
			v.Levels = append(v.Levels, levelCount{Level: l, Count: counts[l]})
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
		v.CopyMix = newRatingMix(v.Copywriting)
		v.ContentMix = newRatingMix(v.Content)
		for _, q := range res.AI.TargetQueries {
			v.Queries = append(v.Queries, newQueryRow(q))
		}
		for _, q := range res.AI.DroppedQueries {
			v.Dropped = append(v.Dropped, newQueryRow(q))
		}
		v.Checked = res.AI.QueriesChecked
	}
	v.Speed = newSpeedView(res.Speed, lang)
	v.Structured = newSDView(res.Structured)
	v.Linking = newLinkingView(res.Linking)
	v.Dups = newDupView(res)
	v.SampleNote = res.Site.LimitReached
	return v
}
