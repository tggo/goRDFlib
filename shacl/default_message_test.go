package shacl

import (
	"slices"
	"strings"
	"testing"
)

const defaultMsgShapes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .
ex:ProjectShape a sh:NodeShape ;
	sh:targetClass ex:Project ;
	sh:closed true ; sh:ignoredProperties ( <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> ) ;
	sh:property [ sh:path ex:title ; sh:minCount 1 ] ;
	sh:property [ sh:path ex:budget ; sh:datatype xsd:integer ; sh:maxInclusive 100 ] ;
	sh:property [ sh:path ex:status ; sh:in ( ex:open ex:closed ) ] ;
	sh:property [ sh:path ex:code ; sh:pattern "^[A-Z]+$" ; sh:message "Code must be upper case"@en ] ;
	sh:property [ sh:path ex:owner ; sh:node ex:PersonShape ] .
ex:PersonShape a sh:NodeShape ; sh:property [ sh:path ex:name ; sh:minCount 1 ] .
`

const defaultMsgData = `
@prefix ex: <http://example.org/> .
ex:p1 a ex:Project ;
	ex:budget "lots" ;
	ex:status ex:pending ;
	ex:code "abc" ;
	ex:owner ex:bob ;
	ex:extra 1 .
`

func defaultMessages(t *testing.T, opts ...Option) []string {
	t.Helper()
	shapes, err := LoadTurtleString(defaultMsgShapes, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(defaultMsgData, "")
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, r := range Validate(data, shapes, opts...).Results {
		var parts []string
		for _, m := range r.ResultMessages {
			parts = append(parts, m.Value())
		}
		msgs = append(msgs, strings.Join(parts, "|"))
	}
	slices.Sort(msgs)
	return msgs
}

// Issue #47.
func TestDefaultMessages(t *testing.T) {
	got := defaultMessages(t, WithDefaultMessages())
	want := []string{
		"Code must be upper case", // the shape's own message wins
		`Less than 1 values on ex:p1->ex:title`,
		`Predicate ex:extra is not allowed on ex:p1 (closed shape)`,
		`Value "lots" on ex:p1->ex:budget is not <= 100`,
		`Value "lots" on ex:p1->ex:budget is not a literal of datatype xsd:integer`,
		`Value ex:bob on ex:p1->ex:owner does not conform to ex:PersonShape`,
		`Value ex:pending on ex:p1->ex:status is not in (ex:open ex:closed)`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDefaultMessagesOffByDefault(t *testing.T) {
	for _, m := range defaultMessages(t) {
		if m != "" && m != "Code must be upper case" {
			t.Errorf("unexpected message without the option: %q", m)
		}
	}
}

func TestDefaultMessagesFillDetails(t *testing.T) {
	shapes, _ := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:node [ sh:property [ sh:path ex:p ; sh:minCount 2 ] ] .
`, "")
	data := NewGraph()
	r := Validate(data, shapes, WithDefaultMessages())
	if len(r.Results) != 1 || len(r.Results[0].Details) != 1 {
		t.Fatalf("unexpected report: %+v", r.Results)
	}
	if got := r.Results[0].ResultMessages[0].Value(); got != "Value ex:a on ex:a does not conform to an anonymous shape" {
		t.Errorf("top: %q", got)
	}
	if got := r.Results[0].Details[0].ResultMessages[0].Value(); got != "Less than 2 values on ex:a->ex:p" {
		t.Errorf("detail: %q", got)
	}
}
