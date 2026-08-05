package shacl

import (
	"errors"
	"strings"
	"testing"
)

// Tests for SHACL-AF node expressions (§6), functions (§5) and custom targets
// (§4). Expressions are exercised through a sh:TripleRule, which is the
// shortest way to observe what an expression evaluates to: the objects of the
// inferred triples are exactly the expression's result set.

// inferObjects applies a rule whose object is the given expression and returns
// what it produced, as strings, in the order the rule emitted them.
func inferObjects(t *testing.T, fixture, objectExpr string) []string {
	t.Helper()
	g := afGraph(t, fixture+`
ex:ExprShape a sh:NodeShape ;
    sh:targetNode ex:focus ;
    sh:rule [
        a sh:TripleRule ;
        sh:subject sh:this ;
        sh:predicate ex:out ;
        sh:object `+objectExpr+` ;
    ] .
`)
	if _, err := ApplyRules(g, g); err != nil {
		t.Fatalf("ApplyRules: %v", err)
	}
	subj, pred := exIRI("focus"), exIRI("out")
	var out []string
	for _, tr := range g.All(&subj, &pred, nil) {
		out = append(out, tr.Object.String())
	}
	return out
}

func containsAll(got []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, g := range got {
			if strings.Contains(g, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestAFExpr_ThisAndConstants(t *testing.T) {
	cases := map[string]struct {
		expr string
		want string
	}{
		"sh:this":      {"sh:this", "focus"},
		"constant IRI": {"ex:someIRI", "someIRI"},
		"literal":      {`"hello"`, "hello"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := inferObjects(t, `ex:focus a ex:Thing .`, tc.expr)
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Errorf("got %v, want one value containing %q", got, tc.want)
			}
		})
	}
}

func TestAFExpr_Path(t *testing.T) {
	fixture := `
ex:focus ex:p ex:a, ex:b ; ex:q ex:c .
ex:a ex:r ex:deep .
ex:back ex:p ex:focus .
`
	cases := map[string]struct {
		expr string
		want []string
	}{
		"predicate":   {`[ sh:path ex:p ]`, []string{"a", "b"}},
		"sequence":    {`[ sh:path ( ex:p ex:r ) ]`, []string{"deep"}},
		"inverse":     {`[ sh:path [ sh:inversePath ex:p ] ]`, []string{"back"}},
		"alternative": {`[ sh:path [ sh:alternativePath ( ex:p ex:q ) ] ]`, []string{"a", "b", "c"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := inferObjects(t, fixture, tc.expr)
			if !containsAll(got, tc.want...) {
				t.Errorf("got %v, want values for %v", got, tc.want)
			}
		})
	}
}

// sh:nodes redirects a path expression away from the focus node. pySHACL does
// not implement this, but SHACL-AF §6.4 defines it and without it a path can
// only ever start where the rule started.
func TestAFExpr_PathWithNodes(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:friend ex:bob .
ex:bob ex:givenName "Bob" .
`, `[ sh:path ex:givenName ; sh:nodes [ sh:path ex:friend ] ]`)

	if len(got) != 1 || !strings.Contains(got[0], "Bob") {
		t.Errorf("got %v, want the friend's name — sh:nodes did not redirect the path", got)
	}
}

func TestAFExpr_UnionAndIntersection(t *testing.T) {
	fixture := `
ex:focus ex:p ex:a, ex:shared ; ex:q ex:b, ex:shared .
`
	t.Run("union merges and de-duplicates", func(t *testing.T) {
		got := inferObjects(t, fixture, `[ sh:union ( [ sh:path ex:p ] [ sh:path ex:q ] ) ]`)
		if len(got) != 3 || !containsAll(got, "a", "b", "shared") {
			t.Errorf("got %v, want exactly a, b and one shared", got)
		}
	})

	t.Run("intersection keeps only common nodes", func(t *testing.T) {
		got := inferObjects(t, fixture, `[ sh:intersection ( [ sh:path ex:p ] [ sh:path ex:q ] ) ]`)
		if len(got) != 1 || !strings.Contains(got[0], "shared") {
			t.Errorf("got %v, want only ex:shared", got)
		}
	})

	t.Run("intersection with a disjoint part is empty", func(t *testing.T) {
		got := inferObjects(t, fixture, `[ sh:intersection ( [ sh:path ex:p ] [ sh:path ex:missing ] ) ]`)
		if len(got) != 0 {
			t.Errorf("got %v, want nothing", got)
		}
	})
}

func TestAFExpr_FilterShape(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:member ex:adult, ex:child .
ex:adult ex:age 30 .
ex:child ex:age 8 .

ex:AdultShape a sh:NodeShape ;
    sh:property [ sh:path ex:age ; sh:minInclusive 18 ] .
`, `[ sh:filterShape ex:AdultShape ; sh:nodes [ sh:path ex:member ] ]`)

	if len(got) != 1 || !strings.Contains(got[0], "adult") {
		t.Errorf("got %v, want only ex:adult", got)
	}
}

// A filter shape may be written inline rather than named.
func TestAFExpr_FilterShapeAnonymous(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:member ex:adult, ex:child .
ex:adult ex:age 30 .
ex:child ex:age 8 .
`, `[ sh:filterShape [ sh:property [ sh:path ex:age ; sh:minInclusive 18 ] ] ;
       sh:nodes [ sh:path ex:member ] ]`)

	if len(got) != 1 || !strings.Contains(got[0], "adult") {
		t.Errorf("got %v, want only ex:adult", got)
	}
}

func TestAFExpr_FunctionCall(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:width 6 ; ex:height 7 .

ex:multiply a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:op1 ; sh:datatype xsd:integer ] ;
    sh:parameter [ sh:path ex:op2 ; sh:datatype xsd:integer ] ;
    sh:returnType xsd:integer ;
    sh:select "SELECT ($op1 * $op2 AS ?result) WHERE { }" .
`, `[ ex:multiply ( [ sh:path ex:width ] [ sh:path ex:height ] ) ]`)

	if len(got) != 1 || !strings.Contains(got[0], "42") {
		t.Errorf("got %v, want 42", got)
	}
}

// An argument that is itself multi-valued makes the call run over every
// combination (SHACL-AF §6.3).
func TestAFExpr_FunctionCallOverCartesianProduct(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:a 2, 3 ; ex:b 10, 20 .

ex:multiply a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:op1 ] ;
    sh:parameter [ sh:path ex:op2 ] ;
    sh:select "SELECT ($op1 * $op2 AS ?result) WHERE { }" .
`, `[ ex:multiply ( [ sh:path ex:a ] [ sh:path ex:b ] ) ]`)

	if len(got) != 4 || !containsAll(got, "20", "30", "40", "60") {
		t.Errorf("got %v, want the four products 20, 30, 40 and 60", got)
	}
}

func TestAFFunction_AskReturnsBoolean(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:age 30 .

ex:isAdult a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:age ] ;
    sh:returnType xsd:boolean ;
    sh:ask "ASK { FILTER ($age >= 18) }" .
`, `[ ex:isAdult ( [ sh:path ex:age ] ) ]`)

	if len(got) != 1 || !strings.Contains(got[0], "true") {
		t.Errorf("got %v, want true", got)
	}
}

// With sh:order on every parameter the arguments are positional in that order;
// without it they follow the local names of the parameter paths.
func TestAFFunction_ParameterOrder(t *testing.T) {
	t.Run("sh:order wins over name", func(t *testing.T) {
		got := inferObjects(t, `
ex:focus ex:x "L" ; ex:y "R" .

ex:concat a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:zzz ; sh:order 1 ] ;
    sh:parameter [ sh:path ex:aaa ; sh:order 2 ] ;
    sh:select "SELECT (CONCAT($zzz, $aaa) AS ?result) WHERE { }" .
`, `[ ex:concat ( [ sh:path ex:x ] [ sh:path ex:y ] ) ]`)
		if len(got) != 1 || !strings.Contains(got[0], "LR") {
			t.Errorf("got %v, want LR — the first argument binds the sh:order 1 parameter", got)
		}
	})

	t.Run("no sh:order falls back to local name", func(t *testing.T) {
		got := inferObjects(t, `
ex:focus ex:x "L" ; ex:y "R" .

ex:concat a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:bbb ] ;
    sh:parameter [ sh:path ex:aaa ] ;
    sh:select "SELECT (CONCAT($aaa, $bbb) AS ?result) WHERE { }" .
`, `[ ex:concat ( [ sh:path ex:x ] [ sh:path ex:y ] ) ]`)
		if len(got) != 1 || !strings.Contains(got[0], "LR") {
			t.Errorf("got %v, want LR — arguments bind aaa then bbb", got)
		}
	})
}

// A required parameter with no value yields no result; an optional one lets the
// call proceed with the argument unbound.
func TestAFFunction_OptionalParameter(t *testing.T) {
	t.Run("required and missing produces nothing", func(t *testing.T) {
		got := inferObjects(t, `
ex:focus ex:present "here" .

ex:combine a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:aaa ] ;
    sh:parameter [ sh:path ex:bbb ] ;
    sh:select "SELECT (CONCAT($aaa, COALESCE($bbb, \"-\")) AS ?result) WHERE { }" .
`, `[ ex:combine ( [ sh:path ex:present ] [ sh:path ex:absent ] ) ]`)
		if len(got) != 0 {
			t.Errorf("got %v, want nothing for a missing required argument", got)
		}
	})

	t.Run("optional and missing still runs", func(t *testing.T) {
		got := inferObjects(t, `
ex:focus ex:present "here" .

ex:combine a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:aaa ] ;
    sh:parameter [ sh:path ex:bbb ; sh:optional true ] ;
    sh:select "SELECT (CONCAT($aaa, COALESCE($bbb, \"-\")) AS ?result) WHERE { }" .
`, `[ ex:combine ( [ sh:path ex:present ] [ sh:path ex:absent ] ) ]`)
		if len(got) != 1 || !strings.Contains(got[0], "here-") {
			t.Errorf("got %v, want \"here-\"", got)
		}
	})
}

// A function is callable from inside an ordinary SPARQL constraint, not just
// from a node expression. This is what query-scoped binding buys.
func TestAFFunction_CallableFromSPARQLConstraint(t *testing.T) {
	g := afGraph(t, `
ex:good a ex:Rect ; ex:width 3 ; ex:height 4 ; ex:area 12 .
ex:bad  a ex:Rect ; ex:width 3 ; ex:height 4 ; ex:area 99 .

ex:multiply a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:op1 ] ;
    sh:parameter [ sh:path ex:op2 ] ;
    sh:select "SELECT ($op1 * $op2 AS ?result) WHERE { }" .

ex:AreaShape a sh:NodeShape ;
    sh:targetClass ex:Rect ;
    sh:sparql [
        sh:message "width * height must equal area" ;
        sh:select """
            SELECT $this ?value
            WHERE {
                $this ex:width ?w .
                $this ex:height ?h .
                $this ex:area ?value .
                FILTER (ex:multiply(?w, ?h) != ?value)
            }
        """ ;
    ] .
`)
	report := Validate(g, g, WithAdvancedFeatures())
	if report.Conforms {
		t.Fatal("ex:bad has the wrong area and should not conform")
	}
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want exactly 1 (only ex:bad)", len(report.Results))
	}
	if !strings.Contains(report.Results[0].FocusNode.String(), "bad") {
		t.Errorf("the violation is on %s, want ex:bad", report.Results[0].FocusNode)
	}

	// Without the option the function is not declared to the query, so the
	// constraint cannot be evaluated as intended.
	if plain := Validate(g, g); plain.Conforms {
		t.Error("without advanced features the function is unavailable; the constraint should not silently pass")
	}
}

func TestAFFunction_Malformed(t *testing.T) {
	cases := map[string]struct{ ttl, says string }{
		"neither select nor ask": {
			ttl:  `ex:fn a sh:SPARQLFunction ; sh:parameter [ sh:path ex:a ] .`,
			says: "neither sh:select nor sh:ask",
		},
		"both select and ask": {
			ttl: `ex:fn a sh:SPARQLFunction ;
                    sh:select "SELECT 1 WHERE {}" ; sh:ask "ASK {}" .`,
			says: "both",
		},
		"parameter without path": {
			ttl: `ex:fn a sh:SPARQLFunction ; sh:parameter [ sh:name "x" ] ;
                    sh:select "SELECT 1 WHERE {}" .`,
			says: "no sh:path",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := afGraph(t, tc.ttl+`
ex:S a sh:NodeShape ; sh:targetNode ex:focus ;
    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:p ; sh:object ex:o ] .
`)
			_, err := ApplyRules(g, g)
			if !errors.Is(err, ErrMalformedFunction) {
				t.Fatalf("err = %v, want ErrMalformedFunction", err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q should mention %q", err, tc.says)
			}
		})
	}
}

// A function whose body calls itself must be stopped by the engine rather than
// by the stack.
func TestAFFunction_RecursionIsBounded(t *testing.T) {
	g := afGraph(t, `
ex:focus ex:n 1 .

ex:loop a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:arg ] ;
    sh:select "SELECT (ex:loop($arg) AS ?result) WHERE { }" .

ex:S a sh:NodeShape ; sh:targetNode ex:focus ;
    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:out ;
        sh:object [ ex:loop ( [ sh:path ex:n ] ) ] ] .
`)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ApplyRules(g, g)
	}()
	<-done // The bound is what keeps this from being an infinite recursion.
}

func TestAFExpr_MalformedExpressions(t *testing.T) {
	cases := map[string]struct{ expr, says string }{
		"filterShape without nodes": {
			expr: `[ sh:filterShape ex:SomeShape ]`,
			says: "no sh:nodes",
		},
		"blank node that is not an expression": {
			expr: `[ ex:notAList ex:alsoNotAList ]`,
			says: "not a node expression",
		},
		"call with the wrong number of arguments": {
			expr: `[ ex:oneArg ( sh:this sh:this ) ]`,
			says: "takes 1 argument",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := afGraph(t, `
ex:focus a ex:Thing .
ex:oneArg a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:a ] ;
    sh:select "SELECT $a AS ?result WHERE { }" .

ex:S a sh:NodeShape ; sh:targetNode ex:focus ;
    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:p ;
        sh:object `+tc.expr+` ] .
`)
			_, err := ApplyRules(g, g)
			if !errors.Is(err, ErrMalformedExpression) {
				t.Fatalf("err = %v, want ErrMalformedExpression", err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q should mention %q", err, tc.says)
			}
		})
	}
}

// --- Custom targets (§4) ---

func TestAFTarget_SPARQLTarget(t *testing.T) {
	g := afGraph(t, `
ex:alice ex:role ex:admin .
ex:bob   ex:role ex:guest .

ex:AdminShape a sh:NodeShape ;
    sh:target [
        a sh:SPARQLTarget ;
        sh:select "SELECT ?this WHERE { ?this <http://example.org/role> <http://example.org/admin> }" ;
    ] ;
    sh:property [ sh:path ex:clearance ; sh:minCount 1 ] .
`)
	report := Validate(g, g, WithAdvancedFeatures())
	if report.Conforms {
		t.Fatal("ex:alice is targeted and has no ex:clearance, so validation should fail")
	}
	if len(report.Results) != 1 || !strings.Contains(report.Results[0].FocusNode.String(), "alice") {
		t.Errorf("results = %v, want exactly one on ex:alice", report.Results)
	}

	if plain := Validate(g, g); !plain.Conforms {
		t.Error("without advanced features sh:target is not a target, so nothing is validated")
	}
}

func TestAFTarget_SPARQLTargetType(t *testing.T) {
	// Data and shapes are kept apart here: the sh:target instance carries an
	// ex:country of its own, and validating the shapes graph as data would let
	// the target query select that blank node too.
	data := afGraph(t, `
ex:alice ex:country ex:US .
ex:bob   ex:country ex:DE .
`)
	shapes := afGraph(t, `
ex:ByCountry a sh:SPARQLTargetType ;
    sh:parameter [ sh:path ex:country ] ;
    sh:select "SELECT ?this WHERE { ?this <http://example.org/country> $country }" .

ex:USShape a sh:NodeShape ;
    sh:target [ a ex:ByCountry ; ex:country ex:US ] ;
    sh:property [ sh:path ex:clearance ; sh:minCount 1 ] .
`)
	report := Validate(data, shapes, WithAdvancedFeatures())
	if len(report.Results) != 1 || !strings.Contains(report.Results[0].FocusNode.String(), "alice") {
		t.Fatalf("results = %v, want exactly one on ex:alice — the parameter should scope the target to ex:US",
			report.Results)
	}
}

func TestAFTarget_Malformed(t *testing.T) {
	cases := map[string]struct{ ttl, says string }{
		"SPARQLTarget without select": {
			ttl:  `ex:S a sh:NodeShape ; sh:target [ a sh:SPARQLTarget ] .`,
			says: "no sh:select",
		},
		"target of no known type": {
			ttl:  `ex:S a sh:NodeShape ; sh:target [ ex:something "x" ] .`,
			says: "neither a sh:SPARQLTarget nor",
		},
		"missing required parameter": {
			ttl: `ex:T a sh:SPARQLTargetType ;
                    sh:parameter [ sh:path ex:country ] ;
                    sh:select "SELECT ?this WHERE { ?this ex:country $country }" .
                  ex:S a sh:NodeShape ; sh:target [ a ex:T ] .`,
			says: "no value for the required parameter",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := afGraph(t, tc.ttl)
			var errs []error
			Validate(g, g, WithAdvancedFeatures(), WithErrorHandler(func(err error) { errs = append(errs, err) }))
			if len(errs) == 0 {
				t.Fatal("a malformed sh:target should be reported to the error handler")
			}
			if !strings.Contains(errs[0].Error(), tc.says) {
				t.Errorf("error %q should mention %q", errs[0], tc.says)
			}
		})
	}
}

// --- Coexistence with SHACL 1.2 ---

// Enabling SHACL-AF must not change how a SHACL 1.2 shnex: expression behaves.
func TestAF_DoesNotDisturbSHACL12Expressions(t *testing.T) {
	g := afGraph(t, `
@prefix shnex: <http://www.w3.org/ns/shacl-node-expr#> .

ex:focus ex:value 5 .

ex:S a sh:NodeShape ;
    sh:targetNode ex:focus ;
    sh:expression [ shnex:pathValues ex:value ] .
`)
	plain := Validate(g, g)
	advanced := Validate(g, g, WithAdvancedFeatures())
	if plain.Conforms != advanced.Conforms {
		t.Errorf("conforms differs with advanced features on: %v vs %v", plain.Conforms, advanced.Conforms)
	}
	if len(plain.Results) != len(advanced.Results) {
		t.Errorf("result count differs: %d vs %d", len(plain.Results), len(advanced.Results))
	}
}

// The AF expression constraint reads sh:expression with AF semantics: sh:this
// is the focus node, so a focus node of false does not conform.
func TestAF_ExpressionConstraint(t *testing.T) {
	g := afGraph(t, `
ex:S a sh:NodeShape ;
    sh:expression sh:this ;
    sh:targetNode true ;
    sh:targetNode false .
`)
	report := Validate(g, g, WithAdvancedFeatures())
	if report.Conforms {
		t.Fatal("the focus node false is not truthy, so validation should fail")
	}
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1 — only the false focus node violates", len(report.Results))
	}
	if got := report.Results[0].Value.Value(); got != "false" {
		t.Errorf("sh:value = %q, want false", got)
	}
}
