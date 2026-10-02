package jsonld

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/provenance"
)

// Invalid JSON used to come back as encoding/json's bare message ("invalid
// character '/' looking for beginning of object key string") with no sentinel
// and no position. It is ErrInvalidJSON now, with the line and column.
func TestParseInvalidJSONIsActionable(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"comment (settings.json style)", "{\n  // a comment\n  \"@id\": \"urn:x\"\n}", "line 2, column 3"},
		{"trailing comma", "{\n  \"@id\": \"urn:x\",\n}", "line 3, column 1"},
		{"garbage on line 1", "@@", "line 1, column 1"},
		{"empty", "", "the input is empty"},
		{"whitespace only", "  \n\t ", "the input is empty"},
		{"truncated", "{\n  \"@id\": \"urn:x\"", "is it truncated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, opts := range [][]Option{nil, {WithProvenance(provenance.NewIndex().Triple)}} {
				err := Parse(rdflibgo.NewGraph(), strings.NewReader(tc.src), opts...)
				if !errors.Is(err, ErrInvalidJSON) {
					t.Fatalf("err = %v, want ErrInvalidJSON", err)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %q, want it to mention %q", err, tc.want)
				}
			}
		})
	}
}

// A syntax error still unwraps to encoding/json's own error for callers that
// want the raw offset.
func TestParseInvalidJSONUnwrapsSyntaxError(t *testing.T) {
	err := Parse(rdflibgo.NewGraph(), strings.NewReader(`{"a" 1}`))
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("err = %v, want it to wrap *json.SyntaxError", err)
	}
}

// A reader that fails is a read error, not invalid JSON, and keeps the cause.
func TestParseReadErrorIsNotInvalidJSON(t *testing.T) {
	boom := errors.New("disk on fire")
	for _, opts := range [][]Option{nil, {WithProvenance(provenance.NewIndex().Triple)}} {
		err := Parse(rdflibgo.NewGraph(), iotest.ErrReader(boom), opts...)
		if errors.Is(err, ErrInvalidJSON) {
			t.Fatalf("read failure reported as invalid JSON: %v", err)
		}
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want it to wrap the reader's error", err)
		}
	}
}

// The line counter must not change what a valid document parses to.
func TestParseValidJSONUnaffected(t *testing.T) {
	g := rdflibgo.NewGraph()
	src := "{\n\"@id\": \"urn:s\",\n\"urn:p\": \"v\"\n}\n"
	if err := Parse(g, iotest.OneByteReader(strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Fatalf("len = %d, want 1", g.Len())
	}
}
