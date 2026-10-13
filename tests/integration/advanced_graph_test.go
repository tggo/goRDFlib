package integration_test

import (
	"testing"

	. "github.com/tggo/goRDFlib"
)

// --- ConjunctiveGraph tests ---
// Ported from: rdflib.graph.ConjunctiveGraph

func TestConjunctiveGraphAdd(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	cg.Add(t.Context(), s, p, NewLiteral("hello"), nil)
	if mustV(cg.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphMultipleContexts(t *testing.T) {
	// Ported from: rdflib.graph.ConjunctiveGraph — multiple named graphs
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1, _ := NewURIRef("http://example.org/g1")
	g2, _ := NewURIRef("http://example.org/g2")

	cg.Add(t.Context(), s, p, NewLiteral("a"), g1)
	cg.Add(t.Context(), s, p, NewLiteral("b"), g2)

	if mustV(cg.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphGetContext(t *testing.T) {
	// Ported from: rdflib.graph.ConjunctiveGraph.get_context
	cg := NewConjunctiveGraph()
	g1, _ := NewURIRef("http://example.org/g1")
	ctx := cg.GetContext(g1)
	if ctx == nil {
		t.Fatal("expected graph")
	}
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	ctx.Add(t.Context(), s, p, NewLiteral("hello"))
	if mustV(cg.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(cg.Len(t.Context())))
	}
}

func TestConjunctiveGraphDefaultContext(t *testing.T) {
	cg := NewConjunctiveGraph()
	def := cg.DefaultContext()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	def.Add(t.Context(), s, p, NewLiteral("default"))
	if mustV(cg.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(cg.Len(t.Context())))
	}
}

// --- Dataset tests ---
// Ported from: test/test_dataset/

func TestDatasetGraph(t *testing.T) {
	// Ported from: rdflib.graph.Dataset.graph
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	ctx := ds.Graph(g1)
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	ctx.Add(t.Context(), s, p, NewLiteral("hello"))
	if mustV(ds.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(ds.Len(t.Context())))
	}
}

func TestDatasetGraphs(t *testing.T) {
	// Ported from: rdflib.graph.Dataset.graphs
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	g2, _ := NewURIRef("http://example.org/g2")
	ds.Graph(g1)
	ds.Graph(g2)

	count := 0
	ds.Graphs()(func(g *Graph) bool {
		count++
		return true
	})
	// default + g1 + g2 = 3
	if count != 3 {
		t.Errorf("expected 3 graphs, got %d", count)
	}
}

func TestDatasetRemoveGraph(t *testing.T) {
	// Ported from: rdflib.graph.Dataset.remove_graph
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	ctx := ds.Graph(g1)
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	ctx.Add(t.Context(), s, p, NewLiteral("hello"))

	ds.RemoveGraph(t.Context(), g1)
	count := 0
	ds.Graphs()(func(g *Graph) bool {
		count++
		return true
	})
	if count != 1 { // only default remains
		t.Errorf("expected 1 graph after remove, got %d", count)
	}
}

// --- Collection tests ---
// Ported from: rdflib.collection.Collection

func TestCollectionAppendAndLen(t *testing.T) {
	// Ported from: rdflib.collection.Collection.append, __len__
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("a"))
	col.Append(t.Context(), NewLiteral("b"))
	col.Append(t.Context(), NewLiteral("c"))

	if mustV(col.Len(t.Context())) != 3 {
		t.Errorf("expected 3, got %d", mustV(col.Len(t.Context())))
	}
}

func TestCollectionGet(t *testing.T) {
	// Ported from: rdflib.collection.Collection.__getitem__
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("a"))
	col.Append(t.Context(), NewLiteral("b"))

	val, ok, _ := col.Get(t.Context(), 0)
	if !ok || val.String() != "a" {
		t.Errorf("expected 'a', got %v", val)
	}
	val, ok, _ = col.Get(t.Context(), 1)
	if !ok || val.String() != "b" {
		t.Errorf("expected 'b', got %v", val)
	}
}

func TestCollectionIndex(t *testing.T) {
	// Ported from: rdflib.collection.Collection.index
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("a"))
	col.Append(t.Context(), NewLiteral("b"))
	col.Append(t.Context(), NewLiteral("c"))

	if idx, _ := col.Index(t.Context(), NewLiteral("b")); idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if idx, _ := col.Index(t.Context(), NewLiteral("z")); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestCollectionIter(t *testing.T) {
	// Ported from: rdflib.collection.Collection.__iter__
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("x"))
	col.Append(t.Context(), NewLiteral("y"))

	var items []string
	col.Iter(t.Context())(func(t Term, _ error) bool {
		items = append(items, t.String())
		return true
	})
	if len(items) != 2 || items[0] != "x" || items[1] != "y" {
		t.Errorf("expected [x y], got %v", items)
	}
}

func TestCollectionClear(t *testing.T) {
	// Ported from: rdflib.collection.Collection.clear
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("a"))
	col.Append(t.Context(), NewLiteral("b"))
	col.Clear(t.Context())

	if mustV(col.Len(t.Context())) != 0 {
		t.Errorf("expected 0 after clear, got %d", mustV(col.Len(t.Context())))
	}
}

// --- Resource tests ---
// Ported from: rdflib.resource.Resource

func TestResourceAdd(t *testing.T) {
	// Ported from: rdflib.resource.Resource.add
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/Alice")
	r := NewResource(g, s)
	name, _ := NewURIRef("http://example.org/name")
	r.Add(t.Context(), name, NewLiteral("Alice"))

	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestResourceValue(t *testing.T) {
	// Ported from: rdflib.resource.Resource.value
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/Alice")
	name, _ := NewURIRef("http://example.org/name")
	g.Add(t.Context(), s, name, NewLiteral("Alice"))

	r := NewResource(g, s)
	val, ok, _ := r.Value(t.Context(), name)
	if !ok || val.String() != "Alice" {
		t.Errorf("expected Alice, got %v", val)
	}
}

func TestResourceSet(t *testing.T) {
	// Ported from: rdflib.resource.Resource.set
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("old"))

	r := NewResource(g, s)
	r.Set(t.Context(), p, NewLiteral("new"))

	val, ok, _ := r.Value(t.Context(), p)
	if !ok || val.String() != "new" {
		t.Errorf("expected new, got %v", val)
	}
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1 after set, got %d", mustV(g.Len(t.Context())))
	}
}

func TestResourceRemove(t *testing.T) {
	// Ported from: rdflib.resource.Resource.remove
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("val"))

	r := NewResource(g, s)
	r.Remove(t.Context(), p, NewLiteral("val"))

	if mustV(g.Len(t.Context())) != 0 {
		t.Errorf("expected 0, got %d", mustV(g.Len(t.Context())))
	}
}

func TestResourceObjects(t *testing.T) {
	// Ported from: rdflib.resource.Resource.objects
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("a"))
	g.Add(t.Context(), s, p, NewLiteral("b"))

	r := NewResource(g, s)
	count := 0
	r.Objects(t.Context(), p)(func(Term, error) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestResourceSubjects(t *testing.T) {
	// Ported from: rdflib.resource.Resource.subjects
	g := NewGraph()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(t.Context(), s1, p, o)
	g.Add(t.Context(), s2, p, o)

	r := NewResource(g, o)
	count := 0
	r.Subjects(t.Context(), p)(func(Term, error) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestResourcePredicateObjects(t *testing.T) {
	// Ported from: rdflib.resource.Resource.predicate_objects
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	g.Add(t.Context(), s, p1, NewLiteral("a"))
	g.Add(t.Context(), s, p2, NewLiteral("b"))

	r := NewResource(g, s)
	count := 0
	r.PredicateObjects(t.Context())(func(pair_ TermPair, _ error) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}
