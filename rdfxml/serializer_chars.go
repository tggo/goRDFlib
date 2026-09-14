package rdfxml

import (
	"errors"
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ErrUnrepresentableChar is returned by Serialize when an IRI or literal
// contains a character that XML 1.0 cannot carry at all. XML 1.0 §2.2 [2]
// allows only #x9, #xA, #xD, #x20-#xD7FF, #xE000-#xFFFD and #x10000-#x10FFFF,
// and §4.1 forbids character references to anything else, so U+0000-U+0008,
// U+000B, U+000C, U+000E-U+001F, U+FFFE, U+FFFF and invalid UTF-8 have no
// spelling. encoding/xml would silently replace them with U+FFFD.
var ErrUnrepresentableChar = errors.New("rdfxml: character cannot be represented in XML 1.0")

// checkXMLChars returns an ErrUnrepresentableChar error when s has a
// character outside the XML 1.0 Char production. what names the value for the
// error message.
func checkXMLChars(what, s string) error {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			return fmt.Errorf("%w: %s has invalid UTF-8 at byte %d (%q); "+
				"fix the data, or serialize as N-Triples, Turtle or JSON-LD", ErrUnrepresentableChar, what, i, s)
		}
		if !isXMLChar(r) {
			return fmt.Errorf("%w: %s contains U+%04X at byte %d (%q), which XML 1.0 cannot carry even as a character reference; "+
				"remove the character, or serialize as N-Triples, Turtle or JSON-LD, which can escape it", ErrUnrepresentableChar, what, r, i, s)
		}
		i += size
	}
	return nil
}

func isXMLChar(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD || r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF
}

// checkObjectChars validates every string of obj that goes into the output.
// Triple term components are checked when they are written.
func checkObjectChars(pred rdflibgo.URIRef, obj rdflibgo.Term) error {
	ctx := "object of <" + pred.Value() + ">"
	switch o := obj.(type) {
	case rdflibgo.URIRef:
		return checkXMLChars("IRI "+ctx, o.Value())
	case rdflibgo.Literal:
		if err := checkXMLChars("literal "+ctx, o.Lexical()); err != nil {
			return err
		}
		if err := checkXMLChars("language tag of "+ctx, o.Language()+o.Dir()); err != nil {
			return err
		}
		return checkXMLChars("datatype of "+ctx, o.Datatype().Value())
	}
	return nil
}
