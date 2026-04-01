package shacl

import (
	"testing"
)

// jsonldContext is a reusable JSON-LD context for tests.
const jsonldContext = `{
  "ex": "http://example.org/",
  "sh": "http://www.w3.org/ns/shacl#",
  "rdf": "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
  "rdfs": "http://www.w3.org/2000/01/rdf-schema#",
  "xsd": "http://www.w3.org/2001/XMLSchema#"
}`

// --- NodeKind constraint ---

func TestJsonLD_NodeKind_IRI(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:nodeKind": {"@id": "sh:IRI"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_NodeKind_Literal_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:nodeKind": {"@id": "sh:Literal"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: IRI node cannot be sh:Literal")
	}
}

// --- Datatype constraint ---

func TestJsonLD_Datatype_Valid(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:age"},
    "sh:datatype": {"@id": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:age": {"@value": "42", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Datatype_Wrong(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:age"},
    "sh:datatype": {"@id": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:age": "not-a-number"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: string is not xsd:integer")
	}
}

// --- MinCount / MaxCount ---

func TestJsonLD_MinCount(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	// Alice has no name → violation
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: minCount 1 but no ex:name")
	}
}

func TestJsonLD_MinCount_Satisfied(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_MaxCount(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:maxCount": 1
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": ["Alice", "Ally"]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: maxCount 1 but 2 names")
	}
}

// --- MinLength / MaxLength ---

func TestJsonLD_MinLength(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minLength": 5
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Ali"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'Ali' is shorter than minLength 5")
	}
}

func TestJsonLD_MaxLength(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:maxLength": 3
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'Alice' is longer than maxLength 3")
	}
}

// --- Pattern ---

func TestJsonLD_Pattern_Match(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:email"},
    "sh:pattern": "^.+@.+$"
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:email": "alice@example.org"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Pattern_NoMatch(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:email"},
    "sh:pattern": "^.+@.+$"
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:email": "not-an-email"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'not-an-email' does not match pattern")
	}
}

// --- MinInclusive / MaxInclusive ---

func TestJsonLD_MinInclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:age"},
    "sh:minInclusive": {"@value": "0", "@type": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:age": {"@value": "-1", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: -1 < minInclusive 0")
	}
}

func TestJsonLD_MaxInclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:age"},
    "sh:maxInclusive": {"@value": "150", "@type": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:age": {"@value": "200", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 200 > maxInclusive 150")
	}
}

// --- Class constraint ---

func TestJsonLD_Class(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:class": {"@id": "ex:Person"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Class_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:class": {"@id": "ex:Animal"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: ex:Alice is ex:Person, not ex:Animal")
	}
}

// --- HasValue ---

func TestJsonLD_HasValue(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:status"},
    "sh:hasValue": {"@id": "ex:Active"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:status": {"@id": "ex:Active"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_HasValue_Missing(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:status"},
    "sh:hasValue": {"@id": "ex:Active"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:status": {"@id": "ex:Inactive"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: ex:Active not in values")
	}
}

// --- In constraint ---

func TestJsonLD_In(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:color"},
    "sh:in": {"@list": ["red", "green", "blue"]}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:color": "red"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_In_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:color"},
    "sh:in": {"@list": ["red", "green", "blue"]}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:color": "yellow"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'yellow' not in [red, green, blue]")
	}
}

// --- Logical constraints: And / Or / Not ---

func TestJsonLD_And(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:and": {"@list": [
        {"sh:class": {"@id": "ex:Person"}},
        {"sh:class": {"@id": "ex:Employee"}}
      ]}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": ["ex:Person", "ex:Employee"]
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_And_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:and": {"@list": [
        {"sh:class": {"@id": "ex:Person"}},
        {"sh:class": {"@id": "ex:Employee"}}
      ]}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Alice is Person but not Employee")
	}
}

func TestJsonLD_Or(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:or": {"@list": [
        {"sh:class": {"@id": "ex:Person"}},
        {"sh:class": {"@id": "ex:Animal"}}
      ]}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming via sh:or, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Not(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:not": {"sh:class": {"@id": "ex:Animal"}}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming (Alice is not Animal), got %d violations", len(report.Results))
	}
}

func TestJsonLD_Not_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:not": {"sh:class": {"@id": "ex:Person"}}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Alice IS a Person, but sh:not says she shouldn't be")
	}
}

// --- TargetClass ---

func TestJsonLD_TargetClass(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:PersonShape",
  "@type": "sh:NodeShape",
  "sh:targetClass": {"@id": "ex:Person"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {"@id": "ex:Alice", "@type": "ex:Person", "ex:name": "Alice"},
    {"@id": "ex:Bob", "@type": "ex:Person"}
  ]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Bob has no name")
	}
	if len(report.Results) != 1 {
		t.Errorf("expected 1 violation (Bob), got %d", len(report.Results))
	}
}

// --- TargetSubjectsOf / TargetObjectsOf ---

func TestJsonLD_TargetSubjectsOf(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetSubjectsOf": {"@id": "ex:knows"},
  "sh:nodeKind": {"@id": "sh:IRI"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:knows": {"@id": "ex:Bob"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_TargetObjectsOf(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetObjectsOf": {"@id": "ex:knows"},
  "sh:nodeKind": {"@id": "sh:IRI"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:knows": {"@id": "ex:Bob"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Deactivated shape ---

func TestJsonLD_Deactivated(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:deactivated": true,
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:nodeKind": {"@id": "sh:Literal"}
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Error("expected deactivated shape to be skipped")
	}
}

// --- Equals / Disjoint property pair ---

func TestJsonLD_Equals(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:givenName"},
    "sh:equals": {"@id": "ex:firstName"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:givenName": "Alice",
  "ex:firstName": "Alice"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Equals_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:givenName"},
    "sh:equals": {"@id": "ex:firstName"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:givenName": "Alice",
  "ex:firstName": "Ally"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: givenName != firstName")
	}
}

func TestJsonLD_Disjoint(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:givenName"},
    "sh:disjoint": {"@id": "ex:familyName"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:givenName": "Alice",
  "ex:familyName": "Alice"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: givenName and familyName share value 'Alice'")
	}
}

// --- UniqueLang ---

func TestJsonLD_UniqueLang(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:label"},
    "sh:uniqueLang": true
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:label": [
    {"@value": "Alice", "@language": "en"},
    {"@value": "Alicia", "@language": "en"}
  ]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: duplicate language 'en'")
	}
}

func TestJsonLD_UniqueLang_Valid(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:label"},
    "sh:uniqueLang": true
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:label": [
    {"@value": "Alice", "@language": "en"},
    {"@value": "Alicia", "@language": "es"}
  ]
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Multiple shapes in @graph ---

func TestJsonLD_MultipleShapes(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:NameShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:name"},
        "sh:minCount": 1,
        "sh:maxCount": 1,
        "sh:datatype": {"@id": "xsd:string"}
      }
    },
    {
      "@id": "ex:AgeShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:age"},
        "sh:minCount": 1,
        "sh:datatype": {"@id": "xsd:integer"}
      }
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Alice",
  "ex:age": {"@value": "30", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_MultipleShapes_Violations(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:NameShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:name"},
        "sh:minCount": 1
      }
    },
    {
      "@id": "ex:AgeShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:age"},
        "sh:minCount": 1
      }
    }
  ]
}`)
	// Alice has neither name nor age
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violations")
	}
	if len(report.Results) < 2 {
		t.Errorf("expected at least 2 violations, got %d", len(report.Results))
	}
}

// --- Multiple data nodes ---

func TestJsonLD_MultipleDataNodes(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:PersonShape",
  "@type": "sh:NodeShape",
  "sh:targetClass": {"@id": "ex:Person"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1,
    "sh:datatype": {"@id": "xsd:string"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {"@id": "ex:Alice", "@type": "ex:Person", "ex:name": "Alice"},
    {"@id": "ex:Bob", "@type": "ex:Person", "ex:name": "Bob"},
    {"@id": "ex:Charlie", "@type": "ex:Person"}
  ]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: Charlie has no name")
	}
	// Only Charlie should violate
	if len(report.Results) != 1 {
		t.Errorf("expected 1 violation (Charlie), got %d", len(report.Results))
	}
}

// --- sh:node (shape reference) ---

func TestJsonLD_NodeReference(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:AddressShape",
      "@type": "sh:NodeShape",
      "sh:property": [
        {"sh:path": {"@id": "ex:street"}, "sh:minCount": 1},
        {"sh:path": {"@id": "ex:city"}, "sh:minCount": 1}
      ]
    },
    {
      "@id": "ex:PersonShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:address"},
        "sh:node": {"@id": "ex:AddressShape"}
      }
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:Alice",
      "ex:address": {"@id": "ex:Addr1"}
    },
    {
      "@id": "ex:Addr1",
      "ex:street": "123 Main St",
      "ex:city": "Springfield"
    }
  ]
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_NodeReference_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:AddressShape",
      "@type": "sh:NodeShape",
      "sh:property": [
        {"sh:path": {"@id": "ex:street"}, "sh:minCount": 1},
        {"sh:path": {"@id": "ex:city"}, "sh:minCount": 1}
      ]
    },
    {
      "@id": "ex:PersonShape",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:address"},
        "sh:node": {"@id": "ex:AddressShape"}
      }
    }
  ]
}`)
	// Address missing city
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {"@id": "ex:Alice", "ex:address": {"@id": "ex:Addr1"}},
    {"@id": "ex:Addr1", "ex:street": "123 Main St"}
  ]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: address missing city")
	}
}

// --- Xone ---

func TestJsonLD_Xone(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:xone": {"@list": [
        {"sh:class": {"@id": "ex:Person"}},
        {"sh:class": {"@id": "ex:Animal"}}
      ]}
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming (exactly one of Person/Animal), got %d violations", len(report.Results))
	}
}

func TestJsonLD_Xone_Both_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:xone": {"@list": [
        {"sh:class": {"@id": "ex:Person"}},
        {"sh:class": {"@id": "ex:Employee"}}
      ]}
    }
  ]
}`)
	// Alice is both Person and Employee → xone fails
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": ["ex:Person", "ex:Employee"]
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: matches both shapes, xone requires exactly one")
	}
}

// --- LessThan ---

func TestJsonLD_LessThan(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:startDate"},
    "sh:lessThan": {"@id": "ex:endDate"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:startDate": {"@value": "2020-01-01", "@type": "xsd:date"},
  "ex:endDate": {"@value": "2020-12-31", "@type": "xsd:date"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

func TestJsonLD_LessThan_Violation(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:startDate"},
    "sh:lessThan": {"@id": "ex:endDate"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:startDate": {"@value": "2020-12-31", "@type": "xsd:date"},
  "ex:endDate": {"@value": "2020-01-01", "@type": "xsd:date"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: startDate > endDate")
	}
}

// --- QualifiedValueShape ---

func TestJsonLD_QualifiedValueShape(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:property": {
        "sh:path": {"@id": "ex:knows"},
        "sh:qualifiedValueShape": {"sh:class": {"@id": "ex:Person"}},
        "sh:qualifiedMinCount": 2
      }
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {"@id": "ex:Alice", "ex:knows": [{"@id": "ex:Bob"}, {"@id": "ex:Charlie"}]},
    {"@id": "ex:Bob", "@type": "ex:Person"},
    {"@id": "ex:Charlie", "@type": "ex:Person"}
  ]
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Severity ---

func TestJsonLD_Severity_Warning(t *testing.T) {
	t.Parallel()
	// sh:severity on a NodeShape with a direct constraint (not nested property).
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:datatype xsd:integer ;
    sh:severity sh:Warning ;
    sh:targetNode "Hello" .
`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected non-conforming even with sh:Warning severity")
	}
	if len(report.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !report.Results[0].ResultSeverity.Equal(SHWarning) {
		t.Errorf("expected sh:Warning severity, got %v", report.Results[0].ResultSeverity)
	}
}

// --- Empty data graph ---

func TestJsonLD_EmptyDataGraph(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:SomethingElse",
  "@type": "ex:Other"
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: target ex:Alice has no name in data")
	}
}

// --- No targets → conforming ---

func TestJsonLD_NoTargets(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetClass": {"@id": "ex:Person"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	// No Person instances in data
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Animal"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Error("expected conforming: no instances of ex:Person in data")
	}
}

// --- MinExclusive / MaxExclusive ---

func TestJsonLD_MinExclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:age"},
    "sh:minExclusive": {"@value": "0", "@type": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:age": {"@value": "0", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 0 is not > 0 (minExclusive)")
	}
}

func TestJsonLD_MaxExclusive(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:score"},
    "sh:maxExclusive": {"@value": "100", "@type": "xsd:integer"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:score": {"@value": "100", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 100 is not < 100 (maxExclusive)")
	}
}

// --- LanguageIn ---

func TestJsonLD_LanguageIn(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:label"},
    "sh:languageIn": {"@list": ["en", "de"]}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:label": {"@value": "Alicia", "@language": "es"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: 'es' not in languageIn [en, de]")
	}
}

func TestJsonLD_LanguageIn_Valid(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:label"},
    "sh:languageIn": {"@list": ["en", "de"]}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:label": {"@value": "Alice", "@language": "en"}
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Closed constraint ---

func TestJsonLD_Closed(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:closed": true,
      "sh:ignoredProperties": {"@list": [{"@id": "rdf:type"}]},
      "sh:property": [
        {"sh:path": {"@id": "ex:name"}}
      ]
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person",
  "ex:name": "Alice",
  "ex:age": {"@value": "30", "@type": "xsd:integer"}
}`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected violation: ex:age not allowed by closed shape")
	}
}

func TestJsonLD_Closed_Valid(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@graph": [
    {
      "@id": "ex:S",
      "@type": "sh:NodeShape",
      "sh:targetNode": {"@id": "ex:Alice"},
      "sh:closed": true,
      "sh:ignoredProperties": {"@list": [{"@id": "rdf:type"}]},
      "sh:property": [
        {"sh:path": {"@id": "ex:name"}}
      ]
    }
  ]
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "@type": "ex:Person",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- PropertyShape with multiple constraints ---

func TestJsonLD_PropertyMultipleConstraints(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:PersonShape",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1,
    "sh:maxCount": 3,
    "sh:minLength": 2,
    "sh:maxLength": 50,
    "sh:datatype": {"@id": "xsd:string"}
  }
}`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming, got %d violations", len(report.Results))
	}
}

// --- Mixed JSON-LD shapes with Turtle data and vice versa ---

func TestJsonLD_Shapes_With_Turtle_Data(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:S",
  "@type": "sh:NodeShape",
  "sh:targetNode": {"@id": "ex:Alice"},
  "sh:property": {
    "sh:path": {"@id": "ex:name"},
    "sh:minCount": 1
  }
}`)
	data := mustParseWithPrefixes(t, `
ex:Alice ex:name "Alice" .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming with JSON-LD shapes + Turtle data, got %d violations", len(report.Results))
	}
}

func TestJsonLD_Data_With_Turtle_Shapes(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:S a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`)
	data := mustParseJsonLD(t, `{
  "@context": `+jsonldContext+`,
  "@id": "ex:Alice",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming with Turtle shapes + JSON-LD data, got %d violations", len(report.Results))
	}
}

// --- LoadJsonLDFile / LoadJsonLD error cases ---

func TestLoadJsonLDString_Invalid(t *testing.T) {
	t.Parallel()
	_, err := LoadJsonLDString("not valid json", "http://example.org/")
	if err == nil {
		t.Error("expected error for invalid JSON-LD")
	}
}

func TestLoadJsonLDFile_NotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadJsonLDFile("/nonexistent/path/to/file.jsonld")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
