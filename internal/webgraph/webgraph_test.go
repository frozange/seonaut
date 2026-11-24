package webgraph

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func gz(t *testing.T, text string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write([]byte(text))
	w.Close()
	return b.Bytes()
}

// sample is a small graph in the published format. Domains (sorted reversed):
// 0 com.big, 1 com.blog, 2 fr.rival, 3 fr.sel-tech, 4 fr.shop, 5 org.wiki
var sampleVertices = "0\tcom.big\t12\n1\tcom.blog\t1\n2\tfr.rival\t1\n3\tfr.sel-tech\t2\n4\tfr.shop\t1\n5\torg.wiki\t40\n"

var sampleEdges = strings.Join([]string{
	"0\t3", "0\t4", // big -> sel-tech, shop
	"1\t2", "1\t3", // blog -> rival, sel-tech
	"2\t3",                 // rival -> sel-tech
	"4\t2",                 // shop -> rival
	"5\t2", "5\t3", "5\t4", // wiki -> rival, sel-tech, shop
}, "\n") + "\n"

var sampleRanks = "#harmonicc_pos\t#harmonicc_val\t#pr_pos\t#pr_val\t#host_rev\t#n_hosts\n" +
	"1\t9.0E2\t1\t0.1\torg.wiki\t40\n2\t8.0E2\t2\t0.1\tcom.big\t12\n3\t5.0E2\t4\t0.1\tfr.sel-tech\t2\n4\t4.0E2\t3\t0.1\tfr.rival\t1\n5\t1.0E2\t5\t0.1\tcom.blog\t1\n"

func writeSample(t *testing.T, dir, vertices, edges, ranks string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	for name, text := range map[string]string{"vertices": vertices, "edges": edges, "ranks": ranks} {
		if err := os.WriteFile(filepath.Join(dir, name+".txt.gz"), gz(t, text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func buildSample(t *testing.T, perBucket uint64) *Index {
	t.Helper()
	dir := t.TempDir()
	writeSample(t, filepath.Join(dir, "dl"), sampleVertices, sampleEdges, sampleRanks)
	meta, err := (&Builder{ArcsPerBucket: perBucket}).Build("cc-main-2026-aug-sep-oct", filepath.Join(dir, "dl"), filepath.Join(dir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Nodes != 6 || meta.Arcs != 9 || meta.Ranked != 5 {
		t.Fatalf("meta = %+v", meta)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.tmp")); !os.IsNotExist(err) {
		t.Fatal("index.tmp left behind")
	}
	ix, err := Open(filepath.Join(dir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ix.Close)
	return ix
}

func names(ix *Index, ids []uint32) []string {
	var out []string
	for _, id := range ids {
		out = append(out, ix.Domain(id))
	}
	sort.Strings(out)
	return out
}

func TestBuildAndQuery(t *testing.T) {
	// One link per bucket forces the bucket boundaries through every target.
	for _, per := range []uint64{0, 1, 2} {
		ix := buildSample(t, per)
		id, ok := ix.Lookup("SEL-TECH.fr.")
		if !ok {
			t.Fatal("sel-tech.fr not found")
		}
		if got := names(ix, ix.Referrers(id)); strings.Join(got, ",") != "big.com,blog.com,rival.fr,wiki.org" {
			t.Fatalf("per %d: referrers = %v", per, got)
		}
		if ix.InDegree(id) != 4 || ix.Rank(id) != 3 {
			t.Fatalf("degree %d rank %d", ix.InDegree(id), ix.Rank(id))
		}
		rival, _ := ix.Lookup("rival.fr")
		if got := names(ix, ix.Referrers(rival)); strings.Join(got, ",") != "blog.com,shop.fr,wiki.org" {
			t.Fatalf("rival referrers = %v", got)
		}
		top := ix.TopReferrers(id, 2)
		if len(top) != 2 || top[0].Domain != "wiki.org" || top[0].Rank != 1 || top[1].Domain != "big.com" {
			t.Fatalf("top = %+v", top)
		}
		shop, _ := ix.Lookup("shop.fr")
		if ix.Rank(shop) != 0 {
			t.Fatal("an unranked domain has a rank")
		}
		// Unranked domains come last.
		all := ix.TopOf([]uint32{shop, id}, 5)
		if all[0].Domain != "sel-tech.fr" || all[1].Domain != "shop.fr" {
			t.Fatalf("unranked order = %+v", all)
		}
		big, _ := ix.Lookup("big.com")
		if ix.InDegree(big) != 0 || ix.Referrers(big) != nil {
			t.Fatal("big.com has referrers")
		}
		if _, ok := ix.Lookup("absent.fr"); ok {
			t.Fatal("absent.fr found")
		}
		if _, ok := ix.Lookup("aaa.com"); ok { // sorts before every name
			t.Fatal("aaa.com found")
		}
	}
}

func TestBuildRefusesBadInput(t *testing.T) {
	cases := map[string][3]string{
		"unsorted":    {"0\tfr.b\t1\n1\tfr.a\t1\n", "", ""},
		"gap in ids":  {"0\tfr.a\t1\n2\tfr.b\t1\n", "", ""},
		"edge out":    {"0\tfr.a\t1\n", "0\t7\n", ""},
		"bad edge":    {"0\tfr.a\t1\n", "x\n", ""},
		"bad rank":    {"0\tfr.a\t1\n", "", "zero\t1\t1\t1\tfr.a\t1\n"},
		"no vertices": {"", "", ""},
	}
	for name, c := range cases {
		dir := t.TempDir()
		writeSample(t, filepath.Join(dir, "dl"), c[0], c[1], c[2])
		if _, err := (&Builder{}).Build("r", filepath.Join(dir, "dl"), filepath.Join(dir, "index")); err == nil {
			t.Errorf("%s: built", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "index", metaFile)); err == nil {
			t.Errorf("%s: meta.json written", name)
		}
	}
	// A truncated gzip is reported as corrupt, so that it is downloaded again.
	dir := t.TempDir()
	writeSample(t, filepath.Join(dir, "dl"), sampleVertices, sampleEdges, sampleRanks)
	full := gz(t, sampleEdges)
	os.WriteFile(filepath.Join(dir, "dl", "edges.txt.gz"), full[:len(full)-12], 0o644)
	_, err := (&Builder{}).Build("r", filepath.Join(dir, "dl"), filepath.Join(dir, "index"))
	var bad *ErrCorrupt
	if !errors.As(err, &bad) || !strings.HasSuffix(bad.File, "edges.txt.gz") {
		t.Fatalf("truncated edges: %v", err)
	}
}

func TestReleaseNames(t *testing.T) {
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if got := ReleaseEnding(end); len(got) != 1 || got[0].Name != "cc-main-2026-aug-sep-oct" {
		t.Fatalf("%+v", got)
	}
	jan := ReleaseEnding(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if len(jan) != 2 || jan[0].Name != "cc-main-2025-26-nov-dec-jan" || jan[1].Name != "cc-main-2025-nov-dec-jan" {
		t.Fatalf("%+v", jan)
	}
	for _, n := range []string{"cc-main-2026-aug-sep-oct", "cc-main-2025-26-nov-dec-jan", "cc-main-2025-26-dec-jan-feb", "cc-main-2026-may-jun-jul"} {
		r, ok := ParseRelease(n)
		if !ok || r.Name != n {
			t.Errorf("ParseRelease(%s) = %+v %v", n, r, ok)
		}
	}
	if r, _ := ParseRelease("cc-main-2025-26-dec-jan-feb"); !r.EndMonth.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("end = %v", r.EndMonth)
	}
	for _, n := range []string{"cc-main-2026", "x-2026-aug-sep-oct", "cc-main-2026-aug-sep-foo", "cc-main-2026-sep-oct-nov-x"} {
		if _, ok := ParseRelease(n); ok {
			t.Errorf("%s parsed", n)
		}
	}
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	c := Candidates(end, now)
	if len(c) != 0 {
		t.Fatalf("nothing can follow aug-sep-oct before December: %+v", c)
	}
	c = Candidates(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), now)
	if len(c) != 3 || c[0].Name != "cc-main-2026-aug-sep-oct" || c[2].Name != "cc-main-2026-jun-jul-aug" {
		t.Fatalf("%+v", c)
	}
	if c := Candidates(time.Time{}, now); len(c) != 10 || c[0].Name != "cc-main-2026-aug-sep-oct" {
		t.Fatalf("first run: %+v", c)
	}
	p := PreviousCandidates(ReleaseEnding(end)[0])
	if p[0].Name != "cc-main-2026-may-jun-jul" {
		t.Fatalf("%+v", p)
	}
	if got := FileURL("cc-main-2026-aug-sep-oct", "edges"); !strings.HasSuffix(got, "/cc-main-2026-aug-sep-oct/domain/cc-main-2026-aug-sep-oct-domain-edges.txt.gz") {
		t.Fatal(got)
	}
}

// fileServer serves files with ranges and an ETag, and can fail on demand.
type fileServer struct {
	files map[string][]byte
	etag  string
	fail  atomic.Int32 // status to answer once, then clear
	gets  atomic.Int32
	heads atomic.Int32
}

func (s *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if code := s.fail.Swap(0); code != 0 {
		w.WriteHeader(int(code))
		return
	}
	if r.Method == http.MethodHead {
		s.heads.Add(1)
	} else {
		s.gets.Add(1)
	}
	b, ok := s.files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("ETag", s.etag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
}

func TestFetchResumesAndBacksOff(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 64<<10) // 1 MiB
	srv := &fileServer{files: map[string][]byte{"/f.gz": data}, etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "f.gz")
	f := &Fetcher{BytesPerSec: 4 << 20}

	// 1 MiB at 4 MiB/s needs 250 ms: a 60 ms slot stops early.
	err := f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(60*time.Millisecond))
	if !errors.Is(err, ErrDeadline) {
		t.Fatalf("first slot: %v", err)
	}
	st, err := os.Stat(dest + ".part")
	if err != nil || st.Size() == 0 || st.Size() >= int64(len(data)) {
		t.Fatalf("partial file: %v %v", st, err)
	}

	// A server error stops the run without losing the partial file.
	srv.fail.Store(503)
	if err := f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(time.Minute)); !errors.Is(err, ErrBackoff) {
		t.Fatalf("503: %v", err)
	}
	if st2, _ := os.Stat(dest + ".part"); st2.Size() != st.Size() {
		t.Fatal("partial file changed on a 503")
	}

	// The next slot resumes from the same byte and completes.
	if err := f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file differs")
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal(".part left behind")
	}
	// A complete file is not asked for again.
	before := srv.heads.Load() + srv.gets.Load()
	if err := f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(time.Minute)); err != nil || srv.heads.Load()+srv.gets.Load() != before {
		t.Fatal("complete file fetched again")
	}
}

func TestFetchRestartsOnNewVersion(t *testing.T) {
	srv := &fileServer{files: map[string][]byte{"/f.gz": bytes.Repeat([]byte("a"), 1<<20)}, etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "f.gz")
	f := &Fetcher{BytesPerSec: 4 << 20}
	f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(50*time.Millisecond))
	// The server now holds another version: appending would mix the two.
	v2 := bytes.Repeat([]byte("b"), 1<<19)
	srv.files["/f.gz"], srv.etag = v2, `"v2"`
	if err := f.Fetch(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, v2) {
		t.Fatal("old bytes kept after a new version")
	}
}

func TestStoreUpdate(t *testing.T) {
	cur, prev := "cc-main-2026-aug-sep-oct", "cc-main-2026-may-jun-jul"
	files := map[string][]byte{}
	put := func(release, vertices, edges, ranks string) {
		for name, text := range map[string]string{"vertices": vertices, "edges": edges, "ranks": ranks} {
			files[fmt.Sprintf("/%s/domain/%s-domain-%s.txt.gz", release, release, name)] = gz(t, text)
		}
	}
	put(cur, sampleVertices, sampleEdges, sampleRanks)
	// Last quarter: blog.com did not link to sel-tech.fr yet, shop.fr did.
	put(prev, sampleVertices, "0\t3\n1\t2\n2\t3\n4\t3\n5\t3\n", sampleRanks)
	srv := &fileServer{files: files, etag: `"x"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	old := BaseURL
	BaseURL = ts.URL
	defer func() { BaseURL = old }()

	now := time.Date(2026, 11, 24, 19, 45, 0, 0, time.UTC)
	s := &Store{Dir: t.TempDir(), Fetcher: &Fetcher{}, Builder: &Builder{}, Pause: -1, Now: func() time.Time { return now }}
	if _, _, err := s.Open(); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("empty store: %v", err)
	}
	// An error status at discovery pauses the store.
	s.Retries = -1
	srv.fail.Store(503)
	if out, err := s.Update(context.Background(), now.Add(time.Hour)); out != Backoff || err != nil {
		t.Fatalf("503: %v %v", out, err)
	}
	if out, _ := s.Update(context.Background(), now.Add(time.Hour)); out != Backoff {
		t.Fatal("pause not kept")
	}
	now = now.Add(2 * time.Hour)
	steps := []Outcome{}
	for i := 0; i < 5; i++ {
		out, err := s.Update(context.Background(), time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, out)
		if out == Idle {
			break
		}
	}
	if fmt.Sprint(steps) != "[built built idle]" {
		t.Fatalf("steps = %v", steps)
	}
	if r := s.Ready(); len(r) != 2 || r[0].Name != cur || r[1].Name != prev {
		t.Fatalf("ready = %+v", r)
	}
	if _, err := os.Stat(s.downloadDir(cur)); !os.IsNotExist(err) {
		t.Fatal("download kept after the build")
	}
	c, p, err := s.Open()
	if err != nil || p == nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer p.Close()
	if c.Meta.Release != cur || p.Meta.Release != prev {
		t.Fatalf("pair = %s %s", c.Meta.Release, p.Meta.Release)
	}
	if !strings.Contains(s.Status(), "ready: "+cur) {
		t.Fatal(s.Status())
	}
	// A day later nothing new: one look, no download.
	gets := srv.gets.Load()
	now = now.Add(25 * time.Hour)
	if out, _ := s.Update(context.Background(), time.Now().Add(time.Minute)); out != Idle || srv.gets.Load() != gets {
		t.Fatalf("idle day: %v", out)
	}
	// An old release lying around is removed.
	os.MkdirAll(s.indexDir("cc-main-2026-feb-mar-apr"), 0o750)
	os.WriteFile(filepath.Join(s.indexDir("cc-main-2026-feb-mar-apr"), metaFile), []byte("{}"), 0o640)
	s.Update(context.Background(), time.Now().Add(time.Minute))
	if _, err := os.Stat(s.releaseDir("cc-main-2026-feb-mar-apr")); !os.IsNotExist(err) {
		t.Fatal("old release kept")
	}
}

func TestLock(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lock(); err == nil {
		t.Fatal("second lock taken")
	}
	unlock()
	if u, err := s.Lock(); err != nil {
		t.Fatal(err)
	} else {
		u()
	}
}

func TestRetryWithinSlot(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 1<<16)
	srv := &fileServer{files: map[string][]byte{"/f.gz": data}, etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	var logs []string
	s := &Store{Fetcher: &Fetcher{}, RetryWait: 10 * time.Millisecond, Log: func(f string, a ...interface{}) { logs = append(logs, fmt.Sprintf(f, a...)) }}
	dest := filepath.Join(t.TempDir(), "f.gz")
	srv.fail.Store(503)
	if err := s.fetchWithRetries(context.Background(), ts.URL+"/f.gz", dest, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("one refusal then success: %v", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "trying again") {
		t.Fatalf("logs = %v", logs)
	}
	// No time left in the slot for a new try: the refusal is returned at once.
	dest2 := filepath.Join(t.TempDir(), "f.gz")
	srv.fail.Store(503)
	s.RetryWait = time.Hour
	if err := s.fetchWithRetries(context.Background(), ts.URL+"/f.gz", dest2, time.Now().Add(time.Minute)); !errors.Is(err, ErrBackoff) {
		t.Fatalf("no time to retry: %v", err)
	}
}
