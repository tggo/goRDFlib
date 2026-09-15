package endpoint_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

func benchGraph(n int) *graph.Graph {
	g := graph.NewGraph()
	p := iri(ex + "p")
	for i := range n {
		g.Add(iri(fmt.Sprintf("%ss%d", ex, i)), p, term.NewLiteral(fmt.Sprintf("value %d", i)))
	}
	return g
}

const benchQuery = `SELECT ?s ?o WHERE { ?s <http://example.org/p> ?o }`

// discard is a ResponseWriter that throws the body away, so the benchmark
// measures the handler rather than a growing recorder buffer.
type discard struct{ h http.Header }

func (d *discard) Header() http.Header         { return d.h }
func (d *discard) Write(p []byte) (int, error) { return len(p), nil }
func (d *discard) WriteHeader(int)             {}

// BenchmarkSelectHandler runs a SELECT through ServeHTTP directly. Compare
// with BenchmarkSelectEngineOnly: the difference in allocs/op must not grow
// with the number of rows (it is a constant ~45 allocations, measured when
// this was written), because the handler and the results writer add no
// per-row allocation to the engine's.
func BenchmarkSelectHandler(b *testing.B) {
	for _, rows := range []int{1_000, 10_000, 100_000} {
		h := endpoint.New(&sparql.Dataset{Default: benchGraph(rows)})
		for _, accept := range []string{"application/sparql-results+json", "application/sparql-results+xml", "text/tab-separated-values"} {
			name := fmt.Sprintf("rows=%d/%s", rows, accept[strings.LastIndexAny(accept, "+/")+1:])
			b.Run(name, func(b *testing.B) {
				req := httptest.NewRequest(http.MethodGet, "/?query="+url.QueryEscape(benchQuery), nil)
				req.Header.Set("Accept", accept)
				w := &discard{h: http.Header{}}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					clear(w.h)
					h.ServeHTTP(w, req)
				}
				b.StopTimer()
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(rows), "ns/row")
			})
		}
	}
}

// BenchmarkSelectHTTP adds a real loopback HTTP connection and the client
// reading the whole body.
func BenchmarkSelectHTTP(b *testing.B) {
	const rows = 100_000
	ts := httptest.NewServer(endpoint.New(&sparql.Dataset{Default: benchGraph(rows)}))
	defer ts.Close()
	u := ts.URL + "?query=" + url.QueryEscape(benchQuery)
	var size int64
	b.ReportAllocs()
	for b.Loop() {
		resp, err := http.Get(u)
		if err != nil {
			b.Fatal(err)
		}
		size, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	b.ReportMetric(float64(size)/rows, "bytes/row")
}

// BenchmarkSelectEngineOnly is the baseline for BenchmarkSelectHandler: the
// same query evaluated without HTTP or serialization.
func BenchmarkSelectEngineOnly(b *testing.B) {
	for _, rows := range []int{1_000, 100_000} {
		g := benchGraph(rows)
		q, err := sparql.Parse(benchQuery)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := sparql.EvalQuery(g, q, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
