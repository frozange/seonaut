package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Target searches checked against Google Suggest, the free completion service
// of the Google search box (unofficial, no volume). A search the AI guessed is
// kept when Google proposes it, or a close variant of it; otherwise it is
// dropped from the report. When Suggest cannot be reached, searches are kept
// and marked as not checked: the report never claims a check it did not make.

// Query check results.
const (
	QueryConfirmed = "confirmed" // Google proposes the search, or a longer form of it
	QueryVariant   = "variant"   // Google proposes a close variant (one word differs)
	QueryUnknown   = "unknown"   // Google proposes nothing close: dropped
	QueryUnchecked = "unchecked" // Suggest could not be reached
)

// SuggestEndpoint is the Suggest address; tests point it to a local server.
var SuggestEndpoint = "https://suggestqueries.google.com/complete/search"

// Suggester asks Google Suggest for completions.
type Suggester struct {
	HTTP     *http.Client
	Lang     string        // hl, e.g. "fr"
	Country  string        // gl, e.g. "fr"
	Pause    time.Duration // between two calls, to stay polite
	lastCall time.Time
}

// Suggest returns the completions Google proposes for a query.
func (s *Suggester) Suggest(ctx context.Context, q string) ([]string, error) {
	if wait := s.Pause - time.Since(s.lastCall); wait > 0 && !s.lastCall.IsZero() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	s.lastCall = time.Now()
	v := url.Values{"client": {"firefox"}, "q": {q}, "hl": {s.Lang}}
	if s.Country != "" {
		v.Set("gl", s.Country)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SuggestEndpoint+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0")
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("suggest: status %d", resp.StatusCode)
	}
	var raw []json.RawMessage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	if len(raw) < 2 {
		return nil, errors.New("suggest: unexpected answer")
	}
	var list []string
	if err := json.Unmarshal(raw[1], &list); err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	return list, nil
}

// stopWords are left out when two searches are compared.
var stopWords = map[string]bool{
	"de": true, "du": true, "des": true, "la": true, "le": true, "les": true, "l": true, "d": true, "a": true,
	"au": true, "aux": true, "en": true, "et": true, "pour": true, "sur": true, "un": true, "une": true,
	"par": true, "avec": true, "dans": true, "the": true, "of": true, "for": true, "in": true, "and": true,
	"to": true, "on": true, "with": true,
}

// ContentTokens are the words of a search that carry its meaning: no stop
// words, accents removed, a final s or x dropped so singular and plural match.
func ContentTokens(q string) []string {
	var out []string
	for _, t := range Tokens(q) {
		if stopWords[t] {
			continue
		}
		if len(t) > 3 && (strings.HasSuffix(t, "s") || strings.HasSuffix(t, "x")) {
			t = t[:len(t)-1]
		}
		out = append(out, t)
	}
	return out
}

// QueryCheck is the verdict on one search.
type QueryCheck struct {
	Status  string   // one of the Query* constants
	Variant string   // the closest proposal, for a variant
	Related []string // other proposals that contain the search
}

// matchSuggestions compares a search with what Google proposed.
func matchSuggestions(q string, suggestions []string) QueryCheck {
	want := ContentTokens(q)
	if len(want) == 0 {
		return QueryCheck{Status: QueryUnknown}
	}
	qKey := QueryKey(q)
	res := QueryCheck{Status: QueryUnknown}
	bestMiss := len(want)
	for _, s := range suggestions {
		got := map[string]bool{}
		for _, t := range ContentTokens(s) {
			got[t] = true
		}
		miss := 0
		for _, t := range want {
			if !got[t] {
				miss++
			}
		}
		switch {
		case miss == 0:
			res.Status, res.Variant = QueryConfirmed, ""
			if QueryKey(s) != qKey && len(res.Related) < 3 {
				res.Related = append(res.Related, s)
			}
		case res.Status != QueryConfirmed && miss == 1 && len(want) >= 2 && miss < bestMiss:
			res.Status, res.Variant, bestMiss = QueryVariant, s, miss
		}
	}
	return res
}

// Check asks Suggest about one search: the search itself, then, if nothing
// matched and the search has three words or more, the search without its last
// word, whose completions often hold the full search in another word order.
func (s *Suggester) Check(ctx context.Context, q string) (QueryCheck, error) {
	list, err := s.Suggest(ctx, q)
	if err != nil {
		return QueryCheck{Status: QueryUnchecked}, err
	}
	res := matchSuggestions(q, list)
	words := strings.Fields(q)
	if res.Status != QueryConfirmed && len(words) >= 3 {
		if more, err := s.Suggest(ctx, strings.Join(words[:len(words)-1], " ")); err == nil {
			list = append(list, more...)
			if r2 := matchSuggestions(q, list); r2.Status == QueryConfirmed || res.Status == QueryUnknown {
				res = r2
			}
		}
	}
	if res.Status == QueryUnknown {
		// What Google proposes instead: searches that share at least two
		// meaningful words with the guess, from its first two words if needed.
		res.Related = alternatives(q, list)
		if len(res.Related) == 0 && len(words) >= 3 {
			if more, err := s.Suggest(ctx, strings.Join(words[:2], " ")); err == nil {
				res.Related = alternatives(q, more)
			}
		}
	}
	return res, nil
}

// alternatives keeps up to three proposals sharing two meaningful words or
// more with the search, most shared first.
func alternatives(q string, suggestions []string) []string {
	want := map[string]bool{}
	for _, t := range ContentTokens(q) {
		want[t] = true
	}
	type cand struct {
		s      string
		shared int
	}
	var list []cand
	seen := map[string]bool{}
	for _, s := range suggestions {
		k := QueryKey(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		n, used := 0, map[string]bool{}
		for _, t := range ContentTokens(s) {
			if want[t] && !used[t] {
				used[t] = true
				n++
			}
		}
		if n >= 2 {
			list = append(list, cand{s, n})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].shared > list[j].shared })
	var out []string
	for i := 0; i < len(list) && i < 3; i++ {
		out = append(out, list[i].s)
	}
	return out
}

// CheckQueries applies Check to the target searches of the AI: searches
// Google does not know are moved to the dropped list. When Suggest fails
// for every search, all are kept and marked unchecked.
func (s *Suggester) CheckQueries(ctx context.Context, queries []TargetQuery) (kept, dropped []TargetQuery) {
	for _, q := range queries {
		res, err := s.Check(ctx, q.Query)
		q.Google, q.GoogleVariant, q.GoogleRelated = res.Status, res.Variant, res.Related
		if err != nil {
			q.Google = QueryUnchecked
		}
		if q.Google == QueryUnknown {
			dropped = append(dropped, q)
			continue
		}
		kept = append(kept, q)
	}
	return kept, dropped
}

// SuggestLocale picks the language and country of the Suggest calls from the
// language the home page declares ("fr-FR"), the site's country domain, then
// the report language.
func SuggestLocale(pageLang, host, fallback string) (lang, country string) {
	l := strings.ToLower(strings.TrimSpace(pageLang))
	lang = primaryLang(l)
	if i := strings.IndexAny(l, "-_"); i >= 0 && len(l) >= i+3 {
		country = l[i+1 : i+3]
	}
	if country == "" {
		if i := strings.LastIndex(host, "."); i >= 0 && len(host)-i == 3 {
			country = host[i+1:]
		}
	}
	if lang == "" {
		lang = fallback
	}
	switch {
	case country == "uk":
		country = "gb"
	case country == "" && (lang == "fr" || lang == "de" || lang == "es" || lang == "it"):
		country = lang
	}
	return lang, country
}
