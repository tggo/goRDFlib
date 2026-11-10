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
	"github.com/tggo/goRDFlib/store"
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

const overlayPrefixes = `@prefix ex: <http://example.org/> . @prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
`

const overlayBase = overlayPrefixes + `ex:Widget rdfs:subClassOf ex:Part . ex:Gadget rdfs:subClassOf ex:Widget .
ex:root a ex:Assembly ; ex:maxSize 4 ; ex:part ex:a , ex:group .
ex:group ex:part ex:b , ex:c .
ex:a a ex:Part ; ex:size 3 . ex:b a ex:Widget ; ex:size 4 ; ex:link ex:a . ex:c a ex:Part ; ex:size 5 .
ex:y rdf:reifies <<( ex:b ex:size 4 )>> .`

// the triples each variant adds over the base (the prefixes are prepended)
var overlayVariants = []string{
	``,
	`ex:group ex:part ex:d . ex:d a ex:Widget ; ex:size 9 .`,
	`ex:group ex:part ex:e . ex:e a ex:Part .`,
	`ex:b ex:link ex:c . ex:a ex:size 3 .`,                                                // a second linked size, and a triple base already holds
	`ex:x rdf:reifies <<( ex:a ex:size 3 )>> . ex:y rdf:reifies <<( ex:b ex:size 4 )>> .`, // a new reifier, and one base holds
	`ex:group ex:part ex:f . ex:f a ex:Gadget ; ex:size 7 .`,                              // a class found through the base's subclass chain
	`ex:Gadget rdfs:subClassOf ex:Part . ex:root ex:maxSize 6 .`,
	`ex:a ex:size 3 . ex:group ex:part ex:b .`, // only triples base already holds: SPARQL reads the base's store alone
}

func overlayCompile(t testing.TB) *CompiledShapes {
	t.Helper()
	sg, err := LoadTurtleString(overlayShapes, "")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := CompileShapes(sg)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

// overlayCase is a base graph with the stores around it, and for variant i the
// triples it adds and a graph that indexes base and additions in full.
type overlayCase struct {
	rg     *graph.Graph
	base   *Graph
	added  [][]term.Triple
	fulls  []*Graph
	before int
}

func newOverlayCase(t testing.TB) *overlayCase {
	t.Helper()
	ctx := context.Background()
	c := &overlayCase{rg: overlayParse(t, graph.NewGraph(), overlayBase)}
	var err error
	if c.base, err = NewGraphFromRDF(ctx, c.rg, ""); err != nil {
		t.Fatal(err)
	}
	c.before = rgLen(t, c.rg)
	for _, v := range overlayVariants {
		c.added = append(c.added, overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayPrefixes+v)))
		full, err := NewGraphFromRDF(ctx, overlayParse(t, graph.NewGraph(), overlayBase+"\n"+v), "")
		if err != nil {
			t.Fatal(err)
		}
		c.fulls = append(c.fulls, full)
	}
	return c
}

// An overlay over a base graph validates and answers lookups exactly as a
// graph indexing base and the added triples in full, for Core and SPARQL
// shapes, and neither the base nor its store is touched. The store the SPARQL
// constraints query is the overlay's own: nothing is added to the base's.
func TestOverlayGraphReadsAsTheWholeGraph(t *testing.T) {
	ctx := t.Context()
	cs := overlayCompile(t)
	c := newOverlayCase(t)
	before := overlayReport(cs.Validate(c.base))
	nonEmpty := 0
	for i := range overlayVariants {
		ov, err := c.base.Overlay(ctx, c.added[i])
		if err != nil {
			t.Fatal(err)
		}
		full := c.fulls[i]
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
		overlaySameReifiers(t, i, ov, full)
		overlaySameStore(t, i, ov, full)
	}
	if nonEmpty < 3 {
		t.Errorf("only %d variants added results: the check is weak", nonEmpty)
	}
	if after := overlayReport(cs.Validate(c.base)); !slices.Equal(after, before) {
		t.Errorf("base report changed: %v, was %v", after, before)
	}
	if n := rgLen(t, c.rg); n != c.before {
		t.Errorf("the base's store holds %d triples after the overlays, had %d", n, c.before)
	}
	if n := c.base.Len(); n != c.before {
		t.Errorf("base Len = %d, had %d", n, c.before)
	}
}

// Overlays share one base from several goroutines (run with -race), SPARQL
// shapes included: each overlay queries its own union store, so no store is
// written. Reports and lookups stay equal to a graph indexing everything.
func TestOverlayGraphsShareABaseConcurrently(t *testing.T) {
	ctx := context.Background()
	cs := overlayCompile(t)
	c := newOverlayCase(t)
	want := make([][]string, len(overlayVariants))
	for i, full := range c.fulls {
		want[i] = overlayReport(cs.Validate(full))
	}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for w := range 8 {
		wg.Go(func() {
			for i := range 20 {
				k := (w + i) % len(overlayVariants)
				ov, err := c.base.Overlay(ctx, c.added[k])
				if err != nil {
					errs[w] = err
					return
				}
				if got := overlayReport(cs.Validate(ov)); !slices.Equal(got, want[k]) {
					errs[w] = fmt.Errorf("worker %d variant %d: %v vs %v", w, k, got, want[k])
					return
				}
				// every lookup form too: a result that combines base and
				// overlay must never be written into base's index slices
				overlaySameLookups(t, k, ov, c.fulls[k])
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if n := rgLen(t, c.rg); n != c.before {
		t.Errorf("the base's store holds %d triples after the overlays, had %d", n, c.before)
	}
}

// One overlay validated from several goroutines at once builds its union graph
// once and every run sees the same report.
func TestOverlayGraphValidatesConcurrently(t *testing.T) {
	cs := overlayCompile(t)
	c := newOverlayCase(t)
	ov, err := c.base.Overlay(t.Context(), c.added[1])
	if err != nil {
		t.Fatal(err)
	}
	want := overlayReport(cs.Validate(c.fulls[1]))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := overlayReport(cs.Validate(ov)); !slices.Equal(got, want) {
				t.Errorf("report %v, want %v", got, want)
			}
		})
	}
	wg.Wait()
}

// An overlay is read-only: Add and Merge fail with ErrReadOnly and change
// nothing, InvalidateIndexes does not drop the indexes, and the store the
// SPARQL constraints query refuses writes too.
func TestOverlayGraphRefusesWrites(t *testing.T) {
	ctx := t.Context()
	c := newOverlayCase(t)
	ov, err := c.base.Overlay(ctx, c.added[1])
	if err != nil {
		t.Fatal(err)
	}
	n := ov.Len()
	a, p := IRI("http://example.org/a"), IRI("http://example.org/size")
	if err := ov.Add(ctx, a, p, IRI("http://example.org/z")); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Add: err = %v, want ErrReadOnly", err)
	}
	if err := ov.Merge(ctx, c.fulls[2]); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Merge: err = %v, want ErrReadOnly", err)
	}
	ov.InvalidateIndexes()
	if got := ov.Len(); got != n {
		t.Errorf("Len = %d after refused writes and InvalidateIndexes, want %d", got, n)
	}
	if ov.idx.Load() == nil {
		t.Error("InvalidateIndexes dropped the overlay's indexes")
	}
	if err := ov.Err(); err != nil {
		t.Errorf("Err = %v", err)
	}
	rg, err := ov.rdf(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d := term.NewURIRefUnsafe("http://example.org/d")
	if err := rg.Add(ctx, d, term.NewURIRefUnsafe("http://example.org/size"), term.NewLiteral(1)); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("write to the SPARQL graph: err = %v, want store.ErrReadOnly", err)
	}
	// the base is still writable, and still itself
	if err := c.base.Add(ctx, a, p, IRI("http://example.org/z")); err != nil {
		t.Errorf("Add on the base: %v", err)
	}
}

// A nil base is refused.
func TestOverlayGraphRefusesNil(t *testing.T) {
	var g *Graph
	if _, err := g.Overlay(t.Context(), nil); !errors.Is(err, ErrNilGraph) {
		t.Errorf("nil base: err = %v, want ErrNilGraph", err)
	}
}

// A done context stops Overlay.
func TestOverlayGraphHonoursContext(t *testing.T) {
	c := newOverlayCase(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.base.Overlay(ctx, c.added[1]); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// An overlay may be the base of another: the second reads as base plus both.
func TestOverlayOfOverlay(t *testing.T) {
	ctx := t.Context()
	cs := overlayCompile(t)
	c := newOverlayCase(t)
	first, err := c.base.Overlay(ctx, c.added[2])
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.Overlay(ctx, c.added[1])
	if err != nil {
		t.Fatal(err)
	}
	full, err := NewGraphFromRDF(ctx, overlayParse(t, graph.NewGraph(), overlayBase+"\n"+overlayVariants[2]+"\n"+overlayVariants[1]), "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := overlayReport(cs.Validate(second)), overlayReport(cs.Validate(full)); !slices.Equal(got, want) {
		t.Errorf("report %v, want %v", got, want)
	}
	if second.Len() != full.Len() {
		t.Errorf("Len = %d, want %d", second.Len(), full.Len())
	}
	overlaySameLookups(t, 0, second, full)
	overlaySameStore(t, 0, second, full)
}

// HasType and IsSubClassOf follow the subclass chain through the base for
// terms the overlay adds, and through the overlay for classes it adds.
func TestOverlayGraphTypeReasoning(t *testing.T) {
	ctx := t.Context()
	c := newOverlayCase(t)
	ex := func(s string) Term { return IRI("http://example.org/" + s) }
	ov, err := c.base.Overlay(ctx, c.added[5]) // ex:f a ex:Gadget; Gadget < Widget < Part in the base
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ node, class string }{{"f", "Gadget"}, {"f", "Widget"}, {"f", "Part"}, {"f", "Assembly"}, {"a", "Part"}, {"b", "Part"}} {
		if got, want := ov.HasType(ex(q.node), ex(q.class)), c.fulls[5].HasType(ex(q.node), ex(q.class)); got != want {
			t.Errorf("HasType(%s, %s) = %v, want %v", q.node, q.class, got, want)
		}
	}
	if !ov.HasType(ex("f"), ex("Part")) || ov.HasType(ex("f"), ex("Assembly")) {
		t.Error("HasType does not follow the base's subclass chain")
	}
	ov, err = c.base.Overlay(ctx, c.added[6]) // the overlay adds Gadget < Part and the base has Gadget < Widget
	if err != nil {
		t.Fatal(err)
	}
	if !ov.IsSubClassOf(ex("Gadget"), ex("Part")) || !ov.IsSubClassOf(ex("Gadget"), ex("Widget")) || ov.IsSubClassOf(ex("Part"), ex("Gadget")) {
		t.Error("IsSubClassOf is wrong across base and overlay")
	}
	if c.base.IsSubClassOf(ex("Part"), ex("Gadget")) || !c.base.IsSubClassOf(ex("Gadget"), ex("Part")) {
		t.Error("the base changed")
	}
}

// A base loaded from a stream has no rdflib graph until something needs one;
// the overlay builds it (once, shared) for its SPARQL constraints.
func TestOverlayOverStreamedBase(t *testing.T) {
	ctx := t.Context()
	cs := overlayCompile(t)
	base, err := LoadJsonLDString(`{"@context":{"ex":"http://example.org/","rdfs":"http://www.w3.org/2000/01/rdf-schema#"},
	"@graph":[{"@id":"ex:Widget","rdfs:subClassOf":{"@id":"ex:Part"}},
	{"@id":"ex:root","@type":"ex:Assembly","ex:maxSize":4,"ex:part":{"@id":"ex:a"}},
	{"@id":"ex:a","@type":"ex:Part","ex:size":3}]}`, "")
	if err != nil {
		t.Fatal(err)
	}
	add := overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayPrefixes+`ex:root ex:part ex:d . ex:d a ex:Widget ; ex:size 9 .`))
	full, err := LoadJsonLDString(`{"@context":{"ex":"http://example.org/","rdfs":"http://www.w3.org/2000/01/rdf-schema#"},
	"@graph":[{"@id":"ex:Widget","rdfs:subClassOf":{"@id":"ex:Part"}},
	{"@id":"ex:root","@type":"ex:Assembly","ex:maxSize":4,"ex:part":[{"@id":"ex:a"},{"@id":"ex:d"}]},
	{"@id":"ex:a","@type":"ex:Part","ex:size":3},{"@id":"ex:d","@type":"ex:Widget","ex:size":9}]}`, "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			ov, err := base.Overlay(ctx, add)
			if err != nil {
				t.Error(err)
				return
			}
			got, want := overlayReport(cs.Validate(ov)), overlayReport(cs.Validate(full))
			if !slices.Equal(got, want) || len(want) == 0 {
				t.Errorf("report %v, want %v (non-empty)", got, want)
			}
		})
	}
	wg.Wait()
}

func overlayParse(t testing.TB, g *graph.Graph, ttl string) *graph.Graph {
	t.Helper()
	if err := turtle.Parse(context.Background(), g, strings.NewReader(ttl)); err != nil {
		t.Fatal(err)
	}
	return g
}

func overlayTriples(t testing.TB, g *graph.Graph) []term.Triple {
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
func overlaySameLookups(t testing.TB, i int, ov, full *Graph) {
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
		t.Helper()
		if !slices.Equal(a, b) {
			t.Errorf("variant %d: %s: overlay %v, full %v", i, what, a, b)
		}
	}
	check("Triples", keys(ov.Triples()), keys(full.Triples()))
	for _, tr := range full.Triples() {
		s, p, o := tr.Subject, tr.Predicate, tr.Object
		// all eight patterns
		for _, bind := range [8][3]bool{
			{false, false, false}, {true, false, false}, {false, true, false}, {false, false, true},
			{true, true, false}, {true, false, true}, {false, true, true}, {true, true, true},
		} {
			var sp, pp, op *Term
			if bind[0] {
				sp = &s
			}
			if bind[1] {
				pp = &p
			}
			if bind[2] {
				op = &o
			}
			check(fmt.Sprintf("All%v", bind), keys(ov.All(sp, pp, op)), keys(full.All(sp, pp, op)))
		}
		check("Objects", tkeys(ov.Objects(s, p)), tkeys(full.Objects(s, p)))
		check("Subjects", tkeys(ov.Subjects(p, o)), tkeys(full.Subjects(p, o)))
	}
}

// overlaySameReifiers compares the reifier lookups, which read their own tables.
func overlaySameReifiers(t testing.TB, i int, ov, full *Graph) {
	t.Helper()
	key := func(e reifyEntry) string { return e.reifier.TermKey() + " " + term.TermKey(e.tt) }
	var a, b []string
	for _, e := range ov.ensureIndexes().reifies {
		a = append(a, key(e))
	}
	for _, e := range full.ensureIndexes().reifies {
		b = append(b, key(e))
	}
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		t.Errorf("variant %d: reifies: overlay %v, full %v", i, a, b)
	}
	if len(ov.ensureIndexes().reifiesBy) != len(full.ensureIndexes().reifiesBy) {
		t.Errorf("variant %d: reifiesBy has %d reifiers, want %d", i, len(ov.ensureIndexes().reifiesBy), len(full.ensureIndexes().reifiesBy))
	}
}

// overlaySameStore compares the graph the SPARQL constraints query with the
// full graph's: same triples, same Len, same namespaces.
func overlaySameStore(t testing.TB, i int, ov, full *Graph) {
	t.Helper()
	ctx := context.Background()
	orig, err := ov.rdf(ctx)
	if err != nil {
		t.Fatal(err)
	}
	frg, err := full.rdf(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rgLen(t, orig) != rgLen(t, frg) {
		t.Errorf("variant %d: SPARQL graph Len %d, want %d", i, rgLen(t, orig), rgLen(t, frg))
	}
	keys := func(g *graph.Graph) []string {
		var out []string
		for _, tr := range overlayTriples(t, g) {
			out = append(out, term.TermKey(tr.Subject)+" "+term.TermKey(tr.Predicate)+" "+term.TermKey(tr.Object))
		}
		slices.Sort(out)
		return out
	}
	if a, b := keys(orig), keys(frg); !slices.Equal(a, b) {
		t.Errorf("variant %d: SPARQL graph triples %v, want %v", i, a, b)
	}
	if a, b := len(ov.namespaces()), len(full.namespaces()); a != b {
		t.Errorf("variant %d: %d namespaces, want %d", i, a, b)
	}
}
