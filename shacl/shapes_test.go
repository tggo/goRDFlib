package shacl

import (
	"strings"
	"testing"
)

const shapesPrefixes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

func loadShapes(t *testing.T, turtle string) map[string]*Shape {
	t.Helper()
	g, err := LoadTurtleString(shapesPrefixes+turtle, "http://example.org/")
	if err != nil {
		t.Fatalf("failed to parse turtle: %v", err)
	}
	return parseShapes(g)
}

func findShape(shapes map[string]*Shape, iriSubstring string) *Shape {
	for key, s := range shapes {
		if strings.Contains(key, iriSubstring) {
			return s
		}
	}
	return nil
}

func TestParseShapes_NodeShapeWithTargets(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:MyShape a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:targetClass ex:Person .
`)
	s := findShape(shapes, "MyShape")
	if s == nil {
		t.Fatal("expected to find MyShape")
	}
	if s.IsProperty {
		t.Error("expected NodeShape, got PropertyShape")
	}
	var hasTargetNode, hasTargetClass bool
	for _, tgt := range s.Targets {
		if tgt.Kind == TargetNode && strings.Contains(tgt.Value.Value(), "Alice") {
			hasTargetNode = true
		}
		if tgt.Kind == TargetClass && strings.Contains(tgt.Value.Value(), "Person") {
			hasTargetClass = true
		}
	}
	if !hasTargetNode {
		t.Error("missing targetNode for Alice")
	}
	if !hasTargetClass {
		t.Error("missing targetClass for Person")
	}
}

func TestParseShapes_PropertyShape(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:MyShape a sh:NodeShape ;
    sh:property [
        sh:path ex:name ;
    ] .
`)
	// Find the property shape (blank node with path)
	var found bool
	for _, s := range shapes {
		if s.IsProperty && s.Path != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find a PropertyShape with a path")
	}
}

func TestParseShapes_Deactivated(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:MyShape a sh:NodeShape ;
    sh:deactivated true ;
    sh:targetNode ex:Alice .
`)
	s := findShape(shapes, "MyShape")
	if s == nil {
		t.Fatal("expected to find MyShape")
	}
	if !s.Deactivated {
		t.Error("expected shape to be deactivated")
	}
}

func TestParseShapes_SeverityOverride(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:MyShape a sh:NodeShape ;
    sh:severity sh:Warning ;
    sh:targetNode ex:Alice .
`)
	s := findShape(shapes, "MyShape")
	if s == nil {
		t.Fatal("expected to find MyShape")
	}
	if !s.Severity.Equal(SHWarning) {
		t.Errorf("expected severity sh:Warning, got %v", s.Severity)
	}
}

func TestParseShapes_Closed(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:MyShape a sh:NodeShape ;
    sh:closed true ;
    sh:ignoredProperties ( rdf:type ) ;
    sh:targetNode ex:Alice .
`)
	s := findShape(shapes, "MyShape")
	if s == nil {
		t.Fatal("expected to find MyShape")
	}
	if !s.Closed {
		t.Error("expected shape to be closed")
	}
	if len(s.IgnoredProperties) != 1 {
		t.Fatalf("expected 1 ignored property, got %d", len(s.IgnoredProperties))
	}
	if !strings.Contains(s.IgnoredProperties[0].Value(), "type") {
		t.Errorf("expected rdf:type in ignored properties, got %v", s.IgnoredProperties[0])
	}
}

func TestParseShapes_ImplicitClassTarget(t *testing.T) {
	t.Parallel()
	shapes := loadShapes(t, `
ex:Person a rdfs:Class, sh:NodeShape ;
    sh:property [
        sh:path ex:name ;
    ] .
`)
	s := findShape(shapes, "Person")
	if s == nil {
		t.Fatal("expected to find Person shape")
	}
	var hasImplicit bool
	for _, tgt := range s.Targets {
		if tgt.Kind == TargetImplicitClass {
			hasImplicit = true
		}
	}
	if !hasImplicit {
		t.Error("expected implicit class target")
	}
}
