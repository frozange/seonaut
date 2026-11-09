package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func newGuard(t *testing.T, deny []string, ports []int) *Guard {
	t.Helper()
	g, err := New(deny, ports)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAllowedAddresses(t *testing.T) {
	g := newGuard(t, []string{"178.63.0.108", "2a01:4f8::/32"}, nil)

	cases := map[string]bool{
		"8.8.8.8:443":                     true,
		"104.21.29.120:80":                true,
		"[2606:4700:3031::6815:1d78]:443": true,
		"127.0.0.1:443":                   false,
		"127.8.9.1:80":                    false,
		"10.1.2.3:443":                    false,
		"172.16.0.1:443":                  false,
		"172.31.255.255:443":              false,
		"192.168.1.1:80":                  false,
		"169.254.169.254:80":              false,
		"100.64.0.1:443":                  false,
		"0.0.0.0:80":                      false,
		"224.0.0.1:80":                    false,
		"255.255.255.255:80":              false,
		"178.63.0.108:443":                false,
		"[::1]:443":                       false,
		"[::ffff:127.0.0.1]:443":          false,
		"[::ffff:10.0.0.1]:80":            false,
		"[64:ff9b::7f00:1]:80":            false,
		"[2002:7f00:1::1]:80":             false,
		"[fd00::1]:443":                   false,
		"[fe80::1]:443":                   false,
		"[2a01:4f8:1:2::3]:443":           false,
		"8.8.8.8:22":                      false,
		"8.8.8.8:3306":                    false,
		"8.8.8.8:8090":                    false,
		"example.com:443":                 false,
		"garbage":                         false,
	}

	for addr, want := range cases {
		if got := g.Allowed(addr); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestCustomPorts(t *testing.T) {
	g := newGuard(t, nil, []int{443, 8443})
	if g.Allowed("8.8.8.8:80") {
		t.Error("port 80 must be refused when not listed")
	}
	if !g.Allowed("8.8.8.8:8443") {
		t.Error("listed port 8443 must be allowed")
	}
}

func TestInvalidConfig(t *testing.T) {
	if _, err := New([]string{"not-an-ip"}, nil); err == nil {
		t.Error("invalid deny entry must fail")
	}
	if _, err := New(nil, []int{0}); err == nil {
		t.Error("invalid port must fail")
	}
}

func TestZonedAddressRefused(t *testing.T) {
	g := newGuard(t, nil, nil)
	if g.AllowedAddr(netip.MustParseAddr("2606:4700::1%eth0")) {
		t.Error("zoned address must be refused")
	}
}

// The real transport must refuse a loopback server even though it is reachable.
func TestTransportRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("internal"))
	}))
	defer srv.Close()

	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	var p int
	for _, c := range port {
		p = p*10 + int(c-'0')
	}

	g := newGuard(t, nil, []int{p})
	client := &http.Client{Transport: g.Transport()}

	resp, err := client.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("request to loopback succeeded, guard is not applied")
	}
	if !errors.Is(err, ErrForbiddenAddress) {
		t.Fatalf("expected ErrForbiddenAddress, got %v", err)
	}

	// "localhost" goes through resolution and must be refused too.
	resp, err = client.Get("http://localhost:" + port + "/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("request to localhost succeeded")
	}
}

func TestCheckHost(t *testing.T) {
	g := newGuard(t, nil, nil)
	ctx := context.Background()
	if err := g.CheckHost(ctx, "127.0.0.1"); !errors.Is(err, ErrForbiddenAddress) {
		t.Errorf("CheckHost(127.0.0.1) = %v", err)
	}
	if err := g.CheckHost(ctx, "localhost"); err == nil {
		t.Error("CheckHost(localhost) must fail")
	}
	if err := g.CheckHost(ctx, "8.8.8.8"); err != nil {
		t.Errorf("CheckHost(8.8.8.8) = %v", err)
	}
}
