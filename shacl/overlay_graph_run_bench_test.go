package shacl

import (
	"fmt"
	"testing"
)

// BenchmarkOverlayGraph validates one small document over a large shared
// graph. "rewrap" is the path available without Overlay: add the document's
// triples to the shared store, wrap it with NewGraphFromRDF (which indexes the
// whole store each time) and remove them again. "overlay" indexes the shared
// graph once; each document costs its own triples, whatever the size of the
// shared graph: compare the two shared sizes. "overlay-parallel" runs
// documents on all cores over the one base, which the rewrap path cannot do
// (it writes the one shared store).
func BenchmarkOverlayGraph(b *testing.B) {
	for _, shapes := range []struct{ name, ttl string }{{"core", benchOverlayShapes}, {"sparql", benchOverlaySPARQLShapes}, {"sparql-shared", benchOverlaySharedShapes}} {
		sg, err := LoadTurtleString(shapes.ttl, "")
		if err != nil {
			b.Fatal(err)
		}
		cs, err := CompileShapes(sg)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range []int{10_000, 100_000} {
			shared := benchVocabulary(b, n)
			base, err := NewGraphFromRDF(b.Context(), shared, "")
			if err != nil {
				b.Fatal(err)
			}
			doc := benchDocument(50, (n-100)/3)
			check := func(b *testing.B, g *Graph) {
				benchReport = cs.Validate(g)
				if !benchReport.Conforms {
					b.Fatalf("the benchmark document should conform: %d results", len(benchReport.Results))
				}
			}
			name := fmt.Sprintf("%s/shared=%d/doc=%d", shapes.name, base.Len(), len(doc))
			b.Run(name+"/rewrap", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					for _, t := range doc {
						if err := shared.Add(b.Context(), t.Subject, t.Predicate, t.Object); err != nil {
							b.Fatal(err)
						}
					}
					g, err := NewGraphFromRDF(b.Context(), shared, "")
					if err != nil {
						b.Fatal(err)
					}
					check(b, g)
					for _, t := range doc {
						if err := shared.Remove(b.Context(), t.Subject, &t.Predicate, t.Object); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
			b.Run(name+"/overlay", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					g, err := base.Overlay(b.Context(), doc)
					if err != nil {
						b.Fatal(err)
					}
					check(b, g)
				}
			})
			// "overlay-held": as in rewrap, the caller's store already holds the
			// document (another of its steps reads or writes it there), and the
			// caller still takes an overlay over the base indexes, built once.
			// Every added triple is then one the base's store holds.
			b.Run(name+"/overlay-held", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					for _, t := range doc {
						if err := shared.Add(b.Context(), t.Subject, t.Predicate, t.Object); err != nil {
							b.Fatal(err)
						}
					}
					g, err := base.Overlay(b.Context(), doc)
					if err != nil {
						b.Fatal(err)
					}
					check(b, g)
					for _, t := range doc {
						if err := shared.Remove(b.Context(), t.Subject, &t.Predicate, t.Object); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
			b.Run(name+"/overlay-parallel", func(b *testing.B) {
				b.ReportAllocs()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						g, err := base.Overlay(b.Context(), doc)
						if err != nil {
							b.Error(err)
							return
						}
						if r := cs.Validate(g); !r.Conforms {
							b.Errorf("the benchmark document should conform: %d results", len(r.Results))
							return
						}
					}
				})
			})
		}
	}
}
