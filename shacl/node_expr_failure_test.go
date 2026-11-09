package shacl

import (
	"context"
	"errors"
	"testing"
)

const nePrefixes = `
@prefix ex: <http://example.com/ns#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix shnex: <http://www.w3.org/ns/shacl-node-expr#> .
@prefix sparql: <http://www.w3.org/ns/sparql#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

// A sh:expression whose evaluation fails makes the validation fail: the
// constraint must not look satisfied (Node Expressions, sh:expression).
func TestExpressionFailureFailsValidation(t *testing.T) {
	g := loadTurtle(t, nePrefixes+`
ex:S a sh:NodeShape ; sh:targetNode ex:n ;
  sh:expression [ shnex:exists [ shnex:instancesOf 42 ] ] .
`)
	_, err := ValidateContext(context.Background(), g, g)
	if !errors.Is(err, ErrNodeExprFailure) {
		t.Fatalf("want ErrNodeExprFailure, got %v", err)
	}
}

// sh:expression is true only for exactly ( true ).
func TestExpressionNeedsExactlyTrue(t *testing.T) {
	g := loadTurtle(t, nePrefixes+`
ex:S a sh:NodeShape ; sh:targetNode ex:n ; sh:expression ( true true ) .
`)
	if Validate(g, g).Conforms {
		t.Fatal("( true true ) must not satisfy sh:expression")
	}
}

func evalOn(t *testing.T, ttl, node string) ([]Term, error) {
	t.Helper()
	g := loadTurtle(t, nePrefixes+ttl)
	n := g.Objects(IRI("http://example.com/ns#root"), IRI("http://example.com/ns#expr"))
	if len(n) == 0 {
		t.Fatal("no ex:expr")
	}
	return evalNodeExpr(&nodeExprContext{dataGraph: g, shapesMap: parseShapes(g)}, parseNodeExpr(g, n[0]))
}

// A function that calls itself fails instead of overflowing the stack.
func TestCustomFunctionRecursionFails(t *testing.T) {
	_, err := evalOn(t, `
ex:loop a sh:ListParameterExpressionFunction ; sh:bodyExpression [ ex:loop 1 ] .
ex:root ex:expr [ ex:loop 1 ] .`, "")
	if !errors.Is(err, ErrNodeExprFailure) {
		t.Fatalf("want ErrNodeExprFailure, got %v", err)
	}
}

// A described blank node no function claims is a failure, not an empty list.
func TestUnknownFunctionIsIllFormed(t *testing.T) {
	_, err := evalOn(t, `ex:root ex:expr [ ex:nope 1 ] .`, "")
	if !errors.Is(err, ErrNodeExprFailure) {
		t.Fatalf("want ErrNodeExprFailure, got %v", err)
	}
}

func TestCallCustomFunction(t *testing.T) {
	g := loadTurtle(t, nePrefixes+`
ex:twice a sh:ListParameterExpressionFunction ;
  sh:parameter [ sh:path shnex:arg0 ] ;
  sh:parameter [ sh:path shnex:arg1 ; sh:optional true ] ;
  sh:bodyExpression [ sparql:multiply ( [ shnex:arg 0 ] 2 ) ] .
`)
	ctx := &evalContext{dataGraph: g, shapesGraph: g, shapesMap: map[string]*Shape{}}
	two := Literal("21", XSD+"integer", "")

	out, err := callCustomFunction(ctx, "http://example.com/ns#twice", []Term{two})
	if err != nil || len(out) != 1 || out[0].Value() != "42" {
		t.Fatalf("got %v, %v", out, err)
	}
	// optional second parameter left unbound
	if _, err := callCustomFunction(ctx, "http://example.com/ns#twice", []Term{two, {}}); err != nil {
		t.Fatalf("optional unbound: %v", err)
	}
	// required first parameter unbound
	if _, err := callCustomFunction(ctx, "http://example.com/ns#twice", []Term{{}}); !errors.Is(err, ErrNodeExprFailure) {
		t.Fatalf("unbound required: %v", err)
	}
	// blank node argument
	if _, err := callCustomFunction(ctx, "http://example.com/ns#twice", []Term{BlankNode("b")}); !errors.Is(err, ErrNodeExprFailure) {
		t.Fatalf("blank argument: %v", err)
	}
	if _, err := callCustomFunction(ctx, "http://example.com/ns#other", nil); !errors.Is(err, ErrNotNodeExprFunction) {
		t.Fatalf("unknown function: %v", err)
	}
}

// Triple terms survive the conversion to and from rdflib terms.
func TestTripleTermRoundTrip(t *testing.T) {
	g := loadTurtle(t, nePrefixes+`ex:s ex:p <<( ex:a ex:b 1 )>> .`)
	objs := g.Objects(IRI("http://example.com/ns#s"), IRI("http://example.com/ns#p"))
	if len(objs) != 1 || !objs[0].IsTriple() {
		t.Fatalf("got %v", objs)
	}
	if back := fromRDFLib(toTerm(objs[0])); back != objs[0] {
		t.Fatalf("round trip changed %v into %v", objs[0], back)
	}
}
