package ntsyntax

import (
	"fmt"
	"strconv"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Term serializes an RDF term to N-Triples syntax.
// Returns an error for unsupported term types instead of falling back to N3.
func Term(t rdflibgo.Term) (string, error) {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return "<" + EscapeIRI(v.Value()) + ">", nil
	case rdflibgo.BNode:
		return "_:" + v.Value(), nil
	case rdflibgo.Literal:
		return Literal(v), nil
	default:
		return "", fmt.Errorf("unsupported term type %T for N-Triples serialization", t)
	}
}

// Literal serializes a Literal to N-Triples syntax.
func Literal(l rdflibgo.Literal) string {
	escaped := EscapeString(l.Lexical())
	quoted := `"` + escaped + `"`
	if l.Language() != "" {
		return quoted + "@" + l.Language()
	}
	if l.Datatype() != (rdflibgo.URIRef{}) && l.Datatype() != rdflibgo.XSDString {
		return quoted + "^^<" + EscapeIRI(l.Datatype().Value()) + ">"
	}
	return quoted
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
