package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Exercise all interface marker methods so coverage counts them.

func TestPatternTypeMarkers(t *testing.T) {
	patterns := []Pattern{
		&BGP{},
		&JoinPattern{},
		&OptionalPattern{},
		&UnionPattern{},
		&FilterPattern{},
		&BindPattern{},
		&ValuesPattern{},
		&GraphPattern{},
		&MinusPattern{},
		&SubqueryPattern{},
	}
	for _, p := range patterns {
		if p.patternType() == "" {
			t.Error("empty pattern type")
		}
	}
}

func TestExprTypeMarkers(t *testing.T) {
	exprs := []Expr{
		&VarExpr{Name: "x"},
		&LiteralExpr{Value: rdflibgo.NewLiteral("v")},
		&IRIExpr{Value: "http://example.org/"},
		&BinaryExpr{Op: "+"},
		&UnaryExpr{Op: "!"},
		&FuncExpr{Name: "STRLEN"},
		&ExistsExpr{},
	}
	for _, e := range exprs {
		if e.exprType() == "" {
			t.Error("empty expr type")
		}
	}
}

func TestUpdateOpMarkers(t *testing.T) {
	ops := []UpdateOperation{
		&InsertDataOp{},
		&DeleteDataOp{},
		&DeleteWhereOp{},
		&ModifyOp{},
		&GraphMgmtOp{},
	}
	for _, o := range ops {
		o.updateOp()
	}
}

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
