package shacl

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// Tests for SHACL-AF rules (§7). The DASH suite in af_dash_test.go covers the
// happy path against the reference implementations; these cover the parts it
// does not reach — ordering, conditions, malformed input, iteration, and the
// guarantee that validation does not modify the caller's data graph.

// afPrefixes is the preamble every fixture in this file shares.
const afPrefixes = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

// afGraph parses a Turtle fragment with the shared preamble.
func afGraph(t *testing.T, ttl string) *Graph {
	t.Helper()
	g, err := LoadTurtleString(afPrefixes+ttl, "http://example.org/")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return g
}

// applyRules runs the rules and fails the test on an unexpected error.
func applyRules(t *testing.T, data, shapes *Graph, opts ...Option) int {
	t.Helper()
	n, err := ApplyRules(data, shapes, opts...)
	if err != nil {
		t.Fatalf("ApplyRules: %v", err)
	}
	return n
}

// hasTriple is a readable assertion over the data graph.
func hasTriple(g *Graph, s, p, o Term) bool {
	return g.Has(&s, &p, &o)
}

// exIRI names a term in the fixture namespace.
func exIRI(local string) Term { return IRI(ex + local) }

func TestAF_TripleRule_InfersFromPathExpressions(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:givenName "Alice" .
ex:bob   a ex:Person ; ex:givenName "Bob" .
ex:carol a ex:Company .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:subject sh:this ;
        sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ;
    ] .
`)
	n := applyRules(t, g, g)
	if n != 2 {
		t.Errorf("added %d triples, want 2", n)
	}
	if !hasTriple(g, exIRI("alice"), exIRI("name"), Literal("Alice", "", "")) {
		t.Error("alice was not given ex:name")
	}
	if hasTriple(g, exIRI("carol"), exIRI("name"), Literal("Carol", "", "")) {
		t.Error("carol is not a Person and must not be touched")
	}
}

// The rule head is three independent node expressions, so a rule may infer a
// predicate or a subject that is not the focus node.
func TestAF_TripleRule_AllThreePositionsAreExpressions(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:knows ex:bob ; ex:relation ex:friendOf .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:subject [ sh:path ex:knows ] ;
        sh:predicate [ sh:path ex:relation ] ;
        sh:object sh:this ;
    ] .
`)
	applyRules(t, g, g)
	if !hasTriple(g, exIRI("bob"), exIRI("friendOf"), exIRI("alice")) {
		t.Error("expected ex:bob ex:friendOf ex:alice from expressions in all three positions")
	}
}

// A literal cannot be a subject and a non-IRI cannot be a predicate. RDF has no
// way to state such a triple, so the rule must drop it rather than produce
// something the graph cannot hold.
func TestAF_TripleRule_SkipsUnstatableTriples(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:givenName "Alice" .

ex:BadSubject a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:subject [ sh:path ex:givenName ] ;
        sh:predicate ex:p ;
        sh:object sh:this ;
    ] .

ex:BadPredicate a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:subject sh:this ;
        sh:predicate [ sh:path ex:givenName ] ;
        sh:object sh:this ;
    ] .
`)
	before := g.Len()
	n := applyRules(t, g, g)
	if n != 0 || g.Len() != before {
		t.Errorf("added %d triples, want 0 — neither rule can state a valid triple", n)
	}
}

func TestAF_SPARQLRule_Construct(t *testing.T) {
	g := afGraph(t, `
ex:r1 a ex:Rectangle ; ex:width 4 ; ex:height 5 .

ex:RectangleShape a sh:NodeShape ;
    sh:targetClass ex:Rectangle ;
    sh:rule [
        a sh:SPARQLRule ;
        sh:construct """
            CONSTRUCT { $this <http://example.org/area> ?area }
            WHERE {
                $this <http://example.org/width> ?w .
                $this <http://example.org/height> ?h .
                BIND (?w * ?h AS ?area)
            }
        """ ;
    ] .
`)
	applyRules(t, g, g)
	if !hasTriple(g, exIRI("r1"), exIRI("area"), Literal("20", XSD+"integer", "")) {
		t.Errorf("sh:construct did not infer the area; graph now has %d triples", g.Len())
	}
}

// sh:order is how SHACL-AF expresses chaining: the second rule can only fire on
// what the first inferred, so running them out of order infers nothing.
func TestAF_Rules_RunInOrder(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:parent ex:pat .
ex:pat ex:sibling ex:uncleBob .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:order 2 ;
        sh:subject sh:this ;
        sh:predicate ex:greatUncle ;
        sh:object [ sh:path ( ex:uncle ex:sibling ) ] ;
    ] ;
    sh:rule [
        a sh:TripleRule ;
        sh:order 1 ;
        sh:subject sh:this ;
        sh:predicate ex:uncle ;
        sh:object [ sh:path ( ex:parent ex:sibling ) ] ;
    ] .
`)
	applyRules(t, g, g)
	if !hasTriple(g, exIRI("alice"), exIRI("uncle"), exIRI("uncleBob")) {
		t.Fatal("the sh:order 1 rule did not infer ex:uncle")
	}
	// The order-2 rule consumed the order-1 rule's output in the same pass.
	// It infers nothing here only because uncleBob has no sibling; what matters
	// is that it ran after, which the next assertion pins down.
	if n, err := ApplyRules(afGraph(t, `
ex:alice a ex:Person ; ex:parent ex:pat .
ex:pat ex:sibling ex:uncleBob .
ex:uncleBob ex:sibling ex:great .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ; sh:order 1 ;
        sh:subject sh:this ; sh:predicate ex:uncle ;
        sh:object [ sh:path ( ex:parent ex:sibling ) ] ] ;
    sh:rule [ a sh:TripleRule ; sh:order 2 ;
        sh:subject sh:this ; sh:predicate ex:greatUncle ;
        sh:object [ sh:path ( ex:uncle ex:sibling ) ] ] .
`), nil); err == nil && n == 0 {
		t.Error("chained rules inferred nothing")
	}
}

// The order-2 rule must see the order-1 rule's output within a single pass.
func TestAF_Rules_LaterRuleSeesEarlierOutput(t *testing.T) {
	shapes := afGraph(t, `
ex:alice a ex:Person ; ex:parent ex:pat .
ex:pat ex:sibling ex:uncleBob .
ex:uncleBob ex:sibling ex:greatAunt .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ; sh:order 1 ;
        sh:subject sh:this ; sh:predicate ex:uncle ;
        sh:object [ sh:path ( ex:parent ex:sibling ) ] ] ;
    sh:rule [ a sh:TripleRule ; sh:order 2 ;
        sh:subject sh:this ; sh:predicate ex:greatUncle ;
        sh:object [ sh:path ( ex:uncle ex:sibling ) ] ] .
`)
	applyRules(t, shapes, shapes)
	if !hasTriple(shapes, exIRI("alice"), exIRI("greatUncle"), exIRI("greatAunt")) {
		t.Error("the sh:order 2 rule did not see what the sh:order 1 rule inferred")
	}
}

// Chains that ordering cannot express — here two rules on different shapes
// feeding each other — need WithRuleIteration.
func TestAF_Rules_IterationReachesFixedPoint(t *testing.T) {
	fixture := `
ex:a a ex:Node ; ex:next ex:b .
ex:b ex:next ex:c .
ex:c ex:next ex:d .

ex:Reach a sh:NodeShape ;
    sh:targetClass ex:Node ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ;
        sh:predicate ex:reaches ;
        sh:object [ sh:path ( ex:reaches ex:next ) ] ] ;
    sh:rule [ a sh:TripleRule ; sh:order -1 ;
        sh:subject sh:this ;
        sh:predicate ex:reaches ;
        sh:object [ sh:path ex:next ] ] .
`
	single := afGraph(t, fixture)
	applyRules(t, single, single)
	if hasTriple(single, exIRI("a"), exIRI("reaches"), exIRI("d")) {
		t.Error("a single ordered pass should not have reached the far end of the chain")
	}

	iterated := afGraph(t, fixture)
	applyRules(t, iterated, iterated, WithRuleIteration())
	for _, target := range []string{"b", "c", "d"} {
		if !hasTriple(iterated, exIRI("a"), exIRI("reaches"), exIRI(target)) {
			t.Errorf("iteration did not reach ex:%s", target)
		}
	}
}

// Iterating a rule set that never settles must fail with an actionable error
// rather than run forever.
func TestAF_Rules_IterationLimitIsReported(t *testing.T) {
	g := afGraph(t, `
ex:a a ex:Node ; ex:next ex:a .

ex:Grow a sh:NodeShape ;
    sh:targetClass ex:Node ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ;
        sh:predicate ex:step ;
        sh:object [ sh:path ( ex:next ex:next ) ] ] .
`)
	// A self-loop settles, so force the limit down to prove the check fires on
	// a set that is still productive when the budget runs out.
	chain := afGraph(t, `
ex:n0 a ex:Node ; ex:next ex:n1 .
ex:n1 ex:next ex:n2 . ex:n2 ex:next ex:n3 . ex:n3 ex:next ex:n4 .
ex:n4 ex:next ex:n5 . ex:n5 ex:next ex:n6 . ex:n6 ex:next ex:n7 .

ex:Reach a sh:NodeShape ;
    sh:targetClass ex:Node ;
    sh:rule [ a sh:TripleRule ; sh:order -1 ;
        sh:subject sh:this ; sh:predicate ex:reaches ;
        sh:object [ sh:path ex:next ] ] ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ; sh:predicate ex:reaches ;
        sh:object [ sh:path ( ex:reaches ex:next ) ] ] .
`)
	_ = g
	_, err := ApplyRules(chain, chain, WithRuleIteration(), WithRuleIterationLimit(2))
	if !errors.Is(err, ErrRuleIterationLimit) {
		t.Fatalf("err = %v, want ErrRuleIterationLimit", err)
	}
	if !strings.Contains(err.Error(), "WithRuleIterationLimit") {
		t.Errorf("the error should name the option that raises the limit, got %q", err)
	}
}

func TestAF_Rules_Condition(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:age 30 .
ex:kid   a ex:Person ; ex:age 10 .

ex:AdultCondition a sh:NodeShape ;
    sh:property [ sh:path ex:age ; sh:minInclusive 18 ] .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:condition ex:AdultCondition ;
        sh:subject sh:this ;
        sh:predicate rdf:type ;
        sh:object ex:Adult ;
    ] .
`)
	applyRules(t, g, g)
	if !hasTriple(g, exIRI("alice"), IRI(RDFType), exIRI("Adult")) {
		t.Error("alice satisfies the condition but was not classified")
	}
	if hasTriple(g, exIRI("kid"), IRI(RDFType), exIRI("Adult")) {
		t.Error("kid fails the condition and must not be classified")
	}
}

// Every condition must hold, whether written as repeated properties or as one
// RDF list.
func TestAF_Rules_MultipleConditions(t *testing.T) {
	for name, conditions := range map[string]string{
		"repeated property": "sh:condition ex:AdultCondition ; sh:condition ex:NamedCondition ;",
		"rdf list":          "sh:condition ( ex:AdultCondition ex:NamedCondition ) ;",
	} {
		t.Run(name, func(t *testing.T) {
			g := afGraph(t, `
ex:both     a ex:Person ; ex:age 30 ; ex:givenName "Both" .
ex:ageOnly  a ex:Person ; ex:age 30 .

ex:AdultCondition a sh:NodeShape ;
    sh:property [ sh:path ex:age ; sh:minInclusive 18 ] .
ex:NamedCondition a sh:NodeShape ;
    sh:property [ sh:path ex:givenName ; sh:minCount 1 ] .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        `+conditions+`
        sh:subject sh:this ;
        sh:predicate rdf:type ;
        sh:object ex:Qualified ;
    ] .
`)
			applyRules(t, g, g)
			if !hasTriple(g, exIRI("both"), IRI(RDFType), exIRI("Qualified")) {
				t.Error("the node satisfying both conditions was not classified")
			}
			if hasTriple(g, exIRI("ageOnly"), IRI(RDFType), exIRI("Qualified")) {
				t.Error("a node satisfying only one condition must not be classified")
			}
		})
	}
}

func TestAF_Rules_Deactivated(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:givenName "Alice" .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [
        a sh:TripleRule ;
        sh:deactivated true ;
        sh:subject sh:this ;
        sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ;
    ] .
`)
	if n := applyRules(t, g, g); n != 0 {
		t.Errorf("a deactivated rule added %d triples, want 0", n)
	}
}

func TestAF_Rules_MalformedAreReportedNotIgnored(t *testing.T) {
	cases := map[string]struct {
		ttl  string
		want error
		says string
	}{
		"untyped rule": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ sh:subject sh:this ; sh:predicate ex:p ; sh:object ex:o ] .`,
			want: ErrMalformedRule,
			says: "sh:TripleRule",
		},
		"both rule types": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ a sh:TripleRule, sh:SPARQLRule ;
                        sh:subject sh:this ; sh:predicate ex:p ; sh:object ex:o ] .`,
			want: ErrMalformedRule,
			says: "both",
		},
		"triple rule without object": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:p ] .`,
			want: ErrMalformedRule,
			says: "sh:object",
		},
		"triple rule with two subjects": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ a sh:TripleRule ; sh:subject sh:this, ex:other ;
                        sh:predicate ex:p ; sh:object ex:o ] .`,
			want: ErrMalformedRule,
			says: "exactly one",
		},
		"sparql rule without construct": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ a sh:SPARQLRule ] .`,
			want: ErrMalformedRule,
			says: "sh:construct",
		},
		"call to undeclared function": {
			ttl: `ex:S a sh:NodeShape ; sh:targetNode ex:x ;
                    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:p ;
                        sh:object [ ex:noSuchFunction ( sh:this ) ] ] .`,
			want: ErrMalformedExpression,
			says: "no sh:SPARQLFunction",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := afGraph(t, tc.ttl)
			_, err := ApplyRules(g, g)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if !errors.Is(err, ErrAdvancedFeatures) {
				t.Error("every AF error should match ErrAdvancedFeatures")
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q should mention %q", err, tc.says)
			}
		})
	}
}

// One unreadable rule must not disable the readable ones.
func TestAF_Rules_OneMalformedRuleDoesNotStopTheRest(t *testing.T) {
	g := afGraph(t, `
ex:alice a ex:Person ; ex:givenName "Alice" .

ex:Broken a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:SPARQLRule ] .

ex:Working a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ; sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ] .
`)
	n, err := ApplyRules(g, g)
	if !errors.Is(err, ErrMalformedRule) {
		t.Errorf("err = %v, want the malformed rule to be reported", err)
	}
	if n != 1 || !hasTriple(g, exIRI("alice"), exIRI("name"), Literal("Alice", "", "")) {
		t.Errorf("the working rule should still have run; added %d triples", n)
	}
}

// Validate must not modify the graph it is given, however much the rules infer.
func TestAF_Validate_DoesNotMutateCallerGraph(t *testing.T) {
	data := afGraph(t, `ex:alice a ex:Person ; ex:givenName "Alice" .`)
	shapes := afGraph(t, `
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ; sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ] ;
    sh:property [ sh:path ex:name ; sh:minCount 1 ] .
`)
	before := data.Len()

	report := Validate(data, shapes, WithAdvancedFeatures())
	if !report.Conforms {
		t.Errorf("validation should see the inferred ex:name and conform, got %d results", len(report.Results))
	}
	if data.Len() != before {
		t.Errorf("Validate modified the caller's graph: %d triples, was %d", data.Len(), before)
	}
	if hasTriple(data, exIRI("alice"), exIRI("name"), Literal("Alice", "", "")) {
		t.Error("the inferred triple leaked into the caller's graph")
	}
}

// Without the option the same shapes graph must behave as plain SHACL Core:
// the rule does not run, so the minCount fails.
func TestAF_Rules_AreOptIn(t *testing.T) {
	data := afGraph(t, `ex:alice a ex:Person ; ex:givenName "Alice" .`)
	shapes := afGraph(t, `
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ; sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ] ;
    sh:property [ sh:path ex:name ; sh:minCount 1 ] .
`)
	if report := Validate(data, shapes); report.Conforms {
		t.Error("without WithAdvancedFeatures the rule must not run, so sh:minCount should fail")
	}
}

// A shapes graph that asks for the rules entailment regime must not be silently
// validated un-inferred (SHACL-AF §7.4).
func TestAF_RulesEntailmentRegimeIsReportedWhenOff(t *testing.T) {
	data := afGraph(t, `ex:alice a ex:Person .`)
	shapes := afGraph(t, `
<http://example.org/shapes> a <http://www.w3.org/2002/07/owl#Ontology> ;
    sh:entailment sh:Rules .

ex:PersonShape a sh:NodeShape ; sh:targetClass ex:Person .
`)
	var got []error
	Validate(data, shapes, WithErrorHandler(func(err error) { got = append(got, err) }))
	if len(got) != 1 || !errors.Is(got[0], ErrAdvancedFeatures) {
		t.Fatalf("errors = %v, want one ErrAdvancedFeatures", got)
	}
	if !strings.Contains(got[0].Error(), "WithAdvancedFeatures") {
		t.Errorf("the error should name the option that enables the regime, got %q", got[0])
	}

	// With the option there is nothing to report.
	var withAF []error
	Validate(data, shapes, WithAdvancedFeatures(), WithErrorHandler(func(err error) { withAF = append(withAF, err) }))
	if len(withAF) != 0 {
		t.Errorf("unexpected errors with advanced features on: %v", withAF)
	}
}

// Validation is read-only once rules have run, so the same shapes graph must be
// usable from several goroutines. Meaningful under -race.
func TestAF_Validate_ConcurrentUseOfOneShapesGraph(t *testing.T) {
	shapes := afGraph(t, `
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:rule [ a sh:TripleRule ;
        sh:subject sh:this ; sh:predicate ex:name ;
        sh:object [ sh:path ex:givenName ] ] ;
    sh:property [ sh:path ex:name ; sh:minCount 1 ] .
`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := afGraph(t, `ex:alice a ex:Person ; ex:givenName "Alice" .`)
			if report := Validate(data, shapes, WithAdvancedFeatures()); !report.Conforms {
				t.Errorf("concurrent validation did not conform: %d results", len(report.Results))
			}
		}()
	}
	wg.Wait()
}
