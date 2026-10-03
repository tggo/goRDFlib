package benchmarks_test

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/turtle"
)

// Streaming parse vs parsing into a graph (issue #45). The graph is the cost
// the stream exists to avoid: compare B/op between the two for each format.
// The handler does nothing, as a converter or counter that keeps no triples
// would.

const streamBenchTriples = 100_000

func streamBenchRDFXML() []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">` + "\n")
	for i := range streamBenchTriples / 2 {
		fmt.Fprintf(&b, `<rdf:Description rdf:about="http://example.org/s%d"><ex:name>name %d</ex:name><ex:next rdf:resource="http://example.org/s%d"/></rdf:Description>`+"\n", i, i, i+1)
	}
	b.WriteString("</rdf:RDF>\n")
	return []byte(b.String())
}

func streamBenchNT() []byte {
	var b strings.Builder
	for i := range streamBenchTriples {
		fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"value %d\" .\n", i, i)
	}
	return []byte(b.String())
}

func BenchmarkStreamParse(b *testing.B) {
	xml, ntData := streamBenchRDFXML(), streamBenchNT()
	discard3 := func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error { return nil }

	b.Run("rdfxml/graph", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := rdfxml.Parse(rdflibgo.NewGraph(), bytes.NewReader(xml)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("rdfxml/stream", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := rdfxml.ParseStream(bytes.NewReader(xml), discard3); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("nt/graph", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := nt.Parse(rdflibgo.NewGraph(), bytes.NewReader(ntData)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("nt/stream", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := nt.ParseStream(bytes.NewReader(ntData), discard3); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("turtle/graph", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := turtle.Parse(rdflibgo.NewGraph(), bytes.NewReader(ntData)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("turtle/stream", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := turtle.ParseStream(bytes.NewReader(ntData), discard3); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStreamPeakHeap reports peak-heap-MB: the live heap at its largest
// while parsing, sampled every 10k triples (and, for the graph, at the end,
// while the graph is still referenced). B/op above counts every allocation,
// including garbage; this is what has to fit in memory. The stream's peak
// should not grow with the input; the graph's does.
func BenchmarkStreamPeakHeap(b *testing.B) {
	xml := streamBenchRDFXML()
	b.Run("rdfxml/graph", func(b *testing.B) {
		for b.Loop() {
			var peak heapPeak
			peak.start()
			g := rdflibgo.NewGraph()
			if err := rdfxml.Parse(g, bytes.NewReader(xml)); err != nil {
				b.Fatal(err)
			}
			peak.sample()
			runtime.KeepAlive(g)
			b.ReportMetric(peak.mb(), "peak-heap-MB")
		}
	})
	b.Run("rdfxml/stream", func(b *testing.B) {
		for b.Loop() {
			var peak heapPeak
			peak.start()
			n := 0
			err := rdfxml.ParseStream(bytes.NewReader(xml), func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error {
				if n++; n%10_000 == 0 {
					peak.sample()
				}
				return nil
			})
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(peak.mb(), "peak-heap-MB")
		}
	})
}

// heapPeak tracks the largest live heap above a baseline taken after a GC.
// Each sample forces a GC so that only reachable memory is counted.
type heapPeak struct{ base, max uint64 }

func (h *heapPeak) start() {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	h.base = m.HeapAlloc
}

func (h *heapPeak) sample() {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.HeapAlloc > h.base && m.HeapAlloc-h.base > h.max {
		h.max = m.HeapAlloc - h.base
	}
}

func (h *heapPeak) mb() float64 { return float64(h.max) / (1 << 20) }
