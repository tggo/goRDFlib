package shacl

import (
	"testing"
)

// Tests ported from pySHACL / DASH test suite.
// These cover edge cases and scenarios beyond the W3C SHACL Core test suite.

func dashValidate(t *testing.T, shapesTTL, dataTTL string) ValidationReport {
	t.Helper()
	sg, err := LoadTurtleString(shapesTTL, "http://example.org/shapes")
	if err != nil {
		t.Fatalf("shapes parse error: %v", err)
	}
	dg, err := LoadTurtleString(dataTTL, "http://example.org/data")
	if err != nil {
		t.Fatalf("data parse error: %v", err)
	}
	return Validate(dg, sg)
}

// --- sh:deactivated ---

func TestDASH_Deactivated_True(t *testing.T) {
	t.Parallel()
	// Deactivated shape should produce no violations even when data is invalid.
	shapes := `
@prefix ex: <http://example.org/deact#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a sh:NodeShape ;
    sh:datatype xsd:boolean ;
    sh:deactivated true ;
    sh:property [
        sh:path ex:property ;
        sh:minCount 1 ;
    ] ;
    sh:targetNode ex:InvalidResource .
`
	data := `
@prefix ex: <http://example.org/deact#> .
ex:InvalidResource a <http://www.w3.org/2000/01/rdf-schema#Resource> .
`
	report := dashValidate(t, shapes, data)
	if !report.Conforms {
		t.Errorf("deactivated shape should produce no violations, got %d", len(report.Results))
	}
}

func TestDASH_Deactivated_False(t *testing.T) {
	t.Parallel()
	// sh:deactivated false means shape IS active.
	shapes := `
@prefix ex: <http://example.org/deact2#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a sh:NodeShape ;
    sh:nodeKind sh:IRI ;
    sh:deactivated false ;
    sh:targetNode ex:Valid .
`
	data := `
@prefix ex: <http://example.org/deact2#> .
ex:Valid a ex:Thing .
`
	report := dashValidate(t, shapes, data)
	if !report.Conforms {
		t.Errorf("IRI node should pass nodeKind IRI, got %d violations", len(report.Results))
	}
}

// --- sh:severity ---

func TestDASH_Severity_Warning(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/sev#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a sh:NodeShape ;
    sh:datatype xsd:integer ;
    sh:severity sh:Warning ;
    sh:targetNode "Hello" .
`
	data := ``
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming report")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if !report.Results[0].ResultSeverity.Equal(SHWarning) {
		t.Errorf("expected sh:Warning severity, got %v", report.Results[0].ResultSeverity)
	}
}

func TestDASH_Severity_Info(t *testing.T) {
	t.Parallel()
	// Property shape with sh:Info severity.
	shapes := `
@prefix ex: <http://example.org/sev2#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a sh:NodeShape ;
    sh:property [
        sh:path ex:property ;
        sh:datatype xsd:integer ;
        sh:severity sh:Info ;
    ] ;
    sh:targetNode ex:Resource1 .
`
	data := `
@prefix ex: <http://example.org/sev2#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:Resource1 ex:property "true"^^xsd:boolean .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming report")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if !report.Results[0].ResultSeverity.Equal(SHInfo) {
		t.Errorf("expected sh:Info severity, got %v", report.Results[0].ResultSeverity)
	}
}

// --- sh:zeroOrOnePath ---

func TestDASH_Path_ZeroOrOne(t *testing.T) {
	t.Parallel()
	// zeroOrOnePath: each target gets self + one step via ex:child.
	// sh:minCount 2 means need at least 2 values.
	// InvalidResource1 has no ex:child -> only self -> 1 value -> violation.
	// ValidResource1 has ex:child -> self + child -> 2 values -> ok.
	shapes := `
@prefix ex: <http://example.org/zoo#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:TestShape a sh:PropertyShape ;
    sh:path [ sh:zeroOrOnePath ex:child ] ;
    sh:minCount 2 ;
    sh:targetNode ex:InvalidResource1 ;
    sh:targetNode ex:ValidResource1 .
`
	data := `
@prefix ex: <http://example.org/zoo#> .
ex:InvalidResource1 a ex:Thing .
ex:ValidResource1 ex:child ex:Person1 .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 violation (InvalidResource1), got %d", len(report.Results))
	}
}

// --- sh:not with nested property shape ---

func TestDASH_Not_NestedProperty(t *testing.T) {
	t.Parallel()
	// sh:not [ sh:property [ sh:path ex:property ; sh:minCount 1 ] ]
	// Nodes WITH ex:property should fail (because they conform to the negated shape).
	// Nodes WITHOUT ex:property should pass.
	shapes := `
@prefix ex: <http://example.org/not2#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:Shape a sh:NodeShape ;
    sh:not [
        a sh:NodeShape ;
        sh:property [
            sh:path ex:property ;
            sh:minCount 1 ;
        ] ;
    ] ;
    sh:targetNode ex:Invalid1 ;
    sh:targetNode ex:Valid1 .
`
	data := `
@prefix ex: <http://example.org/not2#> .
ex:Invalid1 ex:property "Some value" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(report.Results))
	}
	if report.Results[0].FocusNode.Value() != "http://example.org/not2#Invalid1" {
		t.Errorf("expected Invalid1 as focus node, got %v", report.Results[0].FocusNode)
	}
}

// --- sh:closed with ignoredProperties ---

func TestDASH_Closed_IgnoredProperties(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/cl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:MyShape a sh:NodeShape ;
    sh:closed true ;
    sh:ignoredProperties ( rdf:type ) ;
    sh:property [ sh:path ex:someProperty ] ;
    sh:targetNode ex:InvalidInstance1 ;
    sh:targetNode ex:ValidInstance1 .
`
	data := `
@prefix ex: <http://example.org/cl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

ex:InvalidInstance1
    ex:otherProperty 4 ;
    ex:someProperty 3 .

ex:ValidInstance1
    rdf:type ex:SomeClass ;
    ex:someProperty 3 .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming (InvalidInstance1 has ex:otherProperty)")
	}
	// InvalidInstance1 has ex:otherProperty which is not allowed
	found := false
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/cl#InvalidInstance1" {
			found = true
		}
	}
	if !found {
		t.Error("expected violation on InvalidInstance1")
	}
	// ValidInstance1 should NOT have violations (rdf:type is ignored, someProperty is allowed)
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/cl#ValidInstance1" {
			t.Error("ValidInstance1 should not have violations")
		}
	}
}

// --- sh:maxCount 0 (prohibit property) ---

func TestDASH_MaxCount_Zero(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/mc0#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:TestShape a sh:NodeShape ;
    sh:property [
        sh:path owl:versionInfo ;
        sh:maxCount 0 ;
    ] ;
    sh:targetNode ex:InvalidResource ;
    sh:targetNode ex:ValidResource .
`
	data := `
@prefix ex: <http://example.org/mc0#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

ex:InvalidResource owl:versionInfo "1.0" .
ex:ValidResource a ex:Thing .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(report.Results))
	}
}

// --- sh:datatype xsd:string rejects lang-tagged and typed literals ---

func TestDASH_Datatype_StringRejectsLangAndTyped(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/dt#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a sh:NodeShape ;
    sh:property [
        sh:path ex:value ;
        sh:datatype xsd:string ;
    ] ;
    sh:targetNode ex:Invalid1 ;
    sh:targetNode ex:Invalid2 ;
    sh:targetNode ex:Valid1 .
`
	data := `
@prefix ex: <http://example.org/dt#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:Invalid1 ex:value "A"@en .
ex:Invalid2 ex:value "42"^^xsd:integer .
ex:Valid1 ex:value "A" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations (lang-tagged + typed), got %d", len(report.Results))
	}
}

// --- sh:minExclusive on non-comparable values ---

func TestDASH_MinExclusive_NonComparable(t *testing.T) {
	t.Parallel()
	// String and IRI values should fail sh:minExclusive (incomparable with integer).
	shapes := `
@prefix ex: <http://example.org/me#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:path ex:testProperty ;
        sh:minExclusive 40 ;
    ] .
`
	data := `
@prefix ex: <http://example.org/me#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Invalid1 a ex:TestShape ;
    ex:testProperty "A string" .

ex:Invalid2 a ex:TestShape ;
    ex:testProperty rdfs:Resource .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(report.Results))
	}
}

// --- sh:uniqueLang with multiple violations ---

func TestDASH_UniqueLang_MultipleViolations(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/ul#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:TestShape a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:path ex:testProperty ;
        sh:uniqueLang true ;
    ] .
`
	data := `
@prefix ex: <http://example.org/ul#> .

ex:Invalid1 a ex:TestShape ;
    ex:testProperty "Me" ;
    ex:testProperty "Me"@en ;
    ex:testProperty "Moi"@fr ;
    ex:testProperty "Myself"@en .

ex:Valid1 a ex:TestShape ;
    ex:testProperty "Me" ;
    ex:testProperty "Me"@en ;
    ex:testProperty "Moi"@fr ;
    ex:testProperty "Myself" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	// Invalid1 has duplicate "en" -> 1 violation
	// Valid1 has unique langs (plain strings have no lang, so no dups)
	violations := 0
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/ul#Invalid1" {
			violations++
		}
	}
	if violations == 0 {
		t.Error("expected violation for Invalid1 (duplicate en)")
	}
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/ul#Valid1" {
			t.Error("Valid1 should not have violations")
		}
	}
}

// --- sh:lessThan with incomparable types (integer vs string) ---

func TestDASH_LessThan_IncomparableTypes(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/lt#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:TestShape a sh:NodeShape ;
    sh:property [
        sh:path ex:first ;
        sh:lessThan ex:second ;
    ] ;
    sh:targetNode ex:Instance1 .
`
	data := `
@prefix ex: <http://example.org/lt#> .
ex:Instance1
    ex:first 1 ;
    ex:first 2 ;
    ex:second "a" ;
    ex:second "b" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming (integers vs strings are incomparable)")
	}
	// Each of first(1,2) x second("a","b") = 4 pairs, all incomparable -> 4 violations
	if len(report.Results) != 4 {
		t.Fatalf("expected 4 violations, got %d", len(report.Results))
	}
}

// --- Implicit class target with subclass ---

func TestDASH_ImplicitClassTarget_Subclass(t *testing.T) {
	t.Parallel()
	// Shape is also rdfs:Class. Instances of subclasses should be targeted too.
	shapes := `
@prefix ex: <http://example.org/ict#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:SubClass a rdfs:Class ;
    rdfs:subClassOf ex:SuperClass .

ex:SuperClass a rdfs:Class, sh:NodeShape ;
    sh:in ( ex:ValidInstance ) .
`
	data := `
@prefix ex: <http://example.org/ict#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:SubClass a rdfs:Class ;
    rdfs:subClassOf ex:SuperClass .
ex:InvalidInstance a ex:SubClass .
ex:ValidInstance a ex:SubClass .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 violation (InvalidInstance), got %d", len(report.Results))
	}
}

// --- Multiple target declarations (union) ---

func TestDASH_MultipleTargets_Union(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/mt#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:MyClass a rdfs:Class .

ex:TestShape a sh:NodeShape ;
    sh:property [
        sh:path ex:myProperty ;
        sh:maxCount 1 ;
    ] ;
    sh:targetClass ex:MyClass ;
    sh:targetSubjectsOf ex:myProperty .
`
	data := `
@prefix ex: <http://example.org/mt#> .

ex:Invalid1 a ex:MyClass ;
    ex:myProperty "A" ;
    ex:myProperty "B" .

ex:Invalid2
    ex:myProperty "A" ;
    ex:myProperty "B" .

ex:Valid1 a ex:MyClass ;
    ex:myProperty "A" .

ex:Valid2
    ex:myProperty "A" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	// Invalid1 targeted by both targetClass and targetSubjectsOf (deduplicated)
	// Invalid2 targeted by targetSubjectsOf only
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(report.Results))
	}
}

// --- Sequence path with deduplication ---

func TestDASH_Path_Sequence_Dedup(t *testing.T) {
	t.Parallel()
	// Two blank nodes both reach the same literal "value" via ex:p2.
	// Sequence path deduplicates, so maxCount 1 should check unique values.
	shapes := `
@prefix ex: <http://example.org/seqd#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:S a sh:NodeShape ;
    sh:property [
        sh:path ( ex:p1 ex:p2 ) ;
        sh:maxCount 1 ;
        sh:nodeKind sh:IRI ;
    ] ;
    sh:targetNode ex:A .
`
	data := `
@prefix ex: <http://example.org/seqd#> .
ex:A ex:p1 [ ex:p2 "value" ] .
ex:A ex:p1 [ ex:p2 "value" ] .
`
	report := dashValidate(t, shapes, data)
	// "value" is a literal, not IRI -> nodeKind violation.
	// But after dedup there's only 1 unique value, so maxCount 1 passes.
	hasNodeKindViolation := false
	for _, r := range report.Results {
		if r.SourceConstraintComponent.Value() == SH+"NodeKindConstraintComponent" {
			hasNodeKindViolation = true
		}
	}
	if !hasNodeKindViolation {
		t.Error("expected nodeKind violation for literal value")
	}
}

// --- sh:qualifiedValueShapesDisjoint (hand/finger/thumb example) ---

func TestDASH_QualifiedValueShapesDisjoint(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/qvsd#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:Finger a rdfs:Class, sh:NodeShape .
ex:Thumb a rdfs:Class, sh:NodeShape .
ex:Hand a rdfs:Class, sh:NodeShape .

ex:HandShape a sh:NodeShape ;
    sh:property ex:HandShape-thumb ;
    sh:property ex:HandShape-finger ;
    sh:targetClass ex:Hand .

ex:HandShape-thumb
    sh:path ex:digit ;
    sh:qualifiedMinCount 1 ;
    sh:qualifiedMaxCount 1 ;
    sh:qualifiedValueShape [ sh:class ex:Thumb ] ;
    sh:qualifiedValueShapesDisjoint true .

ex:HandShape-finger
    sh:path ex:digit ;
    sh:qualifiedMinCount 4 ;
    sh:qualifiedMaxCount 4 ;
    sh:qualifiedValueShape [ sh:class ex:Finger ] ;
    sh:qualifiedValueShapesDisjoint true .
`
	data := `
@prefix ex: <http://example.org/qvsd#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

ex:Finger1 a ex:Finger .
ex:Finger2 a ex:Finger .
ex:Finger3 a ex:Finger .
ex:Finger4 a ex:Finger .
ex:Thumb1 a ex:Thumb .
ex:FingerAndThumb a ex:Finger, ex:Thumb .

ex:ValidHand a ex:Hand ;
    ex:digit ex:Finger1, ex:Finger2, ex:Finger3, ex:Finger4, ex:Thumb1 .

ex:InvalidHand a ex:Hand ;
    ex:digit ex:Finger1, ex:Finger2, ex:Finger3, ex:FingerAndThumb .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	// ValidHand: 4 fingers + 1 thumb (all disjoint) -> OK
	// InvalidHand: FingerAndThumb is both Finger and Thumb -> not disjoint
	//   thumb constraint: FingerAndThumb conforms to Thumb but also Finger (sibling) -> not counted
	//   -> 0 thumbs found, needs 1 -> violation
	//   finger constraint: Finger1,2,3 are fine, FingerAndThumb conforms but also Thumb -> not counted
	//   -> 3 fingers found, needs 4 -> violation
	validViolations := 0
	invalidViolations := 0
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/qvsd#ValidHand" {
			validViolations++
		}
		if r.FocusNode.Value() == "http://example.org/qvsd#InvalidHand" {
			invalidViolations++
		}
	}
	if validViolations != 0 {
		t.Errorf("ValidHand should have 0 violations, got %d", validViolations)
	}
	if invalidViolations < 1 {
		t.Errorf("InvalidHand should have violations, got %d", invalidViolations)
	}
}
