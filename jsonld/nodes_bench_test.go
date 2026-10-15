package jsonld

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// benchNodes is issue #53's input: newline-delimited node objects sharing
// one context.
func benchNodes(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(`{"@id": "ex:n%d", "@type": "Person", "name": "Name %d", "age": "%d", "knows": "ex:n%d", "address": {"streetAddress": "%d Main St"}}`, i, i, i%90, (i+1)%n, i)
	}
	return out
}

// peakHeap samples HeapAlloc every 4096 triples; it is what the benchmark
// reports as peak-heap-MB.
type peakHeap struct {
	n    int
	peak uint64
}

func (p *peakHeap) handler(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error {
	if p.n++; p.n%4096 == 0 {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		p.peak = max(p.peak, ms.HeapAlloc)
	}
	return nil
}

func BenchmarkParseNodes(b *testing.B) {
	const n = 50_000
	nodes := benchNodes(n)
	ndjson := []byte(strings.Join(nodes, "\n"))
	whole := []byte(`{"@context": ` + nodesContext + `, "@graph": [` + strings.Join(nodes, ",") + `]}`)

	run := func(b *testing.B, f func(h TripleHandler) error) {
		b.ReportAllocs()
		var ph peakHeap
		for b.Loop() {
			runtime.GC()
			ph = peakHeap{}
			if err := f(ph.handler); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(ph.peak)/(1<<20), "peak-heap-MB")
		b.ReportMetric(float64(ph.n), "triples")
	}
	b.Run("whole-document", func(b *testing.B) {
		run(b, func(h TripleHandler) error { return ParseStream(bytes.NewReader(whole), h) })
	})
	b.Run("batches-of-1000", func(b *testing.B) {
		run(b, func(h TripleHandler) error {
			for i := 0; i < n; i += 1000 {
				doc := `{"@context": ` + nodesContext + `, "@graph": [` + strings.Join(nodes[i:i+1000], ",") + `]}`
				if err := ParseStream(strings.NewReader(doc), h); err != nil {
					return err
				}
			}
			return nil
		})
	})
	b.Run("ParseNodes", func(b *testing.B) {
		run(b, func(h TripleHandler) error {
			return ParseNodesStream(b.Context(), bytes.NewReader(ndjson), json.RawMessage(nodesContext), h)
		})
	})
}
