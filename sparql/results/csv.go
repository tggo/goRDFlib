package results

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

func (w *Writer) csvRow(buf []byte, row map[string]rdflibgo.Term) ([]byte, error) {
	var err error
	rowStart := len(buf)
	for i, v := range w.vars {
		if i > 0 {
			buf = append(buf, ',')
		}
		t := row[v]
		if t == nil {
			continue // unbound: empty field
		}
		mark := len(buf)
		if buf, err = w.appendCSVValue(buf, t, false); err != nil {
			return buf, fmt.Errorf("variable ?%s: %w", v, err)
		}
		buf = w.quoteCSVField(buf, mark)
	}
	if len(w.vars) == 1 && len(buf) == rowStart {
		// A single empty field would be a blank line, which CSV readers
		// (encoding/csv, Python csv) skip, losing the row. A quoted empty
		// field keeps it; CSV cannot tell unbound from "" either way.
		buf = append(buf, '"', '"')
	}
	return append(buf, '\r', '\n'), nil
}

// appendCSVValue appends the unquoted field text for t (SPARQL 1.2 CSV §2):
// IRIs and lexical forms as plain strings, blank nodes as _:label, triple
// terms as <<( s p o )>>. inTriple is set for the object of a triple term,
// where a literal is wrapped in quotes with inner quotes doubled.
func (w *Writer) appendCSVValue(buf []byte, t rdflibgo.Term, inTriple bool) ([]byte, error) {
	var err error
	switch t := t.(type) {
	case rdflibgo.URIRef:
		return appendValidUTF8(buf, t.Value())
	case rdflibgo.BNode:
		buf = append(buf, '_', ':')
		return w.appendBNodeLabel(buf, t), nil
	case rdflibgo.Literal:
		if !inTriple {
			return appendValidUTF8(buf, t.Lexical())
		}
		if !utf8.ValidString(t.Lexical()) {
			return buf, invalidUTF8(t.Lexical())
		}
		buf = append(buf, '"')
		buf = appendDoubledQuotes(buf, t.Lexical())
		return append(buf, '"'), nil
	case rdflibgo.TripleTerm:
		if t.Subject() == nil || t.Object() == nil {
			return buf, fmt.Errorf("%w: triple term with a nil component", ErrUnsupportedTerm)
		}
		buf = append(buf, "<<( "...)
		if buf, err = w.appendCSVValue(buf, t.Subject(), true); err != nil {
			return buf, err
		}
		buf = append(buf, ' ')
		if buf, err = w.appendCSVValue(buf, t.Predicate(), true); err != nil {
			return buf, err
		}
		buf = append(buf, ' ')
		if buf, err = w.appendCSVValue(buf, t.Object(), true); err != nil {
			return buf, err
		}
		return append(buf, " )>>"...), nil
	}
	return buf, fmt.Errorf("%w: %T", ErrUnsupportedTerm, t)
}

// quoteCSVField quotes buf[mark:] in place when RFC 4180 requires it: the
// field contains '"', ',', CR or LF.
func (w *Writer) quoteCSVField(buf []byte, mark int) []byte {
	field := buf[mark:]
	if bytes.IndexAny(field, "\",\r\n") < 0 {
		return buf
	}
	w.scratch = append(w.scratch[:0], field...)
	buf = append(buf[:mark], '"')
	buf = appendDoubledQuotes(buf, w.scratch)
	return append(buf, '"')
}

func appendDoubledQuotes[S string | []byte](buf []byte, s S) []byte {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			buf = append(buf, '"')
		}
		buf = append(buf, s[i])
	}
	return buf
}

func appendValidUTF8(buf []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return buf, invalidUTF8(s)
	}
	return append(buf, s...), nil
}
