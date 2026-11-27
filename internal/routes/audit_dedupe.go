package routes

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/stjudewashere/seonaut/internal/audit"
)

// Francis wants every finding said once (2026-11-26, and the reviews of the
// sel-tech.fr report, 2026-11-27): the AI repeated the same facts in the
// summary, the strengths, the priorities and the criterion cards. The report
// is read in page order and a sentence whose main words were already said
// higher up is left out. The closing recap only repeated the rest: not shown.

// dupShare is the share of a sentence's main words already said for it to be a
// repeat.
const dupShare = 0.5

var dupStopwords = map[string]bool{
	"avec": true, "dans": true, "pour": true, "sont": true, "vous": true, "votre": true, "vos": true,
	"nous": true, "plus": true, "elle": true, "elles": true, "cette": true, "page": true, "pages": true,
	"site": true, "leur": true, "leurs": true, "mais": true, "comme": true, "aussi": true, "tout": true,
	"tous": true, "sans": true, "chaque": true, "deja": true, "bien": true, "very": true, "your": true,
	"with": true, "that": true, "this": true, "from": true, "have": true,
}

// mainWords returns the words of four letters or more, lower-cased, without
// accents and without a final "s".
func mainWords(s string) map[string]bool {
	t := norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	out := map[string]bool{}
	for _, w := range strings.Fields(b.String()) {
		w = strings.TrimSuffix(w, "s")
		if len([]rune(w)) >= 4 && !dupStopwords[w] {
			out[w] = true
		}
	}
	return out
}

type sayings struct{ said []map[string]bool }

func (s *sayings) add(text string) {
	for _, sent := range splitReportSentences(text) {
		if w := mainWords(sent); len(w) > 0 {
			s.said = append(s.said, w)
		}
	}
}

// repeats tells whether a sentence was already said, in its main words.
func (s *sayings) repeats(sentence string) bool {
	w := mainWords(sentence)
	if len(w) < 3 {
		return false
	}
	for _, prev := range s.said {
		n := 0
		for k := range w {
			if prev[k] {
				n++
			}
		}
		share := float64(n) / float64(len(w))
		// Same words, or the same handful of specific words told otherwise
		// ("ville, type de projet, surface" of the quote form, sel-tech.fr).
		if share >= dupShare || (n >= 5 && share >= 0.35) {
			return true
		}
	}
	return false
}

// priceWords mark a sentence about prices: the report never advises showing a
// price (Francis, 2026-11-27: "ne mets jamais le prix d'un chantier en ligne").
var priceWords = []string{"prix", "tarif", "budget", "fourchette", "price", "pricing"}

func aboutPrice(sentence string) bool {
	l := strings.ToLower(sentence)
	for _, w := range priceWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// keepNew returns the sentences of text not said before and not about prices,
// and records them.
func (s *sayings) keepNew(text string) string {
	var kept []string
	for _, sent := range splitReportSentences(text) {
		if !s.repeats(sent) && !aboutPrice(sent) {
			kept = append(kept, sent)
		}
	}
	out := strings.Join(kept, " ")
	s.add(out)
	return out
}

func splitReportSentences(text string) []string {
	var out []string
	start := 0
	r := []rune(text)
	for i := 0; i < len(r); i++ {
		if (r[i] == '.' || r[i] == '!' || r[i] == '?') && (i+1 == len(r) || unicode.IsSpace(r[i+1])) {
			if s := strings.TrimSpace(string(r[start : i+1])); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := strings.TrimSpace(string(r[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// dedupeAI returns a copy of the report's AI text in which nothing is said
// twice, in the page order: summary, priorities, strengths, searches,
// criteria, rewrites.
func dedupeAI(res *audit.Result) *audit.AIReport {
	if res.AI == nil {
		return nil
	}
	ai := *res.AI
	var s sayings
	ai.Summary = withoutPrices(ai.Summary)
	s.add(ai.Summary)
	for _, p := range res.Priorities {
		if p.Source != "technical" {
			s.add(p.Title)
			s.add(p.Impact)
			s.add(p.Action)
		}
	}
	var strengths []audit.Strength
	for _, st := range ai.Strengths {
		if t := s.keepNew(st.Text); t != "" {
			st.Text = t
			strengths = append(strengths, st)
		}
	}
	ai.Strengths = strengths
	queries := make([]audit.TargetQuery, len(ai.TargetQueries))
	for i, q := range ai.TargetQueries {
		q.Comment = s.keepNew(q.Comment)
		queries[i] = q
	}
	ai.TargetQueries = queries
	crit := make([]audit.Criterion, len(ai.Criteria))
	for i, c := range ai.Criteria {
		c.Verdict = s.keepNew(c.Verdict)
		c.Detail = s.keepNew(c.Detail)
		crit[i] = c
	}
	ai.Criteria = crit
	rewrites := make([]audit.Rewrite, len(ai.Rewrites))
	for i, r := range ai.Rewrites {
		r.Why = s.keepNew(r.Why)
		rewrites[i] = r
	}
	ai.Rewrites = rewrites
	ai.Conclusion = ""
	return &ai
}

// withoutPrices drops the sentences of a text that speak of prices.
func withoutPrices(text string) string {
	var kept []string
	for _, sent := range splitReportSentences(text) {
		if !aboutPrice(sent) {
			kept = append(kept, sent)
		}
	}
	return strings.Join(kept, " ")
}

// pricelessPriorities removes price sentences from the written priorities; a
// priority left without a title is dropped.
func pricelessPriorities(list []audit.Priority) []audit.Priority {
	out := make([]audit.Priority, 0, len(list))
	for _, p := range list {
		if p.Source != "technical" {
			p.Title, p.Impact, p.Action = withoutPrices(p.Title), withoutPrices(p.Impact), withoutPrices(p.Action)
			if p.Title == "" {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
