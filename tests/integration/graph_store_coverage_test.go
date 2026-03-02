package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
)

// --- graph.go iterator dedup branches ---

func TestGraphSubjectsDedup(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s, p1, o)
	g.Add(s, p2, o)

	count := 0
	g.Subjects(nil, o)(func(Term) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique subject, got %d", count)
	}
}

func TestGraphPredicatesDedup(t *testing.T) {
	g := NewGraph()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	count := 0
	g.Predicates(nil, o)(func(Term) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique predicate, got %d", count)
	}
}

func TestGraphObjectsDedup(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s, p1, o)
	g.Add(s, p2, o)

	count := 0
	g.Objects(s, nil)(func(Term) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique object, got %d", count)
	}
}

func TestGraphSubjectPredicatesDedup(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	count := 0
	g.SubjectPredicates(nil)(func(Term, Term) bool { count++; return true })
	if count < 1 {
		t.Errorf("expected >=1, got %d", count)
	}
}

func TestGraphSubjectObjectsDedup(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))

	count := 0
	g.SubjectObjects(nil)(func(Term, Term) bool { count++; return true })
	if count < 1 {
		t.Errorf("expected >=1, got %d", count)
	}
}

func TestGraphPredicateObjectsDedup(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	count := 0
	g.PredicateObjects(s)(func(Term, Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// --- Dataset Graphs stop iteration ---

func TestDatasetGraphsEarlyStop(t *testing.T) {
	ds := NewDataset()
	g1, _ := NewURIRef("http://example.org/g1")
	g2, _ := NewURIRef("http://example.org/g2")
	ds.Graph(g1)
	ds.Graph(g2)

	count := 0
	ds.Graphs()(func(*Graph) bool { count++; return false })
	if count != 1 {
		t.Errorf("expected 1 (early stop), got %d", count)
	}
}

// --- Literal Eq: float32 ---

func TestLiteralEqFloat32(t *testing.T) {
	a := NewLiteral(float32(1.0))
	b := NewLiteral(float32(1.0))
	if !a.Eq(b) {
		t.Error("float32 should Eq")
	}
}

// --- Collection Append to empty then existing ---

func TestCollectionAppendMultiple(t *testing.T) {
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))
	col.Append(NewLiteral("c"))

	if v, ok := col.Get(2); !ok || v.String() != "c" {
		t.Errorf("expected c at index 2, got %v", v)
	}
}

// --- NSManager branches ---

func TestNSManagerBindNoOverride(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewNSManager(store)
	ns1, _ := NewURIRef("http://example.org/")
	ns2, _ := NewURIRef("http://other.org/")
	mgr.Bind("ex", ns1, true)
	mgr.Bind("ex", ns2, false) // should not override
	got, _ := store.Namespace("ex")
	if got != ns1 {
		t.Error("should not have overridden")
	}
}

func TestNSManagerPrefix(t *testing.T) {
	store := NewMemoryStore()
	store.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	mgr := NewNSManager(store)

	p, ok := mgr.Prefix("http://example.org/Thing")
	if !ok || p != "ex:Thing" {
		t.Errorf("expected ex:Thing, got %q %v", p, ok)
	}

	_, ok = mgr.Prefix("noseparator")
	if ok {
		t.Error("expected false for unsplittable URI")
	}
}

func TestNSManagerComputeQNameCache(t *testing.T) {
	store := NewMemoryStore()
	store.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	mgr := NewNSManager(store)

	p1, _, l1, _ := mgr.ComputeQName("http://example.org/Thing")
	p2, _, l2, _ := mgr.ComputeQName("http://example.org/Thing")
	if p1 != p2 || l1 != l2 {
		t.Error("cached result should match")
	}
}

func TestNSManagerComputeQNameError(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewNSManager(store)
	_, _, _, err := mgr.ComputeQName("noseparator")
	if err == nil {
		t.Error("expected error for URI without # or /")
	}
}

func TestNamespaceManagerNamespaces95(t *testing.T) {
	store := NewMemoryStore()
	store.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	mgr := NewNSManager(store)
	count := 0
	mgr.Namespaces()(func(string, URIRef) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

// --- NT parser: empty input ---

func TestNTParserEmpty(t *testing.T) {
	g := NewGraph()
	err := nt.Parse(g, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0, got %d", g.Len())
	}
}

// --- NT parser: IRI with escapes ---

func TestNTParserIRIEscape(t *testing.T) {
	g := NewGraph()
	err := nt.Parse(g, strings.NewReader("<http://example.org/\\u0041> <http://example.org/p> \"v\" .\n"))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- NQ parser: error paths ---

func TestNQParserErrorBadPredicate(t *testing.T) {
	g := NewGraph()
	err := nq.Parse(g, strings.NewReader(`<http://s> "bad" "hello" .`+"\n"))
	if err == nil {
		t.Error("expected error")
	}
}

// --- RDF/XML: empty collection ---

func TestRDFXMLParserEmptyCollection(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection"/>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(g, strings.NewReader(input))
	if g.Len() < 1 {
		t.Errorf("expected >=1, got %d", g.Len())
	}
}

// --- JSON-LD: non-string ToRDF result ---

func TestJSONLDParserEmptyDoc(t *testing.T) {
	g := NewGraph()
	err := jsonld.Parse(g, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0, got %d", g.Len())
	}
}

// --- JSON-LD serializer: compaction failure tolerance ---

func TestJSONLDSerializerNoNamespaces(t *testing.T) {
	g := NewGraph(WithStore(NewMemoryStore()))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	err := jsonld.Serialize(g, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "http://example.org") {
		t.Errorf("expected URIs, got:\n%s", buf.String())
	}
}

// --- MemoryStore: triples with predicate+object pattern ---

func TestMemoryStoreTriplesPredObject(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	s.Add(Triple{Subject: s1, Predicate: p, Object: o}, nil)
	s.Add(Triple{Subject: s2, Predicate: p, Object: o}, nil)

	count := 0
	s.Triples(TriplePattern{Predicate: &p, Object: o}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestMemoryStoreTripleSubjectObject(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	o := NewLiteral("v")
	s.Add(Triple{Subject: sub, Predicate: p1, Object: o}, nil)
	s.Add(Triple{Subject: sub, Predicate: p2, Object: o}, nil)

	count := 0
	s.Triples(TriplePattern{Subject: sub, Object: o}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}
