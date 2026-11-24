package audit

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseDoc(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func issueSet(items []StructuredItem) map[string]bool {
	m := map[string]bool{}
	for _, it := range items {
		for _, is := range it.Issues {
			m[is.Type+" "+is.Field+" "+is.Kind+" "+is.Severity] = true
		}
	}
	return m
}

func TestStructuredProductComplete(t *testing.T) {
	doc := parseDoc(t, `<html><head><script type="application/ld+json">
	{"@context":"https://schema.org","@type":"Product","name":"Electrolyseur 50m3","image":"https://ex.com/a.jpg",
	 "description":"d","sku":"E50","gtin13":"3760000000000","brand":{"@type":"Brand","name":"SelTech"},
	 "aggregateRating":{"@type":"AggregateRating","ratingValue":4.6,"reviewCount":12},
	 "review":{"@type":"Review","author":{"@type":"Person","name":"Paul"},"reviewRating":{"@type":"Rating","ratingValue":5},"datePublished":"2026-10-01"},
	 "offers":{"@type":"Offer","price":"899.00","priceCurrency":"EUR","availability":"https://schema.org/InStock","priceValidUntil":"2026-12-31",
	   "shippingDetails":{"@type":"OfferShippingDetails","deliveryTime":{"@type":"ShippingDeliveryTime"},"shippingDestination":{"@type":"DefinedRegion","addressCountry":"FR"},
	     "shippingRate":{"@type":"MonetaryAmount","value":0,"currency":"EUR"}},
	   "hasMerchantReturnPolicy":{"@type":"MerchantReturnPolicy","applicableCountry":"FR","returnPolicyCategory":"https://schema.org/MerchantReturnFiniteReturnWindow","merchantReturnDays":30,"returnFees":"https://schema.org/FreeReturn","returnMethod":"https://schema.org/ReturnByMail"}}}
	</script></head><body></body></html>`)
	items, bad := ExtractStructured(doc, "https://ex.com/p/")
	if bad != 0 || len(items) != 1 || items[0].Type != "Product" {
		t.Fatalf("items %+v bad %d", items, bad)
	}
	if len(items[0].Issues) != 0 {
		t.Errorf("a complete product must pass: %+v", items[0].Issues)
	}
}

func TestStructuredProductErrors(t *testing.T) {
	doc := parseDoc(t, `<script type="application/ld+json">{"@context":"https://schema.org","@graph":[
	 {"@type":"Product","name":"Pompe","offers":{"@type":"Offer","price":"sur devis","priceCurrency":"euros"}},
	 {"@type":"Product","name":"Sans offre"},
	 {"@type":"AggregateRating","ratingValue":7,"ratingCount":3,"itemReviewed":{"@type":"Thing","name":"x"}}]}</script>`)
	items, _ := ExtractStructured(doc, "https://ex.com/")
	if len(items) != 3 {
		t.Fatalf("@graph items not flattened: %+v", items)
	}
	got := issueSet(items)
	for _, want := range []string{
		"Offer price invalid error",                           // text is not a price
		"Offer priceCurrency invalid warning",                 // not an ISO code
		"Product image missing error",                         // merchant listing
		"Product aggregateRating|offers|review missing error", // second product
		"AggregateRating ratingValue range error",             // 7 out of the default 5
		"Product gtin missing warning",
		"Offer shippingDetails missing warning",
	} {
		if !got[want] {
			t.Errorf("missing issue %q in %v", want, got)
		}
	}
}

func TestStructuredMicrodataBreadcrumbAndBusiness(t *testing.T) {
	doc := parseDoc(t, `<html><body>
	<ol itemscope itemtype="https://schema.org/BreadcrumbList">
	 <li itemprop="itemListElement" itemscope itemtype="https://schema.org/ListItem"><a itemprop="item" href="/"><span itemprop="name">Accueil</span></a><meta itemprop="position" content="1"></li>
	 <li itemprop="itemListElement" itemscope itemtype="https://schema.org/ListItem"><span itemprop="name">Pompes</span><meta itemprop="position" content="2"></li>
	</ol>
	<div itemscope itemtype="http://schema.org/Plumber"><span itemprop="name">Plombier</span></div>
	<script type="application/ld+json">{"@type": "FAQPage", "mainEntity": [{"@type":"Question","name":"Prix ?"}]}</script>
	<script type="application/ld+json">{"@type": "Organization", "name": "X",}</script>
	</body></html>`)
	items, bad := ExtractStructured(doc, "https://ex.com/a/")
	if bad != 1 {
		t.Errorf("one JSON-LD block has a trailing comma: bad = %d", bad)
	}
	got := issueSet(items)
	if got["BreadcrumbList itemListElement.item missing warning"] {
		t.Error("the last crumb needs no URL")
	}
	for _, want := range []string{"Plumber address missing error", "Plumber telephone missing warning", "Question acceptedAnswer missing error"} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	types := map[string]bool{}
	for _, it := range items {
		types[it.Type] = true
	}
	for _, ty := range []string{"BreadcrumbList", "Plumber", "FAQPage"} {
		if !types[ty] {
			t.Errorf("type %s not read: %v", ty, types)
		}
	}
}

func TestStructuredBreadcrumbMiddleWithoutURL(t *testing.T) {
	doc := parseDoc(t, `<script type="application/ld+json">{"@type":"BreadcrumbList","itemListElement":[
	 {"@type":"ListItem","position":1,"name":"A","item":"https://ex.com/"},
	 {"@type":"ListItem","position":2,"name":"B"},
	 {"@type":"ListItem","position":3,"name":"C"}]}</script>`)
	items, _ := ExtractStructured(doc, "https://ex.com/")
	if !issueSet(items)["BreadcrumbList itemListElement.item missing warning"] {
		t.Error("a middle crumb without URL must be reported")
	}
}

func TestBuildStructuredFamilies(t *testing.T) {
	pages := []*PageSignal{
		{URL: "https://ex.com/a", Status: 200, Structured: []StructuredItem{{Type: "Product", Issues: []SDIssue{{Type: "Offer", Field: "price", Kind: "missing", Severity: "error"}}}}},
		{URL: "https://ex.com/b", Status: 200, Structured: []StructuredItem{{Type: "Product", Issues: []SDIssue{{Type: "Product", Field: "gtin", Kind: "missing", Severity: "warning"}}}, {Type: "WebSite"}}},
		{URL: "https://ex.com/c", Status: 200, Noindex: true, Structured: []StructuredItem{{Type: "Product"}}},
		{URL: "https://ex.com/d", Status: 200, BadJSONLD: 1},
	}
	r := BuildStructured(pages)
	if r.PagesRead != 3 || r.PagesWith != 2 || r.Unreadable != 1 {
		t.Errorf("counts %+v", r)
	}
	f := r.Family("product")
	if f.Pages != 2 || f.ErrorPages != 1 || f.WarningPages != 1 {
		t.Errorf("product family %+v", f)
	}
	if len(f.Problems) != 2 || f.Problems[0].Severity != "error" {
		t.Errorf("errors first: %+v", f.Problems)
	}
	if len(r.OtherTypes) != 1 || r.OtherTypes[0] != "WebSite" {
		t.Errorf("other types %v", r.OtherTypes)
	}
}

// Google's Organization documentation has no required property: a missing
// name is a recommendation, never a blocking error.
func TestStructuredOrganizationHasNoRequiredField(t *testing.T) {
	doc := parseDoc(t, `<script type="application/ld+json">{"@type":"Organization","url":"https://ex.com/"}</script>`)
	got := issueSet(func() []StructuredItem { i, _ := ExtractStructured(doc, "https://ex.com/"); return i }())
	if !got["Organization name missing warning"] || got["Organization name missing error"] {
		t.Errorf("%v", got)
	}
}
