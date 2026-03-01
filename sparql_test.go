package rdflibgo

import "testing"

// Ported from: test/test_sparql/

func makeSPARQLGraph(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	alice, _ := NewURIRef("http://example.org/Alice")
	bob, _ := NewURIRef("http://example.org/Bob")
	charlie, _ := NewURIRef("http://example.org/Charlie")
	name, _ := NewURIRef("http://example.org/name")
	age, _ := NewURIRef("http://example.org/age")
	knows, _ := NewURIRef("http://example.org/knows")
	person, _ := NewURIRef("http://example.org/Person")

	g.Add(alice, RDF.Type, person)
	g.Add(alice, name, NewLiteral("Alice"))
	g.Add(alice, age, NewLiteral(30))
	g.Add(alice, knows, bob)

	g.Add(bob, RDF.Type, person)
	g.Add(bob, name, NewLiteral("Bob"))
	g.Add(bob, age, NewLiteral(25))
	g.Add(bob, knows, charlie)

	g.Add(charlie, RDF.Type, person)
	g.Add(charlie, name, NewLiteral("Charlie"))
	g.Add(charlie, age, NewLiteral(35))
	return g
}

func TestSPARQLSelectAll(t *testing.T) {
	// Ported from: rdflib SPARQL SELECT *
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name "Alice" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true")
	}

	r2, _ := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	r, err := g.Query(`
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
	// Test 'a' as rdf:type in SPARQL
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
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
