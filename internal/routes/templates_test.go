package routes

import (
	"bytes"
	"strings"
	"testing"
	"time"

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
