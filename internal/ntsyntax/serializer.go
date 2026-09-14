package ntsyntax

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ErrInvalidUTF8 is returned when an IRI or literal to be serialized is not
// valid UTF-8. N-Triples and N-Quads documents are UTF-8 (§4 media type
// registration), and neither UCHAR nor ECHAR can spell a byte that is not part
// of a code point.
var ErrInvalidUTF8 = errors.New("invalid UTF-8 cannot be written in N-Triples or N-Quads")

// Term serializes an RDF term to N-Triples syntax.
// Returns an error for unsupported term types instead of falling back to N3,
// and for terms the N-Triples grammar cannot express: relative IRIs
// (ErrRelativeIRI), IRIs containing characters IRIREF excludes (ErrInvalidIRI)
// and invalid UTF-8 (ErrInvalidUTF8).
func Term(t rdflibgo.Term) (string, error) {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return IRI(v.Value())
	case rdflibgo.BNode:
		return "_:" + v.Value(), nil
	case rdflibgo.Literal:
		return Literal(v)
	case rdflibgo.TripleTerm:
		return TripleTermStr(v)
	default:
		return "", fmt.Errorf("unsupported term type %T for N-Triples serialization", t)
	}
}

// TripleTermStr serializes a TripleTerm to N-Triples syntax: <<( <s> <p> <o> )>>.
func TripleTermStr(tt rdflibgo.TripleTerm) (string, error) {
	s, err := Term(tt.Subject())
	if err != nil {
		return "", err
	}
	p, err := Term(tt.Predicate())
	if err != nil {
		return "", err
	}
	o, err := Term(tt.Object())
	if err != nil {
		return "", err
	}
	return "<<( " + s + " " + p + " " + o + " )>>", nil
}

// IRI serializes an IRI as an N-Triples IRIREF, <...>.
//
// IRIREF ([^#x00-#x20<>"{}|^`\] | UCHAR)* excludes those characters raw, and
// writing them as UCHAR would not help: the result is still not an IRI
// (RFC 3987 excludes them too) and this module's parsers reject it. They
// are reported as ErrInvalidIRI instead.
func IRI(iri string) (string, error) {
	if !utf8.ValidString(iri) {
		return "", fmt.Errorf("%w: IRI %q; fix the data at its source", ErrInvalidUTF8, iri)
	}
	for i, r := range iri {
		if r <= 0x20 || isIRIREFExcluded(r) {
			return "", fmt.Errorf("%w: IRI %q has %q at byte %d, which no IRI may contain; percent-encode it (%%%02X) before serializing",
				ErrInvalidIRI, iri, r, i, r)
		}
	}
	if !isAbsoluteIRI(iri) {
		return "", fmt.Errorf("%w: <%s> has no scheme; resolve it against a base IRI before serializing, "+
			"or use Turtle, which can write relative IRIs with @base", ErrRelativeIRI, iri)
	}
	return "<" + EscapeIRI(iri) + ">", nil
}

func isIRIREFExcluded(r rune) bool {
	switch r {
	case '<', '>', '"', '{', '}', '|', '^', '`', '\\':
		return true
	}
	return false
}

// Literal serializes a Literal to N-Triples syntax. It fails with
// ErrInvalidUTF8 when the lexical form is not UTF-8, and with the IRI errors
// when the datatype cannot be written.
func Literal(l rdflibgo.Literal) (string, error) {
	if !utf8.ValidString(l.Lexical()) {
		return "", fmt.Errorf("%w: literal %q; fix the data at its source, or store the bytes as a base64 or hex literal", ErrInvalidUTF8, l.Lexical())
	}
	quoted := `"` + EscapeString(l.Lexical()) + `"`
	if l.Language() != "" {
		if l.Dir() != "" {
			return quoted + "@" + l.Language() + "--" + l.Dir(), nil
		}
		return quoted + "@" + l.Language(), nil
	}
	if l.Datatype() != (rdflibgo.URIRef{}) && l.Datatype() != rdflibgo.XSDString {
		dt, err := IRI(l.Datatype().Value())
		if err != nil {
			return "", fmt.Errorf("datatype of %q: %w", l.Lexical(), err)
		}
		return quoted + "^^" + dt, nil
	}
	return quoted, nil
}

// EscapeString escapes a string per N-Triples spec.
// Uses a fast path when no escaping is needed.
func EscapeString(s string) string {
	// Fast path: check if escaping is needed.
	needsEscape := false
	for _, r := range s {
		if r == '\\' || r == '"' || r == '\n' || r == '\r' || r == '\t' || r < 0x20 || r > 0xFFFF {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(`\u`)
				sb.WriteString(padHex4(uint64(r)))
			} else if r > 0xFFFF {
				sb.WriteString(`\U`)
				sb.WriteString(padHex8(uint64(r)))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

// EscapeIRI escapes an IRI per N-Triples spec.
// Escapes control characters, supplementary plane characters, and < > per W3C spec.
func EscapeIRI(s string) string {
	needsEscape := false
	for _, r := range s {
		if r < 0x20 || r > 0xFFFF || r == '<' || r == '>' {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == '<' || r == '>' {
			sb.WriteString(`\u`)
			sb.WriteString(padHex4(uint64(r)))
		} else if r > 0xFFFF {
			sb.WriteString(`\U`)
			sb.WriteString(padHex8(uint64(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// padHex4 formats a number as a 4-digit uppercase hex string without fmt.Sprintf.
func padHex4(n uint64) string {
	s := strconv.FormatUint(n, 16)
	for len(s) < 4 {
		s = "0" + s
	}
	return strings.ToUpper(s)
}

// padHex8 formats a number as an 8-digit uppercase hex string without fmt.Sprintf.
func padHex8(n uint64) string {
	s := strconv.FormatUint(n, 16)
	for len(s) < 8 {
		s = "0" + s
	}
	return strings.ToUpper(s)
}
