package rdflibgo

import "testing"

// Ported from: test/test_namespace/

func TestNamespaceTerm(t *testing.T) {
	// Ported from: rdflib.namespace.Namespace.__getattr__
	ns := NewNamespace("http://example.org/")
	got := ns.Term("Person")
	if got.Value() != "http://example.org/Person" {
		t.Errorf("got %q", got.Value())
	}
}

func TestNamespaceContains(t *testing.T) {
	// Ported from: rdflib.namespace.Namespace.__contains__
	ns := NewNamespace("http://example.org/")
	if !ns.Contains("http://example.org/Person") {
		t.Error("should contain")
	}
	if ns.Contains("http://other.org/Person") {
		t.Error("should not contain")
	}
}

func TestClosedNamespaceValid(t *testing.T) {
	// Ported from: rdflib.namespace.ClosedNamespace
	ns := NewClosedNamespace("http://example.org/", []string{"Foo", "Bar"})
	u, err := ns.Term("Foo")
	if err != nil {
		t.Fatal(err)
	}
	if u.Value() != "http://example.org/Foo" {
		t.Errorf("got %q", u.Value())
	}
}

func TestClosedNamespaceInvalid(t *testing.T) {
	// Ported from: rdflib.namespace.ClosedNamespace — unknown term error
	ns := NewClosedNamespace("http://example.org/", []string{"Foo"})
	_, err := ns.Term("Unknown")
	if err == nil {
		t.Error("expected error for unknown term")
	}
}

func TestRDFConstants(t *testing.T) {
	// Ported from: rdflib.namespace._RDF
	if RDF.Type.Value() != "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
		t.Errorf("RDF.Type = %q", RDF.Type.Value())
	}
	if RDF.Nil.Value() != "http://www.w3.org/1999/02/22-rdf-syntax-ns#nil" {
		t.Errorf("RDF.Nil = %q", RDF.Nil.Value())
	}
}

func TestRDFSConstants(t *testing.T) {
	// Ported from: rdflib.namespace._RDFS
	if RDFS.Label.Value() != "http://www.w3.org/2000/01/rdf-schema#label" {
		t.Errorf("RDFS.Label = %q", RDFS.Label.Value())
	}
	if RDFS.SubClassOf.Value() != "http://www.w3.org/2000/01/rdf-schema#subClassOf" {
		t.Errorf("RDFS.SubClassOf = %q", RDFS.SubClassOf.Value())
	}
}

func TestOWLConstants(t *testing.T) {
	// Ported from: rdflib.namespace._OWL
	if OWL.Class.Value() != "http://www.w3.org/2002/07/owl#Class" {
		t.Errorf("OWL.Class = %q", OWL.Class.Value())
	}
	if OWL.SameAs.Value() != "http://www.w3.org/2002/07/owl#sameAs" {
		t.Errorf("OWL.SameAs = %q", OWL.SameAs.Value())
	}
}

func TestNSManagerComputeQName(t *testing.T) {
	// Ported from: rdflib.namespace.NamespaceManager.compute_qname
	store := NewMemoryStore()
	store.Bind("rdf", NewURIRefUnsafe(RDFNamespace))
	mgr := NewNSManager(store)

	prefix, ns, local, err := mgr.ComputeQName("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "rdf" || local != "type" {
		t.Errorf("got prefix=%q ns=%q local=%q", prefix, ns, local)
	}
}

func TestNSManagerQName(t *testing.T) {
	// Ported from: rdflib.namespace.NamespaceManager.qname
	store := NewMemoryStore()
	store.Bind("foaf", NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	mgr := NewNSManager(store)

	got, err := mgr.QName("http://xmlns.com/foaf/0.1/Person")
	if err != nil {
		t.Fatal(err)
	}
	if got != "foaf:Person" {
		t.Errorf("got %q", got)
	}
}

func TestNSManagerExpandCURIE(t *testing.T) {
	// Ported from: rdflib.namespace.NamespaceManager.expand_curie
	store := NewMemoryStore()
	store.Bind("foaf", NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	mgr := NewNSManager(store)

	got, err := mgr.ExpandCURIE("foaf:Person")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value() != "http://xmlns.com/foaf/0.1/Person" {
		t.Errorf("got %q", got.Value())
	}
}

func TestNSManagerExpandCURIEUnknown(t *testing.T) {
	// Ported from: rdflib.namespace.NamespaceManager.expand_curie — unknown prefix
	mgr := NewNSManager(NewMemoryStore())
	_, err := mgr.ExpandCURIE("unknown:term")
	if err == nil {
		t.Error("expected error for unknown prefix")
	}
}

func TestNSManagerAutoPrefix(t *testing.T) {
	// Ported from: rdflib.namespace.NamespaceManager.compute_qname auto-generate
	mgr := NewNSManager(NewMemoryStore())
	prefix, _, local, err := mgr.ComputeQName("http://example.org/ns#Thing")
	if err != nil {
		t.Fatal(err)
	}
	if prefix == "" || local != "Thing" {
		t.Errorf("got prefix=%q local=%q", prefix, local)
	}
}

func TestSplitURI(t *testing.T) {
	// Ported from: rdflib.namespace.split_uri
	tests := []struct {
		uri, ns, local string
	}{
		{"http://example.org/ns#Thing", "http://example.org/ns#", "Thing"},
		{"http://example.org/path/Thing", "http://example.org/path/", "Thing"},
	}
	for _, tt := range tests {
		ns, local := splitURI(tt.uri)
		if ns != tt.ns || local != tt.local {
			t.Errorf("splitURI(%q) = (%q, %q), want (%q, %q)", tt.uri, ns, local, tt.ns, tt.local)
		}
	}
}

func TestExtendedNamespaces(t *testing.T) {
	// Verify extended namespace term creation works
	foafPerson := FOAF.Term("Person")
	if foafPerson.Value() != "http://xmlns.com/foaf/0.1/Person" {
		t.Errorf("got %q", foafPerson.Value())
	}
	provEntity := PROV.Term("Entity")
	if provEntity.Value() != "http://www.w3.org/ns/prov#Entity" {
		t.Errorf("got %q", provEntity.Value())
	}
}
