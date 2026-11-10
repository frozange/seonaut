package audit

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// CrawlReader reads what the SEOnaut crawler stored for one crawl.
type CrawlReader struct {
	DB *sql.DB
}

// Facts returns the crawl level measurements.
func (r *CrawlReader) Facts(crawlID int64) (SiteFacts, error) {
	var f SiteFacts
	var start, end sql.NullTime
	err := r.DB.QueryRow(`
		SELECT total_urls, blocked_by_robotstxt, noindex, robotstxt_exists, sitemap_exists,
			sitemap_blocked, critical_issues, alert_issues, warning_issues, start, end
		FROM crawls WHERE id = ?`, crawlID).Scan(&f.PagesCrawled, &f.BlockedByRobots, &f.Noindex,
		&f.RobotstxtExists, &f.SitemapExists, &f.SitemapIsBlocked, &f.CriticalIssues,
		&f.AlertIssues, &f.WarningIssues, &start, &end)
	if err != nil {
		return f, err
	}
	if start.Valid && end.Valid {
		f.DurationSeconds = int(end.Time.Sub(start.Time).Seconds())
	}
	return f, nil
}

// Issues returns every issue type found, with the number of pages affected and
// up to examples URLs each, most widespread first within each priority.
func (r *CrawlReader) Issues(crawlID int64, examples int) ([]IssueSummary, int, error) {
	rows, err := r.DB.Query(`
		SELECT issue_types.type, issue_types.priority, COUNT(DISTINCT issues.pagereport_id) AS c
		FROM issues
		INNER JOIN issue_types ON issue_types.id = issues.issue_type_id
		WHERE issues.crawl_id = ?
		GROUP BY issue_types.id, issue_types.type, issue_types.priority
		ORDER BY issue_types.priority ASC, c DESC`, crawlID)
	if err != nil {
		return nil, 0, err
	}
	var out []IssueSummary
	for rows.Next() {
		var s IssueSummary
		if err := rows.Scan(&s.Type, &s.Priority, &s.Pages); err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, s)
	}
	rows.Close()

	for i := range out {
		ex, err := r.DB.Query(`
			SELECT DISTINCT pagereports.url
			FROM issues
			INNER JOIN issue_types ON issue_types.id = issues.issue_type_id
			INNER JOIN pagereports ON pagereports.id = issues.pagereport_id
			WHERE issues.crawl_id = ? AND issue_types.type = ?
			ORDER BY pagereports.depth ASC, pagereports.url ASC
			LIMIT ?`, crawlID, out[i].Type, examples)
		if err != nil {
			return nil, 0, err
		}
		for ex.Next() {
			var u string
			if ex.Scan(&u) == nil {
				out[i].Examples = append(out[i].Examples, u)
			}
		}
		ex.Close()
	}

	var total int
	if err := r.DB.QueryRow(`SELECT COUNT(*) FROM issue_types`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total - len(out), nil
}

// blogPath matches URLs that belong to a blog, an archive or a listing: they are
// sampled, never treated as the main pages of the site.
var blogPath = regexp.MustCompile(`(?i)/(blog|news|actualites?|articles?|category|categorie|tag|author|auteur|page|feed|20\d\d)(/|$)`)

// KeyPages returns the main pages of the site: the home page first, then the
// shallowest, most linked HTML pages that are not blog posts or listings.
func (r *CrawlReader) KeyPages(crawlID int64, home string, max int) ([]PageFacts, error) {
	rows, err := r.DB.Query(`
		SELECT p.url, p.depth, p.title, p.description, p.h1, p.words, p.lang,
			(SELECT COUNT(*) FROM links l WHERE l.crawl_id = p.crawl_id AND l.url_hash = p.url_hash) AS inlinks
		FROM pagereports p
		WHERE p.crawl_id = ? AND p.crawled = 1 AND p.status_code = 200
			AND p.media_type = 'text/html' AND p.noindex = 0 AND p.depth <= 3
		ORDER BY p.depth ASC, inlinks DESC
		LIMIT 400`, crawlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []PageFacts
	for rows.Next() {
		var p PageFacts
		if err := rows.Scan(&p.URL, &p.Depth, &p.Title, &p.Description, &p.H1, &p.Words, &p.Lang, &p.Inlinks); err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	return pickKeyPages(all, home, max), rows.Err()
}

func pickKeyPages(all []PageFacts, home string, max int) []PageFacts {
	homeNorm := strings.TrimRight(home, "/")
	var out, rest []PageFacts
	for _, p := range all {
		if strings.TrimRight(p.URL, "/") == homeNorm {
			out = append([]PageFacts{p}, out...)
			continue
		}
		u, err := url.Parse(p.URL)
		if err != nil || blogPath.MatchString(u.Path) || u.RawQuery != "" {
			continue
		}
		rest = append(rest, p)
	}
	sort.SliceStable(rest, func(i, j int) bool {
		if rest[i].Depth != rest[j].Depth {
			return rest[i].Depth < rest[j].Depth
		}
		return rest[i].Inlinks > rest[j].Inlinks
	})
	for _, p := range rest {
		if len(out) >= max {
			break
		}
		out = append(out, p)
	}
	return out
}

// PageContent is what a fresh fetch of a page gives the AI.
type PageContent struct {
	Text     string   // visible main text, without navigation, header, footer and forms
	Headings []string // h1 to h4, in order
	Footer   string   // footer text: legal notice, address and phone usually live there
	Actions  []string // buttons and button-like links, header included
	Contacts []string // tel: and mailto: links
}

// PageText fetches a page through the guarded client and returns its content.
func PageText(ctx context.Context, client *http.Client, userAgent, pageURL string, maxWords int) (*PageContent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	pc := &PageContent{}
	pc.Text, pc.Headings = extractText(doc)
	pc.Text = firstWords(pc.Text, maxWords)
	pc.Footer, pc.Actions, pc.Contacts = extractChrome(doc)
	return pc, nil
}

func firstWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}

var buttonClass = regexp.MustCompile(`(?i)(^|[\s_-])(btn|button|cta)([\s_-]|$)`)

// extractChrome reads what extractText leaves out on purpose: the footer, the
// calls to action (often in the header or the hero) and the contact links.
func extractChrome(doc *html.Node) (footer string, actions, contacts []string) {
	var footers []string
	seen := map[string]bool{}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg", "template":
				return
			case "footer":
				footers = append(footers, strings.Join(strings.Fields(nodeText(n)), " "))
			case "a", "button":
				text := strings.Join(strings.Fields(nodeText(n)), " ")
				href := attr(n, "href")
				lower := strings.ToLower(href)
				if (strings.HasPrefix(lower, "tel:") || strings.HasPrefix(lower, "mailto:")) && !seen[lower] && len(contacts) < 8 {
					seen[lower] = true
					contacts = append(contacts, href)
				}
				isButton := n.Data == "button" || attr(n, "role") == "button" || buttonClass.MatchString(attr(n, "class"))
				key := text + "|" + href
				if isButton && text != "" && len(text) < 80 && !seen[key] && len(actions) < 15 {
					seen[key] = true
					if href != "" {
						text += " -> " + href
					}
					actions = append(actions, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return firstWords(strings.Join(footers, " "), 250), actions, contacts
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

var skipText = map[string]bool{
	"script": true, "style": true, "noscript": true, "svg": true, "template": true,
	"nav": true, "header": true, "footer": true, "form": true, "iframe": true, "head": true,
}

func extractText(doc *html.Node) (string, []string) {
	var b strings.Builder
	var headings []string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipText[n.Data] {
				return
			}
			if len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '4' {
				t := strings.Join(strings.Fields(nodeText(n)), " ")
				if t != "" && len(headings) < 40 {
					headings = append(headings, strings.ToUpper(n.Data)+" "+t)
				}
			}
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				b.WriteString(t)
				b.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.Join(strings.Fields(b.String()), " "), headings
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// BuildEvidence numbers the facts the AI may rely on: E1, E2... issueText gives
// the name and the description of an issue type, so the AI knows exactly what
// each check measures.
func BuildEvidence(site SiteFacts, pages []PageFacts, issues []IssueSummary, issueText func(string) (string, string)) []EvidenceEntry {
	var out []EvidenceEntry
	add := func(label string, data interface{}) {
		out = append(out, EvidenceEntry{ID: fmt.Sprintf("E%d", len(out)+1), Label: label, Data: data})
	}

	add("Crawl coverage", map[string]interface{}{
		"pages_crawled": site.PagesCrawled, "crawl_limit": site.CrawlLimit, "limit_reached": site.LimitReached,
		"robots_txt_found": site.RobotstxtExists, "sitemap_found": site.SitemapExists,
		"pages_blocked_by_robots_txt": site.BlockedByRobots, "noindex_pages": site.Noindex,
	})
	for i, p := range pages {
		label := "Key page: " + p.URL
		if i == 0 {
			label = "Home page: " + p.URL
		}
		data := map[string]interface{}{
			"title": p.Title, "meta_description": p.Description, "h1": p.H1,
			"word_count_whole_html_including_menus_and_footer": p.Words,
			"language_declared": p.Lang, "click_depth": p.Depth, "internal_links_to_it": p.Inlinks,
			"headings":                    p.Headings,
			"buttons_and_calls_to_action": p.Actions,
			"phone_and_email_links":       p.Contacts,
		}
		if i == 0 {
			data["footer_text"] = p.Footer
		}
		limit := 350
		if i == 0 {
			limit = 1500
		}
		words := strings.Fields(p.Text)
		if len(words) > limit {
			words = words[:limit]
		}
		data["main_text_excerpt_without_menus_and_footer"] = strings.Join(words, " ")
		add(label, data)
	}
	var measured []map[string]interface{}
	for _, is := range issues {
		name, what := issueText(is.Type)
		measured = append(measured, map[string]interface{}{
			"issue": name, "what_it_checks": what, "priority": []string{"", "critical", "alert", "warning"}[clamp(is.Priority, 1, 3)],
			"pages_affected": is.Pages, "examples": is.Examples,
		})
	}
	add("Technical issues measured by the crawler", measured)
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
