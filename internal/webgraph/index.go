package webgraph

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

// ErrNoIndex means no complete index is available.
var ErrNoIndex = errors.New("webgraph: no complete index")

type mapped struct{ data []byte }

func mapFile(path string) (*mapped, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() == 0 {
		return &mapped{}, nil
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, int(st.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("webgraph: mmap %s: %w", path, err)
	}
	return &mapped{data: data}, nil
}

func (m *mapped) close() {
	if m != nil && len(m.data) > 0 {
		syscall.Munmap(m.data)
		m.data = nil
	}
}

func u32s(b []byte) []uint32 {
	if len(b) < 4 {
		return nil
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(&b[0])), len(b)/4)
}

// nameTable finds a domain id from its reversed name.
type nameTable struct {
	names, idx *mapped
	n          int
}

func openNames(dir string) (*nameTable, error) {
	names, err := mapFile(filepath.Join(dir, "names.bin"))
	if err != nil {
		return nil, err
	}
	idx, err := mapFile(filepath.Join(dir, "names.idx"))
	if err != nil {
		names.close()
		return nil, err
	}
	if len(idx.data) < 8 || len(idx.data)%8 != 0 {
		names.close()
		idx.close()
		return nil, fmt.Errorf("webgraph: %s: bad names.idx", dir)
	}
	return &nameTable{names: names, idx: idx, n: len(idx.data)/8 - 1}, nil
}

func (t *nameTable) close() { t.names.close(); t.idx.close() }

func (t *nameTable) name(id uint32) []byte {
	a := binary.LittleEndian.Uint64(t.idx.data[int(id)*8:])
	b := binary.LittleEndian.Uint64(t.idx.data[int(id)*8+8:])
	return t.names.data[a:b]
}

func (t *nameTable) find(rev []byte) (uint32, bool) {
	i := sort.Search(t.n, func(i int) bool { return bytes.Compare(t.name(uint32(i)), rev) >= 0 })
	if i < t.n && bytes.Equal(t.name(uint32(i)), rev) {
		return uint32(i), true
	}
	return 0, false
}

// Index is an opened, read-only index of one release.
type Index struct {
	Meta    Meta
	names   *nameTable
	ranks   *mapped
	offsets *mapped
	sources *mapped
}

// Open maps the index stored in dir.
func Open(dir string) (*Index, error) {
	raw, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return nil, ErrNoIndex
	}
	ix := &Index{}
	if err := json.Unmarshal(raw, &ix.Meta); err != nil || ix.Meta.Format != FormatVersion {
		return nil, fmt.Errorf("webgraph: %s: unknown index format", dir)
	}
	if ix.names, err = openNames(dir); err != nil {
		return nil, err
	}
	for _, f := range []struct {
		name string
		dst  **mapped
	}{{"ranks.bin", &ix.ranks}, {"offsets.bin", &ix.offsets}, {"sources.bin", &ix.sources}} {
		if *f.dst, err = mapFile(filepath.Join(dir, f.name)); err != nil {
			ix.Close()
			return nil, err
		}
	}
	n := uint64(ix.Meta.Nodes)
	if uint64(ix.names.n) != n || uint64(len(ix.ranks.data)) != 4*n || uint64(len(ix.offsets.data)) != 8*(n+1) || uint64(len(ix.sources.data)) != 4*ix.Meta.Arcs {
		ix.Close()
		return nil, fmt.Errorf("webgraph: %s: file sizes do not match meta.json", dir)
	}
	return ix, nil
}

// Close unmaps the files.
func (ix *Index) Close() {
	if ix == nil {
		return
	}
	if ix.names != nil {
		ix.names.close()
	}
	ix.ranks.close()
	ix.offsets.close()
	ix.sources.close()
}

// Reverse turns "sel-tech.fr" into "fr.sel-tech", the form of the graph.
func Reverse(domain string) string {
	parts := strings.Split(strings.Trim(strings.ToLower(domain), "."), ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}

// Lookup returns the id of a domain ("sel-tech.fr").
func (ix *Index) Lookup(domain string) (uint32, bool) {
	if domain == "" {
		return 0, false
	}
	return ix.names.find([]byte(Reverse(domain)))
}

// Domain returns the name of a domain id, in the usual order.
func (ix *Index) Domain(id uint32) string { return Reverse(string(ix.names.name(id))) }

// Rank returns the harmonic centrality position of a domain, 1 being the
// most central; 0 when Common Crawl did not rank it.
func (ix *Index) Rank(id uint32) uint32 { return u32s(ix.ranks.data)[id] }

// Referrers returns the ids of the domains linking to id. The slice points
// into the mapped file: never modify it.
func (ix *Index) Referrers(id uint32) []uint32 {
	a := binary.LittleEndian.Uint64(ix.offsets.data[int(id)*8:])
	b := binary.LittleEndian.Uint64(ix.offsets.data[int(id)*8+8:])
	if a == b {
		return nil
	}
	return u32s(ix.sources.data)[a:b]
}

// InDegree is the number of domains linking to id.
func (ix *Index) InDegree(id uint32) int {
	a := binary.LittleEndian.Uint64(ix.offsets.data[int(id)*8:])
	b := binary.LittleEndian.Uint64(ix.offsets.data[int(id)*8+8:])
	return int(b - a)
}

// Ranked is one domain with its rank.
type Ranked struct {
	ID     uint32
	Domain string
	Rank   uint32 // 0: unranked
}

// TopReferrers returns the k most central domains linking to id.
func (ix *Index) TopReferrers(id uint32, k int) []Ranked {
	return ix.TopOf(ix.Referrers(id), k)
}

// TopOf returns the k most central of a list of ids (unranked last).
func (ix *Index) TopOf(ids []uint32, k int) []Ranked {
	ranks := u32s(ix.ranks.data)
	key := func(id uint32) uint64 {
		if r := ranks[id]; r > 0 {
			return uint64(r)
		}
		return 1 << 40
	}
	sel := append([]uint32(nil), ids...)
	sort.Slice(sel, func(i, j int) bool {
		a, b := key(sel[i]), key(sel[j])
		if a != b {
			return a < b
		}
		return sel[i] < sel[j]
	})
	if len(sel) > k {
		sel = sel[:k]
	}
	out := make([]Ranked, 0, len(sel))
	for _, id := range sel {
		out = append(out, Ranked{ID: id, Domain: ix.Domain(id), Rank: ranks[id]})
	}
	return out
}
