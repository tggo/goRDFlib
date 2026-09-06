package jsonld

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

func fuzzSeeds(f *testing.F) {
	f.Add(`{"@id": "http://example.org/a", "http://example.org/p": "v"}`)
	f.Add(`{"@context": {"ex": "http://example.org/"}, "@id": "ex:a", "ex:p": "v"}`)
	f.Add(`{"@context": {"id": "@id"}, "id": "http://example.org/a"}`)
	f.Add(`[{"@id": "a"}, {"@id": "b"}]`)
	f.Add(`{"@graph": [{"@id": "_:b"}]}`)
	f.Add(`{"@id": {"nested": "object"}}`)
	f.Add(`{"@id": 42}`)
	f.Add(`{"@id": null}`)
	f.Add(`{"@id": ["array"]}`)
	f.Add(`{`)
	f.Add(`[`)
	f.Add(``)
	f.Add(`null`)
	f.Add(`"@id"`)
	f.Add(`{"@context": [{"id": "@id"}, {"ex": "http://example.org/"}]}`)
	f.Add(`{"@context": {"id": {"@id": "@id"}}, "id": "x"}`)
	f.Add("{\n\n\n\"@id\": \"x\"\n}")
}

// FuzzScanIDPositions checks the two properties the position scan has to hold
// on any input, including input that is not valid JSON at all: it must not
// panic, and every line it reports must exist in the document.
//
// A scan that walks off the end would put a line number in a diagnostic that
// points past the file, which is worse than reporting nothing.
func FuzzScanIDPositions(f *testing.F) {
	fuzzSeeds(f)

	f.Fuzz(func(t *testing.T, src string) {
		positions := scanIDPositions([]byte(src), nil)

		lineCount := 1 + bytes.Count([]byte(src), []byte("\n"))
		for id, line := range positions {
			if line < 1 || line > lineCount {
				t.Fatalf("id %q reported line %d, outside the %d-line document", id, line, lineCount)
			}
			// The scan may only report identifiers that were actually
			// written. The check is limited to valid UTF-8 because
			// encoding/json substitutes U+FFFD for invalid bytes while
			// decoding, so the decoded id legitimately differs from the raw
			// source there — and json-gold, decoding the same way, sees the
			// same substitution.
			if utf8.ValidString(src) && !strings.Contains(src, id) {
				t.Fatalf("id %q was reported but does not occur in the source", id)
			}
		}
	})
}

// FuzzLineIndex checks the offset-to-line mapping against the obvious slow
// implementation, since every reported line goes through it.
func FuzzLineIndex(f *testing.F) {
	f.Add("a\nb\nc", 0)
	f.Add("", 0)
	f.Add("\n\n\n", 2)
	f.Add("no newlines at all", 5)
	f.Add("\n", 1)

	f.Fuzz(func(t *testing.T, src string, offset int) {
		if offset < 0 {
			offset = -offset
		}
		if len(src) > 0 {
			offset %= len(src) + 1
		} else {
			offset = 0
		}

		got := newLineIndex([]byte(src)).at(int64(offset))

		want := 1
		for i := 0; i < offset && i < len(src); i++ {
			if src[i] == '\n' {
				want++
			}
		}
		if got != want {
			t.Fatalf("at(%d) = %d, want %d for %q", offset, got, want, src)
		}
	})
}

// FuzzParseProvenance runs the whole path on arbitrary input. A document that
// does not parse must simply produce no triples and no lines; one that does must
// never report a line outside the source, and never report one for a subject
// the source does not name.
func FuzzParseProvenance(f *testing.F) {
	fuzzSeeds(f)

	f.Fuzz(func(t *testing.T, src string) {
		g := graph.NewGraph()

		type record struct {
			subject term.Term
			line    int
		}
		var seen []record

		err := Parse(g, strings.NewReader(src), WithBase("http://example.org/"),
			WithProvenance(func(s rdflibgo.Subject, _ rdflibgo.URIRef, _ rdflibgo.Term, line int) {
				seen = append(seen, record{subject: s, line: line})
			}))
		if err != nil {
			// A malformed document is reported, not reported *about*.
			if len(seen) != 0 {
				t.Fatalf("parse failed with %v but still reported %d lines", err, len(seen))
			}
			return
		}

		lineCount := 1 + strings.Count(src, "\n")
		for _, r := range seen {
			if r.line < 1 || r.line > lineCount {
				t.Fatalf("subject %s reported line %d, outside the %d-line document",
					r.subject.N3(), r.line, lineCount)
			}
			// Only a subject the graph actually holds may be reported.
			if s, ok := r.subject.(term.Subject); ok {
				found := false
				g.Triples(s, nil, nil)(func(term.Triple) bool {
					found = true
					return false
				})
				if !found {
					t.Fatalf("a line was reported for %s, which is in no triple", r.subject.N3())
				}
			}
		}
	})
}
