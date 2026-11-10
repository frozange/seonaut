package audit

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidURL   = errors.New("audit: invalid URL")
	ErrInvalidEmail = errors.New("audit: invalid email")
	ErrLimit        = errors.New("audit: limit reached")
)

// CrawlRunner crawls a site with the regular SEOnaut crawler and returns once
// the issues are computed. Cleanup removes the project and its crawl data.
type CrawlRunner interface {
	Crawl(ctx context.Context, siteURL string) (projectID, crawlID int64, err error)
	Cleanup(projectID int64)
}

// Mailer sends plain text e-mails.
type Mailer interface {
	Send(to, subject, body string) error
}

// Translate returns the text of a translation key in a language.
type Translate func(lang, key string, args ...interface{}) string

// Config holds the audit settings.
type Config struct {
	BaseURL      string        // public URL of the instance, for report links
	NotifyEmail  string        // copy of every finished or failed audit
	DefaultLang  string        // report language
	LanguageName string        // language name given to the AI, e.g. "French"
	UserAgent    string        // user agent of the page text fetches
	CrawlLimit   int           // pages per crawl, shown in the report
	KeyPages     int           // main pages read by the AI, home page included
	DailyCap     int           // audits per day for the whole instance, 0 = no cap
	PerEmailDay  int           // audits per day for one address, 0 = no cap
	ReuseFor     time.Duration // a site audited less than this ago is not crawled again
	CrawlTimeout time.Duration // whole crawl, issues included
	KeepProjects bool          // keep the SEOnaut project after the report is saved
}

// Service accepts requests and runs them one at a time.
type Service struct {
	Store       *Store
	Reader      *CrawlReader
	Runner      CrawlRunner
	Interpreter *Interpreter // nil: reports are published without the AI part
	Mailer      Mailer
	HTTP        *http.Client // guarded client for the page text fetches
	Trans       Translate
	Cfg         Config

	wake chan struct{}
}

// NormalizeURL keeps the scheme and host of a site URL: an audit always
// starts from the home page.
func NormalizeURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, ErrInvalidURL
	}
	return &url.URL{Scheme: u.Scheme, Host: strings.ToLower(u.Host), Path: "/"}, nil
}

// Request validates and queues an audit. When the same site was audited less
// than ReuseFor ago, that report is returned instead of crawling again.
func (s *Service) Request(rawURL, email, ip string) (*Audit, error) {
	u, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil || strings.ContainsAny(addr.Address, "\r\n") {
		return nil, ErrInvalidEmail
	}
	email = strings.ToLower(addr.Address)

	day := time.Now().Add(-24 * time.Hour)
	if s.Cfg.PerEmailDay > 0 {
		if n, err := s.Store.CountSince(day, email, ""); err != nil || n >= s.Cfg.PerEmailDay {
			return nil, ErrLimit
		}
	}
	if s.Cfg.ReuseFor > 0 {
		if prev, err := s.Store.LatestDoneForHost(u.Host, time.Now().Add(-s.Cfg.ReuseFor)); err == nil {
			go s.mailReport(prev, email)
			return prev, nil
		}
	}
	if s.Cfg.DailyCap > 0 {
		if n, err := s.Store.CountSince(day, "", ""); err != nil || n >= s.Cfg.DailyCap {
			return nil, ErrLimit
		}
	}

	a := &Audit{URL: u.String(), Email: email, Lang: s.Cfg.DefaultLang, IP: ip}
	if err := s.Store.Create(a); err != nil {
		return nil, err
	}
	s.signal()
	return a, nil
}

func (s *Service) signal() {
	if s.wake == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Start runs the queue in the background until ctx ends. Audits a restart
// interrupted are queued again.
func (s *Service) Start(ctx context.Context) {
	s.wake = make(chan struct{}, 1)
	if err := s.Store.RequeueInterrupted(); err != nil {
		log.Printf("audit: requeue: %v", err)
	}
	go func() {
		for {
			a, err := s.Store.NextQueued()
			if err == nil {
				s.Run(ctx, a)
				continue
			}
			if !errors.Is(err, ErrNotFound) {
				log.Printf("audit: queue: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-time.After(30 * time.Second):
			}
		}
	}()
}

// Run executes one audit from crawl to e-mail.
func (s *Service) Run(ctx context.Context, a *Audit) {
	fail := func(step string, err error) {
		log.Printf("audit %d %s: %s: %v", a.ID, a.URL, step, err)
		s.Store.SetStatus(a, StatusFailed, step+": "+err.Error())
		s.notify(a, "failed: "+step+": "+err.Error())
	}

	s.Store.SetStatus(a, StatusCrawling, "")
	timeout := s.Cfg.CrawlTimeout
	if timeout == 0 {
		timeout = 45 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	pid, cid, err := s.Runner.Crawl(cctx, a.URL)
	cancel()
	if err != nil {
		fail("crawl", err)
		return
	}
	s.Store.SetCrawl(a, pid, cid)
	if !s.Cfg.KeepProjects {
		defer s.Runner.Cleanup(pid)
	}

	s.Store.SetStatus(a, StatusAnalysing, "")
	res, err := s.analyse(ctx, a, cid)
	if err != nil {
		fail("analysis", err)
		return
	}
	if err := s.Store.SaveResult(a, res); err != nil {
		fail("save", err)
		return
	}
	s.mailReport(a, a.Email)
	note := "done"
	if res.AIError != "" {
		note += ", without AI: " + res.AIError
	}
	s.notify(a, note)
}

func (s *Service) analyse(ctx context.Context, a *Audit, crawlID int64) (*Result, error) {
	u, _ := url.Parse(a.URL)
	facts, err := s.Reader.Facts(crawlID)
	if err != nil {
		return nil, err
	}
	facts.URL, facts.Host, facts.CrawlLimit = a.URL, u.Host, s.Cfg.CrawlLimit
	facts.LimitReached = s.Cfg.CrawlLimit > 0 && facts.PagesCrawled >= s.Cfg.CrawlLimit
	if facts.PagesCrawled == 0 {
		return nil, errors.New("the site returned no page")
	}

	issues, passed, err := s.Reader.Issues(crawlID, 5)
	if err != nil {
		return nil, err
	}
	keyPages := s.Cfg.KeyPages
	if keyPages <= 0 {
		keyPages = 7
	}
	pages, err := s.Reader.KeyPages(crawlID, a.URL, keyPages)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		max := 400
		if i == 0 {
			max = 1800
		}
		fctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		pages[i].Text, pages[i].Headings, _ = PageText(fctx, s.HTTP, s.Cfg.UserAgent, pages[i].URL, max)
		cancel()
	}

	res := &Result{
		GeneratedAt: time.Now().UTC(),
		Site:        facts,
		Issues:      issues,
		PassedCount: passed,
		KeyPages:    pages,
	}
	res.Scores.Technical = TechnicalScore(issues, facts.PagesCrawled)
	res.Evidence = BuildEvidence(facts, pages, issues, func(t string) string { return s.Trans("en", t) })
	priorities := TechnicalPriorities(issues)

	if s.Interpreter == nil {
		res.AIError = "disabled"
		res.Priorities = rankPriorities(priorities, 8)
		return res, nil
	}
	rep, prio, err := s.Interpreter.Interpret(ctx, res.Evidence, priorities)
	res.Priorities = rankPriorities(prio, 8)
	if err != nil {
		res.AIError = err.Error()
		log.Printf("audit %d: AI: %v", a.ID, err)
	}
	if rep != nil && err == nil {
		res.AI = rep
		res.Scores.Copywriting = GroupScore(rep.Criteria, "copywriting")
		res.Scores.Content = GroupScore(rep.Criteria, "content")
	}
	return res, nil
}

// ReportURL is the private address of a report.
func (s *Service) ReportURL(a *Audit) string {
	return strings.TrimRight(s.Cfg.BaseURL, "/") + "/rapport/" + a.Token
}

func (s *Service) mailReport(a *Audit, to string) {
	if s.Mailer == nil {
		return
	}
	u, _ := url.Parse(a.URL)
	subject := s.Trans(a.Lang, "AUDIT_MAIL_SUBJECT", u.Host)
	body := s.Trans(a.Lang, "AUDIT_MAIL_BODY", u.Host, s.ReportURL(a))
	if err := s.Mailer.Send(to, subject, body); err != nil {
		log.Printf("audit %d: report mail: %v", a.ID, err)
	}
}

func (s *Service) notify(a *Audit, note string) {
	if s.Mailer == nil || s.Cfg.NotifyEmail == "" {
		return
	}
	body := fmt.Sprintf("Site: %s\nRequested by: %s (%s)\nStatus: %s\nReport: %s\n", a.URL, a.Email, a.IP, note, s.ReportURL(a))
	if err := s.Mailer.Send(s.Cfg.NotifyEmail, "[SEO audit] "+a.URL+" "+string(a.Status), body); err != nil {
		log.Printf("audit %d: notify: %v", a.ID, err)
	}
}
