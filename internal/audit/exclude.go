package audit

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Exclusion lists the addresses of a site that its audit leaves out, at the
// owner's request (e.g. the blog of lafactory.com, whose articles share the
// root path of its pages). It lives in <exclude_dir>/<site slug>.txt, one
// entry per line, "#" for a comment:
//
//	https://example.com/some-article/   that address (query and fragment ignored)
//	/some-article/                      the same, on the audited site
//	/blog/*                             every address under /blog/
//
// Hosts are compared without a leading "www.", paths without a trailing "/".
type Exclusion struct {
	exact    map[string]bool
	prefixes []string
	entries  int
}

// LoadExclusion reads the list of the site; no file gives nil, which excludes
// nothing.
func LoadExclusion(dir, siteURL string) (*Exclusion, error) {
	if dir == "" {
		return nil, nil
	}
	site, err := url.Parse(siteURL)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, Slug(siteURL)+".txt"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ex := &Exclusion{exact: map[string]bool{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix := strings.HasSuffix(line, "*")
		line = strings.TrimSuffix(line, "*")
		u, err := url.Parse(line)
		if err != nil {
			continue
		}
		if u.Host == "" {
			u.Host = site.Host
		}
		key := exclusionKey(u)
		if prefix {
			ex.prefixes = append(ex.prefixes, key)
		} else {
			ex.exact[key] = true
		}
		ex.entries++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if ex.entries == 0 {
		return nil, nil
	}
	return ex, nil
}

func exclusionKey(u *url.URL) string {
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	return host + strings.TrimSuffix(u.EscapedPath(), "/")
}

// Match tells whether the address is left out.
func (e *Exclusion) Match(u *url.URL) bool {
	if e == nil || u == nil {
		return false
	}
	key := exclusionKey(u)
	if e.exact[key] {
		return true
	}
	for _, p := range e.prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// Len is the number of entries of the list.
func (e *Exclusion) Len() int {
	if e == nil {
		return 0
	}
	return e.entries
}
