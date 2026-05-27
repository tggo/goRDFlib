package shacl

import (
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/nq"
)

// TestLoadNQuadsString_UnboundedLinesOptIn mirrors the JSON-LD opt-in test for the
// direct N-Quads path: a multi-megabyte literal on one line must fail with the
// default cap and succeed once nq.WithUnboundedLines is forwarded.
func TestLoadNQuadsString_UnboundedLinesOptIn(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 5*1024*1024)
	data := `<http://example.org/s> <http://example.org/p> "` + big + `" .` + "\n"

	if _, err := LoadNQuadsString(data, "http://example.org/"); err == nil {
		t.Fatal("expected large N-Quads line to fail without WithUnboundedLines")
	}

	g, err := LoadNQuadsString(data, "http://example.org/", nq.WithUnboundedLines())
	if err != nil {
		t.Fatalf("unexpected error with WithUnboundedLines: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestLoadNQuadsString_ActionableError confirms the debugging payoff: the default
// failure surfaces nq.ErrLineTooLong with a line number, not an opaque bufio error.
func TestLoadNQuadsString_ActionableError(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 100*1024)
	data := `<http://example.org/s> <http://example.org/p> "` + big + `" .` + "\n"

	_, err := LoadNQuadsString(data, "http://example.org/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, nq.ErrLineTooLong) {
		t.Fatalf("expected errors.Is(err, nq.ErrLineTooLong), got: %v", err)
	}
	if !strings.Contains(err.Error(), "WithUnboundedLines") {
		t.Errorf("expected remediation hint in error, got: %v", err)
	}
}

// TestLoadNQuads_MaxLineLengthOptIn verifies the bounded option also forwards.
func TestLoadNQuads_MaxLineLengthOptIn(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 100*1024)
	data := `<http://example.org/s> <http://example.org/p> "` + big + `" .` + "\n"

	g, err := LoadNQuads(strings.NewReader(data), "http://example.org/", nq.WithMaxLineLength(1<<20))
	if err != nil {
		t.Fatalf("unexpected error with WithMaxLineLength(1MB): %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestLoadTurtleString_VariadicOpts ensures the Turtle loader accepts and applies
// forwarded options without breaking the no-option call.
func TestLoadTurtleString_VariadicOpts(t *testing.T) {
	t.Parallel()
	data := `@prefix ex: <http://example.org/> . ex:Alice ex:knows ex:Bob .`

	if _, err := LoadTurtleString(data, "http://example.org/"); err != nil {
		t.Fatalf("no-option call failed: %v", err)
	}
}
