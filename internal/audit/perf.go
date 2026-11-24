package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Speed through the PageSpeed Insights API v5, ported from audit-seo
// (audit/perf.py). Two kinds of data, never mixed in the report:
//   - field data (CrUX): real Chrome visits, 75th percentile over 28 days,
//     what Google uses for the Core Web Vitals;
//   - lab data (Lighthouse): one simulated visit on a throttled phone, for
//     diagnosis only.
//
// A failed call records an error and no figure: never a 0 that would read as
// a measurement.

// PSIEndpoint is the API address; tests point it to a local server.
var PSIEndpoint = "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"

// Core Web Vitals thresholds at the 75th percentile (web.dev/articles/vitals).
var cwvThresholds = map[string][2]float64{
	"LCP": {2500, 4000},
	"INP": {200, 500},
	"CLS": {0.1, 0.25},
}

var cwvAPINames = map[string]string{
	"LARGEST_CONTENTFUL_PAINT_MS":   "LCP",
	"INTERACTION_TO_NEXT_PAINT":     "INP",
	"CUMULATIVE_LAYOUT_SHIFT_SCORE": "CLS",
}

// SpeedReport holds the measurements of the home page on mobile and desktop.
type SpeedReport struct {
	URL     string    `json:"url"`
	Mobile  *SpeedRun `json:"mobile,omitempty"`
	Desktop *SpeedRun `json:"desktop,omitempty"`
}

// SpeedRun is one PageSpeed Insights call. With Error set, nothing else is.
type SpeedRun struct {
	Error      string     `json:"error,omitempty"`
	Field      *FieldData `json:"field,omitempty"`       // nil: not enough real visits
	FieldScope string     `json:"field_scope,omitempty"` // "page" or "site" (origin)
	Lab        *LabData   `json:"lab,omitempty"`
}

// FieldData are the Core Web Vitals of real visits.
type FieldData struct {
	LCP *FieldMetric `json:"lcp,omitempty"`
	INP *FieldMetric `json:"inp,omitempty"`
	CLS *FieldMetric `json:"cls,omitempty"`
}

// FieldMetric is a 75th percentile and its rating: good, average or poor.
type FieldMetric struct {
	P75    float64 `json:"p75"`
	Rating string  `json:"rating"`
}

// Overall is the worst rating of the three metrics, like Google's assessment.
func (f *FieldData) Overall() string {
	order := map[string]int{"good": 0, "average": 1, "poor": 2}
	worst := ""
	for _, m := range []*FieldMetric{f.LCP, f.INP, f.CLS} {
		if m != nil && (worst == "" || order[m.Rating] > order[worst]) {
			worst = m.Rating
		}
	}
	return worst
}

// LabData is the simulated visit. A nil field was not measured.
type LabData struct {
	Score         *int          `json:"score,omitempty"`
	LCPms         *float64      `json:"lcp_ms,omitempty"`
	CLS           *float64      `json:"cls,omitempty"`
	TBTms         *float64      `json:"tbt_ms,omitempty"`
	FCPms         *float64      `json:"fcp_ms,omitempty"`
	TTFBms        *float64      `json:"ttfb_ms,omitempty"`
	Bytes         *float64      `json:"bytes,omitempty"`
	Opportunities []Opportunity `json:"opportunities,omitempty"`
}

// Opportunity is a Lighthouse advice that would save at least 150 ms.
type Opportunity struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	SavingMs int    `json:"saving_ms"`
}

// RateMetric applies the thresholds.
func RateMetric(name string, v float64) string {
	t := cwvThresholds[name]
	switch {
	case v <= t[0]:
		return "good"
	case v > t[1]:
		return "poor"
	}
	return "average"
}

// SpeedTester calls PageSpeed Insights.
type SpeedTester struct {
	HTTP    *http.Client
	Key     string // sent in a header, never in the address, so no error message holds it
	Locale  string // language of the Lighthouse advice titles
	Retries int
	Backoff time.Duration
}

// Measure runs mobile and desktop at the same time.
func (t *SpeedTester) Measure(ctx context.Context, pageURL string) *SpeedReport {
	desktop := make(chan *SpeedRun, 1)
	go func() { desktop <- t.Run(ctx, pageURL, "desktop") }()
	mobile := t.Run(ctx, pageURL, "mobile")
	return &SpeedReport{URL: pageURL, Mobile: mobile, Desktop: <-desktop}
}

// Run makes one call, retried on 429 and 5xx.
func (t *SpeedTester) Run(ctx context.Context, pageURL, strategy string) *SpeedRun {
	v := url.Values{"url": {pageURL}, "strategy": {strategy}, "category": {"performance"}}
	if t.Locale != "" {
		v.Set("locale", t.Locale)
	}
	client := t.HTTP
	if client == nil {
		client = &http.Client{Timeout: 150 * time.Second}
	}
	retries, backoff := t.Retries, t.Backoff
	if retries <= 0 {
		retries = 3
	}
	if backoff == 0 {
		backoff = 10 * time.Second
	}
	var last string
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return &SpeedRun{Error: ctx.Err().Error()}
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, PSIEndpoint+"?"+v.Encode(), nil)
		if err != nil {
			return &SpeedRun{Error: err.Error()}
		}
		if t.Key != "" {
			req.Header.Set("X-Goog-Api-Key", t.Key)
		}
		resp, err := client.Do(req)
		if err != nil {
			last = strings.ReplaceAll(err.Error(), t.keyOrNone(), "<key>")
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		if err != nil {
			last = err.Error()
			continue
		}
		if resp.StatusCode == http.StatusOK {
			run, err := ParsePSI(body)
			if err != nil {
				return &SpeedRun{Error: err.Error()}
			}
			return run
		}
		last = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, apiMessage(body))
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
	}
	return &SpeedRun{Error: strings.ReplaceAll(last, t.keyOrNone(), "<key>")}
}

func (t *SpeedTester) keyOrNone() string {
	if t.Key == "" {
		return "\x00"
	}
	return t.Key
}

func apiMessage(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return truncate(e.Error.Message, 200)
	}
	return truncate(strings.TrimSpace(string(body)), 200)
}

type psiExperience struct {
	ID             string `json:"id"`
	OriginFallback bool   `json:"origin_fallback"`
	Metrics        map[string]struct {
		Percentile *float64 `json:"percentile"`
	} `json:"metrics"`
}

type psiAudit struct {
	Title        string   `json:"title"`
	Score        *float64 `json:"score"`
	NumericValue *float64 `json:"numericValue"`
	Details      struct {
		OverallSavingsMs *float64 `json:"overallSavingsMs"`
	} `json:"details"`
	MetricSavings struct {
		LCP *float64 `json:"LCP"`
	} `json:"metricSavings"`
}

// ParsePSI reads one API answer.
func ParsePSI(body []byte) (*SpeedRun, error) {
	var j struct {
		Loading          *psiExperience `json:"loadingExperience"`
		Origin           *psiExperience `json:"originLoadingExperience"`
		LighthouseResult *struct {
			Categories struct {
				Performance struct {
					Score *float64 `json:"score"`
				} `json:"performance"`
			} `json:"categories"`
			Audits map[string]psiAudit `json:"audits"`
		} `json:"lighthouseResult"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, fmt.Errorf("unreadable PageSpeed answer: %w", err)
	}
	run := &SpeedRun{}
	if f := fieldData(j.Loading); f != nil && !j.Loading.OriginFallback {
		run.Field, run.FieldScope = f, "page"
	} else if f := fieldData(j.Origin); f != nil {
		run.Field, run.FieldScope = f, "site"
	}
	if lh := j.LighthouseResult; lh != nil {
		lab := &LabData{}
		if s := lh.Categories.Performance.Score; s != nil {
			v := int(*s*100 + 0.5)
			lab.Score = &v
		}
		num := func(id string) *float64 { return lh.Audits[id].NumericValue }
		lab.LCPms, lab.CLS, lab.TBTms = num("largest-contentful-paint"), num("cumulative-layout-shift"), num("total-blocking-time")
		lab.FCPms, lab.TTFBms, lab.Bytes = num("first-contentful-paint"), num("server-response-time"), num("total-byte-weight")
		for id, a := range lh.Audits {
			saving := a.Details.OverallSavingsMs
			if saving == nil {
				saving = a.MetricSavings.LCP
			}
			if a.Score != nil && *a.Score < 0.9 && saving != nil && *saving >= 150 {
				lab.Opportunities = append(lab.Opportunities, Opportunity{ID: id, Title: a.Title, SavingMs: int(*saving)})
			}
		}
		sort.Slice(lab.Opportunities, func(i, k int) bool {
			if lab.Opportunities[i].SavingMs != lab.Opportunities[k].SavingMs {
				return lab.Opportunities[i].SavingMs > lab.Opportunities[k].SavingMs
			}
			return lab.Opportunities[i].ID < lab.Opportunities[k].ID
		})
		if len(lab.Opportunities) > 5 {
			lab.Opportunities = lab.Opportunities[:5]
		}
		if lab.Score != nil || lab.LCPms != nil {
			run.Lab = lab
		}
	}
	if run.Field == nil && run.Lab == nil {
		return nil, errors.New("PageSpeed answered without any measurement")
	}
	return run, nil
}

func fieldData(e *psiExperience) *FieldData {
	if e == nil || len(e.Metrics) == 0 {
		return nil
	}
	f := &FieldData{}
	found := false
	for api, name := range cwvAPINames {
		m, ok := e.Metrics[api]
		if !ok || m.Percentile == nil {
			continue
		}
		v := *m.Percentile
		if name == "CLS" {
			v /= 100 // the API gives CLS times 100
		}
		fm := &FieldMetric{P75: v, Rating: RateMetric(name, v)}
		switch name {
		case "LCP":
			f.LCP = fm
		case "INP":
			f.INP = fm
		case "CLS":
			f.CLS = fm
		}
		found = true
	}
	if !found {
		return nil
	}
	return f
}

// ReadEnvKey reads one variable of a dotenv file by its name. The value is
// returned, never logged.
func ReadEnvKey(path, name string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != name {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if v == "" {
			return "", fmt.Errorf("%s is empty in %s", name, path)
		}
		return v, nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s not found in %s", name, path)
}
