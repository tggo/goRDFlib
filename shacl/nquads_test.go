package shacl

import (
	"testing"
)

func mustParseNQuads(t *testing.T, data string) *Graph {
	t.Helper()
	g, err := LoadNQuadsString(data, "http://example.org/")
	if err != nil {
		t.Fatalf("failed to parse N-Quads: %v", err)
	}
	return g
}

// --- Basic validation ---

func TestNQuads_Conforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:nodeKind sh:IRI .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestNQuads_NonConforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:nodeKind sh:IRI ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Alice" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: literal name is not sh:IRI")
	}
}

// --- NQuads shapes ---

func TestNQuads_ShapesAndData(t *testing.T) {
	t.Parallel()
	shapes := mustParseNQuads(t, `<http://example.org/S> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://www.w3.org/ns/shacl#NodeShape> .
<http://example.org/S> <http://www.w3.org/ns/shacl#targetNode> <http://example.org/Alice> .
<http://example.org/S> <http://www.w3.org/ns/shacl#nodeKind> <http://www.w3.org/ns/shacl#IRI> .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- MinCount ---

func TestNQuads_MinCount(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Alice has no name")
	}
}

func TestNQuads_MinCount_Satisfied(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Alice" .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- MaxCount ---

func TestNQuads_MaxCount(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:maxCount 1 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Alice" .
<http://example.org/Alice> <http://example.org/name> "Ally" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: maxCount 1 but 2 names")
	}
}

// --- Datatype ---

func TestNQuads_Datatype(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/age> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestNQuads_Datatype_Wrong(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/age> "not-a-number" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: plain string is not xsd:integer")
	}
}

// --- Class ---

func TestNQuads_Class(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:class ex:Person .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestNQuads_Class_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:class ex:Animal .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Alice is Person, not Animal")
	}
}

// --- Pattern ---

func TestNQuads_Pattern(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:email ;
        sh:pattern "^.+@.+$" ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/email> "alice@example.org" .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestNQuads_Pattern_NoMatch(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:email ;
        sh:pattern "^.+@.+$" ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/email> "not-an-email" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'not-an-email' does not match pattern")
	}
}

// --- HasValue ---

func TestNQuads_HasValue(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:status ;
        sh:hasValue ex:Active ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/status> <http://example.org/Active> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestNQuads_HasValue_Missing(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:status ;
        sh:hasValue ex:Active ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/status> <http://example.org/Inactive> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: ex:Active not in values")
	}
}

// --- TargetClass ---

func TestNQuads_TargetClass(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
<http://example.org/Alice> <http://example.org/name> "Alice" .
<http://example.org/Bob> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Bob has no name")
	}
	if len(report.Results) != 1 {
		t.Errorf("expected 1 violation, got %d", len(report.Results))
	}
}

// --- Deactivated ---

func TestNQuads_Deactivated(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:deactivated true ;
    sh:targetNode ex:Alice ;
    sh:nodeKind sh:Literal .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Error("expected deactivated shape to be skipped")
	}
}

// --- Mixed: NQuads data + JSON-LD shapes ---

func TestNQuads_Data_With_JsonLD_Shapes(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": {
    "ex": "http://example.org/",
    "sh": "http://www.w3.org/ns/shacl#",
    "xsd": "http://www.w3.org/2001/XMLSchema#"
  },
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Alice" .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming with JSON-LD shapes + NQuads data, got %d violations", len(report.Results))
	}
}

// --- Multiple triples ---

func TestNQuads_MultipleNodes(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
<http://example.org/Alice> <http://example.org/name> "Alice" .
<http://example.org/Bob> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
<http://example.org/Bob> <http://example.org/name> "Bob" .
<http://example.org/Charlie> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Charlie has no name")
	}
	if len(report.Results) != 1 {
		t.Errorf("expected 1 violation (Charlie), got %d", len(report.Results))
	}
}

// --- Named graph (quad with graph component) ---

func TestNQuads_WithNamedGraph(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:nodeKind sh:IRI .
`)
	// N-Quads with graph component — triples still go into the store
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://example.org/Person> <http://example.org/graph1> .
`)
	report := Validate(data, shapes)
	// Alice is an IRI node regardless of graph context
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Error cases ---

func TestLoadNQuadsString_Invalid(t *testing.T) {
	t.Parallel()
	_, err := LoadNQuadsString("this is not valid nquads !", "http://example.org/")
	if err == nil {
		t.Error("expected error for invalid N-Quads")
	}
}

func TestLoadNQuadsFile_NotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadNQuadsFile("/nonexistent/path/to/file.nq")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- MinLength / MaxLength ---

func TestNQuads_MinLength(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:minLength 5 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Ali" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'Ali' shorter than minLength 5")
	}
}

func TestNQuads_MaxLength(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:maxLength 3 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/name> "Alice" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'Alice' longer than maxLength 3")
	}
}

// --- Value range ---

func TestNQuads_MinInclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:age ;
        sh:minInclusive 0 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/age> "-1"^^<http://www.w3.org/2001/XMLSchema#integer> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: -1 < minInclusive 0")
	}
}

func TestNQuads_MaxInclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:age ;
        sh:maxInclusive 150 ;
    ] .
`)
	data := mustParseNQuads(t, `<http://example.org/Alice> <http://example.org/age> "200"^^<http://www.w3.org/2001/XMLSchema#integer> .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 200 > maxInclusive 150")
	}
}
