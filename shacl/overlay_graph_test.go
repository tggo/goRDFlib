package shacl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

const overlayShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
ex:Core a sh:NodeShape ; sh:targetClass ex:Part ;
  sh:property [ sh:path ex:size ; sh:minCount 1 ; sh:maxInclusive 5 ] ;
  sh:property [ sh:path ( ex:link ex:size ) ; sh:maxCount 1 ] .
ex:Sparql a sh:NodeShape ; sh:targetClass ex:Assembly ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/> PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
    SELECT $this ?p WHERE { $this ex:part+ ?p . ?p a/rdfs:subClassOf* ex:Part ; ex:size ?s . $this ex:maxSize ?m FILTER (?s > ?m) }""" ] .`

const overlayBase = `@prefix ex: <http://example.org/> . @prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
ex:Widget rdfs:subClassOf ex:Part .
ex:root a ex:Assembly ; ex:maxSize 4 ; ex:part ex:a , ex:group .
ex:group ex:part ex:b , ex:c .
ex:a a ex:Part ; ex:size 3 . ex:b a ex:Widget ; ex:size 4 ; ex:link ex:a . ex:c a ex:Part ; ex:size 5 .`

// the triples each variant adds over the base
var overlayVariants = []string{
	``,
	`ex:group ex:part ex:d . ex:d a ex:Widget ; ex:size 9 .`,
	`ex:group ex:part ex:e . ex:e a ex:Part .`,
	`ex:b ex:link ex:c . ex:a ex:size 3 .`, // a second linked size, and a triple base already holds
	`ex:x <http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies> <<( ex:a ex:size 3 )>> .`,
}

// An overlay over a base graph validates and answers lookups exactly as a
// graph indexing base and the added triples in full, and leaves base as it was.
func TestOverlayGraphReadsAsTheWholeGraph(t *testing.T) {
	ctx := t.Context()
	sg, err := LoadTurtleString(overlayShapes, "")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := CompileShapes(sg)
	if err != nil {
		t.Fatal(err)
	}
	rg := overlayParse(t, graph.NewGraph(), overlayBase)
	base, err := NewGraphFromRDF(ctx, rg, "")
	if err != nil {
		t.Fatal(err)
	}
	before := overlayReport(cs.Validate(base))
	nonEmpty := 0
	for i, v := range overlayVariants {
		added := overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayBase[:strings.Index(overlayBase, "\n")+1]+v))
		var fresh []term.Triple // to remove afterwards: the ones base does not hold
		for _, tr := range added {
			if ok, _ := rg.Contains(ctx, tr.Subject, tr.Predicate, tr.Object); !ok {
				fresh = append(fresh, tr)
				if err := rg.Add(ctx, tr.Subject, tr.Predicate, tr.Object); err != nil {
					t.Fatal(err)
				}
			}
		}
		ov, err := NewOverlayGraph(ctx, base, rg, added)
		if err != nil {
			t.Fatal(err)
		}
		full, err := NewGraphFromRDF(ctx, rg, "")
		if err != nil {
			t.Fatal(err)
		}
		got, want := overlayReport(cs.Validate(ov)), overlayReport(cs.Validate(full))
		if !slices.Equal(got, want) {
			t.Errorf("variant %d: overlay report %v, full graph report %v", i, got, want)
		}
		if len(want) > len(before) {
			nonEmpty++
		}
		if ov.Len() != full.Len() {
			t.Errorf("variant %d: Len %d, want %d", i, ov.Len(), full.Len())
		}
		overlaySameLookups(t, i, ov, full)
		for _, tr := range fresh {
			if err := rg.Remove(ctx, tr.Subject, &tr.Predicate, tr.Object); err != nil {
				t.Fatal(err)
			}
		}
	}
	if nonEmpty < 2 {
		t.Errorf("only %d variants added results: the check is weak", nonEmpty)
	}
	if after := overlayReport(cs.Validate(base)); !slices.Equal(after, before) {
		t.Errorf("base report changed: %v, was %v", after, before)
	}
}

// Overlays share one base from several goroutines (run with -race): reports
// and lookups stay equal to a graph indexing everything.
func TestOverlayGraphsShareABaseConcurrently(t *testing.T) {
	ctx := context.Background()
	sg, err := LoadTurtleString(overlayShapes, "")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := CompileShapes(sg)
	if err != nil {
		t.Fatal(err)
	}
	base, err := NewGraphFromRDF(ctx, overlayParse(t, graph.NewGraph(), overlayBase), "")
	if err != nil {
		t.Fatal(err)
	}
	// one store per variant (base + added), read concurrently by every worker
	owns := make([]*graph.Graph, len(overlayVariants))
	addeds := make([][]term.Triple, len(overlayVariants))
	for i, v := range overlayVariants {
		owns[i] = overlayParse(t, graph.NewGraph(), overlayBase+"\n"+v)
		addeds[i] = overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayBase[:strings.Index(overlayBase, "\n")+1]+v))
	}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for w := range 8 {
		wg.Go(func() {
			for i := range 20 {
				k := (w + i) % len(overlayVariants)
				ov, err := NewOverlayGraph(ctx, base, owns[k], addeds[k])
				if err != nil {
					errs[w] = err
					return
				}
				full, err := NewGraphFromRDF(ctx, owns[k], "")
				if err != nil {
					errs[w] = err
					return
				}
				if a, b := overlayReport(cs.Validate(ov)), overlayReport(cs.Validate(full)); !slices.Equal(a, b) {
					errs[w] = fmt.Errorf("worker %d variant %d: %v vs %v", w, k, a, b)
					return
				}
				// every lookup form too: a result that combines base and
				// overlay must never be written into base's index slices
				overlaySameLookups(t, k, ov, full)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// A nil base or rdflib graph is refused: a nil rdflib graph would leave the
// SPARQL-based constraints querying an empty graph.
func TestOverlayGraphRefusesNil(t *testing.T) {
	ctx := t.Context()
	rg := overlayParse(t, graph.NewGraph(), overlayBase)
	base, err := NewGraphFromRDF(ctx, rg, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewOverlayGraph(ctx, base, nil, nil); !errors.Is(err, ErrNilGraph) {
		t.Errorf("nil rg: err = %v, want ErrNilGraph", err)
	}
	if _, err := NewOverlayGraph(ctx, nil, rg, nil); !errors.Is(err, ErrNilGraph) {
		t.Errorf("nil base: err = %v, want ErrNilGraph", err)
	}
}

func overlayParse(t *testing.T, g *graph.Graph, ttl string) *graph.Graph {
	t.Helper()
	if err := turtle.Parse(context.Background(), g, strings.NewReader(ttl)); err != nil {
		t.Fatal(err)
	}
	return g
}

func overlayTriples(t *testing.T, g *graph.Graph) []term.Triple {
	t.Helper()
	var out []term.Triple
	for tr, err := range g.Triples(context.Background(), nil, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, tr)
	}
	return out
}

func overlayReport(r ValidationReport) []string {
	var out []string
	var walk func([]ValidationResult)
	walk = func(rs []ValidationResult) {
		for _, x := range rs {
			out = append(out, strings.Join([]string{x.FocusNode.TermKey(), x.ResultPath.TermKey(), x.Value.TermKey(),
				x.SourceShape.TermKey(), x.SourceConstraintComponent.TermKey(), x.ResultSeverity.TermKey()}, " "))
			walk(x.Details)
		}
	}
	walk(r.Results)
	slices.Sort(out)
	return out
}

// overlaySameLookups compares every lookup form, for every term of the full graph.
func overlaySameLookups(t *testing.T, i int, ov, full *Graph) {
	t.Helper()
	keys := func(ts []Triple) []string {
		out := make([]string, len(ts))
		for k, x := range ts {
			out[k] = x.Subject.TermKey() + " " + x.Predicate.TermKey() + " " + x.Object.TermKey()
		}
		slices.Sort(out)
		return out
	}
	tkeys := func(ts []Term) []string {
		out := make([]string, len(ts))
		for k, x := range ts {
			out[k] = x.TermKey()
		}
		slices.Sort(out)
		return out
	}
	check := func(what string, a, b []string) {
		if !slices.Equal(a, b) {
			t.Errorf("variant %d: %s: overlay %v, full %v", i, what, a, b)
		}
	}
	check("Triples", keys(ov.Triples()), keys(full.Triples()))
	for _, tr := range full.Triples() {
		s, p, o := tr.Subject, tr.Predicate, tr.Object
		check("All(s,p,o)", keys(ov.All(&s, &p, &o)), keys(full.All(&s, &p, &o)))
		check("All(s,p,_)", keys(ov.All(&s, &p, nil)), keys(full.All(&s, &p, nil)))
		check("All(_,p,o)", keys(ov.All(nil, &p, &o)), keys(full.All(nil, &p, &o)))
		check("All(_,p,_)", keys(ov.All(nil, &p, nil)), keys(full.All(nil, &p, nil)))
		check("All(s,_,_)", keys(ov.All(&s, nil, nil)), keys(full.All(&s, nil, nil)))
		check("All(s,_,o)", keys(ov.All(&s, nil, &o)), keys(full.All(&s, nil, &o)))
		check("All(_,_,o)", keys(ov.All(nil, nil, &o)), keys(full.All(nil, nil, &o)))
		check("Objects", tkeys(ov.Objects(s, p)), tkeys(full.Objects(s, p)))
		check("Subjects", tkeys(ov.Subjects(p, o)), tkeys(full.Subjects(p, o)))
	}
}
