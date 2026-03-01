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
	cg.Add(s, p, NewLiteral("hello"), nil)
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
	}
}

func TestConjunctiveGraphMultipleContexts(t *testing.T) {
	// Ported from: rdflib.graph.ConjunctiveGraph — multiple named graphs
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1, _ := NewURIRef("http://example.org/g1")
	g2, _ := NewURIRef("http://example.org/g2")

	cg.Add(s, p, NewLiteral("a"), g1)
	cg.Add(s, p, NewLiteral("b"), g2)

	if cg.Len() != 2 {
		t.Errorf("expected 2, got %d", cg.Len())
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
	ctx.Add(s, p, NewLiteral("hello"))
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
	}
}

func TestConjunctiveGraphDefaultContext(t *testing.T) {
	cg := NewConjunctiveGraph()
	def := cg.DefaultContext()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	def.Add(s, p, NewLiteral("default"))
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
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
	ctx.Add(s, p, NewLiteral("hello"))
	if ds.Len() != 1 {
		t.Errorf("expected 1, got %d", ds.Len())
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
	ctx.Add(s, p, NewLiteral("hello"))

	ds.RemoveGraph(g1)
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
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))
	col.Append(NewLiteral("c"))

	if col.Len() != 3 {
		t.Errorf("expected 3, got %d", col.Len())
	}
}

func TestCollectionGet(t *testing.T) {
	// Ported from: rdflib.collection.Collection.__getitem__
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))

	val, ok := col.Get(0)
	if !ok || val.String() != "a" {
		t.Errorf("expected 'a', got %v", val)
	}
	val, ok = col.Get(1)
	if !ok || val.String() != "b" {
		t.Errorf("expected 'b', got %v", val)
	}
}

func TestCollectionIndex(t *testing.T) {
	// Ported from: rdflib.collection.Collection.index
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))
	col.Append(NewLiteral("c"))

	if idx := col.Index(NewLiteral("b")); idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if idx := col.Index(NewLiteral("z")); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestCollectionIter(t *testing.T) {
	// Ported from: rdflib.collection.Collection.__iter__
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("x"))
	col.Append(NewLiteral("y"))

	var items []string
	col.Iter()(func(t Term) bool {
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
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))
	col.Clear()

	if col.Len() != 0 {
		t.Errorf("expected 0 after clear, got %d", col.Len())
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
	r.Add(name, NewLiteral("Alice"))

	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestResourceValue(t *testing.T) {
	// Ported from: rdflib.resource.Resource.value
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/Alice")
	name, _ := NewURIRef("http://example.org/name")
	g.Add(s, name, NewLiteral("Alice"))

	r := NewResource(g, s)
	val, ok := r.Value(name)
	if !ok || val.String() != "Alice" {
		t.Errorf("expected Alice, got %v", val)
	}
}

func TestResourceSet(t *testing.T) {
	// Ported from: rdflib.resource.Resource.set
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("old"))

	r := NewResource(g, s)
	r.Set(p, NewLiteral("new"))

	val, ok := r.Value(p)
	if !ok || val.String() != "new" {
		t.Errorf("expected new, got %v", val)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 after set, got %d", g.Len())
	}
}

func TestResourceRemove(t *testing.T) {
	// Ported from: rdflib.resource.Resource.remove
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("val"))

	r := NewResource(g, s)
	r.Remove(p, NewLiteral("val"))

	if g.Len() != 0 {
		t.Errorf("expected 0, got %d", g.Len())
	}
}

func TestResourceObjects(t *testing.T) {
	// Ported from: rdflib.resource.Resource.objects
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	r := NewResource(g, s)
	count := 0
	r.Objects(p)(func(Term) bool {
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
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	r := NewResource(g, o)
	count := 0
	r.Subjects(p)(func(Term) bool {
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
	g.Add(s, p1, NewLiteral("a"))
	g.Add(s, p2, NewLiteral("b"))

	r := NewResource(g, s)
	count := 0
	r.PredicateObjects()(func(Term, Term) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}
