package audit

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/astappiev/microdata"
	"golang.org/x/net/html"
)

// StructuredItem is one top-level structured data item of a page (JSON-LD or
// microdata) with what the validation found in it and in its nested items.
type StructuredItem struct {
	Type   string    `json:"type"`
	Issues []SDIssue `json:"issues,omitempty"`
}

// SDIssue is one finding of the validation: the type that carries the field,
// the field (dotted path from that type), the kind of problem and its weight.
// Severity "error" means Google cannot show the rich result; "warning" means
// a field Google recommends is missing or malformed.
type SDIssue struct {
	Type     string `json:"type"`
	Field    string `json:"field"`
	Kind     string `json:"kind"` // missing, invalid, range, few_items
	Severity string `json:"severity"`
}

// ExtractStructured reads the JSON-LD and microdata items of a page with
// astappiev/microdata (BSD-2-Clause, see THIRD_PARTY_NOTICES), validates each
// against Google's rich result rules (schema_rules.go) and counts the JSON-LD
// blocks that are not valid JSON.
func ExtractStructured(doc *html.Node, pageURL string) ([]StructuredItem, int) {
	if doc == nil {
		return nil, 0
	}
	bad := countBadJSONLD(doc)
	md, err := microdata.ParseNode(doc, pageURL)
	if err != nil || md == nil {
		return nil, bad
	}
	var out []StructuredItem
	for _, it := range rootItems(md.Items) {
		t := mainType(it)
		if t == "" {
			continue
		}
		var issues []SDIssue
		validateItem(it, []sdStep{{Type: t}}, &issues)
		out = append(out, StructuredItem{Type: t, Issues: dedupeIssues(issues)})
		if len(out) >= 40 {
			break
		}
	}
	return out, bad
}

// rootItems flattens JSON-LD @graph containers into their items.
func rootItems(items []*microdata.Item) []*microdata.Item {
	var out []*microdata.Item
	for _, it := range items {
		if it == nil {
			continue
		}
		if graph, ok := it.GetNested("@graph"); ok && len(it.Types) == 0 {
			out = append(out, rootItems(graph.Items)...)
			continue
		}
		out = append(out, it)
	}
	return out
}

// schemaType drops the vocabulary prefix: "https://schema.org/Product",
// "schema:Product" and "Product" are the same type.
func schemaType(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.LastIndexAny(t, "/:#"); i >= 0 {
		t = t[i+1:]
	}
	return t
}

func itemTypes(it *microdata.Item) []string {
	var out []string
	for _, t := range it.Types {
		if s := schemaType(t); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mainType is the first type of an item, the one the report groups it by.
func mainType(it *microdata.Item) string {
	if t := itemTypes(it); len(t) > 0 {
		return t[0]
	}
	return ""
}

func countBadJSONLD(doc *html.Node) int {
	bad := 0
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			t := strings.ToLower(attr(n, "type"))
			if i := strings.IndexByte(t, ';'); i >= 0 {
				t = t[:i]
			}
			if strings.TrimSpace(t) == "application/ld+json" {
				var b strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					b.WriteString(c.Data)
				}
				body := strings.TrimSpace(b.String())
				body = strings.TrimSuffix(strings.TrimPrefix(body, "<!--"), "-->")
				if strings.TrimSpace(body) != "" && !json.Valid([]byte(body)) {
					bad++
				}
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return bad
}

func dedupeIssues(in []SDIssue) []SDIssue {
	seen := map[SDIssue]bool{}
	var out []SDIssue
	for _, is := range in {
		if !seen[is] {
			seen[is] = true
			out = append(out, is)
		}
	}
	return out
}

// value returns the first value at a dotted path ("offers.price"), following
// nested items.
func value(it *microdata.Item, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	cur := it
	for i, p := range parts {
		if cur == nil {
			return nil, false
		}
		vals, ok := cur.Properties[p]
		if !ok || len(vals) == 0 {
			return nil, false
		}
		if i == len(parts)-1 {
			return vals[0], true
		}
		next, ok := vals[0].(*microdata.Item)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

func values(it *microdata.Item, prop string) []interface{} {
	if it == nil {
		return nil
	}
	return it.Properties[prop]
}

// present is the "required" test of the validator: set and not empty.
func present(v interface{}, ok bool) bool {
	if !ok || v == nil {
		return false
	}
	if s, isStr := v.(string); isStr {
		return strings.TrimSpace(s) != ""
	}
	return true
}

func isNumber(v interface{}) bool {
	switch x := v.(type) {
	case float64, int, int64:
		return true
	case string:
		_, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return err == nil
	}
	return false
}

func toNumber(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func isCurrency(v interface{}) bool {
	s, ok := v.(string)
	if !ok || len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02", "2006-01-02 15:04:05", "2006-01"}

func isDate(v interface{}) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if _, err := time.Parse(l, s); err == nil {
			return true
		}
	}
	return false
}
