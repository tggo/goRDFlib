package results

import (
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

func (w *Writer) tsvRow(buf []byte, row map[string]rdflibgo.Term) ([]byte, error) {
	var err error
	for i, v := range w.vars {
		if i > 0 {
			buf = append(buf, '\t')
		}
		t := row[v]
		if t == nil {
			continue // unbound: empty field
		}
		if buf, err = w.appendTSVTerm(buf, t); err != nil {
			return buf, fmt.Errorf("variable ?%s: %w", v, err)
		}
	}
	return append(buf, '\n'), nil
}

// appendTSVTerm appends t in the TSV encoding (SPARQL 1.2 CSV/TSV §3), which
// is N-Triples term syntax plus Turtle's unquoted numbers.
func (w *Writer) appendTSVTerm(buf []byte, t rdflibgo.Term) ([]byte, error) {
	var err error
	switch t := t.(type) {
	case rdflibgo.URIRef:
		return appendTSVIRI(buf, t.Value())
	case rdflibgo.BNode:
		buf = append(buf, '_', ':')
		return w.appendBNodeLabel(buf, t), nil
	case rdflibgo.Literal:
		lex := t.Lexical()
		if t.Language() == "" && isTurtleNumber(t.Datatype(), lex) {
			return append(buf, lex...), nil
		}
		if buf, err = appendTSVString(buf, lex); err != nil {
			return buf, err
		}
		if t.Language() != "" {
			buf = append(buf, '@')
			if buf, err = appendValidUTF8(buf, t.Language()); err != nil {
				return buf, err
			}
			if t.Dir() != "" {
				buf = append(buf, '-', '-')
				return appendValidUTF8(buf, t.Dir())
			}
			return buf, nil
		}
		if hasDatatype(t) {
			buf = append(buf, '^', '^')
			return appendTSVIRI(buf, t.Datatype().Value())
		}
		return buf, nil
	case rdflibgo.TripleTerm:
		if t.Subject() == nil || t.Object() == nil {
			return buf, fmt.Errorf("%w: triple term with a nil component", ErrUnsupportedTerm)
		}
		buf = append(buf, "<<( "...)
		if buf, err = w.appendTSVTerm(buf, t.Subject()); err != nil {
			return buf, err
		}
		buf = append(buf, ' ')
		if buf, err = w.appendTSVTerm(buf, t.Predicate()); err != nil {
			return buf, err
		}
		buf = append(buf, ' ')
		if buf, err = w.appendTSVTerm(buf, t.Object()); err != nil {
			return buf, err
		}
		return append(buf, " )>>"...), nil
	}
	return buf, fmt.Errorf("%w: %T", ErrUnsupportedTerm, t)
}

func appendTSVIRI(buf []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return buf, invalidUTF8(s)
	}
	if !rdflibgo.ValidIRI(s) {
		return buf, unrepresentableIRI(s)
	}
	buf = append(buf, '<')
	buf = append(buf, s...)
	return append(buf, '>'), nil
}

// appendTSVString appends s as an N-Triples STRING_LITERAL_QUOTE. TAB, LF
// and CR must be escaped in TSV; the other controls are escaped so the
// output stays printable, matching canonical N-Triples.
func appendTSVString(buf []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return buf, invalidUTF8(s)
	}
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' && c != 0x7F {
			continue
		}
		buf = append(buf, s[start:i]...)
		switch c {
		case '"', '\\':
			buf = append(buf, '\\', c)
		case '\t':
			buf = append(buf, '\\', 't')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\b':
			buf = append(buf, '\\', 'b')
		case '\f':
			buf = append(buf, '\\', 'f')
		default:
			buf = append(buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
		}
		start = i + 1
	}
	buf = append(buf, s[start:]...)
	return append(buf, '"'), nil
}

// isTurtleNumber reports whether a literal can be written as a bare Turtle
// number that parses back to the same datatype and lexical form: INTEGER for
// xsd:integer, DECIMAL for xsd:decimal, DOUBLE for xsd:double (Turtle 1.1
// [19]-[21]). The productions are disjoint.
func isTurtleNumber(dt rdflibgo.URIRef, s string) bool {
	switch dt {
	case rdflibgo.XSDInteger:
		i := skipSign(s)
		j := skipDigits(s, i)
		return j > i && j == len(s)
	case rdflibgo.XSDDecimal:
		i := skipDigits(s, skipSign(s))
		if i >= len(s) || s[i] != '.' {
			return false
		}
		j := skipDigits(s, i+1)
		return j > i+1 && j == len(s)
	case rdflibgo.XSDDouble:
		i := skipSign(s)
		intEnd := skipDigits(s, i)
		intDigits := intEnd - i
		i = intEnd
		if i < len(s) && s[i] == '.' {
			fracEnd := skipDigits(s, i+1)
			if intDigits == 0 && fracEnd == i+1 {
				return false
			}
			i = fracEnd
		} else if intDigits == 0 {
			return false
		}
		if i >= len(s) || (s[i] != 'e' && s[i] != 'E') {
			return false
		}
		k := i + 1 + skipSign(s[i+1:])
		m := skipDigits(s, k)
		return m > k && m == len(s)
	}
	return false
}

func skipSign(s string) int {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		return 1
	}
	return 0
}

func skipDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
