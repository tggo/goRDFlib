package shacl

import (
	"testing"
)

// Tests ported from pySHACL issue-based regression tests.
// Each test corresponds to a specific bug report or feature request.

// --- Issue 029: sh:class with implicit class shape ---

func TestIssue029_ClassImplicitShape(t *testing.T) {
	t.Parallel()
	// Func shape requires hasParameter of type FuncParam_Func_a.
	// FuncNode has FuncParam_b (wrong type).
	shapes := `
@prefix ex: <http://semantics.corning.com/ex#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Function a rdfs:Class .
ex:Func a rdfs:Class, sh:NodeShape ;
    rdfs:subClassOf ex:Function ;
    sh:property [
        a sh:PropertyShape ;
        sh:class ex:FuncParam_Func_a ;
        sh:path ex:hasParameter ;
        sh:minCount 1 ;
    ] .
ex:FuncParam_Func_a a rdfs:Class .
ex:FuncParam_Func_b a rdfs:Class .
`
	data := `
@prefix ex: <http://semantics.corning.com/ex#> .
@prefix test: <http://semantics.corning.com/test#> .

test:FuncNode a ex:Func ;
    ex:hasParameter test:FuncParam_b .
test:FuncParam_a a ex:FuncParam_Func_a .
test:FuncParam_b a ex:FuncParam_Func_b .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: FuncParam_b is not FuncParam_Func_a")
	}
}

// --- Issue 038: Implicit class targeting (owl:Class + sh:NodeShape) ---

func TestIssue038_ImplicitClassMinCount(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix ex: <http://example.org/ns#> .

ex:Person a rdfs:Class, sh:NodeShape ;
    sh:property [
        a sh:PropertyShape ;
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`
	data := `
@prefix ex: <http://example.org/ns#> .
ex:Bob a ex:Person .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: Bob has no ex:name")
	}
}

// --- Issue 079: sh:not combined with sh:or ---

func TestIssue079_NotCombinedWithOr(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <https://example.com#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

ex:AAA a owl:Class .
ex:BBB a owl:Class .
ex:CCC a owl:Class .

ex:Shape a sh:NodeShape ;
    sh:targetClass ex:AAA ;
    sh:not [
        sh:or (
            [ sh:class ex:BBB ]
            [ sh:class ex:CCC ]
        )
    ] .
`
	data := `
@prefix ex: <https://example.com#> .
ex:aaa a ex:AAA, ex:BBB, ex:CCC .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: aaa is BBB which matches or, so not fails")
	}
}

// --- Issue 087: sh:class with transitive subClassOf chain ---

func TestIssue087_ClassSubClassOfChain(t *testing.T) {
	t.Parallel()
	// Class0 -> Class1 -> Class2 -> Class3. sh:class Class0 should accept all.
	mixed := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix : <urn:ex#> .

:TargetClass a owl:Class .
:Class0 a owl:Class .
:Class1 a owl:Class ; rdfs:subClassOf :Class0 .
:Class2 a owl:Class ; rdfs:subClassOf :Class1 .
:Class3 a owl:Class ; rdfs:subClassOf :Class2 .

:shape a sh:NodeShape ;
    sh:targetClass :TargetClass ;
    sh:property [
        a sh:PropertyShape ;
        sh:path :prop ;
        sh:class :Class0 ;
    ] .

:s0 a :Class0 .
:vav0 a :TargetClass ; :prop :s0 .
:s1 a :Class1 .
:vav1 a :TargetClass ; :prop :s1 .
:s2 a :Class2 .
:vav2 a :TargetClass ; :prop :s2 .
:s3 a :Class3 .
:vav3 a :TargetClass ; :prop :s3 .
`
	g, err := LoadTurtleString(mixed, "urn:ex")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(g, g)
	if !report.Conforms {
		t.Fatalf("expected conforming: all props are subclasses of Class0, got %d violations", len(report.Results))
	}
}

// --- Issue 096: sh:hasValue with targetClass + transitive subClassOf ---

func TestIssue096_HasValueSubClassTarget(t *testing.T) {
	t.Parallel()
	mixed := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix : <urn:ex#> .

:Class0 a owl:Class .
:Class1 a owl:Class ; rdfs:subClassOf :Class0 .
:Class2 a owl:Class ; rdfs:subClassOf :Class1 .
:Class3 a owl:Class ; rdfs:subClassOf :Class2 .

:shape a sh:NodeShape ;
    sh:targetClass :Class0 ;
    sh:property [
        sh:path :prop ;
        sh:hasValue "test" ;
        sh:minCount 1 ;
    ] .

:s2 a :Class2 ; :prop "fail" .
:s3 a :Class3 ; :prop "fail" .
`
	g, err := LoadTurtleString(mixed, "urn:ex")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(g, g)
	if report.Conforms {
		t.Fatal("expected non-conforming: s2 and s3 have 'fail' not 'test'")
	}
	if len(report.Results) < 2 {
		t.Fatalf("expected at least 2 violations, got %d", len(report.Results))
	}
}

// --- Issue 116: datatype xsd:string vs rdf:langString ---

func TestIssue116_DatatypeStringRejectsLang(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/issue/116#> .

ex:ThingWithAStringProperty a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:datatype xsd:string ;
        sh:path ex:someString ;
    ] .
`
	data := `
@prefix ex: <http://example.org/issue/116#> .
@prefix kb: <http://example.org/kb/> .

kb:someIndividual a ex:ThingWithAStringProperty ;
    ex:someString "A string with a language"@en .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: lang-tagged string is rdf:langString, not xsd:string")
	}
}

func TestIssue116_DatatypeStringAcceptsPlain(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/issue/116#> .

ex:ThingWithAStringProperty a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:datatype xsd:string ;
        sh:path ex:someString ;
    ] .
`
	data := `
@prefix ex: <http://example.org/issue/116#> .
@prefix kb: <http://example.org/kb/> .

kb:someIndividual a ex:ThingWithAStringProperty ;
    ex:someString "A string without a language" .
`
	report := dashValidate(t, shapes, data)
	if !report.Conforms {
		t.Fatalf("expected conforming: plain string is xsd:string, got %d violations", len(report.Results))
	}
}

func TestIssue116_DatatypeLangStringAcceptsLang(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/issue/116#> .

ex:ThingWithAStringProperty a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:datatype rdf:langString ;
        sh:path ex:someString ;
    ] .
`
	data := `
@prefix ex: <http://example.org/issue/116#> .
@prefix kb: <http://example.org/kb/> .

kb:someIndividual a ex:ThingWithAStringProperty ;
    ex:someString "A string with a language"@en .
`
	report := dashValidate(t, shapes, data)
	if !report.Conforms {
		t.Fatalf("expected conforming: lang-tagged is rdf:langString, got %d violations", len(report.Results))
	}
}

func TestIssue116_DatatypeLangStringRejectsPlain(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/issue/116#> .

ex:ThingWithAStringProperty a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:datatype rdf:langString ;
        sh:path ex:someString ;
    ] .
`
	data := `
@prefix ex: <http://example.org/issue/116#> .
@prefix kb: <http://example.org/kb/> .

kb:someIndividual a ex:ThingWithAStringProperty ;
    ex:someString "A string without a language" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: plain string is xsd:string, not rdf:langString")
	}
}

// --- Issue 126: Severity with mixed Warning and Info ---

func TestIssue126_MixedSeverity(t *testing.T) {
	t.Parallel()
	mixed := `
@prefix ex: <http://example.org/ns#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:myProperty-datatype a sh:PropertyShape ;
    sh:datatype xsd:string ;
    sh:path ex:myProperty ;
    sh:severity sh:Warning .

ex:myProperty-maxLength a sh:PropertyShape ;
    sh:maxLength 10 ;
    sh:path ex:myProperty ;
    sh:severity sh:Info .

ex:MyShape a sh:NodeShape ;
    sh:property ex:myProperty-datatype, ex:myProperty-maxLength ;
    sh:targetNode ex:MyInstance .

ex:MyInstance ex:myProperty "http://toomanycharacters"^^xsd:anyURI .
`
	g, err := LoadTurtleString(mixed, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(g, g)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(report.Results))
	}
	hasWarning, hasInfo := false, false
	for _, r := range report.Results {
		if r.ResultSeverity.Equal(SHWarning) {
			hasWarning = true
		}
		if r.ResultSeverity.Equal(SHInfo) {
			hasInfo = true
		}
	}
	if !hasWarning {
		t.Error("expected one sh:Warning result")
	}
	if !hasInfo {
		t.Error("expected one sh:Info result")
	}
}

// --- Issue 133: sh:pattern with sh:flags "i" ---

func TestIssue133_PatternWithFlags(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix geo: <http://www.opengis.net/ont/geosparql#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

<http://example.org/Shape> a sh:NodeShape ;
    sh:property [
        a sh:PropertyShape ;
        sh:path geo:asWKT ;
        sh:pattern "^\\s*$|^\\s*(P|C|S|L|T|<)" ;
        sh:flags "i" ;
    ] ;
    sh:targetSubjectsOf geo:asWKT .
`
	data := `
@prefix geo: <http://www.opengis.net/ont/geosparql#> .

<http://example.com/geometry-a> geo:asWKT "POINT (153.084231 -27.322738)"^^geo:wktLiteral .
<http://example.com/geometry-b> geo:asWKT "xPOINT (153.084231 -27.322738)"^^geo:wktLiteral .
<http://example.com/geometry-c> geo:asWKT "(153.084231 -27.322738)"^^geo:wktLiteral .
<http://example.com/geometry-d> geo:asWKT "     POINT (153.084231 -27.322738)"^^geo:wktLiteral .
<http://example.com/geometry-e> geo:asWKT "     "^^geo:wktLiteral .
<http://example.com/geometry-f> geo:asWKT ""^^geo:wktLiteral .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	// geometry-b ("xPOINT...") and geometry-c ("(153...") should fail
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(report.Results))
	}
}

// --- Issue 160: targetSubjectsOf with sh:class ---

func TestIssue160_TargetSubjectsOfWithClass(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/ontology/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:propertyOfA-nodeshape a sh:NodeShape ;
    sh:class ex:ThingA ;
    sh:targetSubjectsOf ex:propertyOfA .
`
	data := `
@prefix ex: <http://example.org/ontology/> .
@prefix kb: <http://example.org/kb/> .

kb:thing-a-1 a ex:ThingA ;
    ex:propertyOfA "1" .
kb:thing-b-1 a ex:ThingB ;
    ex:propertyOfA "1" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: thing-b-1 is not ThingA")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(report.Results))
	}
}

// --- Issue 162: Multiple sh:node references ---

func TestIssue162_MultipleNodeConstraints(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <urn:ex#> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix schema: <http://schema.org/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

ex:ExampleParentShape a sh:NodeShape ;
    sh:targetClass ex:test_class ;
    sh:node ex:nodeShape1, ex:nodeShape2 .

ex:nodeShape1 a sh:NodeShape ;
    sh:property [
        sh:path schema:name ;
        sh:class ex:test_class_1 ;
    ] .

ex:nodeShape2 a sh:NodeShape ;
    sh:property [
        sh:path schema:subOrganization ;
        sh:class ex:test_class_1 ;
    ] .
`
	data := `
@prefix ex: <urn:ex#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix schema: <http://schema.org/> .

ex:test_class a owl:Class .
ex:test_class_1 a owl:Class .
ex:test_class_2 a owl:Class .
ex:name_1 a ex:test_class_1 .
ex:org_1 a ex:test_class_2 .

ex:test_entity a ex:test_class ;
    schema:name ex:name_1 ;
    schema:subOrganization ex:org_1 .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: org_1 is test_class_2 not test_class_1")
	}
}

// --- Issue 213: qualifiedValueShape + qualifiedMinCount ---

func TestIssue213_QualifiedMinCountZeroValues(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/ontology/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:MyShape a sh:NodeShape ;
    sh:targetClass ex:MyFirstClass ;
    sh:property [
        sh:path ex:myProperty ;
        sh:minCount 1 ;
    ] ;
    sh:property [
        sh:path ex:myProperty ;
        sh:qualifiedValueShape [ sh:class ex:MySecondClass ] ;
        sh:qualifiedMinCount 1 ;
    ] .
`
	data := `
@prefix ex: <http://example.org/ontology/> .
@prefix kb: <http://example.org/kb/> .

kb:not-ok a ex:MyFirstClass .
kb:ok a ex:MyFirstClass ;
    ex:myProperty [ a ex:MySecondClass ] .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: not-ok has no myProperty")
	}
	// not-ok: fails minCount and qualifiedMinCount (no values at all)
	// ok: passes both
	notOkViolations := 0
	okViolations := 0
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/kb/not-ok" {
			notOkViolations++
		}
		if r.FocusNode.Value() == "http://example.org/kb/ok" {
			okViolations++
		}
	}
	if notOkViolations < 2 {
		t.Errorf("expected at least 2 violations for not-ok, got %d", notOkViolations)
	}
	if okViolations != 0 {
		t.Errorf("expected 0 violations for ok, got %d", okViolations)
	}
}

// --- Issue 217: Multiple sh:not constraints ---

func TestIssue217_MultipleNotConstraints(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix ex: <http://example.org/ontology/> .
@prefix sh-ex: <http://example.org/shapes/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

sh-ex:ClassC-shape a sh:NodeShape ;
    sh:not [ a sh:NodeShape ; sh:class ex:ClassA ],
           [ a sh:NodeShape ; sh:class ex:ClassB ] ;
    sh:targetClass ex:ClassC .
`
	data := `
@prefix ex: <http://example.org/ontology/> .
@prefix kb: <http://example.org/kb/> .

kb:Thing-1 a ex:ClassA, ex:ClassB .
kb:Thing-2 a ex:ClassA, ex:ClassC .
kb:Thing-3 a ex:ClassB, ex:ClassC .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming")
	}
	// Thing-2: ClassC + ClassA -> violates sh:not[class ClassA]
	// Thing-3: ClassC + ClassB -> violates sh:not[class ClassB]
	// Thing-1: not targeted (not ClassC)
	thing2 := 0
	thing3 := 0
	for _, r := range report.Results {
		if r.FocusNode.Value() == "http://example.org/kb/Thing-2" {
			thing2++
		}
		if r.FocusNode.Value() == "http://example.org/kb/Thing-3" {
			thing3++
		}
	}
	if thing2 == 0 {
		t.Error("expected violation for Thing-2")
	}
	if thing3 == 0 {
		t.Error("expected violation for Thing-3")
	}
}

// --- Issue 306: Double inversePath (inverse of inverse = forward) ---

func TestIssue306_DoubleInversePath(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix : <https://w3id.org/example#> .

:Shape0 a sh:NodeShape ;
    sh:targetNode :a0 ;
    sh:property [
        sh:path [ sh:inversePath [ sh:inversePath :r0 ] ] ;
        sh:minCount 1 ;
    ] .
`
	data := `
@prefix : <https://w3id.org/example#> .
:a0 :r0 :a1 .
`
	report := dashValidate(t, shapes, data)
	if !report.Conforms {
		t.Fatalf("expected conforming: inverse(inverse(:r0)) = :r0, got %d violations", len(report.Results))
	}
}

// --- Issue 012: sh:minLength + sh:datatype on property ---

func TestIssue012_MinLengthDatatype(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix hei: <http://hei.org/customer/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

hei:HeiAddressShape a sh:NodeShape ;
    sh:property [
        sh:datatype xsd:string ;
        sh:minLength 30 ;
        sh:path hei:Ship_to_street ;
    ] ;
    sh:targetClass hei:Hei_customer .
`
	data := `
@prefix hei: <http://hei.org/customer/> .
hei:hei_cust_1281 a hei:Hei_customer ;
    hei:Ship_to_street "Industrieweg" .
`
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("expected non-conforming: 'Industrieweg' is shorter than 30 chars")
	}
}

// pySHACL #160 — sh:inversePath should work in property shapes
func TestInversePath(t *testing.T) {
	t.Parallel()
	shapes := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path [ sh:inversePath ex:knows ] ;
        sh:minCount 1 ;
    ] .
`
	data := `
@prefix ex: <http://example.org/> .

ex:Alice a ex:Person .
ex:Bob a ex:Person .
ex:Charlie ex:knows ex:Alice .
`
	// Alice is known by Charlie (inverse path satisfied), Bob is not known by anyone
	report := dashValidate(t, shapes, data)
	if report.Conforms {
		t.Fatal("pySHACL#160: expected non-conforming: Bob has no incoming ex:knows")
	}
	// Should have exactly 1 violation (for Bob)
	if len(report.Results) != 1 {
		t.Errorf("pySHACL#160: expected 1 violation (for Bob), got %d", len(report.Results))
	}
}
