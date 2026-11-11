package audit

// Checks of upstream SEOnaut that measure something Google Search says it does
// not use, or has no rule about. They are reported apart as site hygiene and
// never weigh on the technical score. Sources: CHANGELOG.lafactory.md 1.4.0.
var outsideGoogleRanking = map[string]bool{
	"ERROR_NOT_VALID_HEADINGS":        true, // heading order
	"ERROR_NO_H1":                     true, // number of headings
	"ERROR_LITTLE_CONTENT":            true, // content length
	"ERROR_SHORT_TITLE":               true, // title length
	"ERROR_LONG_TITLE":                true,
	"ERROR_SHORT_DESCRIPTION":         true, // description length
	"ERROR_LONG_DESCRIPTION":          true,
	"ERROR_EXTERNAL_WITHOUT_NOFOLLOW": true, // ordinary links need no rel
	"ERROR_TOO_MANY_LINKS":            true,
	"ERROR_LONG_ALT_TEXT":             true,
	"ERROR_NO_LANG":                   true, // lang attribute not used
	"INVALID_LANG":                    true,
	"ERROR_DUPLICATED_ID":             true, // HTML validity
	"ERROR_DOM_SIZE":                  true, // speed is measured on real visits
	"ERROR_IMG_SIZE_ATTR":             true,
	"ERROR_LARGE_IMAGE":               true,
	"ERROR_MISSING_HSTS":              true, // security headers
	"ERROR_MISSING_CSP":               true,
	"ERROR_CONTENT_TYPE_OPTIONS":      true,
}

// SplitByGoogle separates the issues that matter to Google Search from the
// hygiene ones, keeping the order of each.
func SplitByGoogle(issues []IssueSummary) (search, hygiene []IssueSummary) {
	for _, is := range issues {
		if outsideGoogleRanking[is.Type] {
			hygiene = append(hygiene, is)
		} else {
			search = append(search, is)
		}
	}
	return search, hygiene
}
