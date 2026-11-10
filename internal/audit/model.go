// Package audit runs free on-demand audits: a visitor submits a URL, the site is
// crawled with the regular SEOnaut crawler, an evidence pack is built from what
// the crawler stored, an AI interprets it under strict evidence rules, and the
// result is published as a private report page.
//
// The code measures, the AI interprets. Every AI statement must cite evidence
// ids from the pack, numbers absent from the pack are removed, and a second AI
// pass reviews the text against the evidence.
package audit

import (
	"time"
)

// Status of an audit request.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusCrawling  Status = "crawling"
	StatusAnalysing Status = "analysing"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
)

// Audit is one request and, once done, its result.
type Audit struct {
	ID        int64
	Token     string
	URL       string
	Email     string
	Lang      string
	IP        string
	Status    Status
	Error     string
	ProjectID int64
	CrawlID   int64
	Result    *Result
	Created   time.Time
	Updated   time.Time
}

// Result is everything the report page needs. It is stored as JSON.
type Result struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Site        SiteFacts       `json:"site"`
	Scores      Scores          `json:"scores"`
	Issues      []IssueSummary  `json:"issues"`
	PassedCount int             `json:"passed_count"`
	KeyPages    []PageFacts     `json:"key_pages"`
	Priorities  []Priority      `json:"priorities"`
	AI          *AIReport       `json:"ai,omitempty"`
	AIError     string          `json:"ai_error,omitempty"`
	Evidence    []EvidenceEntry `json:"evidence"`
}

// SiteFacts are measured by the crawler.
type SiteFacts struct {
	URL              string `json:"url"`
	Host             string `json:"host"`
	PagesCrawled     int    `json:"pages_crawled"`
	CrawlLimit       int    `json:"crawl_limit"`
	LimitReached     bool   `json:"limit_reached"`
	RobotstxtExists  bool   `json:"robotstxt_exists"`
	SitemapExists    bool   `json:"sitemap_exists"`
	SitemapIsBlocked bool   `json:"sitemap_is_blocked"`
	BlockedByRobots  int    `json:"blocked_by_robots"`
	Noindex          int    `json:"noindex"`
	CriticalIssues   int    `json:"critical_issues"`
	AlertIssues      int    `json:"alert_issues"`
	WarningIssues    int    `json:"warning_issues"`
	DurationSeconds  int    `json:"duration_seconds"`
}

// IssueSummary is one issue type found by the SEOnaut reporters.
type IssueSummary struct {
	Type     string   `json:"type"`
	Priority int      `json:"priority"` // 1 critical, 2 alert, 3 warning
	Pages    int      `json:"pages"`
	Examples []string `json:"examples"`
}

// PageFacts describe a key page of the site, from the crawl and a fresh fetch.
type PageFacts struct {
	URL         string   `json:"url"`
	Depth       int      `json:"depth"`
	Inlinks     int      `json:"inlinks"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	H1          string   `json:"h1"`
	Words       int      `json:"words"`
	Lang        string   `json:"lang"`
	Headings    []string `json:"headings,omitempty"`
	Text        string   `json:"-"`
}

// Scores are computed from a fixed, published scale (see scoring.go).
type Scores struct {
	Technical   float64  `json:"technical"`
	Copywriting *float64 `json:"copywriting,omitempty"`
	Content     *float64 `json:"content,omitempty"`
}

// Priority is an issue the report asks to fix first, technical or editorial.
type Priority struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"` // "technical" or "editorial"
	Level    int      `json:"level"`  // 1 first, 2 strong potential, 3 optimisation
	Title    string   `json:"title"`  // translation key for technical, text for editorial
	Pages    int      `json:"pages,omitempty"`
	Impact   string   `json:"impact,omitempty"`
	Action   string   `json:"action,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// EvidenceEntry is one numbered element of the evidence pack.
type EvidenceEntry struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Data  interface{} `json:"data"`
}

// AIReport is the interpretation written by the AI, after validation.
type AIReport struct {
	Business      Business      `json:"business"`
	Criteria      []Criterion   `json:"criteria"`
	Strengths     []Strength    `json:"strengths"`
	TargetQueries []TargetQuery `json:"target_queries"`
	Summary       string        `json:"summary"`
	Rewrites      []Rewrite     `json:"rewrites"`
	Conclusion    string        `json:"conclusion"`
	Corrections   int           `json:"corrections"`
}

type Business struct {
	Offer      string   `json:"offer"`
	Audience   string   `json:"audience"`
	Area       string   `json:"area"`
	MultiTrade bool     `json:"multi_trade"`
	Services   []string `json:"services"`
	BrandTerms []string `json:"brand_terms"`
	Evidence   []string `json:"evidence"`
}

type Criterion struct {
	ID       string   `json:"id"`
	Group    string   `json:"group"` // "copywriting" or "content"
	Question string   `json:"question"`
	Rating   string   `json:"rating"` // good, partial, poor
	Verdict  string   `json:"verdict"`
	Why      string   `json:"why"`
	Detail   string   `json:"detail"`
	Severity string   `json:"severity"` // high, medium, low
	Evidence []string `json:"evidence"`
}

type Strength struct {
	Text     string   `json:"text"`
	Evidence []string `json:"evidence"`
}

type TargetQuery struct {
	Page      string   `json:"page"`
	Query     string   `json:"query"`
	Alignment string   `json:"alignment"` // good, partial, weak
	Comment   string   `json:"comment"`
	Evidence  []string `json:"evidence"`
}

type Rewrite struct {
	Element string `json:"element"` // title, h1, hook, button, meta_description
	Before  string `json:"before"`
	After   string `json:"after"`
	Why     string `json:"why"`
}
