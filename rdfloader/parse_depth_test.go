package rdfloader

import (
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/trig"
	"github.com/tggo/goRDFlib/turtle"
)

// WithMaxParseDepth must reach the Turtle and TriG parsers; a limit the loader
// cannot pass on is a debugging dead end.
func TestWithMaxParseDepthIsForwarded(t *testing.T) {
	nested := "@prefix : <http://e/> . :s :p " + strings.Repeat("[ :p ", 5) + ":o" + strings.Repeat(" ]", 5) + " ."
	l := &defaultLoader{}
	WithMaxParseDepth(3)(l)

	cases := map[string]error{"turtle": turtle.ErrNestingTooDeep, "trig": trig.ErrNestingTooDeep}
	for format, want := range cases {
		err := l.parseFormat(graph.NewGraph(), strings.NewReader(nested), format)
		if !errors.Is(err, want) {
			t.Errorf("%s with depth 3: err = %v, want %v", format, err, want)
		}
		if err := (&defaultLoader{}).parseFormat(graph.NewGraph(), strings.NewReader(nested), format); err != nil {
			t.Errorf("%s with the default depth: %v", format, err)
		}
	}
}
