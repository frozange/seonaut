package audit

import (
	"net/url"
	"strings"
	"sync"

	"github.com/stjudewashere/seonaut/internal/models"
	"golang.org/x/net/html"
)

// PageSignal is what the audit keeps of one page while the crawler reads it:
// the parts of the HTML the database does not store (structured data, main
// text fingerprint) and the link graph, so the measurements below need no
// second visit of the site.
type PageSignal struct {
	URL         string           `json:"url"`
	Status      int              `json:"status"`
	Redirect    string           `json:"redirect,omitempty"` // absolute target of a redirect
	Canonical   string           `json:"canonical,omitempty"`
	Noindex     bool             `json:"noindex,omitempty"`
	Lang        string           `json:"lang,omitempty"`
	Hreflangs   []string         `json:"hreflangs,omitempty"` // other language versions
	Title       string           `json:"title,omitempty"`
	H1          string           `json:"h1,omitempty"`
	Description string           `json:"description,omitempty"`
	Depth       int              `json:"depth"`
	Words       int              `json:"words"` // words of the main text, menus and footer left out
	Links       []string         `json:"links,omitempty"`
	Shingles    []uint64         `json:"-"`
	Structured  []StructuredItem `json:"structured,omitempty"`
	BadJSONLD   int              `json:"bad_jsonld,omitempty"` // JSON-LD blocks that are not valid JSON
}

// Indexable reports a page Google may show: answered 200, no noindex, and no
// canonical pointing elsewhere.
func (p *PageSignal) Indexable() bool {
	if p.Status != 200 || p.Noindex {
		return false
	}
	return p.Canonical == "" || normalizeURL(p.Canonical) == normalizeURL(p.URL)
}

// Collector receives the pages of the crawls the audit watches. The crawler
// calls ObservePage for every response; pages of other projects are ignored.
type Collector struct {
	mu      sync.Mutex
	watched map[int64][]*PageSignal
	max     int
}

// NewCollector keeps at most max pages per crawl (0: 5000).
func NewCollector(max int) *Collector {
	if max <= 0 {
		max = 5000
	}
	return &Collector{watched: map[int64][]*PageSignal{}, max: max}
}

// Watch starts collecting the pages of a project.
func (c *Collector) Watch(projectID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watched[projectID] = []*PageSignal{}
}

// Take returns the pages collected for a project and stops watching it.
func (c *Collector) Take(projectID int64) []*PageSignal {
	c.mu.Lock()
	defer c.mu.Unlock()
	pages := c.watched[projectID]
	delete(c.watched, projectID)
	return pages
}

// ObservePage records one crawled response: HTML pages answered 200 and
// redirects. It is safe to call from several crawls at once.
func (c *Collector) ObservePage(projectID int64, pr *models.PageReport, doc *html.Node) {
	if c == nil || pr == nil {
		return
	}
	c.mu.Lock()
	pages, ok := c.watched[projectID]
	full := ok && len(pages) >= c.max
	c.mu.Unlock()
	if !ok || full {
		return
	}
	sig := newPageSignal(pr, doc)
	if sig == nil {
		return
	}
	c.mu.Lock()
	if p, ok := c.watched[projectID]; ok && len(p) < c.max {
		c.watched[projectID] = append(p, sig)
	}
	c.mu.Unlock()
}

func newPageSignal(pr *models.PageReport, doc *html.Node) *PageSignal {
	base := pr.ParsedURL
	if base == nil {
		u, err := url.Parse(pr.URL)
		if err != nil {
			return nil
		}
		base = u
	}
	sig := &PageSignal{URL: normalizeURL(pr.URL), Status: pr.StatusCode, Depth: pr.Depth}
	if pr.StatusCode >= 300 && pr.StatusCode < 400 {
		if pr.RedirectURL == "" {
			return nil
		}
		if t, err := base.Parse(pr.RedirectURL); err == nil {
			sig.Redirect = normalizeURL(t.String())
		}
		return sig
	}
	if pr.StatusCode != 200 || pr.MediaType != "text/html" || doc == nil {
		return nil
	}
	sig.Canonical, sig.Noindex, sig.Lang = pr.Canonical, pr.Noindex, pr.Lang
	sig.Title, sig.H1, sig.Description = pr.Title, pr.H1, pr.Description
	for _, h := range pr.Hreflangs {
		if n := normalizeURL(h.URL); n != sig.URL {
			sig.Hreflangs = append(sig.Hreflangs, n)
		}
	}
	if !pr.Nofollow {
		seen := map[string]bool{}
		for _, l := range pr.Links {
			if l.NoFollow || l.ParsedURL == nil {
				continue
			}
			if s := l.ParsedURL.Scheme; s != "http" && s != "https" {
				continue
			}
			n := normalizeURL(l.ParsedURL.String())
			if n != sig.URL && !seen[n] {
				seen[n] = true
				sig.Links = append(sig.Links, n)
			}
		}
	}
	text, _ := extractText(doc)
	sig.Words = len(strings.Fields(text))
	sig.Shingles = Shingles(text)
	sig.Structured, sig.BadJSONLD = ExtractStructured(doc, sig.URL)
	return sig
}

// normalizeURL drops the fragment so "/a#top" and "/a" are one page.
func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Fragment, u.RawFragment = "", ""
	u.Host = strings.ToLower(u.Host)
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
