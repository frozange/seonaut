package security

import "net/http"

// HeadersOptions configures the response headers added to every reply.
type HeadersOptions struct {
	// NoIndex asks search engines not to index or follow anything on the instance.
	NoIndex bool
	// HTTPS adds Strict-Transport-Security; only set it when the public URL is https.
	HTTPS bool
}

// Headers wraps a handler and sets security headers on every response. It also
// answers /robots.txt with a full disallow when NoIndex is set, so the instance
// stays out of search results even if the static robots.txt changes.
func Headers(next http.Handler, o HeadersOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if o.HTTPS {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if o.NoIndex {
			h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			if r.URL.Path == "/robots.txt" {
				h.Set("Content-Type", "text/plain; charset=utf-8")
				w.Write([]byte("User-agent: *\nDisallow: /\n"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
