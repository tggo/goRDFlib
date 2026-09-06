package shacl

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/turtle"
)

// TestLoadBlankNodeScope covers all file, reader and string entry points. Merging
// separately loaded documents must retain their independent blank-node identities.
func TestLoadBlankNodeScope(t *testing.T) {
	t.Run("turtle", func(t *testing.T) {
		checkLoadBlankNodeScope(t, `_:source <http://example.org/p> _:source .`, turtle.WithPreserveBlankNodeIDs(), LoadTurtleFile, LoadTurtle, LoadTurtleString)
	})
	t.Run("nq", func(t *testing.T) {
		checkLoadBlankNodeScope(t, `_:source <http://example.org/p> _:source <http://example.org/g> .`, nq.WithPreserveBlankNodeIDs(), LoadNQuadsFile, LoadNQuads, LoadNQuadsString)
	})
	t.Run("jsonld", func(t *testing.T) {
		checkLoadBlankNodeScope(t, `{"@id":"_:source","http://example.org/p":{"@id":"_:source"}}`, jsonld.WithPreserveBlankNodeIDs(), LoadJsonLDFile, LoadJsonLD, LoadJsonLDString)
	})
}

// checkLoadBlankNodeScope exercises the same identity contract for each typed
// parser option. JSON-LD preserves expansion IDs, so it does not assert source labels.
func checkLoadBlankNodeScope[O any](t *testing.T, data string, preserve O,
	loadFile func(string, ...O) (*Graph, error),
	loadReader func(io.Reader, string, ...O) (*Graph, error),
	loadString func(string, string, ...O) (*Graph, error),
) {
	t.Helper()
	const base = "http://example.org/source"
	path := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		name string
		load func(...O) (*Graph, error)
	}{
		{"file", func(opts ...O) (*Graph, error) { return loadFile(path, opts...) }},
		{"reader", func(opts ...O) (*Graph, error) { return loadReader(strings.NewReader(data), base, opts...) }},
		{"string", func(opts ...O) (*Graph, error) { return loadString(data, base, opts...) }},
	} {
		t.Run(entry.name, func(t *testing.T) {
			for _, keep := range []bool{false, true} {
				name := "default"
				var opts []O
				if keep {
					name = "preserve"
					opts = []O{preserve}
				}
				t.Run(name, func(t *testing.T) {
					first, err := entry.load(opts...)
					if err != nil {
						t.Fatal(err)
					}
					second, err := entry.load(opts...)
					if err != nil {
						t.Fatal(err)
					}
					p := IRI("http://example.org/p")
					var subjects [2]Term
					for i, g := range []*Graph{first, second} {
						if g.Len() != 1 {
							t.Fatalf("load %d: got %d triples, want 1", i, g.Len())
						}
						for _, triple := range g.All(nil, &p, nil) {
							subjects[i] = triple.Subject
							if !triple.Subject.IsBlank() || !triple.Subject.Equal(triple.Object) {
								t.Fatalf("blank-node identity within document lost: %v", triple)
							}
						}
					}
					if subjects[0].Equal(subjects[1]) != keep {
						t.Fatalf("cross-load identity: %v and %v, preserve=%v", subjects[0], subjects[1], keep)
					}
					first.Merge(second)
					want := 2
					if keep {
						want = 1
					}
					if first.Len() != want {
						t.Fatalf("merged graph: got %d triples, want %d", first.Len(), want)
					}
				})
			}
		})
	}
}

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
