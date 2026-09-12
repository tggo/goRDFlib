package shacl

import (
	"sort"
	"testing"
)

// Issue #31: {?varName} / {$varName} in sh:message of SPARQL-based
// constraints and components (SHACL §5.3.2).

func TestExpandTemplate(t *testing.T) {
	vars := map[string]Term{
		"x":     Literal("42", XSD+"integer", ""),
		"iri":   IRI("http://example.org/a"),
		"b":     BlankNode("n1"),
		"naïve": Literal("ok", "", ""),
		"evil":  Literal("{?x}", "", ""),
	}
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"{?x}", "42"},
		{"{$x}", "42"},
		{"a {?iri} b", "a http://example.org/a b"},
		{"{?b}", "_:n1"},
		{"{?naïve}", "ok"},
		{"{?x}{$x}", "4242"},
		{"{?missing}", "{?missing}"},
		{"{?}", "{?}"},
		{"{x}", "{x}"},
		{"{?x", "{?x"},
		{"{?a b}", "{?a b}"},
		{"trailing {", "trailing {"},
		{"{{?x}}", "{42}"},
		// Single pass: a substituted value is never expanded again.
		{"{?evil}", "{?x}"},
	}
	for _, tt := range tests {
		if got := expandTemplate(tt.in, vars); got != tt.want {
			t.Errorf("expandTemplate(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandMessageKeepsLanguageAndNonLiterals(t *testing.T) {
	vars := map[string]Term{"v": Literal("x", "", "")}
	got := expandMessage(Literal("Wert {?v}", "", "de"), vars)
	if got.Value() != "Wert x" || got.Language() != "de" {
		t.Errorf("got %q@%s, want \"Wert x\"@de", got.Value(), got.Language())
	}
	iri := IRI("http://example.org/{?v}")
	if got := expandMessage(iri, vars); !got.Equal(iri) {
		t.Errorf("IRI message changed: %v", got)
	}
}

func TestResultMessagesMessageBindingIsNotATemplate(t *testing.T) {
	called := false
	vars := func() map[string]Term { called = true; return nil }
	row := map[string]Term{"message": Literal("from data {?value}", "", "")}
	got := resultMessages(row, vars, []Term{Literal("template {?value}", "", "")})
	if len(got) != 1 || got[0].Value() != "from data {?value}" {
		t.Fatalf("got %v, want the ?message binding verbatim", got)
	}
	if called {
		t.Error("vars built although ?message won")
	}
}

func TestResultMessagesPlainTextBuildsNoVars(t *testing.T) {
	vars := func() map[string]Term { t.Fatal("vars built for a message without placeholders"); return nil }
	if got := resultMessages(nil, vars, nil, []Term{Literal("plain", "", "")}); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got := resultMessages(nil, vars); got != nil {
		t.Fatalf("no candidates: got %v, want nil", got)
	}
}

func messagesOf(t *testing.T, report ValidationReport) []string {
	t.Helper()
	var out []string
	for _, r := range report.Results {
		for _, m := range r.ResultMessages {
			s := m.Value()
			if m.Language() != "" {
				s += "@" + m.Language()
			}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func assertMessages(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("messages = %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("messages = %q, want %q", got, want)
		}
	}
}

const messagePrefixes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

func TestSPARQLConstraintMessageSubstitution(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+`
ex:S a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:sparql [
        sh:message "{$this} has age {?value} ({?kind})"@en ;
        sh:select """
            SELECT $this ?value ?kind WHERE {
                $this ex:age ?value .
                BIND("too young" AS ?kind)
                FILTER(?value < 18)
            }""" ;
    ] .
ex:alice ex:age 7 .
`)
	assertMessages(t, messagesOf(t, Validate(g, g)), "http://example.org/alice has age 7 (too young)@en")
}

func TestSPARQLConstraintMessageVariableWins(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+`
ex:S a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:sparql [
        sh:message "static" ;
        sh:select """
            SELECT $this ?message WHERE {
                $this ex:age ?a .
                BIND(CONCAT("age ", STR(?a)) AS ?message)
            }""" ;
    ] .
ex:alice ex:age 7 .
`)
	assertMessages(t, messagesOf(t, Validate(g, g)), "age 7")
}

func TestSPARQLConstraintPathVariable(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+`
ex:S a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:age ;
        sh:sparql [
            sh:message "{?path} = {$value} on {?this}" ;
            sh:select """SELECT $this ?value WHERE { $this $PATH ?value . }""" ;
        ] ;
    ] .
ex:alice ex:age 7 .
`)
	assertMessages(t, messagesOf(t, Validate(g, g)), "http://example.org/age = 7 on http://example.org/alice")
}

func TestSPARQLConstraintMessageOnQueryError(t *testing.T) {
	g := NewGraph()
	shape := &Shape{ID: IRI("http://example.org/S")}
	ctx := &evalContext{dataGraph: g, shapesGraph: g, shapesMap: map[string]*Shape{}}
	c := &SPARQLConstraint{
		Select:   "NOT SPARQL {{{",
		Messages: []Term{Literal("failed on {$this}", "", "")},
	}
	rs := c.Evaluate(ctx, shape, IRI("http://example.org/x"), nil)
	if len(rs) != 1 || len(rs[0].ResultMessages) != 1 || rs[0].ResultMessages[0].Value() != "failed on http://example.org/x" {
		t.Fatalf("results = %+v", rs)
	}
}

// The ASK and SELECT examples from SHACL §6.1/§6.2.3, which use a parameter
// name in the message.
const languageComponent = `
ex:LangASK a sh:ConstraintComponent ;
    sh:parameter [ sh:path ex:lang ] ;
    sh:validator [
        a sh:SPARQLAskValidator ;
        sh:message "Values are literals with language \"{$lang}\"" ;
        sh:ask """ASK { FILTER (isLiteral($value) && langMatches(lang($value), $lang)) }""" ;
    ] .
ex:LangSELECT a sh:ConstraintComponent ;
    sh:parameter [ sh:path ex:selLang ] ;
    sh:propertyValidator [
        a sh:SPARQLSelectValidator ;
        sh:message "{?value} is not @{?selLang}" ;
        sh:select """
            SELECT DISTINCT $this ?value WHERE {
                $this $PATH ?value .
                FILTER (!isLiteral(?value) || !langMatches(lang(?value), $selLang))
            }""" ;
    ] .
`

func TestSPARQLComponentMessageParameters(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+languageComponent+`
ex:S a sh:NodeShape ;
    sh:targetNode ex:de ;
    sh:property [ sh:path ex:germanLabel ; ex:lang "de" ] ;
    sh:property [ sh:path ex:germanName ; ex:selLang "de" ] .
ex:de ex:germanLabel "Germany"@en ; ex:germanName "Germany"@en .
`)
	assertMessages(t, messagesOf(t, Validate(g, g)),
		`Values are literals with language "de"`,
		`Germany is not @de`,
	)
}

func TestSPARQLComponentFallsBackToComponentMessage(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+`
ex:MaxLen a sh:ConstraintComponent ;
    sh:message "longer than {$maxLen}: {$value}" ;
    sh:parameter [ sh:path ex:maxLen ] ;
    sh:validator [
        a sh:SPARQLAskValidator ;
        sh:ask """ASK { FILTER (STRLEN(STR($value)) <= $maxLen) }""" ;
    ] .
ex:S a sh:NodeShape ;
    sh:targetNode ex:x ;
    sh:property [ sh:path ex:name ; ex:maxLen 3 ] .
ex:x ex:name "abcdef" .
`)
	assertMessages(t, messagesOf(t, Validate(g, g)), "longer than 3: abcdef")
}

func TestSPARQLComponentSelectPathVariable(t *testing.T) {
	g := loadTurtle(t, messagePrefixes+`
ex:NoFoo a sh:ConstraintComponent ;
    sh:parameter [ sh:path ex:forbidden ] ;
    sh:nodeValidator [
        a sh:SPARQLSelectValidator ;
        sh:message "{?path} must not be used" ;
        sh:select """SELECT $this ?path WHERE { $this ?path ?o . FILTER(?path = $forbidden) }""" ;
    ] .
ex:S a sh:NodeShape ;
    sh:targetNode ex:x ;
    ex:forbidden ex:foo .
ex:x ex:foo 1 .
`)
	report := Validate(g, g)
	assertMessages(t, messagesOf(t, report), "http://example.org/foo must not be used")
	if got := report.Results[0].ResultPath; got.Value() != "http://example.org/foo" {
		t.Errorf("ResultPath = %v, want ex:foo", got)
	}
}

func TestMessageVarNameRunes(t *testing.T) {
	vars := map[string]Term{"naïve": Literal("a", "", ""), "a·b": Literal("b", "", ""), "x‿y": Literal("c", "", "")}
	if got := expandTemplate("{?naïve}{?a·b}{$x‿y}", vars); got != "abc" {
		t.Errorf("got %q, want abc", got)
	}
}

const maxLenComponent = `
ex:MaxLen a sh:ConstraintComponent ;
    sh:message "component: {$maxLen}" ;
    sh:parameter [ sh:path ex:maxLen ] ;
    sh:validator [
        a sh:SPARQLAskValidator ;
        sh:ask """ASK { FILTER (STRLEN(STR($value)) <= $maxLen) }""" ;
    ] .
ex:x ex:name "abcdef" .
`

func TestSPARQLComponentMessagePrecedence(t *testing.T) {
	t.Run("shape message beats component message", func(t *testing.T) {
		g := loadTurtle(t, messagePrefixes+maxLenComponent+`
ex:S a sh:NodeShape ; sh:targetNode ex:x ;
    sh:property [ sh:path ex:name ; ex:maxLen 3 ; sh:message "shape {$maxLen}" ] .
`)
		// Shape messages are copied verbatim (§2.1.5), not templated.
		assertMessages(t, messagesOf(t, Validate(g, g)), "shape {$maxLen}")
	})
	t.Run("validator message beats component message", func(t *testing.T) {
		g := loadTurtle(t, messagePrefixes+`
ex:MinLen a sh:ConstraintComponent ;
    sh:message "component" ;
    sh:parameter [ sh:path ex:minLen ] ;
    sh:validator [
        a sh:SPARQLAskValidator ;
        sh:message "validator {$minLen}"@en , "Validator {$minLen}"@de ;
        sh:ask """ASK { FILTER (STRLEN(STR($value)) >= $minLen) }""" ;
    ] .
ex:S a sh:NodeShape ; sh:targetNode ex:x ; sh:property [ sh:path ex:name ; ex:minLen 9 ] .
ex:x ex:name "abc" .
`)
		assertMessages(t, messagesOf(t, Validate(g, g)), "validator 9@en", "Validator 9@de")
	})
	t.Run("message binding in component SELECT", func(t *testing.T) {
		g := loadTurtle(t, messagePrefixes+`
ex:Bad a sh:ConstraintComponent ;
    sh:message "component" ;
    sh:parameter [ sh:path ex:bad ] ;
    sh:nodeValidator [
        a sh:SPARQLSelectValidator ;
        sh:message "validator" ;
        sh:select """SELECT $this ?message WHERE { $this ?p $bad . BIND("from query" AS ?message) }""" ;
    ] .
ex:S a sh:NodeShape ; sh:targetNode ex:x ; ex:bad "abcdef" .
`+maxLenComponent)
		assertMessages(t, messagesOf(t, Validate(g, g)), "from query")
	})
}

func TestSPARQLComponentMessageOnQueryError(t *testing.T) {
	g := NewGraph()
	shape := &Shape{ID: IRI("http://example.org/S")}
	ctx := &evalContext{dataGraph: g, shapesGraph: g, shapesMap: map[string]*Shape{}}
	lang := IRI("http://example.org/lang")
	c := &SPARQLComponentConstraint{
		ComponentNode: IRI("http://example.org/comp"),
		Parameters:    []paramDef{{Path: lang}},
		ParamValues:   map[string]Term{lang.Value(): Literal("de", "", "")},
		Validator:     &validatorDef{Query: "NOT SPARQL {{{"},
		Messages:      []Term{Literal("{$lang} failed for {$this}", "", "")},
	}
	rs := c.Evaluate(ctx, shape, BlankNode("f"), nil)
	if len(rs) != 1 || len(rs[0].ResultMessages) != 1 || rs[0].ResultMessages[0].Value() != "de failed for _:f" {
		t.Fatalf("results = %+v", rs)
	}
}
