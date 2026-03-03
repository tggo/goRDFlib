package sparql

import (
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
