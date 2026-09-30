package trig_test

import (
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/trig"
)

// IRIREF excludes #x00-#x20 and <>"{}|^`\ (Turtle 1.1 [18]), raw or written
// as a UCHAR. The parser once let '"' through, raw and escaped, and '\' via
// \u005C, producing IRIs the serializer then refused to write.
func TestIRIRejectsExcludedCharacters(t *testing.T) {
	positions := []string{
		"<urn:s%s> <urn:p> <urn:o> .",
		"<urn:s> <urn:p%s> <urn:o> .",
		"<urn:s> <urn:p> <urn:o%s> .",
		"<urn:s> <urn:p> \"v\"^^<urn:d%s> .",
	}
	chars := []string{"\"", "{", "}", "|", "^", "`", " ", "\\u0022", "\\u005C", "\\u003C", "\\u0020"}
	for _, pos := range positions {
		for _, c := range chars {
			doc := fmt.Sprintf(pos, "x"+c+"y")
			if err := trig.Parse(rdflibgo.NewGraph(), strings.NewReader(doc)); err == nil {
				t.Errorf("accepted %s", doc)
			}
		}
	}
	// Allowed: percent-encoding and a UCHAR for an ordinary character.
	for _, ok := range []string{"<urn:x%22y> <urn:p> <urn:o> .", "<urn:x\\u0041y> <urn:p> <urn:o> ."} {
		if err := trig.Parse(rdflibgo.NewGraph(), strings.NewReader(ok)); err != nil {
			t.Errorf("rejected %s: %v", ok, err)
		}
	}
}
