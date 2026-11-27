package urlutils

import (
	"net/url"
	"strings"
)

// SameURL reports whether two absolute URLs name the same resource once
// percent-encoding is normalised: "/5000m²/", "/5000m%c2%b2/" and
// "/5000m%C2%B2/" are one address (RFC 3986 section 6.2.2, RFC 3987 for the
// raw form). Scheme and host compare case-insensitively; path and query keep
// their case; the fragment is ignored.
func SameURL(a, b string) bool {
	if a == b {
		return true
	}
	ua, err := url.Parse(strings.TrimSpace(a))
	if err != nil {
		return false
	}
	ub, err := url.Parse(strings.TrimSpace(b))
	if err != nil {
		return false
	}
	return normalized(ua) == normalized(ub)
}

// Normalize returns the form of raw that SameURL compares, or raw itself when
// it does not parse.
func Normalize(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return normalized(u)
}

func normalized(u *url.URL) string {
	c := *u
	c.Scheme = strings.ToLower(c.Scheme)
	c.Host = strings.ToLower(c.Host)
	path := normalizeEscapes(c.EscapedPath())
	c.Path, c.RawPath = "", ""
	c.Fragment, c.RawFragment = "", ""
	if path == "" {
		path = "/"
	}
	if q, err := url.ParseQuery(c.RawQuery); err == nil && c.RawQuery != "" {
		// Re-encode the query only when it decodes cleanly; key order is kept
		// by sorting, as url.Values.Encode does.
		c.RawQuery = q.Encode()
	}
	c.Opaque = "//" + c.Host + path
	return c.String()
}

// normalizeEscapes applies RFC 3986 section 6.2.2: hexadecimal digits of a
// percent-encoding are upper-cased and encoded unreserved characters are
// decoded. Reserved characters such as an encoded "/" stay encoded.
func normalizeEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			v := unhex(s[i+1])<<4 | unhex(s[i+2])
			if isUnreserved(v) {
				b.WriteByte(v)
			} else {
				b.WriteByte('%')
				b.WriteString(strings.ToUpper(s[i+1 : i+3]))
			}
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}

func isUnreserved(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}
