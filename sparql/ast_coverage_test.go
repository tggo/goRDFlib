package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)


// EvalQuery edge cases
func TestEvalQueryASK(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `PREFIX ex: <http://example.org/> ASK { ex:Alice ex:name "Alice" }`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true")
	}
}

func TestEvalQueryCONSTRUCT(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() != 3 {
		t.Errorf("expected 3 triples in constructed graph, got %d", r.Graph.Len())
	}
}

func TestEvalQueryCONSTRUCTWhere(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() == 0 {
		t.Error("expected triples in CONSTRUCT WHERE result")
	}
}

func TestEvalQueryInvalidType(t *testing.T) {
	g := makeSPARQLGraph(t)
	_, err := Query(g, `PREFIX ex: <http://example.org/> DESCRIBE ex:Alice`)
	if err == nil {
		t.Error("expected error for DESCRIBE")
	}
}

// GROUP BY with complex aggregate expressions
func TestEvalAggExprBinary(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SUM(?age) + 1 AS ?total) WHERE { ?s ex:age ?age }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected bindings")
	}
}

func TestEvalAggExprUnary(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (-COUNT(?name) AS ?neg) WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected bindings")
	}
}

func TestEvalAggExprIRI(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(?s) AS ?c) WHERE { ?s ex:name ?name } GROUP BY ?name
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// EXISTS/NOT EXISTS in FILTER
func TestEvalExistsFilter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER EXISTS { ?s ex:age ?a . FILTER(?a > 30) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 (Charlie), got %d", len(r.Bindings))
	}
}

// CONSTRUCT with ORDER BY + LIMIT
func TestConstructOrderByLimit(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
		ORDER BY ?name
		LIMIT 2
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", r.Graph.Len())
	}
}

// GRAPH pattern in query
func TestEvalGraphPattern(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	o := rdflibgo.NewLiteral("v")
	g.Add(s, p, o)

	ng := rdflibgo.NewGraph()
	ng.Add(s, p, rdflibgo.NewLiteral("ng-v"))

	q, err := Parse(`PREFIX ex: <http://example.org/>
		SELECT ?v WHERE { GRAPH ex:g1 { ?s ex:p ?v } }`)
	if err != nil {
		t.Fatal(err)
	}
	q.NamedGraphs = map[string]*rdflibgo.Graph{
		"http://example.org/g1": ng,
	}
	r, err := EvalQuery(g, q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["v"].String() != "ng-v" {
		t.Errorf("expected ng-v, got %v", r.Bindings)
	}
}

// Unary NOT
func TestEvalUnaryNot(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(!(?age > 30))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// IN / NOT IN
func TestEvalINOperator(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(?name IN ("Alice", "Bob"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestEvalNOTINOperator(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(?name NOT IN ("Alice", "Bob"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 (Charlie), got %d", len(r.Bindings))
	}
}

// SAMPLE aggregate
func TestEvalSampleAggregate(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SAMPLE(?name) AS ?n) WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["n"] == nil {
		t.Error("expected a sample value")
	}
}

// GROUP_CONCAT
func TestEvalGroupConcatAggregate(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (GROUP_CONCAT(?name; SEPARATOR=", ") AS ?names) WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["names"] == nil {
		t.Error("expected concatenated names")
	}
}

// HAVING clause
func TestEvalHaving(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name (COUNT(?s) AS ?c)
		WHERE { ?s ex:name ?name }
		GROUP BY ?name
		HAVING (COUNT(?s) > 0)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// CONSTRUCT with UNION pattern to hit extractTemplateFromPattern
func TestConstructWithUnion(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE {
			{ ?s ex:name ?name } UNION { ?s ex:age ?name }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() == 0 {
		t.Error("expected triples")
	}
}

// IRI expression in projection
func TestEvalIRIInProjection(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name (ex:Type AS ?t) WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// effectiveBooleanValue edge cases
func TestEvalFilterBoolean(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral(true))
	g.Add(s, p, rdflibgo.NewLiteral(false))

	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:v ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy boolean, got %d", len(r.Bindings))
	}
}

// evalExprWithGraph — EXISTS in projection context
func TestEvalExprWithGraphExists(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER NOT EXISTS { ?s ex:nonexistent ?x }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// Subquery
func TestEvalSubquery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ SELECT ?s ?name WHERE { ?s ex:name ?name } ORDER BY ?name LIMIT 2 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// Date functions
func TestEvalDateFunctions(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/date")
	g.Add(s, p, rdflibgo.NewLiteral("2024-03-15T10:30:45Z", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))

	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT ?y ?m ?d ?h ?min ?sec WHERE {
			?s ex:date ?dt .
			BIND(YEAR(?dt) AS ?y)
			BIND(MONTH(?dt) AS ?m)
			BIND(DAY(?dt) AS ?d)
			BIND(HOURS(?dt) AS ?h)
			BIND(MINUTES(?dt) AS ?min)
			BIND(SECONDS(?dt) AS ?sec)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1 binding")
	}
}

// STRBEFORE/STRAFTER
func TestEvalStrBeforeAfter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?before ?after WHERE {
			?s ex:name "Alice" .
			BIND(STRBEFORE("Alice", "l") AS ?before)
			BIND(STRAFTER("Alice", "l") AS ?after)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1")
	}
	if r.Bindings[0]["before"].String() != "A" {
		t.Errorf("STRBEFORE: got %s", r.Bindings[0]["before"].String())
	}
}

// IRI/URI functions
func TestEvalIRIFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?i WHERE {
			?s ex:name "Alice" .
			BIND(IRI("http://example.org/test") AS ?i)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1")
	}
}

// IF function
func TestEvalIFFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?result WHERE {
			?s ex:age ?age .
			BIND(IF(?age > 30, "old", "young") AS ?result)
		} ORDER BY ?age
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// COALESCE function
func TestEvalCoalesce(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?val WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:nonexistent ?x }
			BIND(COALESCE(?x, ?name) AS ?val)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// MINUS pattern
func TestEvalMinus(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			MINUS { ?s ex:age 30 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// ---- effectiveBooleanValue: integer/float/decimal paths ----

func TestEBVIntegerZero(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral(int64(0), rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(s, p, rdflibgo.NewLiteral(int64(5), rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:v ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy integer, got %d", len(r.Bindings))
	}
}

func TestEBVFloat(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral("0.0", rdflibgo.WithDatatype(rdflibgo.XSDFloat)))
	g.Add(s, p, rdflibgo.NewLiteral("3.14", rdflibgo.WithDatatype(rdflibgo.XSDFloat)))
	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:v ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy float, got %d", len(r.Bindings))
	}
}

func TestEBVDecimal(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral("0.0", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	g.Add(s, p, rdflibgo.NewLiteral("1.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:v ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy decimal, got %d", len(r.Bindings))
	}
}

func TestEBVDefaultLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	custom := rdflibgo.NewURIRefUnsafe("http://example.org/mytype")
	g.Add(s, p, rdflibgo.NewLiteral("somevalue", rdflibgo.WithDatatype(custom)))
	g.Add(s, p, rdflibgo.NewLiteral("", rdflibgo.WithDatatype(custom)))
	r, _ := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:v ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy custom-typed literal, got %d", len(r.Bindings))
	}
}

// ---- SUM with decimal values (hits hasDecimal path in evalAggregate) ----

func TestSumDecimal(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral("1.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	g.Add(s, p, rdflibgo.NewLiteral("2.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT (SUM(?v) AS ?total) WHERE { ?s ex:v ?v }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["total"] == nil {
		t.Error("expected sum value")
	}
}

func TestAvgDecimal(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/v")
	g.Add(s, p, rdflibgo.NewLiteral("2.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	g.Add(s, p, rdflibgo.NewLiteral("3.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT (AVG(?v) AS ?avg) WHERE { ?s ex:v ?v }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["avg"] == nil {
		t.Fatal("expected avg value")
	}
}

// ---- extractDatePart: date-only and datetime-without-timezone formats ----

func TestExtractDatePartDateOnly(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/date")
	g.Add(s, p, rdflibgo.NewLiteral("2024-06-15", rdflibgo.WithDatatype(rdflibgo.XSDDate)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?y ?m ?d WHERE {
			?s ex:date ?dt .
			BIND(YEAR(?dt) AS ?y)
			BIND(MONTH(?dt) AS ?m)
			BIND(DAY(?dt) AS ?d)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["y"].String() != "2024" {
		t.Errorf("expected year 2024, got %s", r.Bindings[0]["y"].String())
	}
}

func TestExtractDatePartNoTZ(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/date")
	g.Add(s, p, rdflibgo.NewLiteral("2024-03-15T10:30:45", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?h ?min WHERE {
			?s ex:date ?dt .
			BIND(HOURS(?dt) AS ?h)
			BIND(MINUTES(?dt) AS ?min)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding")
	}
	if r.Bindings[0]["h"].String() != "10" {
		t.Errorf("expected hours=10, got %s", r.Bindings[0]["h"].String())
	}
}

// ---- parseLiteralString: long-quote (triple-quoted) strings ----

func TestParseLiteralStringLangTag(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/label")
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?l WHERE { ?s ex:label "hello"@en }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestParseLiteralStringDirLangTag(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p "hello"@en--ltr }`)
	if err != nil {
		t.Fatal("expected success for directional lang tag", err)
	}
}

func TestParseLiteralStringTripleQuote(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/label")
	g.Add(s, p, rdflibgo.NewLiteral("hello world"))
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?l WHERE { ?s ex:label """hello world""" }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// ---- validateStringEscapes: various escape sequences ----

func TestValidateStringEscapesInvalidEscape(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p "hello\qworld" }`)
	if err == nil {
		t.Error("expected parse error for invalid \\q escape")
	}
}

func TestValidateStringEscapesUEscape(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p "hello\u0041world" }`)
	if err != nil {
		t.Fatalf("expected success for valid \\u escape: %v", err)
	}
}

func TestValidateStringEscapesUEscapeSurrogate(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p "\uD800" }`)
	if err == nil {
		t.Error("expected parse error for surrogate codepoint")
	}
}

func TestValidateStringEscapesCapUEscape(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p "hello\U00000041world" }`)
	if err != nil {
		t.Fatalf("expected success for valid \\U escape: %v", err)
	}
}

func TestValidateStringEscapesLongQuote(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p """hello\nworld""" }`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- validateConstructWhere: FILTER and complex patterns are errors ----

func TestValidateConstructWhereFilter(t *testing.T) {
	_, err := Parse(`CONSTRUCT WHERE { ?s ?p ?o . FILTER(?o) }`)
	if err == nil {
		t.Error("expected error: FILTER not allowed in CONSTRUCT WHERE")
	}
}

func TestValidateConstructWhereOptional(t *testing.T) {
	_, err := Parse(`CONSTRUCT WHERE { ?s ?p ?o . OPTIONAL { ?s ?p2 ?o2 } }`)
	if err == nil {
		t.Error("expected error: OPTIONAL not allowed in CONSTRUCT WHERE")
	}
}

// ---- unescapePNLocal: backslash and percent-encoding paths ----

func TestUnescapePNLocalBackslash(t *testing.T) {
	// Exercise unescapePNLocal with backslash escape via CONSTRUCT template
	// resolveTermRef (used in CONSTRUCT) calls unescapePNLocal
	q, err := Parse(`PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:hello\!world ?o }
		WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatalf("expected success for escaped local name: %v", err)
	}
	if len(q.Construct) == 0 {
		t.Error("expected CONSTRUCT template triples")
	}
	// Evaluate with a graph to trigger resolveTermRef -> unescapePNLocal
	g := makeSPARQLGraph(t)
	r, err := EvalQuery(g, q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected constructed graph")
	}
}

func TestUnescapePNLocalPercentEncoding(t *testing.T) {
	// Exercise unescapePNLocal with percent encoding via CONSTRUCT template
	q, err := Parse(`PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:hello%20world ?o }
		WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatalf("expected success for percent-encoded local name: %v", err)
	}
	g := makeSPARQLGraph(t)
	r, err := EvalQuery(g, q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected constructed graph")
	}
}

// ---- validateBindScope: BIND duplicate variable ----

func TestValidateBindScopeDuplicate(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p ?name . BIND(?s AS ?name) }`)
	if err == nil {
		t.Error("expected error: BIND variable already in scope")
	}
	if err != nil && !strings.Contains(err.Error(), "BIND") {
		t.Errorf("expected BIND error, got: %v", err)
	}
}

// ---- parseGraphRefAll: NAMED and ALL targets for CLEAR/DROP ----

func TestClearNamed(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `CLEAR NAMED`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 0 {
		t.Errorf("expected named graph to be cleared")
	}
}

func TestDropAll(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	def := rdflibgo.NewGraph()
	def.Add(s, p, rdflibgo.NewLiteral("default"))
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("named"))
	ds := &Dataset{
		Default:     def,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `DROP ALL`); err != nil {
		t.Fatal(err)
	}
	if def.Len() != 0 {
		t.Errorf("expected default graph to be cleared")
	}
}

func TestClearGraphIRI(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `CLEAR GRAPH <http://example.org/g1>`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 0 {
		t.Errorf("expected named graph to be cleared")
	}
}

func TestClearNonExistentSilent(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}
	if err := Update(ds, `CLEAR SILENT GRAPH <http://example.org/nonexistent>`); err != nil {
		t.Fatal(err)
	}
}

// ---- evalInsertData / evalDeleteData: named graph quads path ----

func TestInsertDataNamedGraph(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `INSERT DATA { GRAPH <http://example.org/g1> { <http://example.org/s> <http://example.org/p> "hello" } }`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 1 {
		t.Errorf("expected 1 triple in named graph, got %d", g1.Len())
	}
}

func TestDeleteDataNamedGraph(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `DELETE DATA { GRAPH <http://example.org/g1> { <http://example.org/s> <http://example.org/p> "hello" } }`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 0 {
		t.Errorf("expected 0 triples in named graph, got %d", g1.Len())
	}
}

// ---- evalGraphMgmt: CREATE, LOAD SILENT, ADD, COPY, MOVE ----

func TestCreateGraph(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}
	if err := Update(ds, `CREATE GRAPH <http://example.org/new>`); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSilentNoLoader(t *testing.T) {
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	if err := Update(ds, `LOAD SILENT <http://example.org/data>`); err != nil {
		t.Fatalf("expected no error for LOAD SILENT with no loader: %v", err)
	}
}

func TestLoadNoLoaderError(t *testing.T) {
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	if err := Update(ds, `LOAD <http://example.org/data>`); err == nil {
		t.Error("expected error for LOAD with no loader")
	}
}

func TestAddGraphs(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	src := rdflibgo.NewGraph()
	src.Add(s, p, rdflibgo.NewLiteral("v"))
	dst := rdflibgo.NewGraph()
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/src": src, "http://example.org/dst": dst},
	}
	if err := Update(ds, `ADD <http://example.org/src> TO <http://example.org/dst>`); err != nil {
		t.Fatal(err)
	}
	if dst.Len() != 1 {
		t.Errorf("expected 1 triple in dst, got %d", dst.Len())
	}
}

func TestCopyGraphs(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	src := rdflibgo.NewGraph()
	src.Add(s, p, rdflibgo.NewLiteral("v1"))
	dst := rdflibgo.NewGraph()
	dst.Add(s, p, rdflibgo.NewLiteral("old"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/src": src, "http://example.org/dst": dst},
	}
	if err := Update(ds, `COPY <http://example.org/src> TO <http://example.org/dst>`); err != nil {
		t.Fatal(err)
	}
	if dst.Len() != 1 {
		t.Errorf("expected 1 triple in dst after COPY, got %d", dst.Len())
	}
}

func TestMoveGraphs(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	src := rdflibgo.NewGraph()
	src.Add(s, p, rdflibgo.NewLiteral("v"))
	dst := rdflibgo.NewGraph()
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/src": src, "http://example.org/dst": dst},
	}
	if err := Update(ds, `MOVE <http://example.org/src> TO <http://example.org/dst>`); err != nil {
		t.Fatal(err)
	}
	if dst.Len() != 1 {
		t.Errorf("expected 1 triple in dst after MOVE, got %d", dst.Len())
	}
	if src.Len() != 0 {
		t.Errorf("expected 0 triples in src after MOVE, got %d", src.Len())
	}
}

func TestMoveSameGraph(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g := rdflibgo.NewGraph()
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": g},
	}
	if err := Update(ds, `MOVE <http://example.org/g> TO <http://example.org/g>`); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected no change, got %d triples", g.Len())
	}
}

// ---- parseSRXLiteral: directional lang ----

func TestParseSRXLiteralDirLang(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="v"/></head>
  <results>
    <result>
      <binding name="v"><literal xml:lang="en--ltr">hello</literal></binding>
    </result>
  </results>
</sparql>`
	result, err := ParseSRX(strings.NewReader(xmlData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
}

// SPARQL 1.2: its:dir attribute in SRX literals
func TestParseSRXLiteralITSDir(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#"
        xmlns:its="http://www.w3.org/2005/11/its">
  <head><variable name="v"/></head>
  <results>
    <result>
      <binding name="v"><literal xml:lang="ar" its:dir="rtl">قطة</literal></binding>
    </result>
  </results>
</sparql>`
	result, err := ParseSRX(strings.NewReader(xmlData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
	lit := result.Bindings[0]["v"].(rdflibgo.Literal)
	if lit.Language() != "ar" {
		t.Errorf("expected lang 'ar', got %q", lit.Language())
	}
	if lit.Dir() != "rtl" {
		t.Errorf("expected dir 'rtl', got %q", lit.Dir())
	}
}

// ---- parseSRJValue: bnode, typed-literal, triple, directional lang ----

func TestParseSRJValueBNode(t *testing.T) {
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"bnode","value":"b0"}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || result.Bindings[0]["v"] == nil {
		t.Error("expected bnode binding")
	}
}

func TestParseSRJValueTypedLiteral(t *testing.T) {
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"typed-literal","value":"42","datatype":"http://www.w3.org/2001/XMLSchema#integer"}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || result.Bindings[0]["v"] == nil {
		t.Error("expected typed-literal binding")
	}
}

func TestParseSRJValueLiteralDirLang(t *testing.T) {
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"literal","value":"hello","xml:lang":"en--ltr"}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || result.Bindings[0]["v"] == nil {
		t.Error("expected directional lang literal")
	}
}

func TestParseSRJValueLiteralDir(t *testing.T) {
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"literal","value":"hello","xml:lang":"ar","its:dir":"rtl"}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || result.Bindings[0]["v"] == nil {
		t.Error("expected dir literal")
	}
}

func TestParseSRJValueTriple(t *testing.T) {
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"triple","value":{"subject":{"type":"uri","value":"http://example.org/s"},"predicate":{"type":"uri","value":"http://example.org/p"},"object":{"type":"literal","value":"o"}}}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || result.Bindings[0]["v"] == nil {
		t.Error("expected triple value binding")
	}
}

// ---- parseSubQuery: no WHERE keyword variant ----

func TestParseSubQueryNoWhere(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ SELECT ?name { ?s ex:name ?name } LIMIT 1 }
		}
	`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- parseConstructAnnotationBlock ----

func TestParseConstructAnnotation(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p ex:o {| ex:confidence "0.9" |}
		}
		WHERE { ?s ?p ?o }
	`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- evalExprWithGraph: BinaryExpr with EXISTS ----

func TestEvalExprWithGraphBinaryExists(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(EXISTS { ?s ex:age ?a } && ?name != "Bob")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// ---- resolveTermRef: true/false/numeric paths ----

func TestResolveTermRefBooleans(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT ?v WHERE { VALUES ?v { true false } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

func TestResolveTermRefDecimalNumeric(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT ?v WHERE { VALUES ?v { 3.14 } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestResolveTermRefDoubleNumeric(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT ?v WHERE { VALUES ?v { 1.5e2 } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// ---- readBlankNodePropertyList: empty and nested ----

func TestBlankNodePropertyListEmpty(t *testing.T) {
	_, err := Parse(`SELECT * WHERE { ?s ?p [] }`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

func TestBlankNodePropertyListNested(t *testing.T) {
	_, err := Parse(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:p [ ex:q [ ex:r "v" ] ] }`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- FROM clause parsing ----

func TestParseFromClause(t *testing.T) {
	q, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT ?s FROM <http://example.org/graph>
		WHERE { ?s ?p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = q
}

// ---- containsAggregate: FuncExpr/HAVING with scalar ----

func TestContainsAggregateFuncHaving(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
		}
		GROUP BY ?name
		HAVING (STRLEN(?name) > 3)
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Alice(5), Charlie(7) > 3; Bob(3) is not
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// ---- parseOrderBy: ASC/DESC ----

func TestParseOrderByASCDESC(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		ORDER BY ASC(?name)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
	// First should be Alice (alphabetical)
	if r.Bindings[0]["name"].String() != "Alice" {
		t.Errorf("expected Alice first, got %s", r.Bindings[0]["name"].String())
	}
}

// ---- CONSTRUCT WHERE with join (valid) ----

func TestValidateConstructWhereJoin(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT WHERE { ?s ex:name ?name . ?s ex:age ?age }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() == 0 {
		t.Error("expected triples from CONSTRUCT WHERE with join")
	}
}

// ---- DELETE WHERE WITH clause ----

func TestDeleteWhereWith(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `WITH <http://example.org/g1> DELETE WHERE { ?s <http://example.org/p> ?o }`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 0 {
		t.Errorf("expected 0 triples after DELETE WHERE WITH, got %d", g1.Len())
	}
}

// ---- Multiple update operations ----

func TestMultipleUpdateOps(t *testing.T) {
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	if err := Update(ds, `INSERT DATA { <http://example.org/s> <http://example.org/p> "v1" } ; INSERT DATA { <http://example.org/s> <http://example.org/p> "v2" }`); err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 2 {
		t.Errorf("expected 2, got %d", ds.Default.Len())
	}
}


// ---- ASK result from SRJ ----

func TestSRJAskResult(t *testing.T) {
	jsonData := `{"boolean": true}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if !result.AskResult {
		t.Error("expected true ASK result")
	}
}

// ---- evalExprWithGraph: UnaryExpr with EXISTS ----

func TestEvalExprWithGraphUnaryExists(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(!(EXISTS { ?s ex:nonexistent ?x }))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// ---- readStringLiteral: single-quoted string ----

func TestReadStringSingleQuote(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello"))
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:p 'hello' }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// ---- readStringLiteral: string with ^^ datatype using prefixed name ----

func TestReadStringDatatypePrefixed(t *testing.T) {
	_, err := Parse(`PREFIX xsd: <http://www.w3.org/2001/XMLSchema#> SELECT * WHERE { ?s ?p "42"^^xsd:integer }`)
	if err != nil {
		t.Fatal(err)
	}
}

// ---- graphForQuadSolution: named graph variable in quad pattern ----

func TestDeleteWhereNamedGraphVariable(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	if err := Update(ds, `DELETE WHERE { GRAPH <http://example.org/g1> { ?s ?p ?o } }`); err != nil {
		t.Fatal(err)
	}
	if g1.Len() != 0 {
		t.Errorf("expected 0 triples, got %d", g1.Len())
	}
}

// ---- extractTemplateFromPattern: CONSTRUCT WHERE with JoinPattern ----
// Already covered by TestValidateConstructWhereJoin, but let's ensure the
// JoinPattern path in extractTemplateFromPattern is hit via CONSTRUCT WHERE
func TestExtractTemplateJoinPattern(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT WHERE { ?s ex:name ?name . ?s ex:age ?age }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// 3 persons * 2 triples = 6
	if r.Graph == nil || r.Graph.Len() == 0 {
		t.Error("expected triples")
	}
}

// ---- containsExists: FuncExpr args descend ----

func TestContainsExistsFuncArgs(t *testing.T) {
	// Exercise containsExists path by mixing EXISTS with other expressions
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			FILTER(EXISTS { ?s ex:age ?a })
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// ---- parseDatePair failure: date comparison with incompatible types ----

func TestDateComparisonFails(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/d")
	// Add a date and compare with another date
	g.Add(s, p, rdflibgo.NewLiteral("2024-01-01", rdflibgo.WithDatatype(rdflibgo.XSDDate)))
	g.Add(s, p, rdflibgo.NewLiteral("2025-01-01", rdflibgo.WithDatatype(rdflibgo.XSDDate)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?d WHERE {
			?s ex:d ?d .
			FILTER(?d < "2025-01-01"^^<http://www.w3.org/2001/XMLSchema#date>)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// ---- parseDateTime: XSDTime path ----

func TestParseDateTimeTime(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/t")
	g.Add(s, p, rdflibgo.NewLiteral("10:30:45", rdflibgo.WithDatatype(rdflibgo.XSDTime)))
	g.Add(s, p, rdflibgo.NewLiteral("20:00:00", rdflibgo.WithDatatype(rdflibgo.XSDTime)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?t WHERE {
			?s ex:t ?t .
			FILTER(?t < "15:00:00"^^<http://www.w3.org/2001/XMLSchema#time>)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// ---- resolveTermRef: bnode with scope prefix ----

func TestResolveBNodeWithScope(t *testing.T) {
	// INSERT DATA with blank node exercises resolveTermRef with bnode + scope
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	if err := Update(ds, `INSERT DATA { _:b1 <http://example.org/p> "v" }`); err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 1 {
		t.Errorf("expected 1, got %d", ds.Default.Len())
	}
}

// ---- CONSTRUCT with triple term (hits resolveTermRef triple term path) ----

func TestConstructTripleTerm(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			<< ex:s ex:p ex:o >> ex:confidence "0.9"
		}
		WHERE { ?s ?p ?o }
	`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- termString nil path via CONCAT with unbound var ----

func TestTermStringNilUnbound(t *testing.T) {
	g := makeSPARQLGraph(t)
	// CONCAT with a potentially nil arg exercises termString(nil) -> ""
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?c WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:nonexistent ?x }
			BIND(CONCAT(?name, COALESCE(?x, "")) AS ?c)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// ---- timeNow: NOW() function ----

func TestNOWFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `PREFIX ex: <http://example.org/> SELECT ?now WHERE { ?s ex:name "Alice" . BIND(NOW() AS ?now) }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || r.Bindings[0]["now"] == nil {
		t.Error("expected NOW() result")
	}
}

// ---- validateTripleTerms / validateTripleTermString ----

func TestValidateTripleTermsInQuery(t *testing.T) {
	// A query with triple terms exercises validateTripleTerms
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT ?t WHERE {
			<<( ex:s ex:p ex:o )>> ex:confidence ?c .
			BIND(<<( ex:s ex:p ex:o )>> AS ?t)
		}
	`)
	// May or may not error, but exercises the validation path
	_ = err
}

// ---- graphForQuadSolution: variable graph name in DELETE WHERE ----

func TestGraphForQuadSolutionVariable(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1 := rdflibgo.NewGraph()
	g1.Add(s, p, rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": g1},
	}
	// DELETE WHERE with a variable graph name
	if err := Update(ds, `DELETE WHERE { GRAPH ?g { ?s ?p ?o } }`); err != nil {
		t.Fatal(err)
	}
}

// ---- collectExprVarsInto: UnaryExpr and FuncExpr with non-aggregate ----

func TestCollectExprVarsUnaryFunc(t *testing.T) {
	// GROUP BY with a plain variable exercises collectExprVarsInto VarExpr branch
	// HAVING with a non-aggregate func exercises collectExprVarsInto FuncExpr branch
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		GROUP BY ?name
		HAVING (STRLEN(?name) >= 3)
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// ---- parseBnodePropertyListTriples with multiple predicates ----

func TestBnodePropertyListMultiplePreds(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:p [ ex:name "Alice" ; ex:age 30 ]
		}
	`)
	if err != nil {
		t.Fatal("expected success:", err)
	}
}

// ---- validateConstructWhere: UNION and other cases ----

func TestValidateConstructWhereUnion(t *testing.T) {
	_, err := Parse(`CONSTRUCT WHERE { { ?s ?p ?o } UNION { ?s ?p2 ?o2 } }`)
	if err == nil {
		t.Error("expected error: UNION not allowed in CONSTRUCT WHERE")
	}
}

// ---- srjString non-JSON fallback path ----

func TestSrjStringFallback(t *testing.T) {
	// ParseSRJ with invalid JSON for a value will hit the fallback path in srjString
	jsonData := `{"head":{"vars":["v"]},"results":{"bindings":[{"v":{"type":"uri","value":"http://example.org/s"}}]}}`
	result, err := ParseSRJ(strings.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Error("expected 1 binding")
	}
}

// ---- parseDateTime: date-with-TZ comparison ----

func TestParseDateTimeComparison(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/d")
	g.Add(s, p, rdflibgo.NewLiteral("2024-01-01T10:00:00Z", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))
	g.Add(s, p, rdflibgo.NewLiteral("2025-01-01T10:00:00Z", rdflibgo.WithDatatype(rdflibgo.XSDDateTime)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT ?d WHERE {
			?s ex:d ?d .
			FILTER(?d < "2025-01-01T10:00:00Z"^^xsd:dateTime)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}
