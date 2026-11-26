package routes

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// French typography: the space before ":" and "»" and after "«" is a
// no-break space, the one before ";", "!", "?" and "%" a narrow no-break space,
// so a punctuation mark never starts a line. Applied to text only: markup,
// attributes, styles and scripts are copied byte for byte.
var (
	frBeforeWide   = regexp.MustCompile(` ([:»])`)
	frAfterWide    = regexp.MustCompile(`« `)
	frBeforeNarrow = regexp.MustCompile(` ([;!?%])`)
)

// googleVocab puts Google's own French words back where the report's AI
// paraphrased them ("titre Google", "plan du site"), despite its writing rules
// (Francis, 2026-11-26: « ça te couterait quoi de prendre le vocabulaire Google
// au lieu d'inventer le tien ? »). Text tokens are raw HTML: "<" is written
// "&lt;" and an apostrophe "&#39;", as the template escapes them.
var googleVocab = []struct {
	re  *regexp.Regexp
	rep string
}{
	{regexp.MustCompile(`plan du site([^()]{0,80}?)\s*\((?:le |un )?sitemap[^)]*\)`), "sitemap$1"},
	{regexp.MustCompile(`plans du site`), "sitemaps"},
	{regexp.MustCompile(`plan du site`), "sitemap"},
	{regexp.MustCompile(`titres Google`), "éléments &lt;title&gt;"},
	{regexp.MustCompile(`\b([Ll])e titre Google`), "${1}&#39;élément &lt;title&gt;"},
	{regexp.MustCompile(`\bdu titre Google`), "de l&#39;élément &lt;title&gt;"},
	{regexp.MustCompile(`\bau titre Google`), "à l&#39;élément &lt;title&gt;"},
	{regexp.MustCompile(`titre Google`), "élément &lt;title&gt;"},
	{regexp.MustCompile(`descriptions? (?:affichées? par|lues? par|données à) Google`), "meta description"},
	{regexp.MustCompile(`descriptions Google`), "meta descriptions"},
	{regexp.MustCompile(`description Google`), "meta description"},
}

func googleWords(s string) string {
	for _, g := range googleVocab {
		s = g.re.ReplaceAllString(s, g.rep)
	}
	return s
}

func frenchText(s string) string {
	s = googleWords(s)
	s = frBeforeWide.ReplaceAllString(s, " $1")
	s = frAfterWide.ReplaceAllString(s, "« ")
	return frBeforeNarrow.ReplaceAllString(s, " $1")
}

func frenchSpacing(in []byte) []byte {
	var out bytes.Buffer
	z := html.NewTokenizer(bytes.NewReader(in))
	raw := 0 // depth inside style or script
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				return out.Bytes()
			}
			return in
		}
		tok := z.Raw()
		switch tt {
		case html.StartTagToken, html.EndTagToken:
			name, _ := z.TagName()
			if n := strings.ToLower(string(name)); n == "style" || n == "script" {
				if tt == html.StartTagToken {
					raw++
				} else if raw > 0 {
					raw--
				}
			}
			out.Write(tok)
		case html.TextToken:
			if raw > 0 {
				out.Write(tok)
			} else {
				out.WriteString(frenchText(string(tok)))
			}
		default:
			out.Write(tok)
		}
	}
}
