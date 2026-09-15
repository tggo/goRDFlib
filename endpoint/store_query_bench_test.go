package endpoint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

// BenchmarkStoreBackendQuery runs one selective two-pattern query through the
// handler (no network) over 20 000 triples on each store, the shape of the
// sparql-server throughput test.
func BenchmarkStoreBackendQuery(b *testing.B) {
	stores := map[string]func(b *testing.B) store.Store{
		"memory": func(*testing.B) store.Store { return store.NewMemoryStore() },
		"badger": func(b *testing.B) store.Store {
			s, err := badgerstore.New(badgerstore.WithDir(b.TempDir()))
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { s.Close() })
			return s
		},
		"sqlite": func(b *testing.B) store.Store {
			s, err := sqlitestore.New(sqlitestore.WithFile(b.TempDir() + "/s.db"))
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { s.Close() })
			return s
		},
	}
	name := term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/name")
	age := term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/age")
	q := "/?" + url.Values{"query": {`SELECT ?name WHERE { ?p <http://xmlns.com/foaf/0.1/age> 42 ; <http://xmlns.com/foaf/0.1/name> ?name }`}}.Encode()
	for _, kind := range []string{"memory", "badger", "sqlite"} {
		b.Run(kind, func(b *testing.B) {
			st := stores[kind](b)
			ds := graph.NewDataset(graph.WithStore(st))
			def := ds.DefaultContext()
			for i := range 10_000 {
				p := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i))
				def.Add(p, name, term.NewLiteral(fmt.Sprintf("Person %d", i)))
				def.Add(p, age, term.NewLiteral(i%90))
			}
			h := NewForStore(ds)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req := httptest.NewRequest(http.MethodGet, q, nil)
					req.Header.Set("Accept", "application/sparql-results+json")
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, req)
					if rec.Code != 200 {
						b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
				}
			})
		})
	}
}
