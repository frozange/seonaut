package audit

import (
	"math"
	"sort"
)

// The scale below is published in the report: same inputs, same score.
//
// Technical score (0 to 10): start from 10, each issue type found removes
// points by priority (critical 1.2, alert 0.6, warning 0.2), weighted by the
// share of crawled pages it affects (half the points as soon as one page is
// affected, all of them when every page is). Floor at 0.
//
// Copywriting and content scores (0 to 10): average of the AI ratings of their
// criteria, good = 10, partial = 5, poor = 0. No score without AI ratings.
var priorityWeight = map[int]float64{1: 1.2, 2: 0.6, 3: 0.2}

// TechnicalScore applies the published scale.
func TechnicalScore(issues []IssueSummary, pages int) float64 {
	if pages <= 0 {
		return 0
	}
	score := 10.0
	for _, is := range issues {
		share := math.Min(1, float64(is.Pages)/float64(pages))
		score -= priorityWeight[is.Priority] * (0.5 + 0.5*share)
	}
	return round1(math.Max(0, score))
}

var ratingValue = map[string]float64{"good": 10, "partial": 5, "poor": 0}

// GroupScore averages the ratings of one criteria group, nil when none.
func GroupScore(c []Criterion, group string) *float64 {
	var sum float64
	var n int
	for _, x := range c {
		if v, ok := ratingValue[x.Rating]; ok && x.Group == group {
			sum += v
			n++
		}
	}
	if n == 0 {
		return nil
	}
	s := round1(sum / float64(n))
	return &s
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// TechnicalPriorities turns critical and alert issues into priorities. Warnings
// stay in the detailed list: they are optimisation, not urgency.
func TechnicalPriorities(issues []IssueSummary) []Priority {
	var out []Priority
	for _, is := range issues {
		if is.Priority > 2 {
			continue
		}
		out = append(out, Priority{
			ID: "t:" + is.Type, Source: "technical", Level: is.Priority, Title: is.Type, Pages: is.Pages,
		})
	}
	return out
}

// editorialPriorities turns poor or high severity partial criteria into priorities.
func editorialPriorities(c []Criterion) []Priority {
	var out []Priority
	for _, x := range c {
		level := 0
		switch {
		case x.Rating == "poor" && x.Severity == "high":
			level = 1
		case x.Rating == "poor" || (x.Rating == "partial" && x.Severity == "high"):
			level = 2
		case x.Rating == "partial":
			level = 3
		}
		if level == 0 {
			continue
		}
		out = append(out, Priority{
			ID: "e:" + x.ID, Source: "editorial", Level: level, Title: x.Verdict, Evidence: x.Evidence,
		})
	}
	return out
}

// rankPriorities orders by level, then technical issues by pages affected, and
// keeps at most max.
func rankPriorities(p []Priority, max int) []Priority {
	sort.SliceStable(p, func(i, j int) bool {
		if p[i].Level != p[j].Level {
			return p[i].Level < p[j].Level
		}
		return p[i].Pages > p[j].Pages
	})
	if len(p) > max {
		p = p[:max]
	}
	return p
}
