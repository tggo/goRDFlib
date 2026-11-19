package shacl_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// budgetScanStore is a MemoryStore (exact cardinalities, reachability) that
// counts the reads of ex:holdsBudget with no subject: the read a SPARQL plan
// makes when it starts from that triple pattern instead of from the path.
type budgetScanStore struct {
	*store.MemoryStore
	scans atomic.Int64
}

func (s *budgetScanStore) count(p term.TriplePattern) {
	if p.Subject == nil && p.Predicate != nil && p.Predicate.Value() == "http://example.org/holdsBudget" {
		s.scans.Add(1)
	}
}

func (s *budgetScanStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	s.count(p)
	return s.MemoryStore.Triples(ctx, p, g)
}

func (s *budgetScanStore) TriplesWithLimit(ctx context.Context, p term.TriplePattern, g term.Term, limit, offset int) store.TripleIterator {
	s.count(p)
	return s.MemoryStore.TriplesWithLimit(ctx, p, g, limit, offset)
}

// The SPARQL constraint runs once, for ex:org, and its NOT EXISTS plans
// { ?value ex:reportsTo+ ?m . ?m ex:holdsBudget ?b } once per employee (255),
// each plan probing the path once.
const pathProbeBoundShapes = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:OrgShape a sh:NodeShape ;
    sh:targetNode ex:org ;
    sh:sparql [
        sh:select """SELECT $this ?value WHERE {
            ?value <http://example.org/worksFor> $this
            FILTER NOT EXISTS { ?value <http://example.org/reportsTo>+ ?m . ?m <http://example.org/holdsBudget> ?b }
        }""" ;
    ] .
`

// sparql.WithMaxPathProbes passed to ValidateContext reaches the SHACL-SPARQL
// constraint: with the default bound, the employees planned after the bound
// is spent start from ex:holdsBudget and read every budget; with a bound that
// covers them, or none, no plan does. The report is the same every time: only
// the root of the org chart has no one above who holds a budget.
func TestValidateContextPassesMaxPathProbes(t *testing.T) {
	st := &budgetScanStore{MemoryStore: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(st))
	u := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	add := func(s, p, o term.URIRef) {
		if err := g.Add(t.Context(), s, p, o); err != nil {
			t.Fatal(err)
		}
	}
	const employees = 255 // a binary tree; the 127 with reports hold a budget
	for i := range employees {
		e := u("e%d", i)
		add(e, u("worksFor"), u("org"))
		if i > 0 {
			add(e, u("reportsTo"), u("e%d", (i-1)/2))
		}
		if 2*i+1 < employees {
			add(e, u("holdsBudget"), u("budget%d", i))
		}
	}
	data := mustV(shacl.NewGraphFromRDF(t.Context(), g, ""))
	shapes := mustV(shacl.LoadTurtleString(pathProbeBoundShapes, ""))
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		scans func(int64) bool
		want  string
	}{
		{"default", t.Context(), func(n int64) bool { return n > 0 }, "some"},
		{"covers every employee", sparql.WithMaxPathProbes(t.Context(), 1000), func(n int64) bool { return n == 0 }, "none"},
		{"unbounded", sparql.WithMaxPathProbes(t.Context(), 0), func(n int64) bool { return n == 0 }, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st.scans.Store(0)
			report, err := shacl.ValidateContext(tc.ctx, data, shapes)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Results) != 1 || report.Results[0].Value.Value() != "http://example.org/e0" {
				t.Fatalf("results %+v, want one, for ex:e0", report.Results)
			}
			if n := st.scans.Load(); !tc.scans(n) {
				t.Errorf("%d plans read every ex:holdsBudget triple, want %s", n, tc.want)
			}
		})
	}
}
