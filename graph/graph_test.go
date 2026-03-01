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
	g.Add(s, p, o)
	return g, s, p, o
}

func TestGraphAddAndLen(t *testing.T) {
	g, _, _, _ := makeTestGraph(t)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestGraphContains(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	if !g.Contains(s, p, o) {
		t.Error("should contain the triple")
	}
	p2, _ := term.NewURIRef("http://example.org/other")
	if g.Contains(s, p2, o) {
		t.Error("should not contain different predicate")
	}
}

func TestGraphRemove(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	g.Remove(s, &p, o)
	if g.Len() != 0 {
		t.Errorf("expected 0 after remove, got %d", g.Len())
	}
}

func TestGraphSet(t *testing.T) {
	g, s, p, _ := makeTestGraph(t)
	newObj := term.NewLiteral("world")
	g.Set(s, p, newObj)
	if g.Len() != 1 {
		t.Errorf("expected 1 after set, got %d", g.Len())
	}
	val, ok := g.Value(s, &p, nil)
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
	g.Add(s, p2, term.NewLiteral("world"))

	count := 0
	g.Triples(s, nil, nil)(func(term.Triple) bool {
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
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	var subjects []term.Term
	g.Subjects(&p, o)(func(t term.Term) bool {
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
	g.Add(s, p, term.NewLiteral("a"))
	g.Add(s, p, term.NewLiteral("b"))

	var objects []term.Term
	g.Objects(s, &p)(func(t term.Term) bool {
		objects = append(objects, t)
		return true
	})
	if len(objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objects))
	}
}

func TestGraphValue(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok := g.Value(s, &p, nil)
	if !ok || val.N3() != o.N3() {
		t.Errorf("expected %q, got %v", o.N3(), val)
	}
}

func TestGraphFluent(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	g.Add(s, p1, term.NewLiteral("a")).Add(s, p2, term.NewLiteral("b"))
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestGraphDefaultNamespaces(t *testing.T) {
	g := graph.NewGraph()
	count := 0
	g.Namespaces()(func(prefix string, ns term.URIRef) bool {
		count++
		return true
	})
	if count < 4 {
		t.Errorf("expected at least 4 default namespaces, got %d", count)
	}
}

func TestGraphQName(t *testing.T) {
	g := graph.NewGraph()
	got := g.QName("http://www.w3.org/2001/XMLSchema#string")
	if got != "xsd:string" {
		t.Errorf("expected xsd:string, got %q", got)
	}
}

func TestGraphBind(t *testing.T) {
	g := graph.NewGraph()
	ns, _ := term.NewURIRef("http://example.org/ns#")
	g.Bind("ex", ns)
	got := g.QName("http://example.org/ns#Thing")
	if got != "ex:Thing" {
		t.Errorf("expected ex:Thing, got %q", got)
	}
}

func TestGraphUnion(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g1.Add(s, p, term.NewLiteral("a"))
	g2.Add(s, p, term.NewLiteral("b"))

	u := g1.Union(g2)
	if u.Len() != 2 {
		t.Errorf("expected 2, got %d", u.Len())
	}
}

func TestGraphIntersection(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	shared := term.NewLiteral("shared")
	g1.Add(s, p, shared)
	g1.Add(s, p, term.NewLiteral("only1"))
	g2.Add(s, p, shared)
	g2.Add(s, p, term.NewLiteral("only2"))

	inter := g1.Intersection(g2)
	if inter.Len() != 1 {
		t.Errorf("expected 1, got %d", inter.Len())
	}
}

func TestGraphDifference(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	shared := term.NewLiteral("shared")
	g1.Add(s, p, shared)
	g1.Add(s, p, term.NewLiteral("only1"))
	g2.Add(s, p, shared)

	diff := g1.Difference(g2)
	if diff.Len() != 1 {
		t.Errorf("expected 1, got %d", diff.Len())
	}
}

func TestGraphConnected(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(s, p, o)

	if !g.Connected() {
		t.Error("single-edge graph should be connected")
	}

	g2 := graph.NewGraph()
	s2, _ := term.NewURIRef("http://example.org/s2")
	o2, _ := term.NewURIRef("http://example.org/o2")
	g2.Add(s, p, o)
	g2.Add(s2, p, o2)
	if g2.Connected() {
		t.Error("disconnected graph should not be connected")
	}
}

func TestGraphAllNodes(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(s, p, o)

	nodes := g.AllNodes()
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
	g.Add(s, p1, term.NewLiteral("a"))
	g.Add(s, p2, term.NewLiteral("b"))

	count := 0
	g.Predicates(s, nil)(func(term.Term) bool { count++; return true })
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
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	count := 0
	g.SubjectPredicates(o)(func(term.Term, term.Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphSubjectObjects(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(s, p, term.NewLiteral("a"))
	g.Add(s, p, term.NewLiteral("b"))

	count := 0
	g.SubjectObjects(&p)(func(term.Term, term.Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphValuePredicate(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(s, p, term.NewLiteral("v"))

	val, ok := g.Value(s, nil, term.NewLiteral("v"))
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
	g.Add(s, p, term.NewLiteral("v"))

	val, ok := g.Value(nil, &p, term.NewLiteral("v"))
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
	_, ok := g.Value(nil, &p, term.NewLiteral("nope"))
	if ok {
		t.Error("expected not found")
	}
}

func TestGraphQNameNoMatch(t *testing.T) {
	g := graph.NewGraph()
	got := g.QName("http://unknown.org/Thing")
	if got != "http://unknown.org/Thing" {
		t.Errorf("expected raw URI, got %q", got)
	}
}

func TestGraphConnectedEmpty(t *testing.T) {
	g := graph.NewGraph()
	if !g.Connected() {
		t.Error("empty graph should be connected")
	}
}

func TestGraphConnectedSingleNode(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(s, p, term.NewLiteral("v"))
	if !g.Connected() {
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
	g.Add(head, namespace.RDF.First, term.NewLiteral("a"))
	g.Add(head, namespace.RDF.Rest, namespace.RDF.Nil)

	col := graph.NewCollection(g, head)
	if col.Head() != head {
		t.Error("wrong head")
	}
	if col.Len() != 1 {
		t.Errorf("expected 1, got %d", col.Len())
	}
}

func TestCollectionGetOutOfBounds(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(term.NewLiteral("a"))
	_, ok := col.Get(5)
	if ok {
		t.Error("expected out of bounds")
	}
}

func TestCollectionIterEarlyStop(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(term.NewLiteral("a"))
	col.Append(term.NewLiteral("b"))
	col.Append(term.NewLiteral("c"))

	count := 0
	col.Iter()(func(term.Term) bool {
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
	cg.Bind("ex", ns)
	count := 0
	cg.Namespaces()(func(string, term.URIRef) bool { count++; return true })
	if count < 5 {
		t.Errorf("expected >=5, got %d", count)
	}
}

func TestConjunctiveGraphTriples(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(s, p, term.NewLiteral("a"), nil)
	cg.Add(s, p, term.NewLiteral("b"), nil)

	count := 0
	cg.Triples(nil, nil, nil)(func(term.Triple) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestConjunctiveGraphQuads(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(s, p, term.NewLiteral("hello"), nil)

	count := 0
	cg.Quads(nil, nil, nil)(func(term.Quad) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestConjunctiveGraphQuadsHaveGraphContext(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.Add(s, p, term.NewLiteral("hello"), nil)

	cg.Quads(nil, nil, nil)(func(q term.Quad) bool {
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
	cg.Add(s, p, term.NewLiteral("a"), nil)
	cg.Remove(s, &p, nil, nil)
	if cg.Len() != 0 {
		t.Errorf("expected 0, got %d", cg.Len())
	}
}

func TestConjunctiveGraphContexts(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	count := 0
	cg.Contexts(nil)(func(term.Term) bool { count++; return true })
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestConjunctiveGraphAddQuad(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g, _ := term.NewURIRef("http://example.org/g")
	cg.AddQuad(term.Quad{Triple: term.Triple{Subject: s, Predicate: p, Object: term.NewLiteral("v")}, Graph: g})
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
	}
}

func TestConjunctiveGraphAddQuadNilGraph(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	cg.AddQuad(term.Quad{Triple: term.Triple{Subject: s, Predicate: p, Object: term.NewLiteral("v")}})
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
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
	ds.RemoveGraph(ds.DefaultContext().Identifier())
	count := 0
	ds.Graphs()(func(*graph.Graph) bool { count++; return true })
	if count != 1 {
		t.Errorf("default graph should not be removed, got %d", count)
	}
}
