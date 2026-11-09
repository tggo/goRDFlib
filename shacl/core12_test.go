package shacl

import (
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// A triple term must survive fromRDFLib -> toTerm with the same TermKey, or
// anything keyed by it (provenance, initial bindings) misses.
func TestTripleTermRoundTrip(t *testing.T) {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	inner := term.NewTripleTerm(ex("s"), ex("p"), term.NewLiteral("o", term.WithLang("en")))
	outer := term.NewTripleTerm(term.NewBNode("b"), ex("q"), inner)

	for _, in := range []term.Term{inner, outer} {
		got := fromRDFLib(in)
		if !got.IsTripleTerm() || got.IsNone() {
			t.Fatalf("fromRDFLib(%v) = %v, want a triple term", in, got)
		}
		back := toTerm(got)
		if back == nil || term.TermKey(back) != term.TermKey(in) {
			t.Fatalf("round trip changed the term: %v -> %v", in, back)
		}
		if !fromRDFLib(back).Equal(got) {
			t.Fatalf("round trip is not stable for %v", in)
		}
		if got.TermKey() != term.TermKey(in) {
			t.Fatalf("TermKey %q, want %q", got.TermKey(), term.TermKey(in))
		}
	}
	if fromRDFLib(inner).Equal(fromRDFLib(outer)) {
		t.Fatal("different triple terms compare equal")
	}
}

func TestNodeKindTripleTerm(t *testing.T) {
	tt := fromRDFLib(term.NewTripleTerm(term.NewURIRefUnsafe("http://e/s"), term.NewURIRefUnsafe("http://e/p"), term.NewLiteral("o")))
	for _, nk := range []string{"IRI", "BlankNode", "Literal", "BlankNodeOrIRI", "BlankNodeOrLiteral", "IRIOrLiteral"} {
		if matchesNodeKind(tt, SH+nk) {
			t.Errorf("a triple term matches sh:%s", nk)
		}
	}
	if !matchesNodeKind(tt, SH+"TripleTerm") {
		t.Error("a triple term does not match sh:TripleTerm")
	}
	if matchesNodeKind(IRI("http://e/x"), SH+"TripleTerm") {
		t.Error("an IRI matches sh:TripleTerm")
	}
}

func TestPathSPARQLSyntax(t *testing.T) {
	g, err := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://e/> .
ex:s1 sh:path ( ex:a ex:b ) .
ex:s2 sh:path [ sh:inversePath ex:a ] .
ex:s3 sh:path [ sh:alternativePath ( ex:a ( ex:b ex:c ) ) ] .
ex:s4 sh:path [ sh:zeroOrMorePath [ sh:inversePath ex:a ] ] .
ex:s5 sh:path [ sh:oneOrMorePath ex:a ] .
ex:s6 sh:path [ sh:zeroOrOnePath ( ex:a ex:b ) ] .
ex:s7 sh:path ex:a .
`, "http://e/")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"s1": "<http://e/a>/<http://e/b>",
		"s2": "^<http://e/a>",
		"s3": "<http://e/a>|(<http://e/b>/<http://e/c>)",
		"s4": "(^<http://e/a>)*",
		"s5": "<http://e/a>+",
		"s6": "(<http://e/a>/<http://e/b>)?",
		"s7": "<http://e/a>",
	}
	for name, w := range want {
		node := IRI("http://e/" + name)
		paths := g.Objects(node, IRI(SH+"path"))
		if len(paths) != 1 {
			t.Fatalf("%s: %d paths", name, len(paths))
		}
		if got := parsePath(g, paths[0]).sparqlSyntax(); got != w {
			t.Errorf("%s: sparqlSyntax = %q, want %q", name, got, w)
		}
	}
	if (*PropertyPath)(nil).sparqlSyntax() != "" {
		t.Error("a nil path has syntax")
	}
}

func validateTurtle(t *testing.T, ttl string, opts ...Option) ValidationReport {
	t.Helper()
	g, err := LoadTurtleString(`@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://e/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`+ttl, "http://e/")
	if err != nil {
		t.Fatal(err)
	}
	return Validate(g, g, opts...)
}

// "4"^^xsd:integer and "04"^^xsd:byte are different values for uniqueness.
func TestUniqueValuesForIsExact(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetClass ex:C ; sh:uniqueValuesFor ex:id .
ex:a a ex:C ; ex:id "4"^^xsd:integer .
ex:b a ex:C ; ex:id "04"^^xsd:byte .
ex:c a ex:C ; ex:id "5"^^xsd:integer .
ex:d a ex:C .
ex:e a ex:C .
`)
	if !r.Conforms {
		t.Fatalf("distinct literals and nodes without a value are reported: %+v", r.Results)
	}
}

func TestUniqueValuesForCompositeAndValueSets(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetClass ex:C ; sh:uniqueValuesFor ( ex:k ex:l ) .
ex:a a ex:C ; ex:k 1 ; ex:l 1 .
ex:b a ex:C ; ex:k 1 ; ex:l 2 .
ex:c a ex:C ; ex:k 1, 2 ; ex:l 1 .
ex:d a ex:C ; ex:k 1, 2 ; ex:l 1 .
`)
	if len(r.Results) != 2 {
		t.Fatalf("want c and d reported, got %+v", r.Results)
	}
	for _, res := range r.Results {
		if f := res.FocusNode.Value(); f != "http://e/c" && f != "http://e/d" {
			t.Errorf("unexpected focus node %v", res.FocusNode)
		}
		if !res.Value.IsNone() {
			t.Errorf("sh:uniqueValuesFor result carries sh:value %v", res.Value)
		}
	}
}

// sh:rootClass: the class itself, a subclass, or a list member's subclass; a
// literal or a non-class IRI is a violation.
func TestRootClass(t *testing.T) {
	r := validateTurtle(t, `
ex:A a sh:NodeShape ; sh:targetNode ex:Dog, ex:Car, "Dog", ex:Animal ; sh:rootClass ex:Animal .
ex:Mammal <http://www.w3.org/2000/01/rdf-schema#subClassOf> ex:Animal .
ex:Dog <http://www.w3.org/2000/01/rdf-schema#subClassOf> ex:Mammal .
`)
	got := map[string]bool{}
	for _, res := range r.Results {
		got[res.Value.String()] = true
	}
	if len(r.Results) != 2 || !got["<http://e/Car>"] || !got[`"Dog"`] {
		t.Fatalf("want ex:Car and \"Dog\", got %+v", r.Results)
	}
}

// sh:closed applies to the value nodes of a property shape (SHACL 1.2 Core).
func TestClosedAtPropertyShape(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:x ;
  sh:property [ sh:path ex:addr ; sh:closed true ; sh:ignoredProperties ( ex:city ) ] .
ex:x ex:addr ex:ad .
ex:ad ex:city "Paris" ; ex:zip "75001" .
`)
	if len(r.Results) != 1 {
		t.Fatalf("want one violation, got %+v", r.Results)
	}
	res := r.Results[0]
	if res.FocusNode.Value() != "http://e/x" || res.ResultPath.Value() != "http://e/zip" || res.Value.Value() != "75001" {
		t.Fatalf("focus %v path %v value %v", res.FocusNode, res.ResultPath, res.Value)
	}
}

// Value nodes of a property shape are the path's nodes plus sh:values, and
// sh:defaultValue only when that set is empty.
func TestValuesAndDefaultValue(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a, ex:b ; sh:property ex:P , ex:Q .
ex:P sh:path ex:age ; sh:defaultValue 0 ; sh:minInclusive 18 .
ex:Q sh:path ex:tag ; sh:values "extra" ; sh:maxCount 1 .
ex:a ex:age 20 ; ex:tag "x" .
ex:b ex:tag "y" .
`)
	var gotMin, gotMax int
	for _, res := range r.Results {
		switch res.SourceConstraintComponent.Value() {
		case SH + "MinInclusiveConstraintComponent":
			gotMin++
			if res.FocusNode.Value() != "http://e/b" || res.Value.Value() != "0" {
				t.Errorf("default value result: %+v", res)
			}
		case SH + "MaxCountConstraintComponent":
			gotMax++
		}
	}
	if gotMin != 1 || gotMax != 2 {
		t.Fatalf("min=%d max=%d, results %+v", gotMin, gotMax, r.Results)
	}
}

// sh:deactivated may be a node expression, evaluated for each focus node.
func TestDeactivatedByExpression(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a, ex:b ; sh:datatype xsd:integer ;
  sh:deactivated [ sh:sparqlExpr "$this = <http://e/a>" ] .
`)
	if len(r.Results) != 1 || r.Results[0].FocusNode.Value() != "http://e/b" {
		t.Fatalf("want only ex:b validated, got %+v", r.Results)
	}
}

// sh:severity on a sh:sparql constraint overrides the shape's.
func TestSPARQLConstraintSeverity(t *testing.T) {
	r := validateTurtle(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:sparql ex:C .
ex:C sh:severity sh:Warning ; sh:select "SELECT $this WHERE { }" .
`)
	if len(r.Results) != 1 || r.Results[0].ResultSeverity.Value() != SH+"Warning" {
		t.Fatalf("got %+v", r.Results)
	}
}

// Without sh:prefixes the declarations of the ontology and shapes-graph nodes
// are used, and sh:prefixes reaches a graph through owl:versionIRI/owl:imports.
func TestSPARQLPrefixDeclarations(t *testing.T) {
	var errs []error
	r := validateTurtle(t, `
@prefix owl: <http://www.w3.org/2002/07/owl#> .
<http://e/base> owl:versionIRI <http://e/base/v1> ;
  sh:declare [ sh:prefix "e1" ; sh:namespace "http://e/ns1#"^^xsd:anyURI ] .
<http://e/shapes> owl:imports <http://e/base/v1> ; a sh:ShapesGraph ;
  sh:declare [ sh:prefix "e2" ; sh:namespace "http://e/ns2#" ] .
ex:Elsewhere sh:declare [ sh:prefix "e3" ; sh:namespace "http://e/ns3#" ] .

ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:sparql ex:Q1, ex:Q2, ex:Q3 .
ex:Q1 sh:prefixes <http://e/shapes> ;
  sh:select "SELECT $this WHERE { FILTER (<http://e/ns1#x> = e1:x && <http://e/ns2#x> = e2:x) }" .
ex:Q2 sh:select "SELECT $this WHERE { FILTER (<http://e/ns2#x> = e2:x) }" .
ex:Q3 sh:select "SELECT $this WHERE { FILTER (<http://e/ns3#x> = e3:x) }" .
`, WithErrorHandler(func(err error) { errs = append(errs, err) }))
	// Q1 and Q2 parse and match; Q3 uses a prefix declared on a node that is
	// not an ontology/shapes graph, so it is malformed and fails closed.
	if len(r.Results) != 3 {
		t.Fatalf("want 3 results, got %+v", r.Results)
	}
	if len(errs) != 1 || !errors.Is(errs[0], ErrMalformedConstraint) {
		t.Fatalf("want one ErrMalformedConstraint (Q3), got %v", errs)
	}
}
