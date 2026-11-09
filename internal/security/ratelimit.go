package security

import (
	"sync"
	"time"
)

// RateLimiter counts events per key in a sliding window kept in memory. It is
// meant for a single instance; counters reset when the process restarts.
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu     sync.Mutex
	events map[string][]time.Time
	calls  int
}

// NewRateLimiter allows limit events per window for each key. A limit of zero or
// less disables the limiter: Allow always returns true.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		now:    time.Now,
		events: map[string][]time.Time{},
	}
}

// Allow records an event for key and reports whether it is within the limit.
// A refused event is not recorded, so a blocked client recovers once older
// events leave the window.
func (l *RateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	kept := l.recent(key, now)
	if len(kept) >= l.limit {
		l.events[key] = kept
		return false
	}
	l.events[key] = append(kept, now)

	l.calls++
	if l.calls%1024 == 0 {
		l.sweep(now)
	}
	return true
}

// Remaining reports how many events key may still record in the current window.
func (l *RateLimiter) Remaining(key string) int {
	if l == nil || l.limit <= 0 {
		return -1
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit - len(l.recent(key, l.now()))
}

func (l *RateLimiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	events := l.events[key]
	i := 0
	for i < len(events) && !events[i].After(cutoff) {
		i++
	}
	return events[i:]
}

func (l *RateLimiter) sweep(now time.Time) {
	for k := range l.events {
		if len(l.recent(k, now)) == 0 {
			delete(l.events, k)
		}
	}
}
