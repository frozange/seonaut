package crawler_test

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/stjudewashere/seonaut/internal/crawler"
)

func TestAddRequestLeavesExcludedAddressesOut(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	c := crawler.NewCrawler(base, &crawler.Options{CrawlLimit: 10, Exclude: func(u *url.URL) bool {
		return strings.HasPrefix(u.Path, "/blog/")
	}}, &MockClient{})
	article, _ := url.Parse("https://example.com/blog/post/")
	if err := c.AddRequest(&crawler.RequestMessage{URL: article}); !errors.Is(err, crawler.ErrExcluded) {
		t.Errorf("excluded address queued: %v", err)
	}
	page, _ := url.Parse("https://example.com/services/")
	if err := c.AddRequest(&crawler.RequestMessage{URL: page}); err != nil {
		t.Errorf("page refused: %v", err)
	}
}
