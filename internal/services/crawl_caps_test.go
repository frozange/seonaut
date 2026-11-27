package services_test

import (
	"testing"

	"github.com/stjudewashere/seonaut/internal/services"
)

// An audit capped at 500 pages must not stop at 500 URLs (sel-tech.fr,
// 2026-11-27: 46 pages plus their images hit the cap).
func TestCrawlCaps(t *testing.T) {
	cases := []struct{ instance, project, urls, pages int }{
		{20000, 500, 500 * services.ResourcesPerPage, 500},
		{5000, 500, 5000, 500},
		{20000, 0, 20000, 0},
		{300, 500, 300, 0},
	}
	for _, c := range cases {
		u, p := services.CrawlCaps(c.instance, c.project)
		if u != c.urls || p != c.pages {
			t.Errorf("CrawlCaps(%d, %d) = %d, %d; want %d, %d", c.instance, c.project, u, p, c.urls, c.pages)
		}
	}
}
