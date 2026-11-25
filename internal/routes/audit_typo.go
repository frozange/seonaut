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

func frenchText(s string) string {
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
