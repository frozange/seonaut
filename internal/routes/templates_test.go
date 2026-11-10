package routes

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
)

type keyTranslator struct{}

func (keyTranslator) Trans(lang, s string, args ...interface{}) string { return s }
func (keyTranslator) TransDate(lang string, d time.Time, f string) string {
	return d.Format(f)
}

func render(t *testing.T, cfg services.RendererConfig, tpl string, data interface{}) string {
	t.Helper()
	cfg.TemplatesFolder = "../../web/templates"
	r, err := services.NewRenderer(&cfg, keyTranslator{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	r.RenderTemplate(&b, tpl, &PageView{Lang: "en", Theme: "light", PageTitle: "T", Data: data}, "en")
	return b.String()
}

// The instance switches hide some options; everything else on the project forms
// must still render, down to the footer.
func TestProjectFormsRenderWithOptionsSwitchedOff(t *testing.T) {
	always := []string{`name="follow_nofollow"`, `name="include_noindex"`, `name="crawl_sitemap"`, `name="allow_subdomains"`, `name="basic_auth"`, `</html>`}

	for _, tc := range []struct {
		tpl  string
		data interface{}
	}{
		{"project_add", &projectAddView{}},
		{"project_edit", &struct {
			Project         models.Project
			Error           bool
			UserAgentError  bool
			CustomUserAgent bool
		}{Project: models.Project{Id: 7, Host: "example.com"}}},
	} {
		off := render(t, services.RendererConfig{}, tc.tpl, tc.data)
		for _, s := range always {
			if !strings.Contains(off, s) {
				t.Errorf("%s with options off: missing %s", tc.tpl, s)
			}
		}
		for _, s := range []string{`name="ignore_robotstxt"`, `name="custom_user_agent"`} {
			if strings.Contains(off, s) {
				t.Errorf("%s with options off: %s still shown", tc.tpl, s)
			}
		}

		on := render(t, services.RendererConfig{AllowCustomUserAgent: true, AllowIgnoreRobots: true}, tc.tpl, tc.data)
		for _, s := range append(always, `name="ignore_robotstxt"`, `name="custom_user_agent"`) {
			if !strings.Contains(on, s) {
				t.Errorf("%s with options on: missing %s", tc.tpl, s)
			}
		}
	}
}

func TestProjectDeleteIsAPostForm(t *testing.T) {
	out := render(t, services.RendererConfig{}, "project_edit", &struct {
		Project         models.Project
		Error           bool
		UserAgentError  bool
		CustomUserAgent bool
	}{Project: models.Project{Id: 7, Host: "example.com"}})
	if !strings.Contains(out, `<form method="POST" action="/project/delete?pid=7"`) {
		t.Error("project deletion must be a POST form")
	}
	if strings.Contains(out, `href="/project/delete`) {
		t.Error("project deletion must not be reachable by a link")
	}
}

func TestCredentialFormsPostToTheirOwnRoute(t *testing.T) {
	for tpl, action := range map[string]string{"signin": "/signin", "signup": "/signup"} {
		out := render(t, services.RendererConfig{TurnstileSiteKey: "site-key"}, tpl, &credentialsView{})
		if !strings.Contains(out, `action="`+action+`"`) {
			t.Errorf("%s form must post to %s whatever URL rendered it", tpl, action)
		}
		if !strings.Contains(out, `data-sitekey="site-key"`) || !strings.Contains(out, "lfTurnstileReady") {
			t.Errorf("%s: Turnstile widget or submit gate missing", tpl)
		}
		if !strings.Contains(out, `</html>`) {
			t.Errorf("%s did not render completely", tpl)
		}
	}
	out := render(t, services.RendererConfig{}, "signin", &credentialsView{})
	if strings.Contains(out, "cf-turnstile") || strings.Contains(out, "challenges.cloudflare.com") {
		t.Error("Turnstile must not load without a site key")
	}
}

func TestAuditReportRenders(t *testing.T) {
	copy := 7.5
	res := &audit.Result{
		GeneratedAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Site:        audit.SiteFacts{PagesCrawled: 150, CrawlLimit: 150, LimitReached: true},
		Scores:      audit.Scores{Technical: 6.8, Copywriting: &copy},
		Issues:      []audit.IssueSummary{{Type: "ERROR_TIMEOUT", Priority: 1, Pages: 1, Examples: []string{"https://ex.com/"}}, {Type: "ERROR_SLOW_TTFB", Priority: 2, Pages: 14}},
		Priorities:  []audit.Priority{{ID: "t:ERROR_TIMEOUT", Source: "technical", Level: 1, Title: "ERROR_TIMEOUT", Pages: 1, Action: "Fix it."}},
	}
	withAI := *res
	withAI.AI = &audit.AIReport{
		Summary:  "SUMMARY-MARKER",
		Criteria: []audit.Criterion{{ID: "value", Group: "copywriting", Question: "value", Rating: "good", Verdict: "VERDICT-MARKER"}},
		Rewrites: []audit.Rewrite{{Element: "title", Before: "Old", After: "New"}},
	}
	for name, r := range map[string]*audit.Result{"without AI": res, "with AI": &withAI} {
		a := &audit.Audit{URL: "https://ex.com/", Result: r}
		out := render(t, services.RendererConfig{}, "audit_report", newReportView(a, "fr"))
		for _, s := range []string{"</html>", "6,8", "AUDIT_PAGE_AFFECTED_ONE", "AUDIT_METHOD_SAMPLE", "ERROR_SLOW_TTFB"} {
			if !strings.Contains(out, s) {
				t.Errorf("%s: missing %s", name, s)
			}
		}
		if name == "with AI" {
			for _, s := range []string{"SUMMARY-MARKER", "VERDICT-MARKER", "AUDIT_CRITERION_value", "7,5"} {
				if !strings.Contains(out, s) {
					t.Errorf("with AI: missing %s", s)
				}
			}
		} else if !strings.Contains(out, "AUDIT_NO_AI") {
			t.Error("without AI: the report must say the written analysis is missing")
		}
	}
}
