package results

import (
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// checkTerm reports the error the encoder for f would return for t, without
// encoding. The one-shot writers run it over the whole result first so that
// a bad value is found before any byte is written. It must reject exactly
// what the encoders reject; FuzzWriters checks that.
func checkTerm(f Format, t rdflibgo.Term) error {
	switch t := t.(type) {
	case rdflibgo.URIRef:
		return checkIRI(f, t.Value())
	case rdflibgo.BNode:
		return nil // relabelled, the source label is never written
	case rdflibgo.Literal:
		if err := checkString(f, t.Lexical()); err != nil {
			return err
		}
		if t.Language() != "" {
			if err := checkString(f, t.Language()); err != nil {
				return err
			}
			return checkString(f, t.Dir())
		}
		if f != FormatCSV && hasDatatype(t) {
			return checkIRI(f, t.Datatype().Value())
		}
		return nil
	case rdflibgo.TripleTerm:
		if t.Subject() == nil || t.Object() == nil {
			return fmt.Errorf("%w: triple term with a nil component", ErrUnsupportedTerm)
		}
		if err := checkTerm(f, t.Subject()); err != nil {
			return err
		}
		if err := checkTerm(f, t.Predicate()); err != nil {
			return err
		}
		return checkTerm(f, t.Object())
	}
	return fmt.Errorf("%w: %T", ErrUnsupportedTerm, t)
}

func checkIRI(f Format, s string) error {
	if err := checkString(f, s); err != nil {
		return err
	}
	if f == FormatTSV && !rdflibgo.ValidIRI(s) {
		return unrepresentableIRI(s)
	}
	return nil
}

func unrepresentableIRI(s string) error {
	return fmt.Errorf("%w: %q", ErrUnrepresentableIRI, s)
}

func checkString(f Format, s string) error {
	if f != FormatXML {
		if !utf8.ValidString(s) {
			return invalidUTF8(s)
		}
		return nil
	}
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return invalidUTF8(s)
		}
		if !isXMLChar(r) {
			return unrepresentableXMLChar(r)
		}
		i += size
	}
	return nil
}

func invalidUTF8(s string) error {
	return fmt.Errorf("%w: %q", ErrInvalidUTF8, s)
}

func unrepresentableXMLChar(r rune) error {
	return fmt.Errorf("%w: U+%04X (serve the result as JSON or TSV instead)", ErrUnrepresentableXMLChar, r)
}

// isXMLChar reports whether r matches the XML 1.0 Char production:
// #x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF].
func isXMLChar(r rune) bool {
	switch {
	case r == 0x9, r == 0xA, r == 0xD:
		return true
	case r >= 0x20 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	}
	return false
}

// hasDatatype reports whether a literal without a language tag carries a
// datatype worth writing: not xsd:string (a simple literal) and not empty.
func hasDatatype(l rdflibgo.Literal) bool {
	dt := l.Datatype()
	return dt.Value() != "" && dt != rdflibgo.XSDString
}

// validVarName reports whether s matches the SPARQL 1.1 VARNAME production
// [166]: ( PN_CHARS_U | [0-9] ) ( PN_CHARS_U | [0-9] | #x00B7 |
// [#x0300-#x036F] | [#x203F-#x2040] )*.
func validVarName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == utf8.RuneError {
			if _, size := utf8.DecodeRuneInString(s[i:]); size == 1 {
				return false
			}
		}
		if r == '_' || (r >= '0' && r <= '9') || isPNCharsBase(r) {
			continue
		}
		if i > 0 && (r == 0xB7 || (r >= 0x300 && r <= 0x36F) || r == 0x203F || r == 0x2040) {
			continue
		}
		return false
	}
	return true
}

// isPNCharsBase implements the SPARQL PN_CHARS_BASE production [164].
func isPNCharsBase(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
	case r >= 0xC0 && r <= 0xD6, r >= 0xD8 && r <= 0xF6, r >= 0xF8 && r <= 0x2FF:
	case r >= 0x370 && r <= 0x37D, r >= 0x37F && r <= 0x1FFF, r >= 0x200C && r <= 0x200D:
	case r >= 0x2070 && r <= 0x218F, r >= 0x2C00 && r <= 0x2FEF, r >= 0x3001 && r <= 0xD7FF:
	case r >= 0xF900 && r <= 0xFDCF, r >= 0xFDF0 && r <= 0xFFFD, r >= 0x10000 && r <= 0xEFFFF:
	default:
		return false
	}
	return true
}
