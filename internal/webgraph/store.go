package webgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// Store keeps the releases on disk:
//
//	<Dir>/state.json                         what was discovered, when, and any pause asked by the server
//	<Dir>/releases/<name>/download/*.txt.gz   files being downloaded, deleted once indexed
//	<Dir>/releases/<name>/index/              the index, see build.go
//
// It keeps two indexes: the latest release and the quarter before it, which
// gained and lost referring domains are measured against.
type Store struct {
	Dir        string
	Fetcher    *Fetcher
	Builder    *Builder
	CheckEvery time.Duration // between two looks for a new release; default 24 h
	Pause      time.Duration // between two HEAD requests; default 2 s
	BackoffFor time.Duration // after an error status; default 45 min, so the next hourly slot tries again
	Retries    int           // new tries of a file within the slot after an error status; default 2, negative: none
	RetryWait  time.Duration // between two tries; default 1 min
	Now        func() time.Time
	Log        func(format string, args ...interface{})
}

type state struct {
	LastCheck    time.Time `json:"last_check"`
	Latest       string    `json:"latest,omitempty"`
	Previous     string    `json:"previous,omitempty"`
	BackoffUntil time.Time `json:"backoff_until,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) logf(format string, args ...interface{}) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

func (s *Store) releaseDir(name string) string { return filepath.Join(s.Dir, "releases", name) }
func (s *Store) indexDir(name string) string   { return filepath.Join(s.releaseDir(name), "index") }
func (s *Store) downloadDir(name string) string {
	return filepath.Join(s.releaseDir(name), "download")
}

func (s *Store) load() state {
	var st state
	if b, err := os.ReadFile(filepath.Join(s.Dir, "state.json")); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func (s *Store) save(st state) error {
	b, _ := json.MarshalIndent(st, "", " ")
	tmp := filepath.Join(s.Dir, "state.json.tmp")
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.Dir, "state.json"))
}

// Lock takes the store's lock for the whole run; a second run gives up.
func (s *Store) Lock() (func(), error) {
	if err := os.MkdirAll(s.Dir, 0o750); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.Dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("webgraph: another run holds the lock")
	}
	return func() { f.Close() }, nil
}

// Ready lists the releases with a complete index, newest first.
func (s *Store) Ready() []Release {
	entries, _ := os.ReadDir(filepath.Join(s.Dir, "releases"))
	var out []Release
	for _, e := range entries {
		r, ok := ParseRelease(e.Name())
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.indexDir(r.Name), metaFile)); err == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndMonth.After(out[j].EndMonth) })
	return out
}

func (s *Store) isReady(name string) bool {
	_, err := os.Stat(filepath.Join(s.indexDir(name), metaFile))
	return err == nil
}

// Pair returns the newest ready release and the newest ready one that ends at
// least three months before it (nil when none).
func Pair(ready []Release) (cur, prev *Release) {
	if len(ready) == 0 {
		return nil, nil
	}
	cur = &ready[0]
	for i := 1; i < len(ready); i++ {
		if !ready[i].EndMonth.After(cur.EndMonth.AddDate(0, -3, 0)) {
			return cur, &ready[i]
		}
	}
	return cur, nil
}

// Open opens the current index and the previous quarter's (nil when absent).
func (s *Store) Open() (cur, prev *Index, err error) {
	c, p := Pair(s.Ready())
	if c == nil {
		return nil, nil, ErrNoIndex
	}
	if cur, err = Open(s.indexDir(c.Name)); err != nil {
		return nil, nil, err
	}
	if p != nil {
		if prev, err = Open(s.indexDir(p.Name)); err != nil {
			s.logf("previous index %s: %v", p.Name, err)
			prev = nil
		}
	}
	return cur, prev, nil
}

func (s *Store) exists(ctx context.Context, r Release) (bool, error) {
	if s.isReady(r.Name) {
		return true, nil
	}
	_, found, err := s.Fetcher.Head(ctx, FileURL(r.Name, "vertices"))
	if s.Pause >= 0 {
		pause := s.Pause
		if pause == 0 {
			pause = 2 * time.Second
		}
		time.Sleep(pause)
	}
	return found, err
}

// discover looks for a newer release, at most once per CheckEvery.
func (s *Store) discover(ctx context.Context, st *state) error {
	every := s.CheckEvery
	if every == 0 {
		every = 24 * time.Hour
	}
	if !st.LastCheck.IsZero() && s.now().Sub(st.LastCheck) < every && st.Latest != "" {
		return nil
	}
	var after time.Time
	if r, ok := ParseRelease(st.Latest); ok {
		after = r.EndMonth
	}
	for _, c := range Candidates(after, s.now()) {
		found, err := s.exists(ctx, c)
		if err != nil {
			return err
		}
		if found {
			s.logf("release found: %s", c.Name)
			st.Latest, st.Previous = c.Name, ""
			break
		}
	}
	if latest, ok := ParseRelease(st.Latest); ok && st.Previous == "" {
		for _, c := range PreviousCandidates(latest) {
			found, err := s.exists(ctx, c)
			if err != nil {
				return err
			}
			if found {
				st.Previous = c.Name
				break
			}
		}
	}
	st.LastCheck = s.now()
	return nil
}

// Outcome of an update.
type Outcome int

const (
	Idle       Outcome = iota // everything wanted is indexed
	More                      // the time slot ended, a download resumes next time
	Backoff                   // the server answered with an error, wait
	Built                     // a release was indexed during this run
	BuildError                // a build failed; see the log
)

func (o Outcome) String() string {
	return [...]string{"idle", "more", "backoff", "built", "build-error"}[o]
}

// Update discovers, downloads until deadline, and indexes a release once all
// its files are there. Downloads go one release at a time, one file at a time.
func (s *Store) Update(ctx context.Context, deadline time.Time) (Outcome, error) {
	st := s.load()
	if s.now().Before(st.BackoffUntil) {
		return Backoff, nil
	}
	backoff := func(err error) (Outcome, error) {
		wait := s.BackoffFor
		if wait == 0 {
			wait = 45 * time.Minute
		}
		st.BackoffUntil, st.LastError = s.now().Add(wait), err.Error()
		s.logf("backing off until %s: %v", st.BackoffUntil.Format(time.RFC3339), err)
		return Backoff, s.save(st)
	}
	if err := s.discover(ctx, &st); err != nil {
		if errors.Is(err, ErrBackoff) {
			return backoff(err)
		}
		return Idle, err
	}
	st.LastError = ""
	if err := s.save(st); err != nil {
		return Idle, err
	}
	for _, name := range []string{st.Latest, st.Previous} {
		if name == "" || s.isReady(name) {
			continue
		}
		dl := s.downloadDir(name)
		if err := os.MkdirAll(dl, 0o750); err != nil {
			return Idle, err
		}
		for _, f := range Files {
			if !time.Now().Before(deadline) {
				return More, nil
			}
			err := s.fetchWithRetries(ctx, FileURL(name, f), filepath.Join(dl, f+".txt.gz"), deadline)
			switch {
			case errors.Is(err, ErrDeadline):
				return More, nil
			case errors.Is(err, ErrBackoff):
				return backoff(err)
			case err != nil:
				return backoff(err) // network errors are treated as a request to wait
			}
		}
		if err := s.build(name); err != nil {
			return BuildError, err
		}
		s.prune(st)
		return Built, nil
	}
	s.prune(st)
	return Idle, nil
}

// fetchWithRetries tries a file again, a minute later, when the server
// answers with an error status (Common Crawl's "503 SlowDown" comes and goes),
// as long as the slot allows; the caller pauses after the last refusal.
func (s *Store) fetchWithRetries(ctx context.Context, url, dest string, deadline time.Time) error {
	retries, wait := s.Retries, s.RetryWait
	if retries == 0 {
		retries = 2
	}
	if wait == 0 {
		wait = time.Minute
	}
	for try := 0; ; try++ {
		err := s.Fetcher.Fetch(ctx, url, dest, deadline)
		if !errors.Is(err, ErrBackoff) || try >= retries || !time.Now().Add(wait).Before(deadline) {
			return err
		}
		s.logf("trying again in %s: %v", wait, err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
	}
}

func (s *Store) build(name string) error {
	s.logf("building %s", name)
	meta, err := s.Builder.Build(name, s.downloadDir(name), s.indexDir(name))
	var bad *ErrCorrupt
	if errors.As(err, &bad) {
		// A damaged download is fetched again on the next run.
		s.logf("removing %s: %v", bad.File, bad.Err)
		os.Remove(bad.File)
	}
	if err != nil {
		os.RemoveAll(s.indexDir(name) + ".tmp")
		return fmt.Errorf("build %s: %w", name, err)
	}
	s.logf("built %s: %d domains, %d links, in %s", name, meta.Nodes, meta.Arcs, meta.Duration)
	return os.RemoveAll(s.downloadDir(name))
}

// prune keeps the latest and previous releases and the indexes in use.
func (s *Store) prune(st state) {
	keep := map[string]bool{st.Latest: true, st.Previous: true}
	cur, prev := Pair(s.Ready())
	if cur != nil {
		keep[cur.Name] = true
	}
	if prev != nil {
		keep[prev.Name] = true
	}
	entries, _ := os.ReadDir(filepath.Join(s.Dir, "releases"))
	for _, e := range entries {
		if _, ok := ParseRelease(e.Name()); ok && !keep[e.Name()] {
			s.logf("removing old release %s", e.Name())
			os.RemoveAll(s.releaseDir(e.Name()))
		}
	}
}

// Status describes the store for an operator.
func (s *Store) Status() string {
	st := s.load()
	out := fmt.Sprintf("dir: %s\nlast check: %s\nlatest: %s\nprevious: %s\n", s.Dir, st.LastCheck.Format(time.RFC3339), st.Latest, st.Previous)
	if !st.BackoffUntil.IsZero() && s.now().Before(st.BackoffUntil) {
		out += fmt.Sprintf("backing off until %s: %s\n", st.BackoffUntil.Format(time.RFC3339), st.LastError)
	}
	for _, r := range s.Ready() {
		out += "ready: " + r.Name + "\n"
	}
	for _, name := range []string{st.Latest, st.Previous} {
		if name == "" || s.isReady(name) {
			continue
		}
		for _, f := range Files {
			p := filepath.Join(s.downloadDir(name), f+".txt.gz")
			if fi, err := os.Stat(p); err == nil {
				out += fmt.Sprintf("downloaded: %s %s (%d bytes)\n", name, f, fi.Size())
			} else if fi, err := os.Stat(p + ".part"); err == nil {
				out += fmt.Sprintf("partial: %s %s (%d bytes)\n", name, f, fi.Size())
			}
		}
	}
	return out
}
