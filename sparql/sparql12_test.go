package sparql_test

import (
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

func TestTripleTermSpecialChars(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(
		rdflibgo.NewURIRefUnsafe("http://ex/s"),
		rdflibgo.NewURIRefUnsafe("http://ex/p"),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral("hello \"world\" \n\t", rdflibgo.WithDatatype(rdflibgo.XSDString)),
		),
	)
	r, err := sparql.Query(g, `SELECT ?o WHERE { <http://ex/s> <http://ex/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	tt, ok := r.Bindings[0]["o"].(rdflibgo.TripleTerm)
	if !ok {
		t.Fatal("expected TripleTerm")
	}
	lit := tt.Object().(rdflibgo.Literal)
	if lit.Lexical() != "hello \"world\" \n\t" {
		t.Errorf("expected special chars preserved, got %q", lit.Lexical())
	}
}

func TestTripleTermVariableMatching(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(
		rdflibgo.NewURIRefUnsafe("http://ex/s"),
		rdflibgo.NewURIRefUnsafe("http://ex/p"),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)),
		),
	)
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?s ?val WHERE {
			?s :p <<( :a :b ?val )>>
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	b := r.Bindings[0]
	if b["s"].(rdflibgo.URIRef).Value() != "http://ex/s" {
		t.Error("subject not bound correctly")
	}
	if b["val"].(rdflibgo.Literal).Lexical() != "42" {
		t.Error("inner variable not bound correctly")
	}
}

func TestNestedTripleTermVariable(t *testing.T) {
	g := rdflibgo.NewGraph()
	inner := rdflibgo.NewTripleTerm(
		rdflibgo.NewURIRefUnsafe("http://ex/x"),
		rdflibgo.NewURIRefUnsafe("http://ex/y"),
		rdflibgo.NewLiteral("z"),
	)
	outer := rdflibgo.NewTripleTerm(
		rdflibgo.NewURIRefUnsafe("http://ex/a"),
		rdflibgo.NewURIRefUnsafe("http://ex/b"),
		inner,
	)
	g.Add(rdflibgo.NewURIRefUnsafe("http://ex/s"), rdflibgo.NewURIRefUnsafe("http://ex/has"), outer)
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?innerObj WHERE {
			:s :has <<( :a :b <<( :x :y ?innerObj )>> )>>
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	val := r.Bindings[0]["innerObj"]
	if val == nil {
		t.Fatal("innerObj is nil — nested triple term variable not bound")
	}
	if val.(rdflibgo.Literal).Lexical() != "z" {
		t.Errorf("expected 'z', got %q", val.(rdflibgo.Literal).Lexical())
	}
}

func TestAnnotationQuery(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://ex/s")
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	o := rdflibgo.NewURIRefUnsafe("http://ex/o")
	reifier := rdflibgo.NewBNode("r1")
	rdfReifies := rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies")
	g.Add(s, p, o)
	g.Add(reifier, rdfReifies, rdflibgo.NewTripleTerm(s, p, o))
	g.Add(reifier, rdflibgo.NewURIRefUnsafe("http://ex/source"), rdflibgo.NewURIRefUnsafe("http://ex/web"))

	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?src WHERE {
			:s :p :o {| :source ?src |}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["src"].(rdflibgo.URIRef).Value() != "http://ex/web" {
		t.Error("annotation source not matched")
	}
}

func TestTripleTermFunctions(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(
		rdflibgo.NewURIRefUnsafe("http://ex/s"),
		rdflibgo.NewURIRefUnsafe("http://ex/p"),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)),
		),
	)
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?isT ?subj ?pred ?obj WHERE {
			?s :p ?tt .
			BIND(isTriple(?tt) AS ?isT)
			BIND(SUBJECT(?tt) AS ?subj)
			BIND(PREDICATE(?tt) AS ?pred)
			BIND(OBJECT(?tt) AS ?obj)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	b := r.Bindings[0]
	if b["isT"].(rdflibgo.Literal).Lexical() != "true" {
		t.Error("isTriple should be true")
	}
	if b["subj"].(rdflibgo.URIRef).Value() != "http://ex/a" {
		t.Error("SUBJECT wrong")
	}
	if b["pred"].(rdflibgo.URIRef).Value() != "http://ex/b" {
		t.Error("PREDICATE wrong")
	}
	if b["obj"].(rdflibgo.Literal).Lexical() != "42" {
		t.Error("OBJECT wrong")
	}
}

func TestTripleTermInVALUES(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(
		rdflibgo.NewURIRefUnsafe("http://ex/s"),
		rdflibgo.NewURIRefUnsafe("http://ex/p"),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)),
		),
	)
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?s WHERE {
			VALUES ?tt { <<( :a :b 42 )>> <<( :a :b 99 )>> }
			?s :p ?tt
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
}

func TestDirectionalLangTags(t *testing.T) {
	r, err := sparql.Query(rdflibgo.NewGraph(), `
		SELECT
			(LANGDIR("hello"@en--ltr) AS ?dir)
			(hasLANGDIR("hello"@en--ltr) AS ?has)
			(STRLANGDIR("abc", "ar", "rtl") AS ?built)
		WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	b := r.Bindings[0]
	if b["dir"].(rdflibgo.Literal).Lexical() != "ltr" {
		t.Error("LANGDIR wrong")
	}
	if b["has"].(rdflibgo.Literal).Lexical() != "true" {
		t.Error("hasLANGDIR wrong")
	}
	lit := b["built"].(rdflibgo.Literal)
	if lit.Language() != "ar" || lit.Dir() != "rtl" {
		t.Errorf("STRLANGDIR wrong: lang=%q dir=%q", lit.Language(), lit.Dir())
	}
}

func TestTripleTermCONSTRUCT(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(
		rdflibgo.NewURIRefUnsafe("http://ex/s"),
		rdflibgo.NewURIRefUnsafe("http://ex/p"),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)),
		),
	)
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		CONSTRUCT {
			?s :annotated <<( ?s :p ?tt )>>
		} WHERE {
			?s :p ?tt
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("CONSTRUCT graph is nil")
	}
	count := 0
	for range r.Graph.Triples(nil, nil, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 triple, got %d", count)
	}
}

func TestUpdateWithReifiedTriples(t *testing.T) {
	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://ex/g1": func() *rdflibgo.Graph {
				g := rdflibgo.NewGraph()
				g.Add(rdflibgo.NewURIRefUnsafe("http://ex/a"), rdflibgo.NewURIRefUnsafe("http://ex/b"), rdflibgo.NewURIRefUnsafe("http://ex/c"))
				return g
			}(),
		},
	}
	err := sparql.Update(ds, `
		PREFIX : <http://ex/>
		INSERT { << ?s ?p ?o >> :from :g1 }
		WHERE { GRAPH :g1 { ?s ?p ?o } }
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ds.Default.Triples(nil, nil, nil) {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 triples (reifier + rdf:reifies), got %d", count)
	}
}

func TestVersionDirective(t *testing.T) {
	_, err := sparql.Parse(`VERSION "1.2" SELECT * WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Errorf("VERSION directive should be accepted: %v", err)
	}
	_, err = sparql.Parse(`VERSION """1.2""" SELECT * WHERE { ?s ?p ?o }`)
	if err == nil {
		t.Error("triple-quoted VERSION should be rejected")
	}
}

func TestNegativeSyntax(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"literal subject in triple term expr", `SELECT * WHERE { BIND(<<( "lit" <http://p> <http://o> )>> AS ?t) }`},
		{"triple term subject in triple term expr", `SELECT * WHERE { BIND(<<( <<(<http://s> <http://p> <http://o>)>> <http://q> <http://z> )>> AS ?t) }`},
		{"reified triple in BIND", `SELECT * WHERE { ?s ?p ?o . BIND(<< ?s ?p ?o >> AS ?t) }`},
		{"nested aggregates", `SELECT (COUNT(COUNT(*)) AS ?c) WHERE {}`},
		{"duplicate VALUES vars", `SELECT * WHERE { VALUES (?a ?a) { (1 1) } }`},
		{"invalid lang direction", `SELECT ("foo"@en--foo AS ?v) WHERE {}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sparql.Parse(tc.query)
			if err == nil {
				t.Errorf("expected parse error for: %s", tc.query)
			}
		})
	}
}

func TestEmptyGraphTripleTermQuery(t *testing.T) {
	r, err := sparql.Query(rdflibgo.NewGraph(), `SELECT ?s WHERE { <<( ?s ?p ?o )>> ?q ?z }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 0 {
		t.Errorf("expected 0 results from empty graph, got %d", len(r.Bindings))
	}
}

// --- Section 2: Property-Based Tests for Triple Term Functions ---

func TestTripleTermRoundTrip(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://ex/s")
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	terms := []rdflibgo.TripleTerm{
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewLiteral("hello"),
		),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/c"),
			rdflibgo.NewURIRefUnsafe("http://ex/d"),
			rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)),
		),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/e"),
			rdflibgo.NewURIRefUnsafe("http://ex/f"),
			rdflibgo.NewTripleTerm(
				rdflibgo.NewURIRefUnsafe("http://ex/g"),
				rdflibgo.NewURIRefUnsafe("http://ex/h"),
				rdflibgo.NewLiteral("nested"),
			),
		),
	}
	for _, tt := range terms {
		g.Add(s, p, tt)
	}
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?t ?rebuilt WHERE {
			:s :p ?t .
			FILTER(isTriple(?t))
			BIND(TRIPLE(SUBJECT(?t), PREDICATE(?t), OBJECT(?t)) AS ?rebuilt)
			FILTER(?t = ?rebuilt)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != len(terms) {
		t.Errorf("expected %d round-trip matches, got %d", len(terms), len(r.Bindings))
	}
}

func TestIsTripleNegative(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://ex/s")
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://ex/a"))
	g.Add(s, p, rdflibgo.NewLiteral("hello"))
	g.Add(s, p, rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?v WHERE {
			:s :p ?v .
			FILTER(isTriple(?v))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 0 {
		t.Errorf("isTriple should be false for non-triple-terms, got %d results", len(r.Bindings))
	}
}

func TestAccessorsOnNonTripleTerm(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://ex/s")
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://ex/a"))
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT ?subj WHERE {
			:s :p ?v .
			BIND(SUBJECT(?v) AS ?subj)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1 binding")
	}
	if r.Bindings[0]["subj"] != nil {
		t.Error("SUBJECT on URI should return nil")
	}
}

// --- Section 4: Large Graph Stress Tests ---

func TestLargeGraphTripleTerms(t *testing.T) {
	g := rdflibgo.NewGraph()
	for i := 0; i < 5000; i++ {
		g.Add(
			rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://ex/s%d", i)),
			rdflibgo.NewURIRefUnsafe("http://ex/p"),
			rdflibgo.NewTripleTerm(
				rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://ex/a%d", i%100)),
				rdflibgo.NewURIRefUnsafe("http://ex/b"),
				rdflibgo.NewLiteral(i),
			),
		)
	}
	r, err := sparql.Query(g, `
		PREFIX : <http://ex/>
		SELECT (COUNT(*) AS ?c) WHERE {
			?s :p <<( :a0 :b ?val )>>
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := r.Bindings[0]["c"].(rdflibgo.Literal).Lexical()
	if count != "50" {
		t.Errorf("expected 50, got %s", count)
	}
}

func TestDeeplyNestedTripleTerms(t *testing.T) {
	var term rdflibgo.Term = rdflibgo.NewLiteral("leaf")
	for i := 0; i < 10; i++ {
		term = rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://ex/s%d", i)),
			rdflibgo.NewURIRefUnsafe("http://ex/p"),
			term,
		)
	}
	g := rdflibgo.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://ex/root"), rdflibgo.NewURIRefUnsafe("http://ex/has"), term)

	r, err := sparql.Query(g, `SELECT ?o WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1 result")
	}
	n3 := r.Bindings[0]["o"].N3()
	if !strings.HasPrefix(n3, "<<( ") {
		t.Error("not a triple term")
	}
	if strings.Count(n3, "<<( ") != 10 {
		t.Errorf("expected 10 nesting levels, got %d", strings.Count(n3, "<<( "))
	}
}

// --- Section 5: N3/Serialization Round-Trip Tests ---

func TestTripleTermN3RoundTrip(t *testing.T) {
	cases := []rdflibgo.TripleTerm{
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/s"),
			rdflibgo.NewURIRefUnsafe("http://ex/p"),
			rdflibgo.NewLiteral("hello \"world\""),
		),
		rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe("http://ex/a"),
			rdflibgo.NewURIRefUnsafe("http://ex/b"),
			rdflibgo.NewTripleTerm(
				rdflibgo.NewURIRefUnsafe("http://ex/nested"),
				rdflibgo.NewURIRefUnsafe("http://ex/q"),
				rdflibgo.NewLiteral("3.14", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)),
			),
		),
	}
	for i, tt := range cases {
		n3 := tt.N3()
		g := rdflibgo.NewGraph()
		err := turtle.Parse(g, strings.NewReader(
			fmt.Sprintf("<http://ex/s> <http://ex/p> %s .", n3),
		))
		if err != nil {
			t.Fatalf("case %d: turtle parse failed: %v", i, err)
		}
		found := false
		for tr := range g.Triples(nil, nil, nil) {
			reparsed, ok := tr.Object.(rdflibgo.TripleTerm)
			if !ok {
				t.Fatalf("case %d: not a triple term after reparse: %T", i, tr.Object)
			}
			if !tt.Equal(reparsed) {
				t.Errorf("case %d: round-trip failed:\n  original: %s\n  reparsed: %s", i, tt.N3(), reparsed.N3())
			}
			found = true
		}
		if !found {
			t.Fatalf("case %d: no triples found after reparse", i)
		}
	}
}

// --- Section 6: ResultsEqual with Triple Terms ---

func TestResultsEqualWithTripleTermBnodes(t *testing.T) {
	a := &sparql.Result{
		Type: "SELECT",
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{{
			"x": rdflibgo.NewTripleTerm(
				rdflibgo.NewBNode("a1"),
				rdflibgo.NewURIRefUnsafe("http://ex/p"),
				rdflibgo.NewURIRefUnsafe("http://ex/o"),
			),
		}},
	}
	b := &sparql.Result{
		Type: "SELECT",
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{{
			"x": rdflibgo.NewTripleTerm(
				rdflibgo.NewBNode("different_label"),
				rdflibgo.NewURIRefUnsafe("http://ex/p"),
				rdflibgo.NewURIRefUnsafe("http://ex/o"),
			),
		}},
	}
	if !sparql.ResultsEqual(a, b) {
		t.Error("results with different bnode labels in triple terms should be equal")
	}
}

// --- Section 7: SRJ/SRX Triple Term Parsing Edge Cases ---

func TestSRJMalformedTriple(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"missing predicate", `{"head":{"vars":["x"]},"results":{"bindings":[{"x":{"type":"triple","value":{"subject":{"type":"uri","value":"s"}}}}]}}`},
		{"null value", `{"head":{"vars":["x"]},"results":{"bindings":[{"x":{"type":"triple","value":null}}]}}`},
		{"empty object", `{"head":{"vars":["x"]},"results":{"bindings":[{"x":{"type":"triple","value":{}}}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := sparql.ParseSRJ(strings.NewReader(tc.input))
			if err != nil {
				return // parse error is acceptable
			}
			// If parsed, the binding should be nil (graceful degradation)
			if len(r.Bindings) > 0 && r.Bindings[0]["x"] != nil {
				t.Errorf("expected nil for malformed triple, got %v", r.Bindings[0]["x"])
			}
		})
	}
}
