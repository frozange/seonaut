// Package netguard keeps the crawler from reaching anything but the public internet.
//
// The check runs at connect time, on the address the socket is about to use, so a
// hostname that resolves to a public address first and to 127.0.0.1 later (DNS
// rebinding) is still refused. Every redirect target goes through the same dialer
// because the crawler queues redirects as new requests.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"syscall"
	"time"
)

// ErrForbiddenAddress is returned when a connection targets a non public address or port.
var ErrForbiddenAddress = errors.New("netguard: destination address is not allowed")

// reserved lists every range that is not routable on the public internet or that
// belongs to infrastructure a crawler has no business talking to.
var reserved = mustPrefixes(
	"0.0.0.0/8",       // "this" network
	"10.0.0.0/8",      // private
	"100.64.0.0/10",   // carrier-grade NAT
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local, cloud metadata endpoints
	"172.16.0.0/12",   // private
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // documentation
	"192.88.99.0/24",  // 6to4 relay anycast
	"192.168.0.0/16",  // private
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // documentation
	"203.0.113.0/24",  // documentation
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reserved, broadcast
	"::/128",          // unspecified
	"::1/128",         // loopback
	"::ffff:0:0/96",   // IPv4-mapped, checked again after unmapping
	"64:ff9b::/96",    // NAT64, embeds an IPv4 address
	"64:ff9b:1::/48",  // local-use NAT64
	"100::/64",        // discard
	"2001::/23",       // IETF protocol assignments (Teredo, ORCHID...)
	"2001:db8::/32",   // documentation
	"2002::/16",       // 6to4, embeds an IPv4 address
	"fc00::/7",        // unique local
	"fe80::/10",       // link-local
	"ff00::/8",        // multicast
)

// Guard decides whether an address may be dialed.
type Guard struct {
	deny  []netip.Prefix
	ports map[int]bool
}

// New builds a Guard. extraDeny adds ranges on top of the reserved ones (for
// example the public addresses of the host running the crawler). allowedPorts
// restricts destination ports; an empty list means 80 and 443.
func New(extraDeny []string, allowedPorts []int) (*Guard, error) {
	g := &Guard{deny: append([]netip.Prefix{}, reserved...), ports: map[int]bool{}}

	for _, s := range extraDeny {
		p, err := parsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("netguard: invalid deny entry %q: %w", s, err)
		}
		g.deny = append(g.deny, p)
	}

	if len(allowedPorts) == 0 {
		allowedPorts = []int{80, 443}
	}
	for _, p := range allowedPorts {
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("netguard: invalid port %d", p)
		}
		g.ports[p] = true
	}

	return g, nil
}

// PortAllowed reports whether the crawler may connect to the destination port.
func (g *Guard) PortAllowed(port int) bool {
	return g.ports[port]
}

// CheckURL validates a URL entered by a user: http or https, an allowed port and
// a host that resolves only to allowed addresses.
func (g *Guard) CheckURL(ctx context.Context, u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrForbiddenAddress
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return ErrForbiddenAddress
		}
		port = n
	}
	if !g.PortAllowed(port) || u.Hostname() == "" {
		return ErrForbiddenAddress
	}
	return g.CheckHost(ctx, u.Hostname())
}

// AllowedAddr reports whether the address may be contacted, ignoring the port.
func (g *Guard) AllowedAddr(a netip.Addr) bool {
	if !a.IsValid() {
		return false
	}
	a = a.Unmap()
	if a.Zone() != "" {
		return false
	}
	for _, p := range g.deny {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// Allowed reports whether a "host:port" address with a literal IP may be dialed.
func (g *Guard) Allowed(address string) bool {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || !g.ports[port] {
		return false
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return g.AllowedAddr(a)
}

// control is called by the dialer after name resolution, right before connect.
func (g *Guard) control(network, address string, _ syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" {
		return ErrForbiddenAddress
	}
	if !g.Allowed(address) {
		return fmt.Errorf("%w: %s", ErrForbiddenAddress, address)
	}
	return nil
}

// DialContext dials through the guard.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   g.control,
	}
	return d.DialContext(ctx, network, address)
}

// Transport returns an http.Transport that only reaches allowed addresses. It
// ignores proxy environment variables so the guard cannot be routed around.
func (g *Guard) Transport() *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           g.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
}

// CheckHost resolves a hostname and reports an error if it has no allowed
// address. It is meant for early, user-facing validation; the dialer remains
// the enforcement point.
func (g *Guard) CheckHost(ctx context.Context, host string) error {
	if a, err := netip.ParseAddr(host); err == nil {
		if !g.AllowedAddr(a) {
			return ErrForbiddenAddress
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if !g.AllowedAddr(a) {
			return ErrForbiddenAddress
		}
	}
	if len(addrs) == 0 {
		return ErrForbiddenAddress
	}
	return nil
}

func parsePrefix(s string) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}

func mustPrefixes(list ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(list))
	for _, s := range list {
		p, err := parsePrefix(s)
		if err != nil {
			panic(err)
		}
		out = append(out, p)
	}
	return out
}
