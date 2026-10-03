package plugin_test

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/plugin"
)

// Issue #43: Turtle without a leading directive used to be sniffed as N-Triples.
func TestFormatFromContentTurtleWithoutDirective(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"prefixed predicate", `<http://ex/s> ex:p <http://ex/o> .`, "turtle"},
		{"semicolon", "<http://ex/s> <http://ex/p> <http://ex/o> ;\n  <http://ex/q> \"x\" .", "turtle"},
		{"comma", `<http://ex/s> <http://ex/p> <http://ex/o>, <http://ex/o2> .`, "turtle"},
		{"a keyword", `<http://ex/s> a <http://ex/C> .`, "turtle"},
		{"blank node property list", `<http://ex/s> <http://ex/p> [ <http://ex/q> 1 ] .`, "turtle"},
		{"anonymous subject", `[ <http://ex/p> <http://ex/o> ] .`, "turtle"},
		{"collection", `<http://ex/s> <http://ex/p> ( <http://ex/a> ) .`, "turtle"},
		{"long string", `<http://ex/s> <http://ex/p> """multi""" .`, "turtle"},
		{"single quotes", `<http://ex/s> <http://ex/p> 'x' .`, "turtle"},
		{"number", `<http://ex/s> <http://ex/p> 42 .`, "turtle"},
		{"prefixed subject", `ex:s ex:p ex:o .`, "turtle"},
		{"lowercase prefix", "prefix ex: <http://ex/>\nex:s ex:p ex:o .", "turtle"},
		{"directive after comment", "# header\n@prefix ex: <http://ex/> .", "turtle"},
		{"directive later", "<http://ex/s> <http://ex/p> <http://ex/o> .\n@prefix ex: <http://ex/> .", "turtle"},
		{"turtle after many nt lines", strings.Repeat("<http://ex/s> <http://ex/p> <http://ex/o> .\n", 20) + "<http://ex/s> a <http://ex/C> .", "turtle"},

		{"nt with lang and datatype", "<http://ex/s> <http://ex/p> \"a\"@en-GB .\n<http://ex/s> <http://ex/p> \"1\"^^<http://www.w3.org/2001/XMLSchema#integer> .", "nt"},
		{"nt escaped quote", `<http://ex/s> <http://ex/p> "say \"a; b, c\"" .`, "nt"},
		{"nt bnodes no spaces", `_:b1 <http://ex/p> _:b2.`, "nt"},
		{"nt after comment", "# a comment\n<http://ex/s> <http://ex/p> <http://ex/o> .", "nt"},
		{"nt comment with turtle-ish text", "<http://ex/s> <http://ex/p> <http://ex/o> . # ex:x ; a ,", "nt"},
		{"nt triple term", `<http://ex/s> <http://ex/p> <<( <http://ex/a> <http://ex/b> <http://ex/c> )>> .`, "nt"},
		{"nquads", `<http://ex/s> <http://ex/p> "o"@en <http://ex/g> .`, "nquads"},
		{"nquads default graph first", "<http://ex/s> <http://ex/p> <http://ex/o> .\n<http://ex/s> <http://ex/p> <http://ex/o> <http://ex/g> .", "nquads"},
		{"empty bnode subject", `[] <http://ex/p> <http://ex/o> .`, "turtle"},
		{"nquads bnode graph", `_:s <http://ex/p> _:o _:g .`, "nquads"},

		{"json-ld array", `[ {"@id": "x"} ]`, "json-ld"},
		{"empty json array", `[]`, "json-ld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := plugin.FormatFromContent([]byte(tt.content))
			if !ok || got != tt.want {
				t.Errorf("FormatFromContent = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}
}

// A cut-off IRI or string at the sniff limit must not decide the format.
func TestFormatFromContentTruncatedAtLimit(t *testing.T) {
	line := "<http://ex/s> <http://ex/p> \"" + strings.Repeat("x", 5000) + "\" ."
	if got, ok := plugin.FormatFromContent([]byte(line)); !ok || got != "nt" {
		t.Errorf("got %q, %v; want nt", got, ok)
	}
}

func TestFormatFromMIMEGeneric(t *testing.T) {
	for _, ct := range []string{"text/plain", "text/plain; charset=utf-8", "application/octet-stream", ""} {
		if f, ok := plugin.FormatFromMIME(ct); ok {
			t.Errorf("FormatFromMIME(%q) = %q; want no format", ct, f)
		}
	}
}
