package shacl

import (
	"fmt"
	"strings"
	"testing"
)

// pySHACL #304: the order of results changed between runs on the same input,
// because validation followed Go map iteration order.

const orderShapes = `
ex:S a sh:NodeShape ; sh:targetClass ex:T ;
  sh:property [ sh:path ex:a ; sh:minCount 1 ; sh:message "a is missing"@en, "a fehlt"@de ] ,
              [ sh:path ex:b ; sh:minCount 1 ] ,
              [ sh:path ex:c ; sh:datatype xsd:string ] ,
              [ sh:path ex:child ; sh:node ex:Child ] .
ex:S2 a sh:NodeShape ; sh:targetClass ex:T ; sh:property [ sh:path ex:d ; sh:minCount 1 ] .
ex:Child a sh:NodeShape ;
  sh:property [ sh:path ex:name ; sh:minCount 1 ] , [ sh:path ex:age ; sh:minCount 1 ] .
`

const orderData = `
ex:x1 a ex:T ; ex:c 1 ; ex:child ex:k1, ex:k2 .
ex:x2 a ex:T ; ex:c 2, 3 .
ex:x3 a ex:T ; ex:child ex:k1 .
ex:x4 a ex:T .
ex:k1 ex:name "k1" .
`

// dumpResults renders results one per line, nested details indented. With
// hideLabels, blank nodes print without their label, which differs between two
// parses of the same document.
func dumpResults(b *strings.Builder, rs []ValidationResult, indent string, hideLabels bool) {
	term := func(t Term) string {
		if hideLabels && t.IsBlank() {
			return "_:"
		}
		return t.String()
	}
	for _, r := range rs {
		msgs := make([]string, len(r.ResultMessages))
		for i, m := range r.ResultMessages {
			msgs[i] = term(m)
		}
		fmt.Fprintf(b, "%s%s %s %s %s %s %s %v\n", indent, term(r.FocusNode), term(r.ResultPath), term(r.Value),
			term(r.SourceShape), term(r.SourceConstraintComponent), term(r.ResultSeverity), msgs)
		dumpResults(b, r.Details, indent+"  ", hideLabels)
	}
}

func dumpOrder(rep ValidationReport, hideLabels bool) string {
	var b strings.Builder
	dumpResults(&b, rep.Results, "", hideLabels)
	return b.String()
}

func TestResultOrder_SameGraphsSameOrder(t *testing.T) {
	t.Parallel()
	sg, err := LoadTurtleString(recursionPrefixes+orderShapes, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dg, err := LoadTurtleString(recursionPrefixes+orderData, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	first := dumpOrder(Validate(dg, sg), false)
	if strings.Count(first, "\n") < 10 {
		t.Fatalf("the fixture should produce enough results to shuffle, got:\n%s", first)
	}
	for i := 1; i < 30; i++ {
		if got := dumpOrder(Validate(dg, sg), false); got != first {
			t.Fatalf("run %d produced a different order:\n%s\nfirst run:\n%s", i, got, first)
		}
	}
}

func TestResultOrder_ReparsedGraphsSameOrder(t *testing.T) {
	t.Parallel()
	var first string
	for i := 0; i < 30; i++ {
		report, _ := validateTTL(t, orderShapes, orderData)
		got := dumpOrder(report, true)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("parse %d produced a different order:\n%s\nfirst parse:\n%s", i, got, first)
		}
	}
}

func TestResultOrder_MessagesNotSortedInPlace(t *testing.T) {
	t.Parallel()
	// Sorting a result's messages must not reorder the slice the shape and
	// the graph index share with it.
	shared := []Term{Literal("b", "", "en"), Literal("a", "", "en")}
	rs := []ValidationResult{{ResultMessages: shared}}
	orderResults(rs)
	if shared[0].Value() != "b" {
		t.Error("orderResults sorted the caller's message slice in place")
	}
	if rs[0].ResultMessages[0].Value() != "a" {
		t.Error("messages of the result are not sorted")
	}
}
