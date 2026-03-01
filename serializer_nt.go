package rdflibgo

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// NTriplesSerializer serializes a Graph to N-Triples format.
// Ported from: rdflib.plugins.serializers.nt.NTSerializer
type NTriplesSerializer struct{}

func init() {
	RegisterSerializer("nt", func() Serializer { return &NTriplesSerializer{} })
	RegisterSerializer("ntriples", func() Serializer { return &NTriplesSerializer{} })
}

func (s *NTriplesSerializer) Serialize(g *Graph, w io.Writer, base string) error {
	var lines []string
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		lines = append(lines, ntTriple(t))
		return true
	})
	slices.Sort(lines)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func ntTriple(t Triple) string {
	return ntTerm(t.Subject) + " " + ntTerm(t.Predicate) + " " + ntTerm(t.Object) + " ."
}

func ntTerm(t Term) string {
	switch v := t.(type) {
	case URIRef:
		return "<" + ntEscapeIRI(v.Value()) + ">"
	case BNode:
		return "_:" + v.Value()
	case Literal:
		return ntLiteral(v)
	default:
		return t.N3()
	}
}

func ntLiteral(l Literal) string {
	escaped := ntEscapeString(l.Lexical())
	quoted := `"` + escaped + `"`
	if l.Language() != "" {
		return quoted + "@" + l.Language()
	}
	if l.Datatype() != (URIRef{}) && l.Datatype() != XSDString {
		return quoted + "^^<" + l.Datatype().Value() + ">"
	}
	return quoted
}

// ntEscapeString escapes a string per N-Triples spec.
// Ported from: rdflib.plugins.serializers.nt — _nt_unicode_error_resolver
func ntEscapeString(s string) string {
	var sb strings.Builder
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
				sb.WriteString(fmt.Sprintf(`\u%04X`, r))
			} else if r > 0xFFFF {
				sb.WriteString(fmt.Sprintf(`\U%08X`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

func ntEscapeIRI(s string) string {
	// IRIs in N-Triples: escape control chars and supplementary plane per W3C spec.
	needsEscape := false
	for _, r := range s {
		if r < 0x20 || r > 0xFFFF {
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
		if r < 0x20 {
			sb.WriteString(fmt.Sprintf(`\u%04X`, r))
		} else if r > 0xFFFF {
			sb.WriteString(fmt.Sprintf(`\U%08X`, r))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
