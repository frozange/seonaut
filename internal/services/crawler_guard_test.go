package services

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/netguard"
)

// The crawler client must refuse an internal service even when its port is allowed.
func TestCrawlerClientRefusesInternalAddresses(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("internal"))
	}))
	defer internal.Close()

	_, portStr, _ := net.SplitHostPort(internal.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	client, err := newCrawlerHTTPClient(&config.CrawlerConfig{AllowedPorts: []int{port}})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(internal.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("crawler client reached a loopback service")
	}
	if !errors.Is(err, netguard.ErrForbiddenAddress) {
		t.Fatalf("expected ErrForbiddenAddress, got %v", err)
	}
}

// Redirects are never followed by the client, so a public page cannot bounce the
// crawler to an internal address within a single request.
func TestCrawlerClientDoesNotFollowRedirects(t *testing.T) {
	client, err := newCrawlerHTTPClient(&config.CrawlerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if client.CheckRedirect(req, nil) != http.ErrUseLastResponse {
		t.Error("crawler client must not follow redirects")
	}
}

func TestCrawlerClientRejectsInvalidDenyList(t *testing.T) {
	if _, err := newCrawlerHTTPClient(&config.CrawlerConfig{DenyAddresses: []string{"nope"}}); err == nil {
		t.Error("an invalid deny list must stop the crawler from starting")
	}
}
