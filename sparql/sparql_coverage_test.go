package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestSPARQLValues(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			VALUES ?name { "Alice" "Charlie" }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLInitBindings(t *testing.T) {
	g := makeSPARQLGraph(t)
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
	`, map[string]rdflibgo.Term{"s": alice})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["name"].String() != "Alice" {
		t.Errorf("expected Alice, got %v", r.Bindings)
	}
}

func TestSPARQLSubtract(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?age WHERE {
			?s ex:age ?age .
			FILTER(?age - 20 > 5)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 30-20=10>5, 25-20=5 not >5, 35-20=15>5 → 2
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLMultiply(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(?age * 2 > 60)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 30*2=60 not >60, 25*2=50 not, 35*2=70 yes → 1
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 (Charlie), got %d", len(r.Bindings))
	}
}

func TestSPARQLDivide(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(?age / 5 > 5)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 30/5=6>5, 25/5=5 not, 35/5=7>5 → 2
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestSPARQLNegativeUnary(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			BIND(-1 * ?age AS ?neg)
			FILTER(?neg < -30)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// -35 < -30 → 1 (Charlie)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLLangFunction(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/label")
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))
	g.Add(s, p, rdflibgo.NewLiteral("hallo", rdflibgo.WithLang("de")))

	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?l WHERE { ex:s ex:label ?l . FILTER(LANG(?l) = "en") }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLDatatypeFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(DATATYPE(?name) = xsd:string)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLIsNumericFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE { ?s ?p ?o . FILTER(ISNUMERIC(?o)) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 3 age values
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

func TestSPARQLConcatFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?full WHERE {
			?s ex:name ?name .
			BIND(CONCAT("Hello, ", ?name) AS ?full)
			FILTER(?name = "Alice")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["full"].String() != "Hello, Alice" {
		t.Errorf("expected 'Hello, Alice', got %v", r.Bindings)
	}
}

func TestSPARQLReplaceFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?replaced WHERE {
			?s ex:name "Alice" .
			BIND(REPLACE("Alice", "A", "a") AS ?replaced)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["replaced"].String() != "alice" {
		t.Errorf("expected 'alice', got %v", r.Bindings)
	}
}

func TestSPARQLSubstrFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?sub WHERE {
			?s ex:name "Alice" .
			BIND(SUBSTR("Alice", 1, 3) AS ?sub)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["sub"].String() != "Ali" {
		t.Errorf("expected 'Ali', got %v", r.Bindings)
	}
}

func TestSPARQLHashFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?h WHERE {
			?s ex:name "Alice" .
			BIND(MD5("Alice") AS ?h)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["h"].String() == "" {
		t.Error("expected non-empty MD5 hash")
	}

	r2, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?h WHERE { ?s ex:name "Alice" . BIND(SHA1("Alice") AS ?h) }`)
	if len(r2.Bindings) != 1 || r2.Bindings[0]["h"].String() == "" {
		t.Error("expected non-empty SHA1 hash")
	}

	r3, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?h WHERE { ?s ex:name "Alice" . BIND(SHA256("Alice") AS ?h) }`)
	if len(r3.Bindings) != 1 || r3.Bindings[0]["h"].String() == "" {
		t.Error("expected non-empty SHA256 hash")
	}
}

func TestSPARQLRoundCeilFloor(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/val")
	g.Add(s, p, rdflibgo.NewLiteral(3.7))

	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?r ?c ?f WHERE { ?s ex:val ?v . BIND(ROUND(?v) AS ?r) BIND(CEIL(?v) AS ?c) BIND(FLOOR(?v) AS ?f) }`)
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1")
	}
	if r.Bindings[0]["r"].String() != "4" {
		t.Errorf("ROUND: got %s", r.Bindings[0]["r"].String())
	}
}

func TestSPARQLRegexCaseInsensitive(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(REGEX(?name, "^alice", "i"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLEffectiveBooleanValue(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/val")
	g.Add(s, p, rdflibgo.NewLiteral(""))
	g.Add(s, p, rdflibgo.NewLiteral("nonempty"))
	g.Add(s, p, rdflibgo.NewLiteral(0))

	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:val ?v . FILTER(?v) }`)
	// "" is falsy, 0 is falsy, "nonempty" is truthy
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy, got %d", len(r.Bindings))
	}
}
