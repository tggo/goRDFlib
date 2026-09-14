package term

import (
	"errors"
	"testing"
)

// rdflib #625: NewURIRef accepted a TAB, and the serializer then wrote an
// IRIREF that Turtle 1.1 [18] forbids.
func TestNewURIRefRejectsControlCharacters(t *testing.T) {
	for _, s := range []string{"http://e/a\tb", "http://e/a\nb", "http://e/a\x00b", "http://e/a\x1fb", "http://e/a b"} {
		if _, err := NewURIRef(s); !errors.Is(err, ErrInvalidIRI) {
			t.Errorf("NewURIRef(%q) err = %v, want ErrInvalidIRI", s, err)
		}
	}
	if _, err := NewURIRef("http://e/é x"); err != nil {
		t.Errorf("non-ASCII IRI rejected: %v", err)
	}
}
