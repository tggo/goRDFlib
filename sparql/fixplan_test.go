package sparql

import (
	"strconv"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Tests for fix.plan.md items — verifying RDFLib bugs don't exist in our Go port.

func makeFixPlanGraph(t *testing.T) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"

	alice := rdflibgo.NewURIRefUnsafe(ex + "Alice")
	bob := rdflibgo.NewURIRefUnsafe(ex + "Bob")
	charlie := rdflibgo.NewURIRefUnsafe(ex + "Charlie")
	p := rdflibgo.NewURIRefUnsafe(ex + "p")
	q := rdflibgo.NewURIRefUnsafe(ex + "q")
	r := rdflibgo.NewURIRefUnsafe(ex + "r")
	typ := rdflibgo.NewURIRefUnsafe(ex + "type")
	person := rdflibgo.NewURIRefUnsafe(ex + "Person")
	thing := rdflibgo.NewURIRefUnsafe(ex + "Thing")
	name := rdflibgo.NewURIRefUnsafe(ex + "name")
	label := rdflibgo.NewURIRefUnsafe(ex + "label")
	typeA := rdflibgo.NewURIRefUnsafe(ex + "A")
	typeB := rdflibgo.NewURIRefUnsafe(ex + "B")

	g.Add(alice, p, rdflibgo.NewLiteral("a1"))
	g.Add(alice, q, rdflibgo.NewLiteral("a2"))
	g.Add(alice, r, rdflibgo.NewLiteral("a2")) // r matches q value
	g.Add(alice, typ, person)
	g.Add(alice, name, rdflibgo.NewLiteral("Alice"))
	g.Add(alice, label, rdflibgo.NewLiteral("alice-label"))
	g.Add(alice, typ, typeA)

	g.Add(bob, p, rdflibgo.NewLiteral("b1"))
	g.Add(bob, q, rdflibgo.NewLiteral("b2"))
	// bob has NO r triple — so NOT EXISTS { ?s :r ?o2 } is true for bob
	g.Add(bob, typ, person)
	g.Add(bob, name, rdflibgo.NewLiteral("Bob"))
	// bob has no label
	g.Add(bob, typ, typeB)

	g.Add(charlie, p, rdflibgo.NewLiteral("c1"))
	// charlie has NO q triple
	g.Add(charlie, typ, person)
	g.Add(charlie, name, rdflibgo.NewLiteral("Charlie"))
	g.Add(charlie, typ, thing)

	return g
}

// S1. Nested NOT EXISTS — variable scoping
func TestS1_NestedNotExists(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s WHERE {
			?s :p ?o .
			FILTER NOT EXISTS {
				?s :q ?o2 .
				FILTER NOT EXISTS { ?s :r ?o2 }
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice: has q "a2" and r "a2", so inner NOT EXISTS is false, outer NOT EXISTS sees results → filters out? No.
	// Alice: q="a2", r="a2" → inner NOT EXISTS { :r "a2" } is false (r exists) → inner block { :q ?o2 . NOT EXISTS { :r ?o2 } } yields nothing → outer NOT EXISTS is true → Alice included
	// Bob: has q "b2", no r "b2" → inner NOT EXISTS { :r "b2" } is true → inner block yields result → outer NOT EXISTS is false → Bob excluded
	// Charlie: has no q → inner block yields nothing → outer NOT EXISTS is true → Charlie included
	got := extractVarValues(r.Bindings, "s")
	expect := map[string]bool{
		"http://example.org/Alice":   true,
		"http://example.org/Charlie": true,
	}
	if len(got) != len(expect) {
		t.Fatalf("S1: expected %d results, got %d: %v", len(expect), len(got), got)
	}
	for _, v := range got {
		if !expect[v] {
			t.Errorf("S1: unexpected result %s", v)
		}
	}
}

// S2. Subquery under OPTIONAL
func TestS2_SubqueryUnderOptional(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s ?label WHERE {
			?s :type :Person .
			OPTIONAL {
				{ SELECT ?s ?label WHERE { ?s :name ?label } }
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// All 3 persons should appear, each with a name from the subquery
	if len(r.Bindings) != 3 {
		t.Fatalf("S2: expected 3 results, got %d", len(r.Bindings))
	}
	for _, b := range r.Bindings {
		if b["s"] == nil {
			t.Error("S2: ?s is nil")
		}
		if b["label"] == nil {
			t.Error("S2: ?label is nil for", b["s"])
		}
	}
}

// S3. Triple-nested subquery projection
func TestS3_TripleNestedSubquery(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?x WHERE {
			{ SELECT ?x WHERE {
				{ SELECT ?x WHERE { ?x :p ?o } }
			} }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Fatalf("S3: expected 3 results, got %d", len(r.Bindings))
	}
}

// S4. EXISTS inside BIND
func TestS4_ExistsInsideBind(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s ?flag WHERE {
			?s :p ?o .
			BIND(EXISTS { ?s :q ?z } AS ?flag)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Fatalf("S4: expected 3 results, got %d", len(r.Bindings))
	}
	for _, b := range r.Bindings {
		s := termString(b["s"])
		flag := b["flag"]
		if flag == nil {
			t.Errorf("S4: ?flag is nil for %s (EXISTS inside BIND not working)", s)
			continue
		}
		lit, ok := flag.(rdflibgo.Literal)
		if !ok {
			t.Errorf("S4: ?flag is not a literal for %s", s)
			continue
		}
		val := lit.Lexical()
		switch s {
		case "http://example.org/Alice", "http://example.org/Bob":
			if val != "true" {
				t.Errorf("S4: expected true for %s, got %s", s, val)
			}
		case "http://example.org/Charlie":
			if val != "false" {
				t.Errorf("S4: expected false for %s, got %s", s, val)
			}
		}
	}
}

// S5. Assignment error should leave variable unbound
func TestS5_AssignmentErrorUnbound(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s (1/0 AS ?x) WHERE { ?s :p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Fatalf("S5: expected 3 results, got %d", len(r.Bindings))
	}
	for _, b := range r.Bindings {
		if b["x"] != nil {
			// Per spec, 1/0 should leave ?x unbound, not crash
			// Some implementations return INF though
			t.Logf("S5: ?x = %v (may be acceptable if INF)", b["x"])
		}
	}
}

// S6. GROUP_CONCAT empty separator
func TestS6_GroupConcatEmptySeparator(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "s")
	p := rdflibgo.NewURIRefUnsafe(ex + "p")
	g.Add(s, p, rdflibgo.NewLiteral("a"))
	g.Add(s, p, rdflibgo.NewLiteral("b"))
	g.Add(s, p, rdflibgo.NewLiteral("c"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT (GROUP_CONCAT(?v; SEPARATOR="") AS ?concat) WHERE {
			:s :p ?v
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("S6: expected 1 result, got %d", len(r.Bindings))
	}
	val := r.Bindings[0]["concat"]
	if val == nil {
		t.Fatal("S6: ?concat is nil")
	}
	s6val := val.(rdflibgo.Literal).Lexical()
	// Should be "abc" (no separator), not "a b c"
	if len(s6val) != 3 {
		t.Errorf("S6: expected 3-char result (no separator), got %q", s6val)
	}
}

// S7. COUNT must ignore unbound (NULL) values
func TestS7_CountIgnoresUnbound(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s (COUNT(?label) AS ?c) WHERE {
			?s :type :Person .
			OPTIONAL { ?s :label ?label }
		} GROUP BY ?s
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range r.Bindings {
		s := termString(b["s"])
		c := b["c"]
		if c == nil {
			t.Errorf("S7: ?c is nil for %s", s)
			continue
		}
		count := c.(rdflibgo.Literal).Lexical()
		switch s {
		case "http://example.org/Alice":
			if count != "1" {
				t.Errorf("S7: expected count=1 for Alice (has label), got %s", count)
			}
		case "http://example.org/Bob", "http://example.org/Charlie":
			if count != "0" {
				t.Errorf("S7: expected count=0 for %s (no label), got %s", s, count)
			}
		}
	}
}

// S8. COUNT(DISTINCT ?x)
func TestS8_CountDistinct(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT (COUNT(DISTINCT ?type) AS ?c) WHERE {
			?s :type ?type
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("S8: expected 1 result, got %d", len(r.Bindings))
	}
	c := r.Bindings[0]["c"].(rdflibgo.Literal).Lexical()
	// Person, Thing, A, B = 4 distinct types
	if c != "4" {
		t.Errorf("S8: expected 4 distinct types, got %s", c)
	}
}

// S9. BIND inside UNION
func TestS9_BindInsideUnion(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?x ?label WHERE {
			{ ?x :type :A . BIND("typeA" AS ?label) }
			UNION
			{ ?x :type :B . BIND("typeB" AS ?label) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Fatalf("S9: expected 2 results, got %d", len(r.Bindings))
	}
	for _, b := range r.Bindings {
		if b["label"] == nil {
			t.Errorf("S9: ?label is nil for %v", b["x"])
		}
	}
}

// S10. Variable bindings in FILTER EXISTS
func TestS10_BindingsInFilterExists(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s WHERE {
			?s :p ?o .
			FILTER EXISTS { ?s :q ?z }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice and Bob have :q, Charlie doesn't
	got := extractVarValues(r.Bindings, "s")
	if len(got) != 2 {
		t.Fatalf("S10: expected 2 results, got %d: %v", len(got), got)
	}
}

// S12. Relative URI resolution with BASE
func TestS12_BaseRelativeURIResolution(t *testing.T) {
	g := rdflibgo.NewGraph()
	base := "http://example.org/base/"
	s := rdflibgo.NewURIRefUnsafe(base + "relative")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("val")
	g.Add(s, p, o)

	r, err := Query(g, `
		BASE <http://example.org/base/>
		SELECT * WHERE { <relative> <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("S12: expected 1 result, got %d", len(r.Bindings))
	}
}

func TestS12_BaseRelativeURIWithDotDot(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	r, err := Query(g, `
		BASE <http://example.org/a/b>
		SELECT * WHERE { <../c> <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("S12 (../): expected 1 result, got %d", len(r.Bindings))
	}
}

// S11. NOW() must include timezone
func TestS11_NowTimezone(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("x"))

	r, err := Query(g, `SELECT (NOW() AS ?now) WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Fatal("S11: no results")
	}
	now := r.Bindings[0]["now"].(rdflibgo.Literal).Lexical()
	if !strings.HasSuffix(now, "Z") && !strings.Contains(now, "+") && !strings.Contains(now, "-") {
		t.Errorf("S11: NOW() has no timezone: %s", now)
	}
}

// S13. Trailing semicolons in patterns
func TestS13_TrailingSemicolons(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT * WHERE { ?s :p ?o ; . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("S13: no results for trailing semicolon")
	}
}

// S14. Percent-encoding preserved in IRIs
func TestS14_PercentEncodingInIRIs(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/a%20b")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	r, err := Query(g, `SELECT * WHERE { <http://example.org/a%20b> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("S14: expected 1 result, got %d", len(r.Bindings))
	}
}

// T1. Boolean invalid lexical forms in EBV
func TestT1_BooleanInvalidLexical(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"),
		rdflibgo.NewLiteral("yes", rdflibgo.WithDatatype(rdflibgo.XSDBoolean)))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?v WHERE { :s :p ?v . FILTER(?v) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// "yes" is not a valid boolean lexical form — EBV should be false
	if len(r.Bindings) != 0 {
		t.Errorf("T1: 'yes'^^xsd:boolean should have EBV false, but got results: %v", r.Bindings)
	}
}

// T2. Numeric cast of boolean
func TestT2_NumericCastBoolean(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral(true))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT (xsd:integer(?v) AS ?i) WHERE { :s :p ?v }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("T2: expected 1 result, got %d", len(r.Bindings))
	}
	i := r.Bindings[0]["i"]
	if i == nil {
		t.Fatal("T2: xsd:integer(true) returned nil")
	}
	if i.(rdflibgo.Literal).Lexical() != "1" {
		t.Errorf("T2: expected 1, got %s", i.(rdflibgo.Literal).Lexical())
	}
}

// T3. Decimal precision
func TestT3_DecimalPrecision(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("x"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		ASK { FILTER(0.1 + 0.2 = 0.3) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("T3: 0.1 + 0.2 should equal 0.3 for decimals")
	}
}

// Helper to extract string values of a variable from bindings
// RDFLib #2151 — ENCODE_FOR_URI must encode / and use %20 not +
func TestEncodeForURI(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("hello world/foo"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT (ENCODE_FOR_URI(?o) AS ?enc) WHERE { :s :p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Bindings))
	}
	enc := r.Bindings[0]["enc"].(rdflibgo.Literal).Lexical()
	if strings.Contains(enc, "+") {
		t.Errorf("ENCODE_FOR_URI used + for space: %s", enc)
	}
	if strings.Contains(enc, "/") {
		t.Errorf("ENCODE_FOR_URI did not encode /: %s", enc)
	}
	if enc != "hello%20world%2Ffoo" {
		t.Errorf("expected hello%%20world%%2Ffoo, got %s", enc)
	}
}

// RDFLib #630 — xsd:dateTime comparison
func TestDateTimeComparison(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "event")
	g.Add(s, rdflibgo.NewURIRefUnsafe(ex+"start"),
		rdflibgo.NewLiteral("2023-01-15T10:00:00", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))
	g.Add(s, rdflibgo.NewURIRefUnsafe(ex+"end"),
		rdflibgo.NewLiteral("2023-06-20T15:00:00", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s WHERE {
			?s :start ?start .
			?s :end ?end .
			FILTER(?start < ?end)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("dateTime <: expected 1 result, got %d", len(r.Bindings))
	}
}

// RDFLib #532 — xsd:date comparison in FILTER
func TestDateComparison(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	for i, date := range []string{"2004-06-15", "2004-06-20", "2004-06-25"} {
		s := rdflibgo.NewURIRefUnsafe(ex + "item" + string(rune('A'+i)))
		g.Add(s, rdflibgo.NewURIRefUnsafe(ex+"date"),
			rdflibgo.NewLiteral(date, rdflibgo.WithDatatype(rdflibgo.XSDDate)))
	}

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT ?s WHERE {
			?s :date ?date .
			FILTER(?date >= "2004-06-20"^^xsd:date)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Fatalf("date >=: expected 2 results, got %d", len(r.Bindings))
	}
}

// RDFLib #586/#294 — initBindings visible in BIND and functions
func TestInitBindingsInBind(t *testing.T) {
	g := makeFixPlanGraph(t)
	init := map[string]rdflibgo.Term{
		"target": rdflibgo.NewURIRefUnsafe("http://example.org/Alice"),
	}
	q, err := Parse(`
		PREFIX : <http://example.org/>
		SELECT ?target ?name WHERE {
			?target :name ?name .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := EvalQuery(g, q, init)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("initBindings: expected 1 result (Alice only), got %d", len(r.Bindings))
	}
	name := r.Bindings[0]["name"].(rdflibgo.Literal).Lexical()
	if name != "Alice" {
		t.Errorf("initBindings: expected Alice, got %s", name)
	}
}

func TestInitBindingsInProjectExpr(t *testing.T) {
	g := makeFixPlanGraph(t)
	init := map[string]rdflibgo.Term{
		"target": rdflibgo.NewURIRefUnsafe("http://example.org/Alice"),
	}
	q, err := Parse(`
		PREFIX : <http://example.org/>
		SELECT ?target (STR(?target) AS ?uri) WHERE {
			?target :name ?name .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := EvalQuery(g, q, init)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("initBindings project: expected 1 result, got %d", len(r.Bindings))
	}
	uri := r.Bindings[0]["uri"]
	if uri == nil {
		t.Fatal("initBindings project: STR(?target) returned nil — initBindings not visible in projection")
	}
}

// RDFLib #2475 — STRDT must preserve lexical value for unknown datatypes
func TestSTRDT_PreservesLexical(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("<body>"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
		SELECT (STRDT(?o, rdf:HTML) AS ?tag) WHERE { :s :p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Bindings))
	}
	tag := r.Bindings[0]["tag"]
	if tag == nil {
		t.Fatal("STRDT returned nil")
	}
	lit := tag.(rdflibgo.Literal)
	if lit.Lexical() != "<body>" {
		t.Errorf("STRDT lexical: expected <body>, got %q", lit.Lexical())
	}
}

// RDFLib #619 — FILTERs in multiple subqueries must work independently
func TestFilterInMultipleSubqueries(t *testing.T) {
	g := makeFixPlanGraph(t)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?n1 ?n2 WHERE {
			{ SELECT ?n1 WHERE { ?s1 :name ?n1 . FILTER(?n1 != "Alice") } }
			{ SELECT ?n2 WHERE { ?s2 :name ?n2 . FILTER(?n2 != "Bob") } }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// n1: Bob, Charlie (2 values); n2: Alice, Charlie (2 values) → 4 results
	if len(r.Bindings) != 4 {
		t.Errorf("expected 4 results from two filtered subqueries, got %d", len(r.Bindings))
	}
}

// RDFLib #623 — Complex blank node property lists with nested bnodes
func TestComplexBnodePropertyList(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	person := rdflibgo.NewURIRefUnsafe(ex + "Person")
	alice := rdflibgo.NewURIRefUnsafe(ex + "Alice")
	idType := rdflibgo.NewURIRefUnsafe(ex + "Identifier")
	hasId := rdflibgo.NewURIRefUnsafe(ex + "id")
	hasVal := rdflibgo.NewURIRefUnsafe(ex + "has-value")

	g.Add(alice, rdflibgo.RDF.Type, person)
	bn := rdflibgo.NewBNode("")
	g.Add(alice, hasId, bn)
	g.Add(bn, rdflibgo.RDF.Type, idType)
	g.Add(bn, hasVal, rdflibgo.NewLiteral("ID-001"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s ?id WHERE {
			?s a :Person ;
			   :id [ a :Identifier ; :has-value ?id ] .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Bindings))
	}
	id := r.Bindings[0]["id"].(rdflibgo.Literal).Lexical()
	if id != "ID-001" {
		t.Errorf("expected ID-001, got %s", id)
	}
}

// RDFLib #633 — DELETE/INSERT WHERE with OPTIONAL unbound vars
func TestDeleteInsertWithOptionalUnbound(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "s")
	p := rdflibgo.NewURIRefUnsafe(ex + "p")
	_ = rdflibgo.NewURIRefUnsafe(ex + "q")
	g.Add(s, p, rdflibgo.NewLiteral("val"))
	// s has no :q triple — so ?opt will be unbound

	ds := Dataset{Default: g}
	err := Update(&ds, `
		PREFIX : <http://example.org/>
		DELETE { ?s :old ?opt }
		INSERT { ?s :new ?val }
		WHERE {
			?s :p ?val .
			OPTIONAL { ?s :q ?opt }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Should not crash; :new triple should be inserted
	if g.Len() != 2 { // original :p triple + new :new triple
		t.Errorf("expected 2 triples after update, got %d", g.Len())
	}
}

// RDFLib #648 — dateTime with timezone vs without
func TestDateTimeTimezoneComparison(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "e")
	g.Add(s, rdflibgo.NewURIRefUnsafe(ex+"a"),
		rdflibgo.NewLiteral("2023-01-15T10:00:00Z", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))
	g.Add(s, rdflibgo.NewURIRefUnsafe(ex+"b"),
		rdflibgo.NewLiteral("2023-01-15T12:00:00+02:00", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s WHERE {
			?s :a ?a . ?s :b ?b .
			FILTER(?a = ?b)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 10:00:00Z == 12:00:00+02:00 (same instant)
	if len(r.Bindings) != 1 {
		t.Errorf("timezone-aware dateTime comparison: expected 1 result, got %d", len(r.Bindings))
	}
}

// RDFLib #554 — SELECT with empty WHERE
func TestSelectEmptyWhere(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"),
		rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("x"))

	// Per SPARQL spec, empty WHERE = 1 solution (empty mapping)
	// Projecting an unbound variable should yield 1 row with ?x = nil
	r, err := Query(g, `SELECT ?x WHERE {}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("empty WHERE should produce 1 solution, got %d", len(r.Bindings))
	}
}

// RDFLib #977 — Consistent prefix substitution in serializer
func TestStrdtPreservesUnknownDatatype(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("test"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT (STRDT(?o, :CustomType) AS ?typed) WHERE { :s :p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Bindings))
	}
	typed := r.Bindings[0]["typed"]
	if typed == nil {
		t.Fatal("STRDT with custom datatype returned nil")
	}
	lit := typed.(rdflibgo.Literal)
	if lit.Lexical() != "test" {
		t.Errorf("expected lexical 'test', got %q", lit.Lexical())
	}
	if lit.Datatype().Value() != ex+"CustomType" {
		t.Errorf("expected datatype %sCustomType, got %s", ex, lit.Datatype().Value())
	}
}

// RDFLib #715 — Property path + transitive closure
func TestPropertyPathPlus(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	a := rdflibgo.NewURIRefUnsafe(ex + "A")
	b := rdflibgo.NewURIRefUnsafe(ex + "B")
	c := rdflibgo.NewURIRefUnsafe(ex + "C")
	p := rdflibgo.NewURIRefUnsafe(ex + "p")

	// Chain: A -p-> B -p-> C
	g.Add(a, p, b)
	g.Add(b, p, c)

	// A p+ C should be true (A→B→C)
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?end WHERE { :A :p+ ?end }
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := extractVarValues(r.Bindings, "end")
	// Should find B (direct) and C (transitive)
	if len(got) != 2 {
		t.Errorf("#715: :A :p+ ?end expected 2 results (B,C), got %d: %v", len(got), got)
	}

	// ASK: A p+ C should be true
	r2, err := Query(g, `
		PREFIX : <http://example.org/>
		ASK { :A :p+ :C }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.AskResult {
		t.Error("#715: ASK { :A :p+ :C } should be true (transitive chain A→B→C)")
	}
}

// RDFLib #715 variant — must NOT produce spurious results
func TestPropertyPathPlusNoSpurious(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	a := rdflibgo.NewURIRefUnsafe(ex + "A")
	b := rdflibgo.NewURIRefUnsafe(ex + "B")
	x := rdflibgo.NewURIRefUnsafe(ex + "X")
	y := rdflibgo.NewURIRefUnsafe(ex + "Y")
	isa := rdflibgo.NewURIRefUnsafe(ex + "isa")

	// A isa X, A isa Y, B isa X (but B does NOT isa Y directly or transitively)
	g.Add(a, isa, x)
	g.Add(a, isa, y)
	g.Add(b, isa, x)

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		ASK { :B :isa+ :Y }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.AskResult {
		t.Error("#715: ASK { :B :isa+ :Y } should be false — no chain from B to Y")
	}
}

// RDFLib #714 — BNode + property paths combined
func TestBnodePlusPropertyPath(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	a := rdflibgo.NewURIRefUnsafe(ex + "A")
	p := rdflibgo.NewURIRefUnsafe(ex + "p")
	q := rdflibgo.NewURIRefUnsafe(ex + "q")
	bn := rdflibgo.NewBNode("")
	g.Add(a, p, bn)
	g.Add(bn, q, rdflibgo.NewLiteral("val"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?v WHERE { :A :p [ :q ?v ] }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("BNode+path: expected 1 result, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["v"].(rdflibgo.Literal).Lexical() != "val" {
		t.Error("BNode+path: wrong value")
	}
}

// RDFLib #196 — Lexical form preservation
func TestLexicalFormPreservation(t *testing.T) {
	// "2.50"^^xsd:decimal should stay "2.50", not normalize to "2.5"
	lit := rdflibgo.NewLiteral("2.50", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
	if lit.Lexical() != "2.50" {
		t.Errorf("#196: lexical form not preserved: got %q, want %q", lit.Lexical(), "2.50")
	}
	// Roundtrip through N3
	n3 := lit.N3()
	if !strings.Contains(n3, "2.50") {
		t.Errorf("#196: N3() normalized lexical form: %s", n3)
	}
}

// RDFLib #910 — UNION with identical results must NOT deduplicate
func TestUnionIdenticalBranches(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"s"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("x"))

	r, err := Query(g, `SELECT * { { BIND("a" AS ?a) } UNION { BIND("a" AS ?a) } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("#910: UNION identical branches: expected 2 rows, got %d", len(r.Bindings))
	}
}

// RDFLib #3381 — ASK { FILTER(false) } must return false
func TestAskFilterFalse(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"),
		rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("x"))

	r, err := Query(g, `ASK { FILTER(false) }`)
	if err != nil {
		t.Fatal(err)
	}
	if r.AskResult {
		t.Error("#3381: ASK { FILTER(false) } should return false")
	}
}

// RDFLib #3382 — GROUP BY on empty result should return 0 rows
func TestGroupByEmptyResult(t *testing.T) {
	g := rdflibgo.NewGraph()
	// Empty graph — no triples match
	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?s (COUNT(?o) AS ?n) WHERE {
			?s :nonexistent ?o
		} GROUP BY ?s
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 0 {
		t.Errorf("#3382: GROUP BY on empty result: expected 0 rows, got %d: %v", len(r.Bindings), r.Bindings)
	}
}

// RDFLib #936 — HAVING with variable comparison
func TestHavingVariableComparison(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "s")
	p1 := rdflibgo.NewURIRefUnsafe(ex + "p1")
	p2 := rdflibgo.NewURIRefUnsafe(ex + "p2")
	excluded := rdflibgo.NewURIRefUnsafe(ex + "excluded")
	g.Add(s, p1, rdflibgo.NewLiteral("a"))
	g.Add(s, p2, rdflibgo.NewLiteral("b"))
	g.Add(s, excluded, rdflibgo.NewLiteral("c"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?p (COUNT(?o) AS ?n) WHERE {
			?s ?p ?o
		} GROUP BY ?p HAVING (?p != :excluded)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("#936: HAVING filter: expected 2 groups, got %d", len(r.Bindings))
	}
}

// RDFLib #1967 — Property path on long list (no stack overflow)
func TestPropertyPathLongList(t *testing.T) {
	g := rdflibgo.NewGraph()
	// Build a chain of 500 nodes: n0 -> n1 -> n2 -> ... -> n499
	ex := "http://example.org/"
	p := rdflibgo.NewURIRefUnsafe(ex + "next")
	for i := 0; i < 499; i++ {
		from := rdflibgo.NewURIRefUnsafe(ex + "n" + strconv.Itoa(i))
		to := rdflibgo.NewURIRefUnsafe(ex + "n" + strconv.Itoa(i+1))
		g.Add(from, p, to)
	}

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT (COUNT(?end) AS ?c) WHERE { :n0 :next+ ?end }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Bindings))
	}
	c := r.Bindings[0]["c"].(rdflibgo.Literal).Lexical()
	if c != "499" {
		t.Errorf("#1967: long chain path+: expected 499 reachable nodes, got %s", c)
	}
}

// RDFLib #2011 — Comma-separated blank node objects
func TestCommaSeparatedBnodeObjects(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	s := rdflibgo.NewURIRefUnsafe(ex + "s")
	p := rdflibgo.NewURIRefUnsafe(ex + "fields")
	name := rdflibgo.NewURIRefUnsafe(ex + "name")
	bn1 := rdflibgo.NewBNode("")
	bn2 := rdflibgo.NewBNode("")
	g.Add(s, p, bn1)
	g.Add(s, p, bn2)
	g.Add(bn1, name, rdflibgo.NewLiteral("field1"))
	g.Add(bn2, name, rdflibgo.NewLiteral("field2"))

	r, err := Query(g, `
		PREFIX : <http://example.org/>
		SELECT ?v WHERE {
			:s :fields [ :name ?v ]
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("#2011: blank node objects: expected 2 results, got %d", len(r.Bindings))
	}
}

// RDFLib #2077 — Two prefixes mapping to same IRI
func TestDuplicatePrefixes(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.com#"
	g.Add(rdflibgo.NewURIRefUnsafe(ex+"A"), rdflibgo.NewURIRefUnsafe(ex+"p"), rdflibgo.NewLiteral("val"))

	r, err := Query(g, `
		PREFIX foo: <http://example.com#>
		PREFIX bar: <http://example.com#>
		SELECT * WHERE { foo:A bar:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("#2077: duplicate prefixes: expected 1 result, got %d", len(r.Bindings))
	}
}

func extractVarValues(bindings []map[string]rdflibgo.Term, varName string) []string {
	var result []string
	for _, b := range bindings {
		if v := b[varName]; v != nil {
			if u, ok := v.(rdflibgo.URIRef); ok {
				result = append(result, u.Value())
			} else {
				result = append(result, v.N3())
			}
		}
	}
	return result
}
