package shacl_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// failingData returns a data graph over a store that can be told to fail.
func failingData(t *testing.T) (*graph.Graph, *storetest.FailingStore) {
	t.Helper()
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	if err := turtle.Parse(t.Context(), g, strings.NewReader(contextData)); err != nil {
		t.Fatal(err)
	}
	return g, fs
}

// flakyStore fails reads or writes while the matching flag is set, and works
// again when it is cleared, which FailingStore (once failed, always failed)
// cannot do.
type flakyStore struct {
	store.Store
	failReads, failWrites atomic.Bool
}

func (f *flakyStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	if f.failReads.Load() {
		return store.ErrorIterator[term.Triple](storetest.ErrInjected)
	}
	return f.Store.Triples(ctx, p, g)
}

func (f *flakyStore) Add(ctx context.Context, t term.Triple, g term.Term) error {
	if f.failWrites.Load() {
		return storetest.ErrInjected
	}
	return f.Store.Add(ctx, t, g)
}

func flakyData(t *testing.T) (*graph.Graph, *flakyStore) {
	t.Helper()
	fs := &flakyStore{Store: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(fs))
	if err := turtle.Parse(t.Context(), g, strings.NewReader(contextData)); err != nil {
		t.Fatal(err)
	}
	return g, fs
}

func mustShapes(t *testing.T, text string) *shacl.Graph {
	t.Helper()
	s, err := shacl.LoadTurtleString(text, "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewGraphFromRDFReturnsStoreError(t *testing.T) {
	g, fs := failingData(t)
	fs.Fail(storetest.ErrInjected)
	sg, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("err = %v, want the store's error", err)
	}
	if sg != nil {
		t.Fatal("a graph that could not be read was returned")
	}
}

// The index of a wrapped graph is dropped by InvalidateIndexes; rebuilding it
// from a failing store must be the error of the run, not an empty data graph
// that conforms.
func TestValidateContextRebuildStoreError(t *testing.T) {
	g, fs := failingData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	shapes := mustShapes(t, contextShapes)
	if r, err := shacl.ValidateContext(t.Context(), data, shapes, shacl.WithAdvancedFeatures()); err != nil || r.Conforms {
		t.Fatalf("baseline: err = %v, report = %+v", err, r)
	}

	data.InvalidateIndexes()
	fs.Fail(storetest.ErrInjected)
	report, err := shacl.ValidateContext(t.Context(), data, shapes, shacl.WithAdvancedFeatures())
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("err = %v, want the store's error", err)
	}
	if report.Conforms || report.Results != nil {
		t.Fatalf("a failed run returned a report: %+v", report)
	}
}

// A store that fails after the index was built is reached only by SPARQL. A
// constraint whose query could not run must not be reported as a violation or,
// worse, as satisfied: the run fails.
func TestValidateContextQueryStoreError(t *testing.T) {
	for name, text := range map[string]string{"function": contextShapes, "SPARQL target": contextShapesSPARQLTarget} {
		t.Run(name, func(t *testing.T) {
			g, fs := flakyData(t)
			data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
			if err != nil {
				t.Fatal(err)
			}
			shapes := mustShapes(t, text)
			prepared, err := shacl.PrepareContext(t.Context(), data, shapes, shacl.WithAdvancedFeatures())
			if err != nil {
				t.Fatal(err)
			}

			fs.failReads.Store(true)
			report, err := prepared.ValidateContext(t.Context())
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("err = %v, want the store's error", err)
			}
			if report.Conforms || report.Results != nil {
				t.Fatalf("a failed run returned a report: %+v", report)
			}

			// The failure belongs to the run: once the store is back, the
			// same prepared state validates again.
			fs.failReads.Store(false)
			again, err := prepared.ValidateContext(t.Context())
			if err != nil || len(again.Results) != 1 {
				t.Fatalf("later run: err = %v, report = %+v", err, again)
			}
		})
	}
}

func TestConformsQueryStoreError(t *testing.T) {
	g, fs := flakyData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	shapes := mustShapes(t, contextShapes)
	prepared, err := shacl.PrepareContext(t.Context(), data, shapes, shacl.WithAdvancedFeatures())
	if err != nil {
		t.Fatal(err)
	}
	fs.failReads.Store(true)
	ok, err := prepared.Conforms(t.Context(),
		shacl.IRI("http://example.org/alice"), shacl.IRI("http://example.org/PersonShape"))
	if !errors.Is(err, storetest.ErrInjected) || ok {
		t.Fatalf("ok = %v, err = %v; want the store's error", ok, err)
	}
}

func TestGraphAddReturnsStoreError(t *testing.T) {
	g, fs := failingData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	fs.Fail(storetest.ErrInjected)
	err = data.Add(t.Context(), shacl.IRI("http://example.org/x"), shacl.IRI("http://example.org/p"), shacl.Literal("v", "", ""))
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Add err = %v, want the store's error", err)
	}
	other := shacl.NewGraph()
	if err := other.Add(t.Context(), shacl.IRI("http://example.org/x"), shacl.IRI("http://example.org/p"), shacl.Literal("v", "", "")); err != nil {
		t.Fatal(err)
	}
	if err := data.Merge(t.Context(), other); !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Merge err = %v, want the store's error", err)
	}
}

// ApplyRules writes to the caller's graph; a store that refuses the write ends
// the pass with that error.
func TestApplyRulesContextStoreWriteError(t *testing.T) {
	g, fs := flakyData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	shapes := mustShapes(t, `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
ex:AdultRule a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this a <http://example.org/Adult> } WHERE { $this <http://example.org/age> ?a FILTER(?a >= 18) }""" ] .
`)
	fs.failWrites.Store(true)
	_, err = shacl.ApplyRulesContext(t.Context(), data, shapes)
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("err = %v, want the store's error", err)
	}
}

// brokenData returns a data graph whose indexes were dropped while its store is
// failing: the state that used to leave a nil *Prepared behind the legacy
// entry points.
func brokenData(t *testing.T) (*shacl.Graph, *flakyStore) {
	t.Helper()
	g, fs := flakyData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	data.InvalidateIndexes()
	fs.failReads.Store(true)
	return data, fs
}

func checkFailedReport(t *testing.T, report shacl.ValidationReport, handled []error) {
	t.Helper()
	if !errors.Is(report.Err, storetest.ErrInjected) {
		t.Fatalf("report.Err = %v, want the store's error", report.Err)
	}
	if report.Conforms || report.Results != nil {
		t.Fatalf("a failed run produced a report: %+v", report)
	}
	if len(handled) == 0 || !errors.Is(handled[0], storetest.ErrInjected) {
		t.Fatalf("error handler got %v, want the store's error", handled)
	}
}

func TestLegacyEntryPointsStoreError(t *testing.T) {
	shapes := mustShapes(t, contextShapes)

	t.Run("Validate", func(t *testing.T) {
		data, _ := brokenData(t)
		var handled []error
		report := shacl.Validate(data, shapes, shacl.WithErrorHandler(func(err error) { handled = append(handled, err) }))
		checkFailedReport(t, report, handled)
	})

	t.Run("Prepare", func(t *testing.T) {
		data, _ := brokenData(t)
		var handled []error
		p := shacl.Prepare(data, shapes, shacl.WithErrorHandler(func(err error) { handled = append(handled, err) }))
		if p == nil {
			t.Fatal("Prepare returned nil")
		}
		report := p.Validate()
		checkFailedReport(t, report, handled)
		if len(handled) != 1 {
			t.Fatalf("error reported %d times, want once", len(handled))
		}
		if _, err := p.ValidateContext(t.Context()); !errors.Is(err, storetest.ErrInjected) {
			t.Fatalf("ValidateContext err = %v", err)
		}
		if ok, err := p.Conforms(t.Context(), shacl.IRI("http://example.org/alice"), shacl.IRI("http://example.org/PersonShape")); ok || !errors.Is(err, storetest.ErrInjected) {
			t.Fatalf("Conforms = %v, %v", ok, err)
		}
		if got := p.Targets(shacl.IRI("http://example.org/PersonShape")); got != nil {
			t.Fatalf("Targets on a failed Prepared = %v", got)
		}
	})

	t.Run("CompiledShapes.Validate", func(t *testing.T) {
		data, _ := brokenData(t)
		compiled, err := shacl.CompileShapes(shapes)
		if err != nil {
			t.Fatal(err)
		}
		var handled []error
		report := compiled.Validate(data, shacl.WithErrorHandler(func(err error) { handled = append(handled, err) }))
		checkFailedReport(t, report, handled)
	})

	t.Run("Graph.Err", func(t *testing.T) {
		data, fs := brokenData(t)
		_ = data.Len() // lazy rebuild fails
		if !errors.Is(data.Err(), storetest.ErrInjected) {
			t.Fatalf("Err = %v, want the store's error", data.Err())
		}
		fs.failReads.Store(false)
		if n := data.Len(); n == 0 || data.Err() != nil {
			t.Fatalf("after recovery Len = %d, Err = %v", n, data.Err())
		}
	})
}

func TestContextEntryPointsStoreError(t *testing.T) {
	shapes := mustShapes(t, contextShapes)

	t.Run("CompileShapesContext", func(t *testing.T) {
		sg, _ := brokenData(t)
		if c, err := shacl.CompileShapesContext(t.Context(), sg); !errors.Is(err, storetest.ErrInjected) || c != nil {
			t.Fatalf("CompileShapesContext = %v, %v", c, err)
		}
		if c, err := shacl.CompileShapes(sg); !errors.Is(err, storetest.ErrInjected) || c != nil {
			t.Fatalf("CompileShapes = %v, %v", c, err)
		}
	})

	t.Run("EvalRuleSetContext", func(t *testing.T) {
		data, _ := brokenData(t)
		rs, err := shacl.ParseSRL(`RULE { ?s <http://example.org/q> ?o } WHERE { ?s <http://example.org/age> ?o }`)
		if err != nil {
			t.Fatal(err)
		}
		if g, err := shacl.EvalRuleSetContext(t.Context(), rs, data); !errors.Is(err, storetest.ErrInjected) || g != nil {
			t.Fatalf("EvalRuleSetContext = %v, %v", g, err)
		}
		if g, err := shacl.EvalRuleSet(rs, data); !errors.Is(err, storetest.ErrInjected) || g != nil {
			t.Fatalf("EvalRuleSet = %v, %v", g, err)
		}
	})

	t.Run("ValidateContext", func(t *testing.T) {
		data, _ := brokenData(t)
		if _, err := shacl.ValidateContext(t.Context(), data, shapes); !errors.Is(err, storetest.ErrInjected) {
			t.Fatalf("err = %v", err)
		}
	})
}

// A rule's sh:construct that cannot read the store is the store's error, not a
// malformed rule, and the pass stops instead of going on to the next focus node.
func TestApplyRulesContextQueryStoreError(t *testing.T) {
	g, fs := flakyData(t)
	data, err := shacl.NewGraphFromRDF(t.Context(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	shapes := mustShapes(t, `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
ex:AdultRule a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this a <http://example.org/Adult> } WHERE { $this <http://example.org/age> ?a FILTER(?a >= 18) }""" ] .
`)
	var handled []error
	fs.failReads.Store(true)
	_, err = shacl.ApplyRulesContext(t.Context(), data, shapes, shacl.WithErrorHandler(func(err error) { handled = append(handled, err) }))
	if !errors.Is(err, shacl.ErrQueryFailed) || !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("err = %v, want ErrQueryFailed wrapping the store's error", err)
	}
	if errors.Is(err, shacl.ErrMalformedRule) {
		t.Fatalf("a store failure was reported as a malformed rule: %v", err)
	}
}
