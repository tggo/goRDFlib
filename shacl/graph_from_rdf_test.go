package shacl

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

func TestNewGraphFromRDF(t *testing.T) {
	src := `
@prefix ex: <http://example.org/> .
ex:Alice a ex:Person ;
	ex:name "Alice" .
`
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(src)); err != nil {
		t.Fatalf("parse turtle: %v", err)
	}

	sg := NewGraphFromRDF(g, "http://example.org/")

	if got, want := sg.BaseURI(), "http://example.org/"; got != want {
		t.Errorf("BaseURI() = %q, want %q", got, want)
	}
	if got, want := sg.Len(), 2; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	alice := IRI("http://example.org/Alice")
	name := IRI("http://example.org/name")
	values := sg.Objects(alice, name)
	if len(values) != 1 || values[0].Value() != "Alice" {
		t.Errorf("Objects(Alice, name) = %v, want [Alice]", values)
	}

	// Mutating the wrapped graph directly must be picked up after an
	// explicit InvalidateIndexes call.
	g.Add(term.NewURIRefUnsafe("http://example.org/Alice"),
		term.NewURIRefUnsafe("http://example.org/age"),
		term.NewLiteral("30"))
	sg.InvalidateIndexes()

	age := IRI("http://example.org/age")
	values = sg.Objects(alice, age)
	if len(values) != 1 || values[0].Value() != "30" {
		t.Errorf("Objects(Alice, age) after mutation = %v, want [30]", values)
	}
}
