package graph_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// --- graph_test.go ---

func makeTestGraph(t *testing.T) (*graph.Graph, term.URIRef, term.URIRef, term.Literal) {
	t.Helper()
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("hello")
	g.Add(t.Context(), s, p, o)
	return g, s, p, o
}

func TestGraphAddAndLen(t *testing.T) {
	g, _, _, _ := makeTestGraph(t)
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestGraphContains(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	if !mustV(g.Contains(t.Context(), s, p, o)) {
		t.Error("should contain the triple")
	}
	p2, _ := term.NewURIRef("http://example.org/other")
	if mustV(g.Contains(t.Context(), s, p2, o)) {
		t.Error("should not contain different predicate")
	}
}

func TestGraphRemove(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	g.Remove(t.Context(), s, &p, o)
	if mustV(g.Len(t.Context())) != 0 {
		t.Errorf("expected 0 after remove, got %d", mustV(g.Len(t.Context())))
	}
}

func TestGraphSet(t *testing.T) {
	g, s, p, _ := makeTestGraph(t)
	newObj := term.NewLiteral("world")
	g.Set(t.Context(), s, p, newObj)
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1 after set, got %d", mustV(g.Len(t.Context())))
	}
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
	if !ok {
		t.Fatal("expected a value")
	}
	if val.N3() != newObj.N3() {
		t.Errorf("expected %q, got %q", newObj.N3(), val.N3())
	}
}

func TestGraphTriples(t *testing.T) {
	g, s, _, _ := makeTestGraph(t)
	p2, _ := term.NewURIRef("http://example.org/p2")
	g.Add(t.Context(), s, p2, term.NewLiteral("world"))

	count := 0
	g.Triples(t.Context(), s, nil, nil)(func(term.Triple, error) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphSubjects(t *testing.T) {
	g := graph.NewGraph()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")
	g.Add(t.Context(), s1, p, o)
	g.Add(t.Context(), s2, p, o)

	var subjects []term.Term
	g.Subjects(t.Context(), &p, o)(func(t term.Term, _ error) bool {
		subjects = append(subjects, t)
		return true
	})
	if len(subjects) != 2 {
		t.Errorf("expected 2 subjects, got %d", len(subjects))
	}
}

func TestGraphObjects(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("a"))
	g.Add(t.Context(), s, p, term.NewLiteral("b"))

	var objects []term.Term
	g.Objects(t.Context(), s, &p)(func(t term.Term, _ error) bool {
		objects = append(objects, t)
		return true
	})
	if len(objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objects))
	}
}

func TestGraphValue(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
	if !ok || val.N3() != o.N3() {
		t.Errorf("expected %q, got %v", o.N3(), val)
	}
}

func TestGraphAddTwice(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	if err := g.Add(t.Context(), s, p1, term.NewLiteral("a")); err != nil {
		t.Fatal(err)
	}
	if err := g.Add(t.Context(), s, p2, term.NewLiteral("b")); err != nil {
		t.Fatal(err)
	}
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestGraphDefaultNamespaces(t *testing.T) {
	g := graph.NewGraph()
	count := 0
	g.Namespaces(t.Context())(func(pair_ store.NamespaceBinding, _ error) bool {
		count++
		return true
	})
	if count < 4 {
		t.Errorf("expected at least 4 default namespaces, got %d", count)
	}
}

func TestGraphQName(t *testing.T) {
	g := graph.NewGraph()
	got, _ := g.QName(t.Context(), "http://www.w3.org/2001/XMLSchema#string")
	if got != "xsd:string" {
		t.Errorf("expected xsd:string, got %q", got)
	}
}

func TestGraphBind(t *testing.T) {
	g := graph.NewGraph()
	ns, _ := term.NewURIRef("http://example.org/ns#")
	g.Bind(t.Context(), "ex", ns)
	got, _ := g.QName(t.Context(), "http://example.org/ns#Thing")
	if got != "ex:Thing" {
		t.Errorf("expected ex:Thing, got %q", got)
	}
}

func TestGraphUnion(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g1.Add(t.Context(), s, p, term.NewLiteral("a"))
	g2.Add(t.Context(), s, p, term.NewLiteral("b"))

	u, _ := g1.Union(t.Context(), g2)
	if mustV(u.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(u.Len(t.Context())))
	}
}

func TestGraphIntersection(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	shared := term.NewLiteral("shared")
	g1.Add(t.Context(), s, p, shared)
	g1.Add(t.Context(), s, p, term.NewLiteral("only1"))
	g2.Add(t.Context(), s, p, shared)
	g2.Add(t.Context(), s, p, term.NewLiteral("only2"))

	inter, _ := g1.Intersection(t.Context(), g2)
	if mustV(inter.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(inter.Len(t.Context())))
	}
}

func TestGraphDifference(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	shared := term.NewLiteral("shared")
	g1.Add(t.Context(), s, p, shared)
	g1.Add(t.Context(), s, p, term.NewLiteral("only1"))
	g2.Add(t.Context(), s, p, shared)

	diff, _ := g1.Difference(t.Context(), g2)
	if mustV(diff.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(diff.Len(t.Context())))
	}
}

func TestGraphConnected(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(t.Context(), s, p, o)

	if !mustV(g.Connected(t.Context())) {
		t.Error("single-edge graph should be connected")
	}

	g2 := graph.NewGraph()
	s2, _ := term.NewURIRef("http://example.org/s2")
	o2, _ := term.NewURIRef("http://example.org/o2")
	g2.Add(t.Context(), s, p, o)
	g2.Add(t.Context(), s2, p, o2)
	if mustV(g2.Connected(t.Context())) {
		t.Error("disconnected graph should not be connected")
	}
}

func TestGraphAllNodes(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(t.Context(), s, p, o)

	nodes, _ := g.AllNodes(t.Context())
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes (s and o, not p), got %d", len(nodes))
	}
}

// --- graph_coverage_test.go ---

func TestGraphWithStore(t *testing.T) {
	s := store.NewMemoryStore()
	g := graph.NewGraph(graph.WithStore(s))
	if g.Store() != s {
		t.Error("WithStore not applied")
	}
}

func TestGraphWithIdentifier(t *testing.T) {
	id, _ := term.NewURIRef("http://example.org/g")
	g := graph.NewGraph(graph.WithIdentifier(id))
	if g.Identifier().(term.URIRef) != id {
		t.Error("WithIdentifier not applied")
	}
}

func TestGraphWithBase(t *testing.T) {
	g := graph.NewGraph(graph.WithBase("http://example.org/"))
	_ = g // just verify construction
}

func TestGraphPredicates(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	g.Add(t.Context(), s, p1, term.NewLiteral("a"))
	g.Add(t.Context(), s, p2, term.NewLiteral("b"))

	count := 0
	g.Predicates(t.Context(), s, nil)(func(term.Term, error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2 predicates, got %d", count)
	}
}

func TestGraphSubjectPredicates(t *testing.T) {
	g := graph.NewGraph()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(t.Context(), s1, p, o)
	g.Add(t.Context(), s2, p, o)

	count := 0
	g.SubjectPredicates(t.Context(), o)(func(pair_ store.TermPair, _ error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphSubjectObjects(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("a"))
	g.Add(t.Context(), s, p, term.NewLiteral("b"))

	count := 0
	g.SubjectObjects(t.Context(), &p)(func(pair_ store.TermPair, _ error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphValuePredicate(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("v"))

	val, ok, _ := g.Value(t.Context(), s, nil, term.NewLiteral("v"))
	if !ok {
		t.Fatal("expected value")
	}
	if val.(term.URIRef) != p {
		t.Errorf("expected %v, got %v", p, val)
	}
}

func TestGraphValueSubject(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("v"))

	val, ok, _ := g.Value(t.Context(), nil, &p, term.NewLiteral("v"))
	if !ok {
		t.Fatal("expected subject")
	}
	if val.(term.URIRef) != s {
		t.Errorf("expected %v, got %v", s, val)
	}
}

func TestGraphValueNotFound(t *testing.T) {
	g := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")
	_, ok, _ := g.Value(t.Context(), nil, &p, term.NewLiteral("nope"))
	if ok {
		t.Error("expected not found")
	}
}

func TestGraphQNameNoMatch(t *testing.T) {
	g := graph.NewGraph()
	got, _ := g.QName(t.Context(), "http://unknown.org/Thing")
	if got != "http://unknown.org/Thing" {
		t.Errorf("expected raw URI, got %q", got)
	}
}

func TestGraphConnectedEmpty(t *testing.T) {
	g := graph.NewGraph()
	if !mustV(g.Connected(t.Context())) {
		t.Error("empty graph should be connected")
	}
}

func TestGraphConnectedSingleNode(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("v"))
	if !mustV(g.Connected(t.Context())) {
		t.Error("single-subject graph should be connected")
	}
}

func TestGraphStoreAccessor(t *testing.T) {
	g := graph.NewGraph()
	if g.Store() == nil {
		t.Error("expected non-nil store")
	}
}

// --- collection_coverage_test.go ---

func TestNewCollectionExisting(t *testing.T) {
	g := graph.NewGraph()
	head := term.NewBNode("list")
	g.Add(t.Context(), head, namespace.RDF.First, term.NewLiteral("a"))
	g.Add(t.Context(), head, namespace.RDF.Rest, namespace.RDF.Nil)

	col := graph.NewCollection(g, head)
	if col.Head() != head {
		t.Error("wrong head")
	}
	if mustV(col.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(col.Len(t.Context())))
	}
}

func TestCollectionGetOutOfBounds(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(t.Context(), term.NewLiteral("a"))
	_, ok, _ := col.Get(t.Context(), 5)
	if ok {
		t.Error("expected out of bounds")
	}
}

func TestCollectionIterEarlyStop(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(t.Context(), term.NewLiteral("a"))
	col.Append(t.Context(), term.NewLiteral("b"))
	col.Append(t.Context(), term.NewLiteral("c"))

	count := 0
	col.Iter(t.Context())(func(term.Term, error) bool {
		count++
		return count < 2
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// --- conjunctive_graph_test.go ---

func TestConjunctiveGraphStore(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	if cg.Store() == nil {
		t.Error("expected store")
	}
}

func TestConjunctiveGraphBind(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	ns, _ := term.NewURIRef("http://example.org/")
	cg.Bind(t.Context(), "ex", ns)
	count := 0
	cg.Namespaces(t.Context())(func(pair_ store.NamespaceBinding, _ error) bool { count++; return true })
	if count < 5 {
		t.Errorf("expected >=5, got %d", count)
	}
}

func TestConjunctiveGraphTriples(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(t.Context(), s, p, term.NewLiteral("a"), nil)
	cg.Add(t.Context(), s, p, term.NewLiteral("b"), nil)

	count := 0
	cg.Triples(t.Context(), nil, nil, nil)(func(term.Triple, error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestConjunctiveGraphQuads(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(t.Context(), s, p, term.NewLiteral("hello"), nil)

	count := 0
	cg.Quads(t.Context(), nil, nil, nil)(func(term.Quad, error) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestConjunctiveGraphQuadsHaveGraphContext(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(t.Context(), s, p, term.NewLiteral("hello"), nil)

	cg.Quads(t.Context(), nil, nil, nil)(func(q term.Quad, _ error) bool {
		if q.Graph == nil {
			t.Error("Quad.Graph should not be nil")
		}
		return true
	})
}

func TestConjunctiveGraphRemove(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(t.Context(), s, p, term.NewLiteral("a"), nil)
	cg.Remove(t.Context(), s, &p, nil, nil)
	if mustV(cg.Len(t.Context())) != 0 {
		t.Errorf("expected 0, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphContexts(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	count := 0
	cg.Contexts(t.Context(), nil)(func(term.Term, error) bool { count++; return true })
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestConjunctiveGraphAddQuad(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g, _ := term.NewURIRef("http://example.org/g")
	cg.AddQuad(t.Context(), term.Quad{Triple: term.Triple{Subject: s, Predicate: p, Object: term.NewLiteral("v")}, Graph: g})
	if mustV(cg.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphAddQuadNilGraph(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.AddQuad(t.Context(), term.Quad{Triple: term.Triple{Subject: s, Predicate: p, Object: term.NewLiteral("v")}})
	if mustV(cg.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphGetContextNil(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	ctx := cg.GetContext(nil)
	if ctx != cg.DefaultContext() {
		t.Error("nil should return default")
	}
}

func TestDatasetAddGraph(t *testing.T) {
	ds := graph.NewDataset()
	g1, _ := term.NewURIRef("http://example.org/g1")
	ctx := ds.Graph(g1)
	ds.AddGraph(ctx)
	count := 0
	ds.Graphs()(func(*graph.Graph) bool { count++; return true })
	if count < 2 {
		t.Errorf("expected >=2, got %d", count)
	}
}

func TestDatasetGraphNil(t *testing.T) {
	ds := graph.NewDataset()
	g := ds.Graph(nil)
	if g != ds.DefaultContext() {
		t.Error("nil should return default")
	}
}

func TestDatasetGraphExisting(t *testing.T) {
	ds := graph.NewDataset()
	g1, _ := term.NewURIRef("http://example.org/g1")
	a := ds.Graph(g1)
	b := ds.Graph(g1)
	if a != b {
		t.Error("same id should return same graph")
	}
}

func TestDatasetRemoveDefaultGraph(t *testing.T) {
	ds := graph.NewDataset()
	ds.RemoveGraph(t.Context(), ds.DefaultContext().Identifier())
	count := 0
	ds.Graphs()(func(*graph.Graph) bool { count++; return true })
	if count != 1 {
		t.Errorf("default graph should not be removed, got %d", count)
	}
}
