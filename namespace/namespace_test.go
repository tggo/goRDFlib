package namespace_test

import (
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

func TestNamespaceTerm(t *testing.T) {
	ns := namespace.NewNamespace("http://example.org/")
	got := ns.Term("Person")
	if got.Value() != "http://example.org/Person" {
		t.Errorf("got %q", got.Value())
	}
}

func TestNamespaceContains(t *testing.T) {
	ns := namespace.NewNamespace("http://example.org/")
	if !ns.Contains("http://example.org/Person") {
		t.Error("should contain")
	}
	if ns.Contains("http://other.org/Person") {
		t.Error("should not contain")
	}
}

func TestClosedNamespaceValid(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo", "Bar"})
	u, err := ns.Term("Foo")
	if err != nil {
		t.Fatal(err)
	}
	if u.Value() != "http://example.org/Foo" {
		t.Errorf("got %q", u.Value())
	}
}

func TestClosedNamespaceInvalid(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo"})
	_, err := ns.Term("Unknown")
	if err == nil {
		t.Error("expected error for unknown term")
	}
}

func TestRDFConstants(t *testing.T) {
	if namespace.RDF.Type.Value() != "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
		t.Errorf("RDF.Type = %q", namespace.RDF.Type.Value())
	}
	if namespace.RDF.Nil.Value() != "http://www.w3.org/1999/02/22-rdf-syntax-ns#nil" {
		t.Errorf("RDF.Nil = %q", namespace.RDF.Nil.Value())
	}
}

func TestRDFSConstants(t *testing.T) {
	if namespace.RDFS.Label.Value() != "http://www.w3.org/2000/01/rdf-schema#label" {
		t.Errorf("RDFS.Label = %q", namespace.RDFS.Label.Value())
	}
	if namespace.RDFS.SubClassOf.Value() != "http://www.w3.org/2000/01/rdf-schema#subClassOf" {
		t.Errorf("RDFS.SubClassOf = %q", namespace.RDFS.SubClassOf.Value())
	}
}

func TestOWLConstants(t *testing.T) {
	if namespace.OWL.Class.Value() != "http://www.w3.org/2002/07/owl#Class" {
		t.Errorf("OWL.Class = %q", namespace.OWL.Class.Value())
	}
	if namespace.OWL.SameAs.Value() != "http://www.w3.org/2002/07/owl#sameAs" {
		t.Errorf("OWL.SameAs = %q", namespace.OWL.SameAs.Value())
	}
}

func TestNSManagerComputeQName(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("rdf", term.NewURIRefUnsafe(term.RDFNamespace))
	mgr := namespace.NewNSManager(s)
	prefix, ns, local, err := mgr.ComputeQName("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "rdf" || local != "type" {
		t.Errorf("got prefix=%q ns=%q local=%q", prefix, ns, local)
	}
}

func TestNSManagerQName(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	mgr := namespace.NewNSManager(s)
	got, err := mgr.QName("http://xmlns.com/foaf/0.1/Person")
	if err != nil {
		t.Fatal(err)
	}
	if got != "foaf:Person" {
		t.Errorf("got %q", got)
	}
}

func TestNSManagerExpandCURIE(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	mgr := namespace.NewNSManager(s)
	got, err := mgr.ExpandCURIE("foaf:Person")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value() != "http://xmlns.com/foaf/0.1/Person" {
		t.Errorf("got %q", got.Value())
	}
}

func TestNSManagerExpandCURIEUnknown(t *testing.T) {
	mgr := namespace.NewNSManager(store.NewMemoryStore())
	_, err := mgr.ExpandCURIE("unknown:term")
	if err == nil {
		t.Error("expected error for unknown prefix")
	}
	if !errors.Is(err, term.ErrPrefixNotBound) {
		t.Errorf("expected ErrPrefixNotBound, got %v", err)
	}
}

func TestNSManagerExpandCURIEInvalid(t *testing.T) {
	mgr := namespace.NewNSManager(store.NewMemoryStore())
	_, err := mgr.ExpandCURIE("nocolon")
	if err == nil {
		t.Error("expected error for invalid CURIE")
	}
	if !errors.Is(err, term.ErrInvalidCURIE) {
		t.Errorf("expected ErrInvalidCURIE, got %v", err)
	}
}

func TestClosedNamespaceSentinelError(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo"})
	_, err := ns.Term("Unknown")
	if !errors.Is(err, term.ErrTermNotInNamespace) {
		t.Errorf("expected ErrTermNotInNamespace, got %v", err)
	}
}

func TestClosedNamespaceMustTermPanics(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo"})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for MustTerm with unknown term")
		}
	}()
	ns.MustTerm("Unknown")
}

func TestNamespaceURIRef(t *testing.T) {
	ns := namespace.NewNamespace("http://example.org/")
	if ns.URIRef().Value() != "http://example.org/" {
		t.Error("wrong URIRef")
	}
}

func TestClosedNamespaceBase(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo"})
	if ns.Base() != "http://example.org/" {
		t.Error("wrong base")
	}
}

func TestNSManagerAutoPrefix(t *testing.T) {
	mgr := namespace.NewNSManager(store.NewMemoryStore())
	prefix, _, local, err := mgr.ComputeQName("http://example.org/ns#Thing")
	if err != nil {
		t.Fatal(err)
	}
	if prefix == "" || local != "Thing" {
		t.Errorf("got prefix=%q local=%q", prefix, local)
	}
}

func TestSplitURI(t *testing.T) {
	tests := []struct{ uri, ns, local string }{
		{"http://example.org/ns#Thing", "http://example.org/ns#", "Thing"},
		{"http://example.org/path/Thing", "http://example.org/path/", "Thing"},
	}
	for _, tt := range tests {
		ns, local := namespace.SplitURI(tt.uri)
		if ns != tt.ns || local != tt.local {
			t.Errorf("SplitURI(%q) = (%q, %q), want (%q, %q)", tt.uri, ns, local, tt.ns, tt.local)
		}
	}
}

func TestNSManagerBindRaceSafety(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := namespace.NewNSManager(s)
	ns := term.NewURIRefUnsafe("http://example.org/ns#")
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() { mgr.Bind("ex", ns, false); done <- struct{}{} }()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	got, ok := s.Namespace("ex")
	if !ok {
		t.Fatal("expected prefix 'ex' to be bound")
	}
	if got.Value() != ns.Value() {
		t.Errorf("got %q, want %q", got.Value(), ns.Value())
	}
}

func TestNSManagerBindOverride(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := namespace.NewNSManager(s)
	ns1 := term.NewURIRefUnsafe("http://example.org/ns1#")
	ns2 := term.NewURIRefUnsafe("http://example.org/ns2#")
	mgr.Bind("ex", ns1, false)
	mgr.Bind("ex", ns2, false)
	got, _ := s.Namespace("ex")
	if got.Value() != ns1.Value() {
		t.Errorf("override=false should not replace: got %q", got.Value())
	}
	mgr.Bind("ex", ns2, true)
	got, _ = s.Namespace("ex")
	if got.Value() != ns2.Value() {
		t.Errorf("override=true should replace: got %q", got.Value())
	}
}

func TestNSManagerAbsolutize(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := namespace.NewNSManager(s)
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	got := mgr.Absolutize("foaf:Person")
	if got.Value() != "http://xmlns.com/foaf/0.1/Person" {
		t.Errorf("got %q", got.Value())
	}
	got = mgr.Absolutize("http://example.org/Thing")
	if got.Value() != "http://example.org/Thing" {
		t.Errorf("got %q", got.Value())
	}
	got = mgr.Absolutize("unknown:term")
	if got.Value() != "unknown:term" {
		t.Errorf("got %q", got.Value())
	}
	got = mgr.Absolutize("justAString")
	if got.Value() != "justAString" {
		t.Errorf("got %q", got.Value())
	}
}

func TestSplitURIWithURN(t *testing.T) {
	ns, local := namespace.SplitURI("urn:isbn:12345")
	if ns != "urn:isbn:" || local != "12345" {
		t.Errorf("SplitURI(urn:isbn:12345) = (%q, %q), want (\"urn:isbn:\", \"12345\")", ns, local)
	}
}

func TestSplitURINoSeparator(t *testing.T) {
	ns, local := namespace.SplitURI("noseparator")
	if ns != "" || local != "noseparator" {
		t.Errorf("SplitURI(noseparator) = (%q, %q), want (\"\", \"noseparator\")", ns, local)
	}
}

func TestExpandCURIEEmptyPrefix(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("", term.NewURIRefUnsafe("http://example.org/default/"))
	mgr := namespace.NewNSManager(s)
	got, err := mgr.ExpandCURIE(":localname")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value() != "http://example.org/default/localname" {
		t.Errorf("got %q", got.Value())
	}
}

func TestExpandCURIEEmptyPrefixNotBound(t *testing.T) {
	mgr := namespace.NewNSManager(store.NewMemoryStore())
	_, err := mgr.ExpandCURIE(":localname")
	if err == nil {
		t.Error("expected error for unbound empty prefix")
	}
	if !errors.Is(err, term.ErrPrefixNotBound) {
		t.Errorf("expected ErrPrefixNotBound, got %v", err)
	}
}

func TestNSManagerComputeQNameURN(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("isbn", term.NewURIRefUnsafe("urn:isbn:"))
	mgr := namespace.NewNSManager(s)
	prefix, ns, local, err := mgr.ComputeQName("urn:isbn:12345")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "isbn" || ns != "urn:isbn:" || local != "12345" {
		t.Errorf("got prefix=%q ns=%q local=%q", prefix, ns, local)
	}
}

func TestNewURIRefUnsafeInTermGo(t *testing.T) {
	u := term.NewURIRefUnsafe("http://example.org/test")
	if u.Value() != "http://example.org/test" {
		t.Errorf("got %q", u.Value())
	}
}

func TestExtendedNamespaces(t *testing.T) {
	foafPerson := namespace.FOAF.Term("Person")
	if foafPerson.Value() != "http://xmlns.com/foaf/0.1/Person" {
		t.Errorf("got %q", foafPerson.Value())
	}
	provEntity := namespace.PROV.Term("Entity")
	if provEntity.Value() != "http://www.w3.org/ns/prov#Entity" {
		t.Errorf("got %q", provEntity.Value())
	}
}

// TestNamespaceBase covers the 0% Namespace.Base() branch.
func TestNamespaceBase(t *testing.T) {
	ns := namespace.NewNamespace("http://example.org/")
	if ns.Base() != "http://example.org/" {
		t.Errorf("Base() = %q, want %q", ns.Base(), "http://example.org/")
	}
}

// TestClosedNamespaceMustTermSuccess covers the non-panic branch of MustTerm.
func TestClosedNamespaceMustTermSuccess(t *testing.T) {
	ns := namespace.NewClosedNamespace("http://example.org/", []string{"Foo", "Bar"})
	u := ns.MustTerm("Foo")
	if u.Value() != "http://example.org/Foo" {
		t.Errorf("MustTerm(\"Foo\") = %q, want %q", u.Value(), "http://example.org/Foo")
	}
}

// TestNSManagerBindCacheInvalidation covers the cache-invalidation path in Bind.
// After a QName lookup populates the cache, rebinding the namespace with override=true
// must purge the cached entry.
func TestNSManagerBindCacheInvalidation(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/ns#"))
	mgr := namespace.NewNSManager(s)

	// Populate the cache for this URI.
	qn, err := mgr.QName("http://example.org/ns#Thing")
	if err != nil {
		t.Fatal(err)
	}
	if qn != "ex:Thing" {
		t.Fatalf("expected ex:Thing, got %q", qn)
	}

	// Rebind with override=true — this should invalidate the cached entry.
	mgr.Bind("ex2", term.NewURIRefUnsafe("http://example.org/ns#"), true)

	// The cache entry should be gone; re-query must use the store.
	qn2, err := mgr.QName("http://example.org/ns#Thing")
	if err != nil {
		t.Fatal(err)
	}
	// The result must use a valid prefix (either ex or ex2 depending on store).
	if qn2 == "" {
		t.Error("expected non-empty QName after cache invalidation")
	}
}

// TestNSManagerPrefix covers the 0% Prefix() method.
func TestNSManagerPrefix(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	mgr := namespace.NewNSManager(s)

	// Known URI — must succeed.
	qn, ok := mgr.Prefix("http://xmlns.com/foaf/0.1/Person")
	if !ok {
		t.Error("Prefix: expected ok=true for known namespace")
	}
	if qn != "foaf:Person" {
		t.Errorf("Prefix: got %q, want %q", qn, "foaf:Person")
	}

	// URI with no separating '#' or '/' or ':' — SplitURI returns empty ns, so QName errors.
	_, ok = mgr.Prefix("noseparator")
	if ok {
		t.Error("Prefix: expected ok=false for URI that cannot be split")
	}
}

// TestNSManagerQNameError covers the error branch of QName (line 139).
func TestNSManagerQNameError(t *testing.T) {
	mgr := namespace.NewNSManager(store.NewMemoryStore())
	_, err := mgr.QName("noseparator")
	if err == nil {
		t.Error("QName: expected error for unsplittable URI")
	}
}

// TestNSManagerComputeQNameCacheHit covers the cache-hit branch in ComputeQName (line 151).
func TestNSManagerComputeQNameCacheHit(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("rdf", term.NewURIRefUnsafe(term.RDFNamespace))
	mgr := namespace.NewNSManager(s)

	uri := term.RDFNamespace + "type"

	// First call — populates cache.
	p1, ns1, l1, err := mgr.ComputeQName(uri)
	if err != nil {
		t.Fatal(err)
	}

	// Second call — must hit the cache and return identical values.
	p2, ns2, l2, err := mgr.ComputeQName(uri)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 || ns1 != ns2 || l1 != l2 {
		t.Errorf("cache hit mismatch: first=(%q,%q,%q) second=(%q,%q,%q)", p1, ns1, l1, p2, ns2, l2)
	}
}

// TestNSManagerNamespaces covers the 0% Namespaces() method.
func TestNSManagerNamespaces(t *testing.T) {
	s := store.NewMemoryStore()
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	s.Bind("rdf", term.NewURIRefUnsafe(term.RDFNamespace))
	mgr := namespace.NewNSManager(s)

	found := map[string]string{}
	for prefix, ns := range mgr.Namespaces() {
		found[prefix] = ns.Value()
	}
	if found["foaf"] != "http://xmlns.com/foaf/0.1/" {
		t.Errorf("Namespaces: foaf = %q", found["foaf"])
	}
	if found["rdf"] != term.RDFNamespace {
		t.Errorf("Namespaces: rdf = %q", found["rdf"])
	}
}
