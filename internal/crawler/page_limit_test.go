package crawler_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stjudewashere/seonaut/internal/crawler"
)

// imageSiteClient serves /p1/ to /pN/, each page with three images and a link
// to the next page. robots.txt and sitemap.xml answer 404.
type imageSiteClient struct{}

func (imageSiteClient) Head(u string) (*crawler.ClientResponse, error) {
	return imageSiteClient{}.Get(u)
}

func (imageSiteClient) Get(u string) (*crawler.ClientResponse, error) {
	r := &http.Response{StatusCode: 200, Header: http.Header{}}
	switch {
	case strings.HasSuffix(u, ".webp"):
		r.Header.Set("Content-Type", "image/webp")
	case strings.Contains(u, "/p"):
		r.Header.Set("Content-Type", "text/html; charset=UTF-8")
	default:
		r.StatusCode = 404
		r.Header.Set("Content-Type", "text/plain")
	}
	r.Body = io.NopCloser(bytes.NewBufferString(""))
	return &crawler.ClientResponse{Response: r}, nil
}

func (imageSiteClient) GetUAName() string { return "SEOnautBot" }

// crawlImageSite crawls a site of `pages` pages and returns the final status.
func crawlImageSite(t *testing.T, pages int, opts *crawler.Options) crawler.Status {
	t.Helper()
	base, _ := url.Parse("https://example.com/p1/")
	c := crawler.NewCrawler(base, opts, imageSiteClient{})
	var mu sync.Mutex
	c.OnResponse(func(rm *crawler.ResponseMessage) {
		mu.Lock()
		defer mu.Unlock()
		p := rm.URL.Path
		if !strings.HasPrefix(p, "/p") || strings.HasSuffix(p, ".webp") {
			return
		}
		n := 0
		for _, ch := range strings.Trim(p, "/p") {
			n = n*10 + int(ch-'0')
		}
		for _, img := range []string{"a", "b", "c"} {
			u, _ := url.Parse("https://example.com" + p + img + ".webp")
			_ = c.AddRequest(&crawler.RequestMessage{URL: u})
		}
		if n < pages {
			u, _ := url.Parse("https://example.com/p" + string(rune('0'+n+1)) + "/")
			_ = c.AddRequest(&crawler.RequestMessage{URL: u})
		}
	})
	if err := c.AddRequest(&crawler.RequestMessage{URL: base}); err != nil {
		t.Fatal(err)
	}
	c.Start()
	return c.GetStatus()
}

// Images must not eat an audit's page budget (sel-tech.fr, 2026-11-27: 46
// pages and their images made 500 URLs, reported as 500 pages, limit reached).
func TestPageLimitCountsHTMLPagesOnly(t *testing.T) {
	st := crawlImageSite(t, 3, &crawler.Options{CrawlLimit: 100, PageLimit: 3})
	// The crawl ends on the third page, before its own images are fetched.
	if st.Pages != 3 || st.Crawled < 7 {
		t.Errorf("pages %d, urls %d; want 3 pages read past at least 4 images", st.Pages, st.Crawled)
	}
}

func TestPageLimitEndsTheCrawl(t *testing.T) {
	st := crawlImageSite(t, 5, &crawler.Options{CrawlLimit: 100, PageLimit: 2})
	if st.Pages != 2 {
		t.Errorf("pages %d, want the crawl to stop at 2", st.Pages)
	}
}
