package shacl

import (
	"errors"
	"strings"
	"testing"
)

// Tests for SHACL-AF custom targets (§4), and specifically for the guarantee
// that a sh:target selects the same focus nodes for a sh:rule as it does for a
// constraint. It once did not: rules re-parsed the shapes without attaching the
// AF targets, so a shape with no Core target inferred nothing and said nothing.

// afTargetShapes states one target set two ways: as a Core sh:targetClass and
// as an AF sh:SPARQLTarget. The rule attached to each is identical, so the two
// must produce the same triple.
var afTargetShapes = map[string]string{
	"sh:targetClass": `
ex:S a sh:NodeShape ;
    sh:targetClass ex:Thing ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this ex:derived "yes" } WHERE { $this ex:p ?v }""" ] .
`,
	"sh:target": `
ex:S a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ;
        sh:select """SELECT ?this WHERE { ?this a ex:Thing }""" ] ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this ex:derived "yes" } WHERE { $this ex:p ?v }""" ] .
`,
}

const afTargetData = `ex:a a ex:Thing ; ex:p "v" .`

func TestAF_Target_SelectsFocusNodesForRules(t *testing.T) {
	for name, shapesTTL := range afTargetShapes {
		t.Run(name, func(t *testing.T) {
			data := afGraph(t, afTargetData)
			shapes := afGraph(t, shapesTTL)

			n := applyRules(t, data, shapes)
			if n != 1 {
				t.Fatalf("ApplyRules added %d triples, want 1", n)
			}
			if !hasTriple(data, exIRI("a"), exIRI("derived"), Literal("yes", "", "")) {
				t.Error("the rule did not derive ex:a ex:derived \"yes\"")
			}
		})
	}
}

// A rule reached through Validate must see the AF targets too: Validate applies
// the rules before it attaches them, so the rule pass has to attach its own.
func TestAF_Target_SelectsFocusNodesForRulesUnderValidate(t *testing.T) {
	for name, shapesTTL := range afTargetShapes {
		t.Run(name, func(t *testing.T) {
			data := afGraph(t, afTargetData)
			shapes := afGraph(t, shapesTTL+`
ex:S sh:property [ sh:path ex:derived ; sh:minCount 1 ] .
`)
			report := Validate(data, shapes, WithAdvancedFeatures())
			if !report.Conforms {
				t.Errorf("validation should see the inferred triple, got %d results: %v",
					len(report.Results), report.Results)
			}
		})
	}
}

// The AF targets must not leak into a Core run: without WithAdvancedFeatures a
// sh:target is an unrecognised property and selects nothing.
func TestAF_Target_IgnoredWithoutAdvancedFeatures(t *testing.T) {
	data := afGraph(t, afTargetData)
	shapes := afGraph(t, afTargetShapes["sh:target"]+`
ex:S sh:property [ sh:path ex:p ; sh:minCount 5 ] .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("a sh:target must select nothing when AF is off, got %d results", len(report.Results))
	}
}

func TestAF_Target_BrokenSelectIsReported(t *testing.T) {
	data := afGraph(t, afTargetData)
	shapes := afGraph(t, `
ex:S a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ;
        sh:select """SELECT ?this WHERE { this is not sparql }""" ] ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this ex:derived "yes" } WHERE { $this ex:p ?v }""" ] .
`)

	var reported []error
	n, err := ApplyRules(data, shapes, WithErrorHandler(func(e error) { reported = append(reported, e) }))
	if n != 0 || err != nil {
		t.Fatalf("ApplyRules = (%d, %v), want (0, nil)", n, err)
	}
	if len(reported) == 0 {
		t.Fatal("a target that cannot be run must be reported, not silently select nothing")
	}
	if !errors.Is(reported[0], ErrMalformedTarget) {
		t.Errorf("reported %v, want ErrMalformedTarget", reported[0])
	}
	if !strings.Contains(reported[0].Error(), ex+"S") {
		t.Errorf("the error should name the shape, got %q", reported[0])
	}
}

func TestAF_Target_MissingSelectIsReported(t *testing.T) {
	data := afGraph(t, afTargetData)
	shapes := afGraph(t, `
ex:S a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ] ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """CONSTRUCT { $this ex:derived "yes" } WHERE { $this ex:p ?v }""" ] .
`)
	_, err := ApplyRules(data, shapes)
	if !errors.Is(err, ErrAdvancedFeatures) {
		t.Fatalf("err = %v, want the unreadable target to be returned", err)
	}
	if !strings.Contains(err.Error(), "sh:select") {
		t.Errorf("the error should name the missing property, got %q", err)
	}
}
