package services

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestCountWordsSkipsCode(t *testing.T) {
	doc, _ := html.Parse(strings.NewReader(`<html><body><p>three visible words</p>
		<style>.a { position: fixed; bottom: 20px; }</style><noscript>no script here</noscript></body></html>`))
	body := doc.FirstChild.LastChild // <html> then <body>
	if got := countWords(body); got != 3 {
		t.Errorf("countWords = %d, want 3", got)
	}
}
