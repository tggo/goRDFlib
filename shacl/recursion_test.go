package shacl

import (
	"testing"
)

// Recursive shapes over cyclic data (pySHACL #154). Before the recursion guard
// every one of these died with "fatal error: stack overflow", which takes the
// whole test binary down rather than failing one test.
//
// SHACL 1.0 §3.4.3 leaves recursion to the processor; this one assumes that a
// (shape, focus node) pair already being validated on the current path
// conforms. See recursionGuard.

const recursionPrefixes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix shnex: <http://www.w3.org/ns/shacl-node-expr#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix ex: <http://example.org/> .
`

// validateTTL loads both Turtle documents (with the common prefixes prepended),
// validates, and returns the report plus anything sent to the error handler.
func validateTTL(t *testing.T, shapes, data string, opts ...Option) (ValidationReport, []error) {
	t.Helper()
	sg, err := LoadTurtleString(recursionPrefixes+shapes, "http://example.org/")
	if err != nil {
		t.Fatalf("shapes: %v", err)
	}
	dg, err := LoadTurtleString(recursionPrefixes+data, "http://example.org/")
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	var errs []error
	opts = append(opts, WithErrorHandler(func(e error) { errs = append(errs, e) }))
	return Validate(dg, sg, opts...), errs
}

// findDetail reports whether a result anywhere under rs, at any nesting depth,
// has the given focus node and constraint component.
func findDetail(rs []ValidationResult, focus, component string) bool {
	for _, r := range rs {
		if r.FocusNode.Value() == focus && r.SourceConstraintComponent.Value() == component {
			return true
		}
		if findDetail(r.Details, focus, component) {
			return true
		}
	}
	return false
}

const organizationShape = `
ex:Organization a rdfs:Class, sh:NodeShape ;
  sh:property [ sh:path ex:name ; sh:datatype xsd:string ; sh:minCount 1 ] ,
              [ sh:path ex:subOrganization ; sh:node ex:Organization ] .
`

func TestRecursion_NodeCycleConforms(t *testing.T) {
	t.Parallel()
	report, _ := validateTTL(t, organizationShape, `
ex:a a ex:Organization ; ex:name "A" ; ex:subOrganization ex:b .
ex:b ex:name "B" ; ex:subOrganization ex:a .
`)
	if !report.Conforms {
		t.Fatalf("a two-organization cycle where both have a name must conform, got %d results", len(report.Results))
	}
}

func TestRecursion_NodeCycleStillReportsFailure(t *testing.T) {
	t.Parallel()
	// ex:b has no name; the cycle a -> b -> c -> a must not hide that.
	report, _ := validateTTL(t, organizationShape, `
ex:a a ex:Organization ; ex:name "A" ; ex:subOrganization ex:b .
ex:b ex:subOrganization ex:c .
ex:c ex:name "C" ; ex:subOrganization ex:a .
`)
	if report.Conforms {
		t.Fatal("ex:b lacks ex:name, expected a violation")
	}
	if !findDetail(report.Results, "http://example.org/b", SH+"MinCountConstraintComponent") {
		t.Errorf("no sh:minCount result for ex:b in the report: %+v", report.Results)
	}
}

func TestRecursion_NodeAcyclicChainReportsDeepFailure(t *testing.T) {
	t.Parallel()
	// Four levels down, ex:c111 has no name. The guard must not treat an
	// acyclic chain as a cycle.
	report, _ := validateTTL(t, organizationShape, `
ex:c a ex:Organization ; ex:name "C" ; ex:subOrganization ex:c1 .
ex:c1 ex:name "C1" ; ex:subOrganization ex:c11 .
ex:c11 ex:name "C11" ; ex:subOrganization ex:c111 .
ex:c111 ex:subOrganization ex:c1111 .
ex:c1111 ex:name "C1111" .
`)
	if report.Conforms {
		t.Fatal("ex:c111 lacks ex:name, expected ex:c to fail its sh:node chain")
	}
	if !findDetail(report.Results, "http://example.org/c111", SH+"MinCountConstraintComponent") {
		t.Errorf("the missing name of ex:c111 is not in the report details: %+v", report.Results)
	}
}

func TestRecursion_PropertyShapeSelfReference(t *testing.T) {
	t.Parallel()
	shapes := `
ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:property ex:Knows .
ex:Knows a sh:PropertyShape ; sh:path ex:knows ; sh:nodeKind sh:IRI ; sh:property ex:Knows .
`
	report, _ := validateTTL(t, shapes, `ex:a ex:knows ex:b . ex:b ex:knows ex:a .`)
	if !report.Conforms {
		t.Fatalf("expected conforms, got %d results", len(report.Results))
	}
	report, _ = validateTTL(t, shapes, `ex:a ex:knows ex:b . ex:b ex:knows ex:a, "not an IRI" .`)
	if !findDetail(report.Results, "http://example.org/b", SH+"NodeKindConstraintComponent") {
		t.Errorf("the literal known by ex:b is not reported: %+v", report.Results)
	}
}

func TestRecursion_QualifiedValueShape(t *testing.T) {
	t.Parallel()
	report, _ := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:property [ sh:path ex:child ; sh:qualifiedValueShape ex:S ; sh:qualifiedMinCount 1 ] .
`, `ex:a ex:child ex:b . ex:b ex:child ex:a .`)
	if !report.Conforms {
		t.Fatalf("expected conforms, got %d results", len(report.Results))
	}
}

func TestRecursion_LogicalConstraints(t *testing.T) {
	t.Parallel()
	cycle := `ex:a ex:next ex:b . ex:b ex:next ex:a .`
	tests := map[string]struct {
		shapes   string
		conforms bool
	}{
		"and": {`
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:property [ sh:path ex:next ; sh:and ( ex:S [ sh:nodeKind sh:IRI ] ) ] .
`, true},
		"or": {`
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:property [ sh:path ex:next ; sh:or ( [ sh:nodeKind sh:Literal ] ex:S ) ] .
`, true},
		"xone": {`
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:property [ sh:path ex:next ; sh:xone ( ex:S [ sh:nodeKind sh:Literal ] ) ] .
`, true},
		// S(a) needs b not to conform to T; T(b) needs a not to conform to S,
		// which is assumed on the way back round, so T(b) fails and S(a) holds.
		"not": {`
ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:property [ sh:path ex:next ; sh:not ex:T ] .
ex:T a sh:NodeShape ; sh:property [ sh:path ex:next ; sh:not ex:S ] .
`, true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			report, _ := validateTTL(t, tt.shapes, cycle)
			if report.Conforms != tt.conforms {
				t.Errorf("conforms = %v, want %v (%d results)", report.Conforms, tt.conforms, len(report.Results))
			}
		})
	}
}

func TestRecursion_SHACL12SomeValue(t *testing.T) {
	t.Parallel()
	report, _ := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:property [ sh:path ex:next ; sh:someValue ex:S ] .
`, `ex:a ex:next ex:b . ex:b ex:next ex:a .`)
	if !report.Conforms {
		t.Fatalf("expected conforms, got %d results", len(report.Results))
	}
}

func TestRecursion_SHACL12FilterShapeExpression(t *testing.T) {
	t.Parallel()
	// The cycle passes through a node expression context and back into a
	// validation context, which is why the guard has to be shared across both.
	report, _ := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:expression [ shnex:filterShape ex:S ; shnex:nodes [ shnex:pathValues ex:next ] ] .
`, `ex:a ex:next ex:b . ex:b ex:next ex:a .`)
	if !report.Conforms {
		t.Fatalf("expected conforms, got %d results", len(report.Results))
	}
}

func TestRecursion_AFFilterShapeExpression(t *testing.T) {
	t.Parallel()
	report, errs := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:expression [ sh:filterShape ex:S ; sh:nodes [ sh:path ex:next ] ] .
`, `ex:a ex:next ex:b . ex:b ex:next ex:a .`, WithAdvancedFeatures())
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !report.Conforms {
		t.Fatalf("expected conforms, got %d results", len(report.Results))
	}
}
