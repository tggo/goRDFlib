package shacl

import (
	"strings"
	"testing"
)

const fnShape = `ex:S a sh:NodeShape ; sh:targetNode ex:t ; sh:sparql [ sh:select """
PREFIX ex: <http://e/>
SELECT $this ?value WHERE { BIND (CALL AS ?value) }""" ] .
`

func fnValue(t *testing.T, call, decls string) (string, bool) {
	t.Helper()
	r := validateTurtle(t, strings.Replace(fnShape, "CALL", call, 1)+decls)
	if len(r.Results) != 1 {
		t.Fatalf("%s: %d results", call, len(r.Results))
	}
	v := r.Results[0].Value
	// An unbound ?value falls back to the focus node.
	return v.Value(), v.Value() != "http://e/t"
}

// A function whose body is SPARQL is called from SPARQL without
// WithAdvancedFeatures; zero or several output nodes, an unbound required
// argument and a blank node argument are SPARQL errors.
func TestSPARQLBodyFunctions(t *testing.T) {
	decls := `
ex:one a sh:ListParameterExpressionFunction ;
  sh:bodyExpression [ sh:select "SELECT ?r WHERE { ?r a $arg0 }" ] ;
  sh:parameter [ sh:path <http://www.w3.org/ns/shacl-node-expr#arg0> ] .
ex:first a ex:A . ex:second a ex:B . ex:third a ex:B .
ex:concat a sh:ListParameterExpressionFunction ;
  sh:bodyExpression [ sh:sparqlExpr "CONCAT(STR($arg0), COALESCE(STR($arg1), '-'))" ] ;
  sh:parameter [ sh:path <http://www.w3.org/ns/shacl-node-expr#arg0> ] ;
  sh:parameter [ sh:path <http://www.w3.org/ns/shacl-node-expr#arg1> ; sh:optional true ] .
`
	for _, c := range []struct {
		call, want string
		bound      bool
	}{
		{"ex:one(ex:A)", "http://e/first", true},
		{"ex:one(ex:B)", "", false},         // two instances
		{"ex:one(ex:C)", "", false},         // none
		{"ex:one(?unbound)", "", false},     // required argument unbound
		{"ex:one(BNODE())", "", false},      // blank node argument
		{"ex:concat('a', 'b')", "ab", true}, // sh:sparqlExpr
		{"ex:concat('a')", "a-", true},      // optional argument left unbound
	} {
		got, bound := fnValue(t, c.call, decls)
		if bound != c.bound || (bound && got != c.want) {
			t.Errorf("%s = %q (bound %v), want %q (bound %v)", c.call, got, bound, c.want, c.bound)
		}
	}
}

// A function that calls itself does not run away.
func TestSPARQLBodyFunctionRecursionIsBounded(t *testing.T) {
	_, bound := fnValue(t, "ex:loop(1)", `
ex:loop a sh:ListParameterExpressionFunction ;
  sh:bodyExpression [ sh:sparqlExpr "<http://e/loop>($arg0)" ] ;
  sh:parameter [ sh:path <http://www.w3.org/ns/shacl-node-expr#arg0> ] .
`)
	if bound {
		t.Fatal("an endless recursion produced a value")
	}
}
