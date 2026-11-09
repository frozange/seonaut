// Package security holds the HTTP-level protections of a public SEOnaut instance:
// client IP resolution behind a reverse proxy, rate limiting, Cloudflare Turnstile
// verification and response headers.
package security

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIPResolver returns the address of the visitor. A forwarded header is only
// honoured when the request comes from a trusted proxy, otherwise anyone could
// pick their own address and walk around every per-IP limit.
type ClientIPResolver struct {
	trusted []netip.Prefix
	header  string
}

// NewClientIPResolver builds a resolver. trusted lists the proxies (IPs or CIDR)
// allowed to set header; header is typically "CF-Connecting-IP" or "X-Forwarded-For".
// With no trusted proxy the socket address is always used.
func NewClientIPResolver(trusted []string, header string) (*ClientIPResolver, error) {
	r := &ClientIPResolver{header: header}
	for _, s := range trusted {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			a, aerr := netip.ParseAddr(s)
			if aerr != nil {
				return nil, err
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		r.trusted = append(r.trusted, p.Masked())
	}
	return r, nil
}

// IP returns the client address as a string. It never returns an empty string:
// an unparsable remote address yields "unknown", which still gets rate limited.
func (r *ClientIPResolver) IP(req *http.Request) string {
	remote := socketAddr(req.RemoteAddr)
	if !remote.IsValid() {
		return "unknown"
	}
	if r.header == "" || !r.isTrusted(remote) {
		return remote.String()
	}

	value := strings.TrimSpace(req.Header.Get(r.header))
	if value == "" {
		return remote.String()
	}
	// X-Forwarded-For is a list: the right-most entry not added by a trusted
	// proxy is the one we can believe.
	parts := strings.Split(value, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			return remote.String()
		}
		a = a.Unmap()
		if i > 0 && r.isTrusted(a) {
			continue
		}
		return a.String()
	}
	return remote.String()
}

func (r *ClientIPResolver) isTrusted(a netip.Addr) bool {
	for _, p := range r.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func socketAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}
