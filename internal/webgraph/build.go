package webgraph

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Index files, all little endian:
//
//	names.bin    reversed domain names ("fr.sel-tech"), concatenated, sorted bytewise
//	names.idx    uint64 offsets of each name in names.bin, n+1 entries
//	ranks.bin    uint32 harmonic centrality position of each domain (1 = most central, 0 = unranked)
//	offsets.bin  uint64 position of each domain's first referrer in sources.bin, n+1 entries
//	sources.bin  uint32 ids of the domains linking to each domain, grouped by target
//	meta.json    written last: an index without it is incomplete
const (
	FormatVersion = 1
	metaFile      = "meta.json"
)

// Meta describes a built index.
type Meta struct {
	Format   int       `json:"format"`
	Release  string    `json:"release"`
	Nodes    uint32    `json:"nodes"`
	Arcs     uint64    `json:"arcs"`
	Ranked   uint32    `json:"ranked"`
	BuiltAt  time.Time `json:"built_at"`
	Duration string    `json:"duration"`
}

// Builder converts the three downloaded files of a release into an index.
type Builder struct {
	// ArcsPerBucket bounds the memory of the inversion: the referrers of one
	// bucket of targets are placed in RAM, 4 bytes each.
	ArcsPerBucket uint64
	Log           func(format string, args ...interface{})
}

func (b *Builder) logf(format string, args ...interface{}) {
	if b.Log != nil {
		b.Log(format, args...)
	}
}

// ErrCorrupt names a downloaded file that cannot be read to its end; the
// caller deletes it so that it is downloaded again.
type ErrCorrupt struct {
	File string
	Err  error
}

func (e *ErrCorrupt) Error() string { return fmt.Sprintf("webgraph: %s: %v", e.File, e.Err) }
func (e *ErrCorrupt) Unwrap() error { return e.Err }

// Build reads src/{vertices,edges,ranks}.txt.gz and writes the index to dst,
// through dst+".tmp" so that a crash never leaves a half index in place.
func (b *Builder) Build(release, src, dst string) (*Meta, error) {
	began := time.Now()
	tmp := dst + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return nil, err
	}
	in := func(name string) string { return filepath.Join(src, name+".txt.gz") }

	n, err := b.vertices(in("vertices"), tmp)
	if err != nil {
		return nil, err
	}
	b.logf("vertices: %d domains", n)
	names, err := openNames(tmp)
	if err != nil {
		return nil, err
	}
	ranked, err := b.ranks(in("ranks"), tmp, names, n)
	names.close()
	if err != nil {
		return nil, err
	}
	b.logf("ranks: %d domains ranked", ranked)
	arcs, err := b.edges(in("edges"), tmp, n)
	if err != nil {
		return nil, err
	}
	b.logf("edges: %d links", arcs)

	meta := &Meta{Format: FormatVersion, Release: release, Nodes: n, Arcs: arcs, Ranked: ranked,
		BuiltAt: time.Now().UTC(), Duration: time.Since(began).Round(time.Second).String()}
	raw, _ := json.MarshalIndent(meta, "", " ")
	if err := os.WriteFile(filepath.Join(tmp, metaFile), raw, 0o640); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(dst); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return nil, err
	}
	return meta, nil
}

// lineReader reads the lines of a gzip file.
type lineReader struct {
	path string
	f    *os.File
	gz   *gzip.Reader
	r    *bufio.Reader
	line uint64
}

func openLines(path string) (*lineReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bufio.NewReaderSize(f, 4<<20))
	if err != nil {
		f.Close()
		return nil, &ErrCorrupt{File: path, Err: err}
	}
	return &lineReader{path: path, f: f, gz: gz, r: bufio.NewReaderSize(gz, 4<<20)}, nil
}

// next returns the next line without its newline; io.EOF at the end.
func (l *lineReader) next() ([]byte, error) {
	line, err := l.r.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		return nil, fmt.Errorf("%s line %d: line too long", l.path, l.line+1)
	}
	if err != nil && err != io.EOF {
		return nil, &ErrCorrupt{File: l.path, Err: err}
	}
	if err == io.EOF && len(line) == 0 {
		return nil, io.EOF
	}
	l.line++
	return bytes.TrimRight(line, "\r\n"), nil
}

func (l *lineReader) close() { l.gz.Close(); l.f.Close() }

func parseUint(b []byte) (uint64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var v uint64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + uint64(c-'0')
	}
	return v, true
}

// field returns the i-th tab separated field of a line.
func field(line []byte, i int) []byte {
	for ; i > 0; i-- {
		t := bytes.IndexByte(line, '\t')
		if t < 0 {
			return nil
		}
		line = line[t+1:]
	}
	if t := bytes.IndexByte(line, '\t'); t >= 0 {
		return line[:t]
	}
	return line
}

func (b *Builder) vertices(path, dir string) (uint32, error) {
	lr, err := openLines(path)
	if err != nil {
		return 0, err
	}
	defer lr.close()
	nf, err := os.Create(filepath.Join(dir, "names.bin"))
	if err != nil {
		return 0, err
	}
	defer nf.Close()
	xf, err := os.Create(filepath.Join(dir, "names.idx"))
	if err != nil {
		return 0, err
	}
	defer xf.Close()
	nw, xw := bufio.NewWriterSize(nf, 4<<20), bufio.NewWriterSize(xf, 4<<20)
	var off uint64
	var id uint64
	var prev []byte
	var u8 [8]byte
	for {
		line, err := lr.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		got, ok := parseUint(field(line, 0))
		name := field(line, 1)
		if !ok || len(name) == 0 {
			return 0, fmt.Errorf("%s line %d: unreadable vertex", path, lr.line)
		}
		if got != id {
			return 0, fmt.Errorf("%s line %d: id %d where %d was expected", path, lr.line, got, id)
		}
		if prev != nil && bytes.Compare(name, prev) <= 0 {
			return 0, fmt.Errorf("%s line %d: names are not sorted", path, lr.line)
		}
		if id >= 1<<32-1 {
			return 0, errors.New("webgraph: more domains than a uint32 holds")
		}
		binary.LittleEndian.PutUint64(u8[:], off)
		xw.Write(u8[:])
		nw.Write(name)
		off += uint64(len(name))
		prev = append(prev[:0], name...)
		id++
	}
	binary.LittleEndian.PutUint64(u8[:], off)
	xw.Write(u8[:])
	if err := nw.Flush(); err != nil {
		return 0, err
	}
	if err := xw.Flush(); err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("%s: no vertex", path)
	}
	return uint32(id), nil
}

func (b *Builder) ranks(path, dir string, names *nameTable, n uint32) (uint32, error) {
	lr, err := openLines(path)
	if err != nil {
		return 0, err
	}
	defer lr.close()
	ranks := make([]uint32, n)
	var ranked uint32
	for {
		line, err := lr.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		pos, ok := parseUint(field(line, 0))
		name := field(line, 4)
		if !ok || len(name) == 0 || pos == 0 || pos >= 1<<32 {
			return 0, fmt.Errorf("%s line %d: unreadable rank", path, lr.line)
		}
		id, found := names.find(name)
		if !found {
			continue
		}
		if ranks[id] == 0 {
			ranked++
		}
		ranks[id] = uint32(pos)
	}
	return ranked, writeUint32s(filepath.Join(dir, "ranks.bin"), ranks)
}

func writeUint32s(path string, v []uint32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 4<<20)
	var u4 [4]byte
	for _, x := range v {
		binary.LittleEndian.PutUint32(u4[:], x)
		w.Write(u4[:])
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// edges inverts the link list (sorted by source) into referrers grouped by
// target, in two passes: count the referrers of each target, then spread
// the links into buckets of targets small enough to be placed in RAM.
func (b *Builder) edges(path, dir string, n uint32) (uint64, error) {
	counts := make([]uint32, n)
	var arcs uint64
	err := eachEdge(path, n, func(from, to uint32) error {
		counts[to]++
		arcs++
		return nil
	})
	if err != nil {
		return 0, err
	}
	b.logf("edges: counted %d links", arcs)

	// Offsets of each target, and the bucket boundaries.
	per := b.ArcsPerBucket
	if per == 0 {
		per = 64 << 20
	}
	of, err := os.Create(filepath.Join(dir, "offsets.bin"))
	if err != nil {
		return 0, err
	}
	ow := bufio.NewWriterSize(of, 4<<20)
	var u8 [8]byte
	var off uint64
	lows := []uint32{0}
	bases := []uint64{0}
	for id := uint32(0); id < n; id++ {
		if off-bases[len(bases)-1] >= per && counts[id] > 0 {
			lows = append(lows, id)
			bases = append(bases, off)
		}
		binary.LittleEndian.PutUint64(u8[:], off)
		ow.Write(u8[:])
		off += uint64(counts[id])
	}
	binary.LittleEndian.PutUint64(u8[:], off)
	ow.Write(u8[:])
	if err := ow.Flush(); err != nil {
		return 0, err
	}
	of.Close()
	bases = append(bases, off)
	nb := len(lows)
	b.logf("edges: %d buckets", nb)

	// Pass 2: spread (target, source) pairs into one file per bucket.
	bdir := filepath.Join(dir, "buckets")
	if err := os.MkdirAll(bdir, 0o750); err != nil {
		return 0, err
	}
	files := make([]*os.File, nb)
	writers := make([]*bufio.Writer, nb)
	for i := range files {
		if files[i], err = os.Create(filepath.Join(bdir, fmt.Sprintf("%04d", i))); err != nil {
			return 0, err
		}
		writers[i] = bufio.NewWriterSize(files[i], 1<<20)
	}
	var seen uint64
	err = eachEdge(path, n, func(from, to uint32) error {
		i := sort.Search(nb, func(k int) bool { return lows[k] > to }) - 1
		var pair [8]byte
		binary.LittleEndian.PutUint32(pair[0:], to)
		binary.LittleEndian.PutUint32(pair[4:], from)
		_, err := writers[i].Write(pair[:])
		seen++
		return err
	})
	if err != nil {
		return 0, err
	}
	if seen != arcs {
		return 0, &ErrCorrupt{File: path, Err: fmt.Errorf("second pass read %d links, first %d", seen, arcs)}
	}
	for i := range files {
		if err := writers[i].Flush(); err != nil {
			return 0, err
		}
		files[i].Close()
	}

	// Place each bucket in RAM, then append it to sources.bin.
	sf, err := os.Create(filepath.Join(dir, "sources.bin"))
	if err != nil {
		return 0, err
	}
	defer sf.Close()
	sw := bufio.NewWriterSize(sf, 4<<20)
	for i := 0; i < nb; i++ {
		lo := lows[i]
		hi := n
		if i+1 < nb {
			hi = lows[i+1]
		}
		size := bases[i+1] - bases[i]
		dst := make([]uint32, size)
		cursor := make([]uint32, hi-lo)
		var acc uint32
		for id := lo; id < hi; id++ {
			cursor[id-lo] = acc
			acc += counts[id]
		}
		bp := filepath.Join(bdir, fmt.Sprintf("%04d", i))
		f, err := os.Open(bp)
		if err != nil {
			return 0, err
		}
		r := bufio.NewReaderSize(f, 4<<20)
		var pair [8]byte
		var placed uint64
		for {
			if _, err := io.ReadFull(r, pair[:]); err == io.EOF {
				break
			} else if err != nil {
				f.Close()
				return 0, err
			}
			to := binary.LittleEndian.Uint32(pair[0:])
			from := binary.LittleEndian.Uint32(pair[4:])
			c := &cursor[to-lo]
			dst[*c] = from
			*c++
			placed++
		}
		f.Close()
		if placed != size {
			return 0, fmt.Errorf("webgraph: bucket %d holds %d links, %d expected", i, placed, size)
		}
		var u4 [4]byte
		for _, x := range dst {
			binary.LittleEndian.PutUint32(u4[:], x)
			sw.Write(u4[:])
		}
		os.Remove(bp)
	}
	if err := sw.Flush(); err != nil {
		return 0, err
	}
	os.Remove(bdir)
	return arcs, sf.Close()
}

func eachEdge(path string, n uint32, fn func(from, to uint32) error) error {
	lr, err := openLines(path)
	if err != nil {
		return err
	}
	defer lr.close()
	for {
		line, err := lr.next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		t := bytes.IndexByte(line, '\t')
		if t < 0 {
			return fmt.Errorf("%s line %d: unreadable link", path, lr.line)
		}
		from, ok1 := parseUint(line[:t])
		to, ok2 := parseUint(line[t+1:])
		if !ok1 || !ok2 || from >= uint64(n) || to >= uint64(n) {
			return fmt.Errorf("%s line %d: unreadable link", path, lr.line)
		}
		if err := fn(uint32(from), uint32(to)); err != nil {
			return err
		}
	}
}
