package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

// Store stress measurements at 3M triples, the numbers in the README.
//
// Ingest time and RAM are one-shot by nature and come from TestStress3M.
// Reads come from BenchmarkStress3M, which loads each backend once per process
// and repeats every read under testing.B: a single timed loop right after an
// 8 GB allocation measures GC pauses as much as the code, and gave numbers that
// differed by 47x between runs of the same Len().
//
//	go test ./store/ -run TestStress3M -v -count=1
//	go test ./store/ -run '^$' -bench BenchmarkStress3M -count 6 | tee new.txt

const (
	stressN     = 3_000_000
	stressBatch = 50_000
)

// Fixed probes into genQuads data: s500000 has one triple, and every 200th
// triple uses p42.
var (
	stressSubject   = term.NewURIRefUnsafe("http://example.org/s500000")
	stressPredicate = term.NewURIRefUnsafe("http://example.org/p42")
)

func genQuads(n int) []term.Quad {
	quads := make([]term.Quad, n)
	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%200))
		obj := term.NewLiteral(fmt.Sprintf("value_%d", i))
		quads[i] = term.Quad{Triple: term.Triple{Subject: subj, Predicate: pred, Object: obj}}
	}
	return quads
}

func heapMB() float64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc) / 1024 / 1024
}

type stressBackend struct {
	name string
	// inProcess marks backends whose data lives in the Go heap, where the
	// heap delta is a meaningful RAM figure.
	inProcess bool
	open      func(dir string) (s store.Store, closeFn func(), err error)
}

var stressBackends = []stressBackend{
	{"Memory", true, func(string) (store.Store, func(), error) {
		return store.NewMemoryStore(), func() {}, nil
	}},
	{"Badger", true, func(string) (store.Store, func(), error) {
		s, err := badgerstore.New(badgerstore.WithInMemory())
		return s, closer(s), err
	}},
	{"BadgerDisk", false, func(dir string) (store.Store, func(), error) {
		s, err := badgerstore.New(badgerstore.WithDir(dir))
		return s, closer(s), err
	}},
	{"SQLite", true, func(string) (store.Store, func(), error) {
		s, err := sqlitestore.New(sqlitestore.WithInMemory())
		return s, closer(s), err
	}},
	{"SQLiteDisk", false, func(dir string) (store.Store, func(), error) {
		s, err := sqlitestore.New(sqlitestore.WithFile(filepath.Join(dir, "stress.db")))
		return s, closer(s), err
	}},
}

func closer(s interface{ Close() error }) func() {
	return func() {
		if s != nil {
			_ = s.Close()
		}
	}
}

func ingestStress(s store.Store, quads []term.Quad) {
	for i := 0; i < len(quads); i += stressBatch {
		s.AddN(quads[i:min(i+stressBatch, len(quads))])
	}
}

var (
	stressQuadsOnce sync.Once
	stressQuads     []term.Quad
)

func sharedStressQuads() []term.Quad {
	stressQuadsOnce.Do(func() { stressQuads = genQuads(stressN) })
	return stressQuads
}

func TestStress3M(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	quads := sharedStressQuads()
	for _, bk := range stressBackends {
		t.Run(bk.name, func(t *testing.T) {
			before := heapMB()
			s, closeFn, err := bk.open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer closeFn()

			start := time.Now()
			ingestStress(s, quads)
			took := time.Since(start)
			t.Logf("%s ingest: %d triples in %v (%.0fK triples/s)",
				bk.name, stressN, took.Round(time.Millisecond), float64(stressN)/took.Seconds()/1000)
			if bk.inProcess {
				t.Logf("%s heap delta: %.0f MB", bk.name, heapMB()-before)
			}
			runtime.KeepAlive(s)
			if n := s.Len(nil); n != stressN {
				t.Fatalf("Len = %d after ingest, want %d", n, stressN)
			}
		})
	}
}

// loadedStress is one backend holding the 3M triples, shared by every
// BenchmarkStress3M run in the process so -count does not reload it.
type loadedStress struct {
	once  sync.Once
	store store.Store
	err   error
}

var (
	loadedStressMu sync.Mutex
	loadedStressBy = map[string]*loadedStress{}
	stressRootDir  string
)

func loadStress(b *testing.B, bk stressBackend) store.Store {
	loadedStressMu.Lock()
	l := loadedStressBy[bk.name]
	if l == nil {
		l = &loadedStress{}
		loadedStressBy[bk.name] = l
	}
	loadedStressMu.Unlock()

	l.once.Do(func() {
		dir, err := os.MkdirTemp(stressRoot(), bk.name)
		if err != nil {
			l.err = err
			return
		}
		// Stores stay open until the process exits; TestMain removes the
		// directories.
		s, _, err := bk.open(dir)
		if err != nil {
			l.err = err
			return
		}
		ingestStress(s, sharedStressQuads())
		l.store = s
	})
	if l.err != nil {
		b.Fatal(l.err)
	}
	return l.store
}

func stressRoot() string {
	loadedStressMu.Lock()
	defer loadedStressMu.Unlock()
	if stressRootDir == "" {
		dir, err := os.MkdirTemp("", "rdflibgo-stress-")
		if err != nil {
			panic(err)
		}
		stressRootDir = dir
	}
	return stressRootDir
}

func TestMain(m *testing.M) {
	code := m.Run()
	if stressRootDir != "" {
		_ = os.RemoveAll(stressRootDir)
	}
	os.Exit(code)
}

func BenchmarkStress3M(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping stress benchmark in short mode")
	}
	for _, bk := range stressBackends {
		b.Run(bk.name, func(b *testing.B) {
			s := loadStress(b, bk)
			countMatches := func(p term.TriplePattern) int {
				n := 0
				for range s.Triples(p, nil) {
					n++
				}
				return n
			}
			reads := []struct {
				name string
				pat  term.TriplePattern
				want int
			}{
				{"SubjectLookup", term.TriplePattern{Subject: stressSubject}, 1},
				{"PredicateScan", term.TriplePattern{Predicate: &stressPredicate}, stressN / 200},
				{"FullScan", term.TriplePattern{}, stressN},
			}
			for _, r := range reads {
				b.Run(r.name, func(b *testing.B) {
					for b.Loop() {
						if n := countMatches(r.pat); n != r.want {
							b.Fatalf("%s matched %d triples, want %d", r.name, n, r.want)
						}
					}
				})
			}
			b.Run("Len", func(b *testing.B) {
				for b.Loop() {
					if n := s.Len(nil); n != stressN {
						b.Fatalf("Len = %d, want %d", n, stressN)
					}
				}
			})
		})
	}
}
