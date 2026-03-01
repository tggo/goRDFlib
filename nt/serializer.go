package nt

import (
	"fmt"
	"io"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Serialize writes the graph in N-Triples format.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	var lines []string
	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
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

func ntTriple(t rdflibgo.Triple) string {
	return ntTerm(t.Subject) + " " + ntTerm(t.Predicate) + " " + ntTerm(t.Object) + " ."
}

func ntTerm(t rdflibgo.Term) string {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return "<" + ntEscapeIRI(v.Value()) + ">"
	case rdflibgo.BNode:
		return "_:" + v.Value()
	case rdflibgo.Literal:
		return ntLiteral(v)
	default:
		return t.N3()
	}
}

func ntLiteral(l rdflibgo.Literal) string {
	escaped := ntEscapeString(l.Lexical())
	quoted := `"` + escaped + `"`
	if l.Language() != "" {
		return quoted + "@" + l.Language()
	}
	if l.Datatype() != (rdflibgo.URIRef{}) && l.Datatype() != rdflibgo.XSDString {
		return quoted + "^^<" + l.Datatype().Value() + ">"
	}
	return quoted
}

// ntEscapeString escapes a string per N-Triples spec.
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
