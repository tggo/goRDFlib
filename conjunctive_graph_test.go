package rdflibgo

import "testing"

func TestConjunctiveGraphStore(t *testing.T) {
	cg := NewConjunctiveGraph()
	if cg.Store() == nil {
		t.Error("expected store")
	}
}

func TestConjunctiveGraphBind(t *testing.T) {
	cg := NewConjunctiveGraph()
	ns, _ := NewURIRef("http://example.org/")
	cg.Bind("ex", ns)
	count := 0
	cg.Namespaces()(func(string, URIRef) bool { count++; return true })
	if count < 5 { // 4 defaults + ex
		t.Errorf("expected >=5, got %d", count)
	}
}

func TestConjunctiveGraphTriples(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	cg.Add(s, p, NewLiteral("a"), nil)
	cg.Add(s, p, NewLiteral("b"), nil)

	count := 0
	cg.Triples(nil, nil, nil)(func(Triple) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestConjunctiveGraphQuads(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	cg.Add(s, p, NewLiteral("hello"), nil)

	count := 0
	cg.Quads(nil, nil, nil)(func(Quad) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestConjunctiveGraphRemove(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	cg.Add(s, p, NewLiteral("a"), nil)
	cg.Remove(s, &p, nil, nil)
	if cg.Len() != 0 {
		t.Errorf("expected 0, got %d", cg.Len())
	}
}

func TestConjunctiveGraphContexts(t *testing.T) {
	cg := NewConjunctiveGraph()
	count := 0
	cg.Contexts(nil)(func(Term) bool { count++; return true })
	// SimpleMemory is not context-aware, returns empty
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestConjunctiveGraphAddQuad(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g, _ := NewURIRef("http://example.org/g")
	cg.AddQuad(Quad{Triple: Triple{Subject: s, Predicate: p, Object: NewLiteral("v")}, Graph: g})
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
	}
}

func TestConjunctiveGraphAddQuadNilGraph(t *testing.T) {
	cg := NewConjunctiveGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	cg.AddQuad(Quad{Triple: Triple{Subject: s, Predicate: p, Object: NewLiteral("v")}})
	if cg.Len() != 1 {
		t.Errorf("expected 1, got %d", cg.Len())
	}
}

func TestConjunctiveGraphGetContextNil(t *testing.T) {
	cg := NewConjunctiveGraph()
	ctx := cg.GetContext(nil)
	if ctx != cg.DefaultContext() {
		t.Error("nil should return default")
	}
}

func TestDatasetAddGraph(t *testing.T) {
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	ctx := ds.Graph(g1)
	ds.AddGraph(ctx)
	count := 0
	ds.Graphs()(func(*Graph) bool { count++; return true })
	if count < 2 {
		t.Errorf("expected >=2, got %d", count)
	}
}

func TestDatasetGraphNil(t *testing.T) {
	ds := NewDataset()
	g := ds.Graph(nil)
	if g != ds.DefaultContext() {
		t.Error("nil should return default")
	}
}

func TestDatasetGraphExisting(t *testing.T) {
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	a := ds.Graph(g1)
	b := ds.Graph(g1)
	if a != b {
		t.Error("same id should return same graph")
	}
}

func TestDatasetRemoveDefaultGraph(t *testing.T) {
	ds := NewDataset()
	ds.RemoveGraph(ds.DefaultContext().Identifier())
	count := 0
	ds.Graphs()(func(*Graph) bool { count++; return true })
	if count != 1 {
		t.Errorf("default graph should not be removed, got %d", count)
	}
}
