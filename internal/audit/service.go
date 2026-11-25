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

	"github.com/stjudewashere/seonaut/internal/webgraph"
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

// LeftoverLister is implemented by a CrawlRunner that can list the projects
// it still holds. At startup no audit is running, so every one of them was
// left by an interruption (restart, crash) and is removed.
type LeftoverLister interface {
	LeftoverProjects() []int64
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
	ReportURL    string        // report address on the main site, {slug} or {token} replaced; empty: BaseURL/rapport/
	NotifyEmail  string        // copy of every finished or failed audit
	DefaultLang  string        // report language
	LanguageName string        // language name given to the AI, e.g. "French"
	UserAgent    string        // user agent of the page text fetches
	CrawlLimit   int           // pages per crawl, shown in the report
	ExcludeDir   string        // per-site exclusion lists, see Exclusion
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
	Signals     *Collector      // pages seen by the crawler; nil: no structured data, linking or duplicates
	Speed       *SpeedTester    // nil: no speed section
	Suggest     *Suggester      // nil: target searches are not checked against Google
	Graph       *webgraph.Store // nil or no index: no popularity section

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
	// Listed before the queue starts, so a project the queue creates next is
	// never taken for a leftover; deleted in the background.
	if leftovers := s.leftovers(); len(leftovers) > 0 {
		go func() {
			for _, id := range leftovers {
				s.Runner.Cleanup(id)
			}
			log.Printf("audit: removed %d project(s) left by an interrupted audit", len(leftovers))
		}()
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

// leftovers returns the projects to remove at startup: none when projects are
// kept on purpose.
func (s *Service) leftovers() []int64 {
	if s.Cfg.KeepProjects {
		return nil
	}
	l, ok := s.Runner.(LeftoverLister)
	if !ok {
		return nil
	}
	return l.LeftoverProjects()
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
	var signals []*PageSignal
	if s.Signals != nil && pid > 0 {
		signals = s.Signals.Take(pid)
	}
	if err != nil {
		fail("crawl", err)
		return
	}
	s.Store.SetCrawl(a, pid, cid)
	if !s.Cfg.KeepProjects {
		defer s.Runner.Cleanup(pid)
	}

	s.Store.SetStatus(a, StatusAnalysing, "")
	res, err := s.analyse(ctx, a, cid, signals)
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

func (s *Service) analyse(ctx context.Context, a *Audit, crawlID int64, signals []*PageSignal) (*Result, error) {
	u, _ := url.Parse(a.URL)
	var speed chan *SpeedReport
	if s.Speed != nil {
		speed = make(chan *SpeedReport, 1)
		go func() { speed <- s.Speed.Measure(ctx, a.URL) }()
	}
	facts, err := s.Reader.Facts(crawlID)
	if err != nil {
		return nil, err
	}
	facts.URL, facts.Host, facts.CrawlLimit = a.URL, u.Host, s.Cfg.CrawlLimit
	facts.LimitReached = s.Cfg.CrawlLimit > 0 && facts.PagesCrawled >= s.Cfg.CrawlLimit
	if ex, err := LoadExclusion(s.Cfg.ExcludeDir, a.URL); err == nil {
		facts.Excluded = ex.Len()
	}
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
		if pc, err := PageText(fctx, s.HTTP, s.Cfg.UserAgent, pages[i].URL, max); err == nil {
			pages[i].Text, pages[i].Headings = pc.Text, pc.Headings
			pages[i].Footer, pages[i].Actions, pages[i].Contacts = pc.Footer, pc.Actions, pc.Contacts
			pages[i].Forms, pages[i].Alternates = pc.Forms, pc.Alternates
		}
		cancel()
	}

	issues, hygiene := SplitByGoogle(issues)
	res := &Result{
		GeneratedAt: time.Now().UTC(),
		Site:        facts,
		Issues:      issues,
		Hygiene:     hygiene,
		PassedCount: passed,
		KeyPages:    pages,
	}
	res.Scores.Technical = TechnicalScore(issues, facts.PagesCrawled)
	if s.Signals != nil {
		res.Measured = true
		res.Structured = BuildStructured(signals)
		res.Linking = BuildLinking(signals, normalizeURL(a.URL), keyURLs(pages))
		res.Duplicates = NearDuplicates(signals)
	}
	if speed != nil {
		res.Speed = <-speed
	}
	s.measurePopularity(res, u.Host)
	res.Evidence = BuildEvidence(facts, pages, issues, func(t string) (string, string) {
		return s.Trans("en", t), s.Trans("en", t+"_DESC")
	})
	var evIDs map[string]string
	res.Evidence, evIDs = MeasureEvidence(res, res.Evidence)
	priorities := append(TechnicalPriorities(issues), MeasuredPriorities(res, evIDs)...)

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
		s.checkQueries(ctx, res, pages, u.Host)
	}
	return res, nil
}

// Requery checks the target searches of a finished report against Google
// Suggest again and drops the measured priorities the current rules no longer
// make, without crawling or calling the AI.
func (s *Service) Requery(ctx context.Context, a *Audit) error {
	res := a.Result
	if res == nil || res.AI == nil {
		return errors.New("the report has no AI part, hence no target search")
	}
	if s.Suggest == nil {
		return errors.New("audit.suggest is off")
	}
	all := append(append([]TargetQuery{}, res.AI.TargetQueries...), res.AI.DroppedQueries...)
	for i := range all {
		all[i].Google, all[i].GoogleVariant, all[i].GoogleRelated = "", "", nil
	}
	res.AI.TargetQueries, res.AI.DroppedQueries = all, nil
	u, err := url.Parse(a.URL)
	if err != nil {
		return err
	}
	s.checkQueries(ctx, res, res.KeyPages, u.Host)
	RefreshMeasuredPriorities(res)
	return s.Store.SaveResult(a, res)
}

// measurePopularity reads the sites linking to the audited site, and to the
// competitors of the result, in the latest Common Crawl graph.
func (s *Service) measurePopularity(res *Result, host string) {
	if s.Graph == nil {
		return
	}
	cur, prev, err := s.Graph.Open()
	if err != nil {
		if !errors.Is(err, webgraph.ErrNoIndex) {
			log.Printf("audit: popularity: %v", err)
		}
		return
	}
	defer cur.Close()
	defer prev.Close()
	res.Popularity = BuildPopularity(cur, prev, host, res.Competitors)
}

// RefreshPopularity measures the popularity of a finished report again,
// with these competitors when given, without crawling or calling the AI.
func (s *Service) RefreshPopularity(a *Audit, competitors []string) error {
	res := a.Result
	if res == nil {
		return errors.New("the audit has no report")
	}
	if s.Graph == nil {
		return errors.New("audit.webgraph_dir is empty")
	}
	if competitors != nil {
		res.Competitors = competitors
	}
	u, err := url.Parse(a.URL)
	if err != nil {
		return err
	}
	res.Popularity = nil
	s.measurePopularity(res, u.Host)
	if res.Popularity == nil {
		return webgraph.ErrNoIndex
	}
	return s.Store.SaveResult(a, res)
}

// NeedsPopularity tells whether the latest report of a site should get its
// popularity measured now: it has none, or it was measured on the current
// release before the previous quarter was there to compare with. A report
// measured on an older release keeps it: a new audit brings the new one.
func NeedsPopularity(p *PopularityReport, cur, prev *webgraph.Release) bool {
	if cur == nil {
		return false
	}
	if p == nil {
		return true
	}
	return p.Release == cur.Name && p.Previous == "" && prev != nil
}

// FillPopularity measures the popularity of the latest finished report of
// each site when NeedsPopularity says so. It returns the reports updated.
func (s *Service) FillPopularity() (int, error) {
	if s.Graph == nil {
		return 0, errors.New("audit.webgraph_dir is empty")
	}
	cur, prev := webgraph.Pair(s.Graph.Ready())
	if cur == nil {
		return 0, webgraph.ErrNoIndex
	}
	slugs, err := s.Store.DoneSlugs()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sl := range slugs {
		a, err := s.Store.BySlug(sl)
		if err != nil || a.Status != StatusDone || a.Result == nil || !NeedsPopularity(a.Result.Popularity, cur, prev) {
			continue
		}
		if err := s.RefreshPopularity(a, nil); err != nil {
			return n, fmt.Errorf("%s: %w", sl, err)
		}
		n++
	}
	return n, nil
}

// checkQueries confronts the AI's target searches with Google Suggest.
func (s *Service) checkQueries(ctx context.Context, res *Result, pages []PageFacts, host string) {
	rep := res.AI
	if s.Suggest != nil && len(rep.TargetQueries) > 0 {
		sg := *s.Suggest
		if sg.Lang == "" {
			sg.Lang, sg.Country = SuggestLocale(pageLang(pages), host, s.Cfg.DefaultLang)
		}
		rep.TargetQueries, rep.DroppedQueries = sg.CheckQueries(ctx, rep.TargetQueries)
		rep.QueriesChecked = true
	}
}

// ReportURL is the private address of a report.
func (s *Service) ReportURL(a *Audit) string {
	if s.ReportOnSite() {
		r := strings.ReplaceAll(s.Cfg.ReportURL, "{slug}", Slug(a.URL))
		return strings.ReplaceAll(r, "{token}", url.PathEscape(a.Token))
	}
	return strings.TrimRight(s.Cfg.BaseURL, "/") + "/rapport/" + a.Token
}

// ReportOnSite tells whether reports are shown by a page of the main site.
func (s *Service) ReportOnSite() bool {
	return strings.Contains(s.Cfg.ReportURL, "{slug}") || strings.Contains(s.Cfg.ReportURL, "{token}")
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
