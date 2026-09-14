package turtle

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// nested builds a statement whose object nests one construct depth times.
func nested(kind string, depth int) string {
	var open, close string
	switch kind {
	case "property list":
		open, close = "[ :p ", " ]"
	case "collection":
		open, close = "( ", " )"
	case "triple term":
		open, close = "<<( :s :p ", " )>>"
	case "reified triple":
		open, close = "<< :s :p ", " >>"
	case "annotation":
		// Each annotation block holds a triple whose object carries the next.
		open, close = ":o {| :q ", " |}"
	}
	var sb strings.Builder
	sb.WriteString("@prefix : <http://example.org/> .\n:s :p ")
	for i := 0; i < depth; i++ {
		sb.WriteString(open)
	}
	sb.WriteString(":o")
	for i := 0; i < depth; i++ {
		sb.WriteString(close)
	}
	sb.WriteString(" .\n")
	return sb.String()
}

func parseNested(src string, opts ...Option) error {
	return Parse(rdflibgo.NewGraph(), strings.NewReader(src), opts...)
}

// TestNestingDepthIsBounded is the stack-overflow guard: "[ :p " repeated two
// million times used to crash the process with a fatal stack overflow, which
// no recover can catch. Every recursive construct must stop at the limit with
// ErrNestingTooDeep instead.
func TestNestingDepthIsBounded(t *testing.T) {
	for _, kind := range []string{"property list", "collection", "triple term", "reified triple", "annotation"} {
		t.Run(kind, func(t *testing.T) {
			if err := parseNested(nested(kind, 3), WithMaxParseDepth(3)); err != nil {
				t.Errorf("depth 3 with limit 3: %v", err)
			}
			err := parseNested(nested(kind, 4), WithMaxParseDepth(3))
			if !errors.Is(err, ErrNestingTooDeep) {
				t.Fatalf("depth 4 with limit 3: got %v, want ErrNestingTooDeep", err)
			}
			if !strings.Contains(err.Error(), "WithMaxParseDepth") || !strings.Contains(err.Error(), "line ") {
				t.Errorf("error must name the option and the position: %v", err)
			}
		})
	}

	t.Run("default limit", func(t *testing.T) {
		if err := parseNested(nested("property list", DefaultMaxParseDepth)); err != nil {
			t.Errorf("depth %d with the default limit: %v", DefaultMaxParseDepth, err)
		}
		if err := parseNested(nested("property list", DefaultMaxParseDepth+1)); !errors.Is(err, ErrNestingTooDeep) {
			t.Errorf("depth %d with the default limit: got %v, want ErrNestingTooDeep", DefaultMaxParseDepth+1, err)
		}
	})

	t.Run("untrusted input two million levels deep", func(t *testing.T) {
		src := "@prefix : <http://example.org/> .\n:s :p " + strings.Repeat("[ :p ", 2_000_000)
		if err := parseNested(src); !errors.Is(err, ErrNestingTooDeep) {
			t.Errorf("got %v, want ErrNestingTooDeep", err)
		}
	})
}
