package shacl

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

// srlRun parses rules, evaluates them over the Turtle data and returns the
// inferred triples as sorted "s p o" strings (blank nodes as _:).
func srlRun(t *testing.T, rules, data string) []string {
	t.Helper()
	rs, err := ParseSRL(rules)
	if err != nil {
		t.Fatalf("ParseSRL: %v", err)
	}
	g, err := LoadTurtleString(data, "")
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	out, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatalf("EvalRuleSet: %v", err)
	}
	return srlLines(out)
}

func srlLines(g *Graph) []string {
	var lines []string
	for _, tr := range g.Triples() {
		lines = append(lines, srlShort(tr.Subject)+" "+srlShort(tr.Predicate)+" "+srlShort(tr.Object))
	}
	sort.Strings(lines)
	return lines
}

func srlShort(t Term) string {
	switch {
	case t.IsBlank():
		return "_:"
	case t.IsIRI():
		return ":" + strings.TrimPrefix(t.Value(), "http://example/")
	case t.IsLiteral():
		s := `"` + t.Value() + `"`
		switch {
		case t.Language() != "":
			s += "@" + t.Language()
		case t.Datatype() != "" && t.Datatype() != XSD+"string":
			s += "^^" + strings.TrimPrefix(t.Datatype(), XSD)
		}
		return s
	}
	return "?"
}

func srlWant(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("inferred\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

const srlP = "PREFIX : <http://example/>\nPREFIX xsd: <http://www.w3.org/2001/XMLSchema#>\n"
const srlD = "@prefix : <http://example/> .\n"

func TestSRLGroupCountsPerKey(t *testing.T) {
	// The example of "Grouping and Aggregation": a component with no
	// vulnerability has no group, so no triple is inferred for it.
	got := srlRun(t, srlP+`
RULE { ?x :n ?n }
WHERE { ?x a :Component . GROUP BY ?x { ?x :exposedTo ?v } [ ?n := COUNT(*) ] }`,
		srlD+`:frontend a :Component ; :exposedTo :v1, :v2 . :db a :Component ; :exposedTo :v1 . :logger a :Component .`)
	srlWant(t, got, ":frontend :n \"2\"^^integer", ":db :n \"1\"^^integer")
}

func TestSRLGroupWithoutKeysAggregatesEverything(t *testing.T) {
	got := srlRun(t, srlP+`
RULE { :stats :count ?n ; :sum ?s ; :min ?lo ; :max ?hi ; :distinct ?d }
WHERE { GROUP { ?x :v ?v } [ ?n := COUNT(*), ?s := SUM(?v), ?lo := MIN(?v), ?hi := MAX(?v), ?d := COUNT(DISTINCT ?v) ] }`,
		srlD+`:a :v 1 . :b :v 2 . :c :v 2 . :d :v 5 .`)
	srlWant(t, got,
		`:stats :count "4"^^integer`, `:stats :sum "10"^^integer`, `:stats :min "1"^^integer`,
		`:stats :max "5"^^integer`, `:stats :distinct "3"^^integer`)
}

func TestSRLGroupOverNothingCountsZero(t *testing.T) {
	// Without BY there is exactly one group, even over no solutions.
	got := srlRun(t, srlP+`RULE { :stats :count ?n } WHERE { GROUP { ?x :v ?v } [ ?n := COUNT(*) ] }`, srlD+`:a :w 1 .`)
	srlWant(t, got, `:stats :count "0"^^integer`)
}

func TestSRLGroupAggregateErrorDropsGroup(t *testing.T) {
	// SUM over a non-number is an error, and the group is not added: the
	// rule must not fire with ?s unbound.
	got := srlRun(t, srlP+`RULE { ?k :sum ?s } WHERE { GROUP BY ?k { ?k :v ?v } [ ?s := SUM(?v) ] }`,
		srlD+`:good :v 1, 2 . :bad :v "x" .`)
	srlWant(t, got, `:good :sum "3"^^integer`)
}

func TestSRLGroupPatternSeesOnlyKeysAndSet(t *testing.T) {
	got := srlRun(t, srlP+`
RULE { ?s :total ?t }
WHERE { ?s a :Order . GROUP BY ?s { ?s :line ?l . ?l :price ?p . SET ( ?q := ?p * 2 ) } [ ?t := SUM(?q) ] }`,
		srlD+`:o1 a :Order ; :line :l1, :l2 . :l1 :price 10 . :l2 :price 5 .`)
	srlWant(t, got, `:o1 :total "30"^^integer`)
}

func TestSRLSetErrorRejectsSolution(t *testing.T) {
	// An error in an assignment rejects the solution; it must not survive
	// with the variable unbound and let a later pattern bind it freely.
	got := srlRun(t, srlP+`
RULE { ?x :inv ?z } WHERE { ?x :n ?o . SET ( ?z := 1/?o ) ?y :seen ?z }`,
		srlD+`:a :n 0 . :b :n 4 . :c :seen 0.25 . :d :seen 7 .`)
	srlWant(t, got, ":b :inv \"0.25\"^^decimal")
}

func TestSRLFilterIsARealExpression(t *testing.T) {
	// Not a string comparison: numeric promotion, string functions, language.
	got := srlRun(t, srlP+`
RULE { ?s :ok ?v } WHERE { ?s :v ?v FILTER ( ?v > 2.5 && STRLEN(STR(?s)) < 20 ) }
RULE { ?s :en ?l } WHERE { ?s :label ?l FILTER langMatches(lang(?l), "en") }
RULE { ?s :iri ?o } WHERE { ?s :ref ?o FILTER isIRI(?o) }
`,
		srlD+`:a :v 3 . :b :v 2 . :c :v 2.6e0 . :d :v "10" .
:a :label "x"@en-GB . :b :label "y"@fr . :c :label "z" .
:a :ref :b . :b :ref "lit" .`)
	srlWant(t, got, `:a :ok "3"^^integer`, `:c :ok "2.6e0"^^double`, `:a :en "x"@en-gb`, `:a :iri :b`)
}

func TestSRLFilterErrorIsFalse(t *testing.T) {
	got := srlRun(t, srlP+`RULE { ?x :out ?o } WHERE { ?x :data ?o . FILTER( 1/?o ) }`, srlD+`:x1 :data 0 . :x2 :data 2 .`)
	srlWant(t, got, `:x2 :out "2"^^integer`)
}

func TestSRLPropertyPaths(t *testing.T) {
	got := srlRun(t, srlP+`
RULE { ?x :gp ?o } WHERE { ?x :parent/:parent ?o }
RULE { ?x :child ?y } WHERE { ?x ^:parent ?y }
RULE { ?x :uncle ?u } WHERE { ?x :parent/^:child/:brother ?u }`,
		srlD+`:c :parent :b . :b :parent :a . :a :child :b . :a :brother :u .`)
	srlWant(t, got, `:c :gp :a`, `:b :child :c`, `:c :uncle :u`)
}

func TestSRLWhereDataMatchesOnlyTheBaseGraph(t *testing.T) {
	// :a :p :o is inferred by the first rule, not in the input: WHERE DATA
	// does not see it.
	got := srlRun(t, srlP+`
RULE { :a :p :o } WHERE { }
RULE { :seen :base ?o } WHERE DATA { ?s :p ?o }`, srlD+`:x :p :y .`)
	srlWant(t, got, `:a :p :o`, `:seen :base :y`)
}

func TestSRLNotDataMatchesOnlyTheBaseGraph(t *testing.T) {
	got := srlRun(t, srlP+`
RULE { :a :p :o } WHERE { }
RULE { :msg :text "inferred" } WHERE { :a :p ?o NOT DATA { :a :p ?o } }
RULE { :msg :text "never" } WHERE { :a :p ?o NOT { :a :p ?o } }`, srlD)
	srlWant(t, got, `:a :p :o`, `:msg :text "inferred"`)
}

func TestSRLDataBlocksAreNotBaseGraph(t *testing.T) {
	got := srlRun(t, srlP+`DATA { :s :p :o } RULE { :m :t "x" } WHERE DATA { :s :p :o }`, srlD)
	srlWant(t, got, `:s :p :o`)
}

func TestSRLHeadBlankNodesAreFreshPerSolution(t *testing.T) {
	got := srlRun(t, srlP+`RULE { [] :about ?x ; :kind :K } WHERE { ?x a :T }`, srlD+`:a a :T . :b a :T .`)
	srlWant(t, got, `_: :about :a`, `_: :about :b`, `_: :kind :K`, `_: :kind :K`)
}

func TestSRLBodyBlankNodeIsAVariable(t *testing.T) {
	got := srlRun(t, srlP+`RULE { ?s :hasP :yes } WHERE { ?s :p [] . ?s :q _:b . _:b :r ?z }`,
		srlD+`:a :p :x ; :q :n . :n :r 1 . :b :p :y ; :q :m .`)
	srlWant(t, got, `:a :hasP :yes`)
}

func TestSRLBlankNodeInFilterPositionMatchesNothingExtra(t *testing.T) {
	got := srlRun(t, srlP+`RULE { :r :found ?x } WHERE { ?x :p ?y FILTER ( ?y != :skip ) NOT { ?x :q [] } }`,
		srlD+`:a :p :b . :c :p :skip . :d :p :e ; :q :f .`)
	srlWant(t, got, `:r :found :a`)
}

func TestSRLLegacyBindStillWorks(t *testing.T) {
	got := srlRun(t, srlP+`RULE { ?s :d ?d } WHERE { ?s :v ?v . BIND ( ?v * 2 AS ?d ) }`, srlD+`:a :v 4 .`)
	srlWant(t, got, `:a :d "8"^^integer`)
}

func TestSRLReifierAnnotationAndTripleTerms(t *testing.T) {
	got := srlRun(t, srlP+`
RULE { ?s :p ?o {| :src ?w |} } WHERE { ?s :claim ?o . ?s :wiki ?w }
RULE { ?r :flag :yes } WHERE { ?r <http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies> <<( ?s :p ?o )>> }`,
		srlD+`:a :claim :b ; :wiki :w .`)
	// The reifier is a blank node: one rdf:reifies triple, the asserted
	// triple, and the annotation.
	if len(got) != 4 {
		t.Fatalf("got %v", got)
	}
}

func TestSRLDoesNotModifyTheBaseGraph(t *testing.T) {
	rs, _ := ParseSRL(srlP + `RULE { :x :y :z } WHERE { } DATA { :d :e :f }`)
	g, _ := LoadTurtleString(srlD+`:a :b :c .`, "")
	if _, err := EvalRuleSet(rs, g); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("the input graph has %d triples after evaluation, want 1", g.Len())
	}
}

func TestSRLInferenceGraphExcludesBaseTriples(t *testing.T) {
	got := srlRun(t, srlP+`RULE { ?s :q ?o } WHERE { ?s :p ?o }`, srlD+`:a :p :b . :a :q :b .`)
	srlWant(t, got)
}

func TestSRLTransitiveClosureReachesAFixpoint(t *testing.T) {
	got := srlRun(t, srlP+`RULE { ?a :reach ?c } WHERE { ?a :next ?c }
RULE { ?a :reach ?c } WHERE { ?a :reach ?b . ?b :next ?c }`, srlD+`:1 :next :2 . :2 :next :3 . :3 :next :4 . :4 :next :1 .`)
	if len(got) != 16 {
		t.Errorf("%d reach triples, want 16", len(got))
	}
}

func TestSRLEvalRejectsImportsAndIllFormedRules(t *testing.T) {
	g, _ := LoadTurtleString(srlD, "")
	rs, err := ParseSRL(srlP + `IMPORTS <http://example/lib> RULE { } WHERE { }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvalRuleSet(rs, g); !errors.Is(err, ErrSRLImports) {
		t.Errorf("imports: %v", err)
	}
	rs, _ = ParseSRL(srlP + `RULE { ?s :p ?o } WHERE { ?s :q ?z }`)
	if _, err := EvalRuleSet(rs, g); !errors.Is(err, ErrSRLNotWellFormed) {
		t.Errorf("ill-formed: %v", err)
	}
	rs, _ = ParseSRL(srlP + `RULE { :s :p :o } WHERE { NOT { :s :p :o } }`)
	if _, err := EvalRuleSet(rs, g); !errors.Is(err, ErrSRLNotStratifiable) {
		t.Errorf("unstratifiable: %v", err)
	}
}

func TestSRLCancelledContext(t *testing.T) {
	rs, _ := ParseSRL(srlP + `RULE { :s :p :o } WHERE { }`)
	g, _ := LoadTurtleString(srlD, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := EvalRuleSetContext(ctx, rs, g); err == nil || out != nil {
		t.Errorf("done context: %v, %v", out, err)
	}
}

func TestSRLSyntaxErrors(t *testing.T) {
	for name, in := range map[string]string{
		"BOUND is not a SPARQL-RL function":  srlP + `RULE { } WHERE { ?s :p ?o FILTER ( BOUND(?o) ) }`,
		"EXISTS is not part of expressions":  srlP + `RULE { } WHERE { ?s :p ?o FILTER ( EXISTS { ?o :q ?z } ) }`,
		"aggregates only in GROUP":           srlP + `RULE { } WHERE { ?s :p ?o FILTER ( COUNT(?o) > 1 ) }`,
		"undeclared prefix in an expression": `RULE { } WHERE { ?s ?p ?o FILTER ( ?o = ex:y ) }`,
		"unbalanced expression":              srlP + `RULE { } WHERE { ?s ?p ?o FILTER ( ?o = 1 }`,
		"garbled expression":                 srlP + `RULE { } WHERE { ?s ?p ?o SET ( ?x := 1 + ) }`,
		"nested group":                       srlP + `RULE { } WHERE { GROUP { GROUP { ?s ?p ?o } [] } [] }`,
		"NOT inside GROUP":                   srlP + `RULE { } WHERE { GROUP { NOT { ?s ?p ?o } } [] }`,
		"SET inside NOT":                     srlP + `RULE { } WHERE { NOT { SET ( ?x := 1 ) } }`,
		"path in a head":                     srlP + `RULE { ?s :p/:q ?o } WHERE { ?s ?p ?o }`,
		"missing dot between triples":        srlP + `RULE { } WHERE { ?s ?p ?o ?a ?b ?c }`,
	} {
		_, err := ParseSRL(in)
		if !errors.Is(err, ErrSRLSyntax) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	_, err := ParseSRL(srlP + "RULE { } WHERE {\n  ?s ?p ?o FILTER ( BOUND(?o) )\n}")
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Errorf("a syntax error carries the position of the offending token: %v", err)
	}
}

func TestSRLExpressionsUseThePrefixesInForceAtTheRule(t *testing.T) {
	// The same prefix redeclared between two rules means different IRIs.
	got := srlRun(t, `PREFIX : <http://example/>
PREFIX e: <http://example/one/>
RULE { :s :first ?x } WHERE { ?x :p ?v FILTER ( ?x = e:a ) }
PREFIX e: <http://example/two/>
RULE { :s :second ?x } WHERE { ?x :p ?v FILTER ( ?x = e:a ) }`, srlD+`<http://example/one/a> :p 1 . <http://example/two/a> :p 1 .`)
	if len(got) != 2 || got[0] == got[1] {
		t.Errorf("got %v", got)
	}
}

func TestSRLWellformedGroupAndSetRules(t *testing.T) {
	bad := map[string]string{
		"SET reuses a variable":        `RULE { } WHERE { ?s ?p ?o SET ( ?o := 1 ) }`,
		"SET twice":                    `RULE { } WHERE { SET ( ?x := 1 ) SET ( ?x := 2 ) }`,
		"FILTER before binding":        `RULE { } WHERE { FILTER ( ?o ) ?s ?p ?o }`,
		"group var not bound":          `RULE { } WHERE { GROUP BY ?k { ?s ?p ?o } [] }`,
		"aggregate var reused":         `RULE { } WHERE { ?n :p ?o GROUP { ?s ?p ?v } [ ?n := COUNT(*) ] }`,
		"aggregate twice":              `RULE { } WHERE { GROUP { ?s ?p ?v } [ ?n := COUNT(*), ?n := SUM(?v) ] }`,
		"aggregate arg not bound":      `RULE { } WHERE { GROUP { ?s ?p ?v } [ ?n := SUM(?z) ] }`,
		"group var not in scope after": `RULE { ?v :p :o } WHERE { GROUP { ?s ?p ?v } [] }`,
	}
	for name, in := range bad {
		rs, err := ParseSRL(srlP + in)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := CheckWellformed(rs); !errors.Is(err, ErrSRLNotWellFormed) {
			t.Errorf("%s: %v", name, err)
		}
	}
	rs, _ := ParseSRL(srlP + `RULE { ?k :n ?n } WHERE { ?k a :K GROUP BY ?k { ?k :p ?v FILTER ( ?v > 0 ) } [ ?n := COUNT(*) ] }`)
	if err := CheckWellformed(rs); err != nil {
		t.Errorf("a key bound outside the group is visible inside it: %v", err)
	}
}

func TestSRLStratificationLayers(t *testing.T) {
	// The worked example of the specification: R1..R3 general in stratum 0;
	// R5 (blank node in the head) run-once and R4 (negation) above.
	rs, err := ParseSRL(srlP + `
RULE { ?x :exposedTo ?v } WHERE { ?x :hasVulnerability ?v }
RULE { ?x :exposedTo ?v } WHERE { ?x :dependsOn ?y . ?y :exposedTo ?v }
RULE { ?x :status :criticallyExposed } WHERE { ?x :exposedTo ?v . ?v :severity ?s . FILTER ( ?s >= 9.0 ) }
RULE { ?x :status :safeToDeploy } WHERE { ?x a :Component . NOT { ?x :status :criticallyExposed } }
RULE { [] a :Notification ; :concerns ?x } WHERE { ?x :status :criticallyExposed }`)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := stratify(rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("%d layers, want 2: %+v", len(layers), layers)
	}
	if len(layers[0].once) != 0 || len(layers[0].general) != 3 {
		t.Errorf("stratum 0: %+v", layers[0])
	}
	if len(layers[1].once) != 1 || layers[1].once[0] != 4 || len(layers[1].general) != 1 || layers[1].general[0] != 3 {
		t.Errorf("stratum 1: %+v", layers[1])
	}
}

func TestSRLOnlyCyclesThroughClosedDependenciesAreRejected(t *testing.T) {
	ok := srlP + `RULE { ?x :a ?y } WHERE { ?x :b ?y } RULE { ?x :b ?y } WHERE { ?x :a ?y }`
	rs, _ := ParseSRL(ok)
	if _, err := Stratify(rs); err != nil {
		t.Errorf("an open cycle is legal: %v", err)
	}
	for name, in := range map[string]string{
		"negation":    `RULE { ?x :a ?y } WHERE { ?x :c ?y NOT { ?x :b ?y } } RULE { ?x :b ?y } WHERE { ?x :a ?y }`,
		"blank node":  `RULE { [] :a ?y } WHERE { ?x :b ?y } RULE { ?x :b ?y } WHERE { ?x :a ?y }`,
		"triple term": `RULE { ?x :a <<( ?x :a ?y )>> } WHERE { ?x :a ?y }`,
		"assignment":  `RULE { ?x :a ?z } WHERE { ?x :a ?y SET ( ?z := ?y ) }`,
		"group":       `RULE { ?x :a ?n } WHERE { GROUP BY ?x { ?x :a ?y } [ ?n := COUNT(*) ] }`,
	} {
		rs, err := ParseSRL(srlP + in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := Stratify(rs); !errors.Is(err, ErrSRLNotStratifiable) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// The same negation against the base graph depends on nothing.
	rs, _ = ParseSRL(srlP + `RULE { ?x :a ?y } WHERE { ?x :c ?y NOT DATA { ?x :a ?y } }`)
	if _, err := Stratify(rs); err != nil {
		t.Errorf("NOT DATA: %v", err)
	}
}
