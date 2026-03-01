package sparql

import (
	"fmt"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Ported from: test/test_sparql/

func makeSPARQLGraph(t *testing.T) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	bob, _ := rdflibgo.NewURIRef("http://example.org/Bob")
	charlie, _ := rdflibgo.NewURIRef("http://example.org/Charlie")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	age, _ := rdflibgo.NewURIRef("http://example.org/age")
	knows, _ := rdflibgo.NewURIRef("http://example.org/knows")
	person, _ := rdflibgo.NewURIRef("http://example.org/Person")

	g.Add(alice, rdflibgo.RDF.Type, person)
	g.Add(alice, name, rdflibgo.NewLiteral("Alice"))
	g.Add(alice, age, rdflibgo.NewLiteral(30))
	g.Add(alice, knows, bob)

	g.Add(bob, rdflibgo.RDF.Type, person)
	g.Add(bob, name, rdflibgo.NewLiteral("Bob"))
	g.Add(bob, age, rdflibgo.NewLiteral(25))
	g.Add(bob, knows, charlie)

	g.Add(charlie, rdflibgo.RDF.Type, person)
	g.Add(charlie, name, rdflibgo.NewLiteral("Charlie"))
	g.Add(charlie, age, rdflibgo.NewLiteral(35))
	return g
}

func TestSPARQLSelectAll(t *testing.T) {
	// Ported from: rdflib SPARQL SELECT *
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT * WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectVars(t *testing.T) {
	// Ported from: rdflib SPARQL SELECT with specific vars
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Vars) != 1 || r.Vars[0] != "name" {
		t.Errorf("expected [name], got %v", r.Vars)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectFilter(t *testing.T) {
	// Ported from: rdflib SPARQL FILTER
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(?age > 28)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2 (Alice=30, Charlie=35), got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectOptional(t *testing.T) {
	// Ported from: rdflib SPARQL OPTIONAL
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?friend WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:knows ?f . ?f ex:name ?friend }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice→Bob, Bob→Charlie, Charlie→(no knows) = 3 results
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectUnion(t *testing.T) {
	// Ported from: rdflib SPARQL UNION
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			{ ?x ex:name "Alice" }
			UNION
			{ ?x ex:name "Bob" }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLAsk(t *testing.T) {
	// Ported from: rdflib SPARQL ASK
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name "Alice" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true")
	}

	r2, _ := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name "Nobody" }
	`)
	if r2.AskResult {
		t.Error("expected false")
	}
}

func TestSPARQLConstruct(t *testing.T) {
	// Ported from: rdflib SPARQL CONSTRUCT
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 3 {
		t.Errorf("expected 3, got %d", r.Graph.Len())
	}
}

func TestSPARQLOrderBy(t *testing.T) {
	// Ported from: rdflib SPARQL ORDER BY
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		ORDER BY ?name
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Fatalf("expected 3, got %d", len(r.Bindings))
	}
	names := []string{r.Bindings[0]["name"].String(), r.Bindings[1]["name"].String(), r.Bindings[2]["name"].String()}
	if names[0] != "Alice" || names[1] != "Bob" || names[2] != "Charlie" {
		t.Errorf("expected sorted, got %v", names)
	}
}

func TestSPARQLOrderByDesc(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?age WHERE { ?s ex:name ?name . ?s ex:age ?age }
		ORDER BY DESC(?age)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Bindings[0]["name"].String() != "Charlie" {
		t.Errorf("expected Charlie first, got %s", r.Bindings[0]["name"].String())
	}
}

func TestSPARQLLimit(t *testing.T) {
	// Ported from: rdflib SPARQL LIMIT
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		LIMIT 2
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLOffset(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		ORDER BY ?name
		OFFSET 1
		LIMIT 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["name"].String() != "Bob" {
		t.Errorf("expected Bob, got %v", r.Bindings)
	}
}

func TestSPARQLDistinct(t *testing.T) {
	// Ported from: rdflib SPARQL DISTINCT
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT DISTINCT ?type WHERE { ?s a ?type }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 distinct type, got %d", len(r.Bindings))
	}
}

func TestSPARQLBind(t *testing.T) {
	// Ported from: rdflib SPARQL BIND
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?upper WHERE {
			?s ex:name ?name .
			BIND(UCASE(?name) AS ?upper)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Fatalf("expected 3, got %d", len(r.Bindings))
	}
	// Check one binding has upper case
	found := false
	for _, b := range r.Bindings {
		if b["name"].String() == "Alice" && b["upper"].String() == "ALICE" {
			found = true
		}
	}
	if !found {
		t.Error("expected UCASE binding")
	}
}

func TestSPARQLFilterRegex(t *testing.T) {
	// Ported from: rdflib SPARQL FILTER REGEX
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(REGEX(?name, "^A"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["name"].String() != "Alice" {
		t.Errorf("expected Alice, got %v", r.Bindings)
	}
}

func TestSPARQLFilterBound(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:knows ?f }
			FILTER(BOUND(?f))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice and Bob have knows, Charlie doesn't
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLFilterIsIRI(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			ex:Alice ?p ?o .
			FILTER(ISIRI(?o))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// ex:Person (type) and ex:Bob (knows) = 2 IRIs
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLBuiltinStringFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(CONTAINS(?name, "li"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice and Charlie contain "li"
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLMultipleTriplePatterns(t *testing.T) {
	// Ported from: rdflib SPARQL — join via shared variable
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?friendName WHERE {
			?s ex:name ?name .
			?s ex:knows ?f .
			?f ex:name ?friendName .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice→Bob, Bob→Charlie
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLTypeShorthand(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE { ?s a ex:Person }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLArithmetic(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?nextAge WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			BIND(?age + 1 AS ?nextAge)
		}
		ORDER BY ?name
		LIMIT 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["nextAge"].String() != "31" {
		t.Errorf("expected 31, got %v", r.Bindings[0]["nextAge"])
	}
}

func TestSPARQLComparisonOps(t *testing.T) {
	g := makeSPARQLGraph(t)
	tests := []struct {
		filter string
		count  int
	}{
		{`FILTER(?age >= 30)`, 2},
		{`FILTER(?age <= 25)`, 1},
		{`FILTER(?age != 30)`, 2},
		{`FILTER(?age = 30)`, 1},
	}
	for _, tt := range tests {
		r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:age ?age . `+tt.filter+` }`)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Bindings) != tt.count {
			t.Errorf("%s: expected %d, got %d", tt.filter, tt.count, len(r.Bindings))
		}
	}
}

func TestSPARQLBooleanOps(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(?age > 20 && ?age < 35)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2 (Alice=30, Bob=25), got %d", len(r.Bindings))
	}
}

func TestSPARQLOrFilter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(?name = "Alice" || ?name = "Charlie")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLNotFilter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(!(?name = "Alice"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2 (Bob, Charlie), got %d", len(r.Bindings))
	}
}

func TestSPARQLBuiltinFunctionsExtended(t *testing.T) {
	g := makeSPARQLGraph(t)
	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{"STRSTARTS", `SELECT ?n WHERE { ?s ex:name ?n . FILTER(STRSTARTS(?n, "A")) }`, 1},
		{"STRENDS", `SELECT ?n WHERE { ?s ex:name ?n . FILTER(STRENDS(?n, "e")) }`, 2},
		{"LCASE", `SELECT ?n WHERE { ?s ex:name ?n . BIND(LCASE(?n) AS ?l) FILTER(?l = "alice") }`, 1},
		{"STRLEN", `SELECT ?n WHERE { ?s ex:name ?n . FILTER(STRLEN(?n) > 4) }`, 2},
		{"ISBLANK", `SELECT ?s WHERE { ?s ex:name ?n . FILTER(!ISBLANK(?s)) }`, 3},
		{"ISLITERAL", `SELECT ?n WHERE { ?s ex:name ?n . FILTER(ISLITERAL(?n)) }`, 3},
		{"STR", `SELECT ?n WHERE { ?s ex:name ?n . BIND(STR(?s) AS ?u) FILTER(CONTAINS(?u, "Alice")) }`, 1},
		{"IF", `SELECT ?n WHERE { ?s ex:name ?n . ?s ex:age ?a . BIND(IF(?a > 30, "old", "young") AS ?cat) FILTER(?cat = "old") }`, 1},
	}
	for _, tt := range tests {
		r, err := Query(g, `PREFIX ex: <http://example.org/> `+tt.query)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if len(r.Bindings) != tt.expect {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.expect, len(r.Bindings))
		}
	}
}

func TestSPARQLNumericFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(ABS(?age - 30) <= 5)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice=30 (diff 0), Bob=25 (diff 5), Charlie=35 (diff 5)
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLCoalesce(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?friend WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:knows ?f . ?f ex:name ?friend }
			BIND(COALESCE(?friend, "none") AS ?result)
			FILTER(?result = "none")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 (Charlie), got %d", len(r.Bindings))
	}
}

func TestSPARQLSameTerm(t *testing.T) {
	g := makeSPARQLGraph(t)
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(SAMETERM(?s, ex:Alice))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = alice
	if len(r.Bindings) != 1 || r.Bindings[0]["name"].String() != "Alice" {
		t.Errorf("expected Alice, got %v", r.Bindings)
	}
}

func TestSPARQLParseError(t *testing.T) {
	g := makeSPARQLGraph(t)
	_, err := Query(g, `NOT A VALID QUERY`)
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- Benchmarks ---

func BenchmarkSPARQLSelectAll(b *testing.B) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	for i := 0; i < 100; i++ {
		s := rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		p, _ := rdflibgo.NewURIRef("http://example.org/name")
		g.Add(s, p, rdflibgo.NewLiteral(fmt.Sprintf("name%d", i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Query(g, `PREFIX ex: <http://example.org/> SELECT * WHERE { ?s ex:name ?name }`)
	}
}

func BenchmarkSPARQLFilter(b *testing.B) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	p, _ := rdflibgo.NewURIRef("http://example.org/val")
	for i := 0; i < 100; i++ {
		s := rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(s, p, rdflibgo.NewLiteral(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val ?v . FILTER(?v > 50) }`)
	}
}
