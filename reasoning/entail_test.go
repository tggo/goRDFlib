package reasoning

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

const entPrefixes = `@prefix : <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

func ttl(t *testing.T, body string) *graph.Graph {
	t.Helper()
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader(entPrefixes+body)); err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	return g
}

var xsdInt = []string{nsXSD + "integer"}

// TestEntailsTable pins each rule and each datatype behaviour of the checker.
// Every row fails if the rule it names is removed (checked by mutation).
func TestEntailsTable(t *testing.T) {
	tests := []struct {
		name       string
		sem        Semantics
		rec        []string
		premise    string
		conclusion string
		want       bool
	}{
		// simple entailment
		{"simple: identity", SimpleEntailment, nil, ":a :p :b .", ":a :p :b .", true},
		{"simple: blank node maps to a term", SimpleEntailment, nil, ":a :p :b .", ":a :p _:x .", true},
		{"simple: one blank node, two uses", SimpleEntailment, nil, ":a :p :b . :c :p :d .", ":a :p _:x . :c :p _:x .", false},
		{"simple: premise blank nodes are constants", SimpleEntailment, nil, "_:b :p :c .", ":a :p :c .", false},
		{"simple: rdf1 is not simple entailment", SimpleEntailment, nil, ":a :p :b .", ":p rdf:type rdf:Property .", false},
		{"simple: a blank node may denote a literal", SimpleEntailment, nil, ":a :p 10 . :c :q 10 .", ":a :p _:x . :c :q _:x .", true},

		// RDF
		{"RDF: rdf1", RDFEntailment, nil, ":a :p :b .", ":p rdf:type rdf:Property .", true},
		{"RDF: axiom rdf:type", RDFEntailment, nil, "", "rdf:type rdf:type rdf:Property .", true},
		{"RDF: no rdfs4", RDFEntailment, nil, ":a :p :b .", ":a rdf:type rdfs:Resource .", false},

		// RDFS
		{"RDFS: rdfs4a", RDFSEntailment, nil, ":a :p :b .", ":a rdf:type rdfs:Resource .", true},
		{"RDFS: rdfs4b", RDFSEntailment, nil, ":a :p :b .", ":b rdf:type rdfs:Resource .", true},
		{"RDFS: rdfs2", RDFSEntailment, nil, ":p rdfs:domain :C . :a :p :b .", ":a rdf:type :C .", true},
		{"RDFS: rdfs3", RDFSEntailment, nil, ":p rdfs:range :C . :a :p :b .", ":b rdf:type :C .", true},
		{"RDFS: domain is not range", RDFSEntailment, nil, ":p rdfs:domain :C . :a :p :b .", ":b rdf:type :C .", false},
		{"RDFS: rdfs9", RDFSEntailment, nil, ":C rdfs:subClassOf :D . :a a :C .", ":a a :D .", true},
		{"RDFS: rdfs11", RDFSEntailment, nil, ":C rdfs:subClassOf :D . :D rdfs:subClassOf :E .", ":C rdfs:subClassOf :E .", true},
		{"RDFS: rdfs5+7", RDFSEntailment, nil, ":p rdfs:subPropertyOf :q . :q rdfs:subPropertyOf :r . :a :p :b .", ":a :r :b .", true},
		{"RDFS: rdfs12", RDFSEntailment, nil, "rdf:_3 a rdfs:ContainerMembershipProperty .", "rdf:_3 rdfs:subPropertyOf rdfs:member .", true},
		{"RDFS: rdf:_n axiom without mention in the premise", RDFSEntailment, nil, "", "rdf:_7 a rdfs:ContainerMembershipProperty .", true},
		{"RDFS: untyped IRI is not a class", RDFSEntailment, nil, ":C rdfs:subClassOf :D .", ":E rdfs:subClassOf :D .", false},

		// datatypes
		{"D: integers by value", SimpleEntailment, xsdInt, `:a :p "010"^^xsd:integer .`, `:a :p "10"^^xsd:integer .`, true},
		{"D: without recognition they differ", SimpleEntailment, nil, `:a :p "010"^^xsd:integer .`, `:a :p "10"^^xsd:integer .`, false},
		{"D: integer and decimal share values", SimpleEntailment, []string{nsXSD + "integer", nsXSD + "decimal"}, `:a :p "10"^^xsd:integer .`, `:a :p "10.0"^^xsd:decimal .`, true},
		{"D: decimal alone does not equate an unrecognized integer", SimpleEntailment, []string{nsXSD + "decimal"}, `:a :p "10"^^xsd:integer .`, `:a :p "10.0"^^xsd:decimal .`, false},
		{"D: float is not decimal", SimpleEntailment, []string{nsXSD + "float", nsXSD + "decimal"}, `:a :p "1.0"^^xsd:float .`, `:a :p "1.0"^^xsd:decimal .`, false},
		{"D: float rounds", SimpleEntailment, []string{nsXSD + "float"}, `:a :p "16777205.5"^^xsd:float .`, `:a :p "16777206.5"^^xsd:float .`, true},
		{"D: float zero has a sign", SimpleEntailment, []string{nsXSD + "float"}, `:a :p "0"^^xsd:float .`, `:a :p "-0"^^xsd:float .`, false},
		{"D: boolean forms", SimpleEntailment, []string{nsXSD + "boolean"}, `:a :p "1"^^xsd:boolean .`, `:a :p "true"^^xsd:boolean .`, true},
		{"D: language tags ignore case", SimpleEntailment, nil, `:a :p "x"@EN-us .`, `:a :p "x"@en-US .`, true},
		{"D: literal typing under RDF", RDFEntailment, xsdInt, `:a :p "5"^^xsd:integer .`, `:a :p _:x . _:x a xsd:integer .`, true},
		{"D: derived type", RDFEntailment, []string{nsXSD + "integer", nsXSD + "byte"}, `:a :p "5"^^xsd:byte .`, `:a :p _:x . _:x a xsd:integer .`, true},
		{"D: out-of-range value is not of the narrower type", RDFEntailment, []string{nsXSD + "integer", nsXSD + "byte"}, `:a :p "500"^^xsd:integer .`, `:a :p _:x . _:x a xsd:byte .`, false},
		{"D: no typing without recognition", RDFEntailment, nil, `:a :p "5"^^xsd:integer .`, `:a :p _:x . _:x a xsd:integer .`, false},
		{"D: subclass between recognized datatypes", RDFSEntailment, []string{nsXSD + "integer", nsXSD + "decimal"}, "", "xsd:integer rdfs:subClassOf xsd:decimal .", true},
		{"D: no subclass without recognition", RDFSEntailment, []string{nsXSD + "integer"}, "", "xsd:integer rdfs:subClassOf xsd:decimal .", false},
		{"D: JSON object members are unordered", SimpleEntailment, []string{nsRDF + "JSON"},
			`:a :p '{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8}'^^rdf:JSON .`,
			`:a :p '{"h":8,"g":7,"f":6,"e":5,"d":4,"c":3,"b":2,"a":1}'^^rdf:JSON .`, true},
		{"D: JSON arrays are ordered", SimpleEntailment, []string{nsRDF + "JSON"}, `:a :p "[1,2]"^^rdf:JSON .`, `:a :p "[2,1]"^^rdf:JSON .`, false},

		// inconsistency: an inconsistent premise entails everything
		{"ill-typed literal entails anything", SimpleEntailment, xsdInt, `:a :p "x"^^xsd:integer .`, ":no :such :thing .", true},
		{"ill-typed but unrecognized entails nothing new", SimpleEntailment, nil, `:a :p "x"^^xsd:integer .`, ":no :such :thing .", false},
		{"range clash entails anything", RDFSEntailment, xsdInt, `:p rdfs:range xsd:string . :a :p 5 .`, ":no :such :thing .", true},
		{"disjoint classes entails anything", RDFSEntailment, []string{nsXSD + "integer"}, `:a a xsd:integer, xsd:string .`, ":no :such :thing .", true},
		{"subclass of a disjoint datatype entails anything", RDFSEntailment, xsdInt, `xsd:integer rdfs:subClassOf xsd:string .`, ":no :such :thing .", true},
		{"a clash is only seen under RDFS", RDFEntailment, xsdInt, `:p rdfs:range xsd:string . :a :p 5 .`, ":no :such :thing .", false},

		// triple terms (RDF 1.2)
		{"triple term: blank node inside", SimpleEntailment, nil, `:s :p <<( :a :b :c )>> .`, `:s :p <<( _:x :b :c )>> .`, true},
		{"triple term: shared blank node", SimpleEntailment, nil, `:s :p <<( :a :b :c )>> .`, `:s :p <<( _:x :b _:x )>> .`, false},
		{"triple term: a blank node is not a triple term", SimpleEntailment, nil, `:s :p :o .`, `:s :p <<( _:x :b :c )>> .`, false},
		{"triple term: is a proposition under RDFS", RDFSEntailment, nil, `:s :p <<( :a :b :c )>> .`, `:s :p _:x . _:x a rdfs:Proposition .`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Entails(t.Context(), ttl(t, tc.premise), ttl(t, tc.conclusion),
				WithSemantics(tc.sem), WithRecognizedDatatypes(tc.rec...))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("Entails = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSatisfiable(t *testing.T) {
	tests := []struct {
		name string
		sem  Semantics
		rec  []string
		g    string
		want bool
	}{
		{"empty", RDFSEntailment, nil, "", true},
		{"ill-typed recognized", SimpleEntailment, xsdInt, `:a :p "1.5"^^xsd:integer .`, false},
		{"ill-typed unrecognized", RDFSEntailment, nil, `:a :p "1.5"^^xsd:integer .`, true},
		{"integer with whitespace is ill-typed", SimpleEntailment, xsdInt, `:a :p " 3 "^^xsd:integer .`, false},
		{"ill-typed XML literal", SimpleEntailment, []string{nsRDF + "XMLLiteral"}, `:a :p "<"^^rdf:XMLLiteral .`, false},
		{"well-formed XML literal", SimpleEntailment, []string{nsRDF + "XMLLiteral"}, `:a :p "<b>x</b>"^^rdf:XMLLiteral .`, true},
		{"ill-typed JSON", SimpleEntailment, []string{nsRDF + "JSON"}, `:a :p "{"^^rdf:JSON .`, false},
		{"ill-typed inside a triple term", SimpleEntailment, xsdInt, `:a :p <<( :a :b "c"^^xsd:integer )>> .`, false},
		{"byte out of range", SimpleEntailment, []string{nsXSD + "byte"}, `:a :p "128"^^xsd:byte .`, false},
		{"byte in range", SimpleEntailment, []string{nsXSD + "byte"}, `:a :p "-128"^^xsd:byte .`, true},
		{"float overflow is infinity, not an error", SimpleEntailment, []string{nsXSD + "float"}, `:a :p "1E99"^^xsd:float .`, true},
		{"integer range clash needs RDFS", RDFEntailment, xsdInt, `:p rdfs:range xsd:string . :a :p 5 .`, true},
		{"integer range clash", RDFSEntailment, xsdInt, `:p rdfs:range xsd:string . :a :p 5 .`, false},
		{"plain literal in an integer range", RDFSEntailment, xsdInt, `:p rdfs:range xsd:integer . :a :p "5" .`, false},
		{"unrecognized datatype in range is fine", RDFSEntailment, nil, `:p rdfs:range xsd:integer . :a :p 5 .`, true},
		{"overlapping datatypes", RDFSEntailment, []string{nsXSD + "byte", nsXSD + "unsignedByte"}, `:a a xsd:byte, xsd:unsignedByte .`, true},
		{"disjoint number ranges", RDFSEntailment, []string{nsXSD + "positiveInteger", nsXSD + "negativeInteger"}, `:a a xsd:positiveInteger, xsd:negativeInteger .`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Satisfiable(t.Context(), ttl(t, tc.g), WithSemantics(tc.sem), WithRecognizedDatatypes(tc.rec...))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("Satisfiable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEntailsUnsupportedDatatype(t *testing.T) {
	g := ttl(t, "")
	_, err := Satisfiable(t.Context(), g, WithRecognizedDatatypes("http://www.w3.org/2001/XMLSchema#dateTime"))
	if !errors.Is(err, ErrUnsupportedDatatype) {
		t.Fatalf("err = %v, want ErrUnsupportedDatatype", err)
	}
	if !strings.Contains(err.Error(), nsXSD+"integer") {
		t.Errorf("error must list the recognizable datatypes: %v", err)
	}
}

func TestEntailsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	g := ttl(t, ":a :p :b .")
	if _, err := Entails(ctx, g, g, WithSemantics(RDFSEntailment)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
