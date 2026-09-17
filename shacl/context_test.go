package shacl_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

type spanKey struct{}

// spanStore stands in for an instrumented backend (issue #35). It records the
// span of every read, and can cancel a context after a number of reads to
// simulate a caller giving up halfway.
type spanStore struct {
	store.Store
	ctx   context.Context
	state *spanState
}

type spanState struct {
	mu       sync.Mutex
	spans    []string
	cancelAt int
	cancel   context.CancelFunc
}

func (s *spanStore) BindContext(ctx context.Context) store.Store {
	c := *s
	c.ctx = ctx
	return &c
}

func (s *spanStore) Triples(p term.TriplePattern, g term.Term) store.TripleIterator {
	span := ""
	if s.ctx != nil {
		span, _ = s.ctx.Value(spanKey{}).(string)
	}
	st := s.state
	st.mu.Lock()
	st.spans = append(st.spans, span)
	if st.cancel != nil && len(st.spans) >= st.cancelAt {
		st.cancel()
	}
	st.mu.Unlock()
	return s.Store.Triples(p, g)
}

func (st *spanState) reset() {
	st.mu.Lock()
	st.spans = nil
	st.mu.Unlock()
}

func (st *spanState) all() []string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]string(nil), st.spans...)
}

const contextData = `
@prefix ex: <http://example.org/> .
ex:alice a ex:Person ; ex:age 17 .
ex:bob a ex:Person ; ex:age 42 .
`

// The constraint is a SPARQL constraint that calls a SHACL function, whose
// body is a query of its own: the context must reach the store through both.
const contextShapes = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:isMinor a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:person ] ;
    sh:returnType xsd:boolean ;
    sh:ask """ASK { $person <http://example.org/age> ?a FILTER(?a < 18) }""" .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:sparql [
        sh:select """SELECT $this WHERE { $this a ?type FILTER(<http://example.org/isMinor>($this)) }""" ;
    ] .
`

// contextShapesSPARQLTarget selects its focus nodes with a query, so target
// selection itself reads the store.
const contextShapesSPARQLTarget = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:PersonShape a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { ?this a <http://example.org/Person> }""" ] ;
    sh:sparql [
        sh:select """SELECT $this WHERE { $this <http://example.org/age> ?a FILTER(?a < 18) }""" ;
    ] .
`

func spanGraphs(t *testing.T, shapesText ...string) (*shacl.Graph, *shacl.Graph, *spanState) {
	t.Helper()
	st := &spanState{}
	g := graph.NewGraph(graph.WithStore(&spanStore{Store: store.NewMemoryStore(), state: st}))
	if err := turtle.Parse(g, strings.NewReader(contextData)); err != nil {
		t.Fatal(err)
	}
	text := contextShapes
	if len(shapesText) > 0 {
		text = shapesText[0]
	}
	shapes, err := shacl.LoadTurtleString(text, "")
	if err != nil {
		t.Fatal(err)
	}
	st.reset()
	return shacl.NewGraphFromRDF(g, ""), shapes, st
}

func TestValidateContextReachesStore(t *testing.T) {
	data, shapes, st := spanGraphs(t)
	ctx := context.WithValue(context.Background(), spanKey{}, "span-1")

	report, err := shacl.ValidateContext(ctx, data, shapes, shacl.WithAdvancedFeatures())
	if err != nil {
		t.Fatal(err)
	}
	if report.Conforms || len(report.Results) != 1 || report.Results[0].FocusNode.Value() != "http://example.org/alice" {
		t.Fatalf("report = %+v; want one result for ex:alice", report)
	}
	spans := st.all()
	// One read builds the index; the rest are the constraint's query and the
	// function's query, once per person.
	if len(spans) < 5 {
		t.Fatalf("store reads = %d, want the index build plus the queries", len(spans))
	}
	for i, s := range spans {
		if s != "span-1" {
			t.Fatalf("store read %d of %d ran without the caller's context", i+1, len(spans))
		}
	}
}

func TestValidateContextCancelled(t *testing.T) {
	data, shapes, _ := spanGraphs(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := shacl.ValidateContext(ctx, data, shapes, shacl.WithAdvancedFeatures())
	if !errors.Is(err, shacl.ErrCancelled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if report.Conforms || report.Results != nil {
		t.Fatalf("a stopped run returned a report: %+v", report)
	}
	if _, err := shacl.PrepareContext(ctx, data, shapes); !errors.Is(err, shacl.ErrCancelled) {
		t.Fatalf("PrepareContext err = %v", err)
	}
	if _, err := shacl.ApplyRulesContext(ctx, data, shapes); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyRulesContext err = %v", err)
	}
}

// A run stopped halfway, where the store gave up on a read, must report the
// stop rather than a report missing that read's results, and must not leave the
// prepared run with focus nodes it selected only partly.
func TestValidateContextStoppedHalfway(t *testing.T) {
	for name, text := range map[string]string{"function": contextShapes, "SPARQL target": contextShapesSPARQLTarget} {
		t.Run(name, func(t *testing.T) { testStoppedHalfway(t, text) })
	}
}

func testStoppedHalfway(t *testing.T, shapesText string) {
	data, shapes, st := spanGraphs(t, shapesText)
	want := shacl.Validate(data, shapes, shacl.WithAdvancedFeatures())
	if len(want.Results) != 1 {
		t.Fatalf("baseline report = %+v", want)
	}
	for cancelAt := 1; cancelAt <= 6; cancelAt++ {
		// A fresh prepared run each time, whose first validation is the one
		// that gets stopped, so nothing is cached before it.
		var diagnostics []error
		prepared, err := shacl.PrepareContext(context.Background(), data, shapes, shacl.WithAdvancedFeatures(),
			shacl.WithErrorHandler(func(err error) { diagnostics = append(diagnostics, err) }))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		st.mu.Lock()
		st.spans, st.cancelAt, st.cancel = nil, cancelAt, cancel
		st.mu.Unlock()
		report, err := prepared.ValidateContext(ctx)
		st.mu.Lock()
		reads := len(st.spans)
		st.cancel = nil
		st.mu.Unlock()
		cancel()
		if reads < cancelAt {
			if err != nil {
				t.Fatalf("cancelAt %d: finished after %d reads but err = %v", cancelAt, reads, err)
			}
			continue
		}
		if !errors.Is(err, shacl.ErrCancelled) {
			t.Fatalf("cancelAt %d: err = %v, report = %+v; want the stop", cancelAt, err, report)
		}
		if len(diagnostics) != 0 {
			t.Fatalf("cancelAt %d: the stop was reported as a problem with the shapes: %v", cancelAt, diagnostics)
		}
		if again := prepared.Validate(); len(again.Results) != len(want.Results) {
			t.Fatalf("cancelAt %d: a later run has %d results, want %d", cancelAt, len(again.Results), len(want.Results))
		}
	}
}

func TestApplyRulesContextReachesStore(t *testing.T) {
	data, _, st := spanGraphs(t)
	shapes, err := shacl.LoadTurtleString(`
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
ex:AdultRule a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this a <http://example.org/Adult> } WHERE { $this <http://example.org/age> ?a FILTER(?a >= 18) }""" ] .
`, "")
	if err != nil {
		t.Fatal(err)
	}
	n, err := shacl.ApplyRulesContext(context.WithValue(context.Background(), spanKey{}, "span-1"), data, shapes)
	if err != nil || n != 1 {
		t.Fatalf("ApplyRulesContext = %d, %v", n, err)
	}
	spans := st.all()
	if len(spans) < 2 {
		t.Fatalf("store reads = %d, want the index build and the rule's queries", len(spans))
	}
	for i, s := range spans {
		if s != "span-1" {
			t.Fatalf("store read %d of %d ran without the caller's context", i+1, len(spans))
		}
	}
}
