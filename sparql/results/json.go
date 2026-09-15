package results

import (
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

func (w *Writer) jsonRow(buf []byte, row map[string]rdflibgo.Term) ([]byte, error) {
	if w.rows > 0 {
		buf = append(buf, ',')
	}
	buf = append(buf, '\n', '{')
	first := true
	var err error
	for i, v := range w.vars {
		t := row[v]
		if t == nil {
			continue
		}
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = append(buf, w.prefix[i]...)
		if buf, err = w.appendJSONTerm(buf, t); err != nil {
			return buf, fmt.Errorf("variable ?%s: %w", v, err)
		}
	}
	return append(buf, '}'), nil
}

// appendJSONTerm appends the SPARQL 1.2 JSON object for t.
func (w *Writer) appendJSONTerm(buf []byte, t rdflibgo.Term) ([]byte, error) {
	var err error
	switch t := t.(type) {
	case rdflibgo.URIRef:
		buf = append(buf, `{"type":"uri","value":`...)
		if buf, err = appendJSONString(buf, t.Value()); err != nil {
			return buf, err
		}
		return append(buf, '}'), nil
	case rdflibgo.BNode:
		buf = append(buf, `{"type":"bnode","value":"`...)
		buf = w.appendBNodeLabel(buf, t)
		return append(buf, '"', '}'), nil
	case rdflibgo.Literal:
		buf = append(buf, `{"type":"literal","value":`...)
		if buf, err = appendJSONString(buf, t.Lexical()); err != nil {
			return buf, err
		}
		if t.Language() != "" {
			buf = append(buf, `,"xml:lang":`...)
			if buf, err = appendJSONString(buf, t.Language()); err != nil {
				return buf, err
			}
			if t.Dir() != "" {
				buf = append(buf, `,"its:dir":`...)
				if buf, err = appendJSONString(buf, t.Dir()); err != nil {
					return buf, err
				}
			}
		} else if hasDatatype(t) {
			buf = append(buf, `,"datatype":`...)
			if buf, err = appendJSONString(buf, t.Datatype().Value()); err != nil {
				return buf, err
			}
		}
		return append(buf, '}'), nil
	case rdflibgo.TripleTerm:
		if t.Subject() == nil || t.Object() == nil {
			return buf, fmt.Errorf("%w: triple term with a nil component", ErrUnsupportedTerm)
		}
		buf = append(buf, `{"type":"triple","value":{"subject":`...)
		if buf, err = w.appendJSONTerm(buf, t.Subject()); err != nil {
			return buf, err
		}
		buf = append(buf, `,"predicate":`...)
		if buf, err = w.appendJSONTerm(buf, t.Predicate()); err != nil {
			return buf, err
		}
		buf = append(buf, `,"object":`...)
		if buf, err = w.appendJSONTerm(buf, t.Object()); err != nil {
			return buf, err
		}
		return append(buf, '}', '}'), nil
	}
	return buf, fmt.Errorf("%w: %T", ErrUnsupportedTerm, t)
}

const hexDigits = "0123456789abcdef"

// jsonSafe[c] is true for an ASCII byte that goes into a JSON string as is.
var jsonSafe = func() (t [utf8.RuneSelf]bool) {
	for c := 0x20; c < utf8.RuneSelf; c++ {
		t[c] = c != '"' && c != '\\'
	}
	return t
}()

// appendJSONString appends s as a quoted JSON string (RFC 8259 §7). Control
// characters, U+2028 and U+2029 are escaped; invalid UTF-8 is an error.
func appendJSONString(buf []byte, s string) ([]byte, error) {
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if jsonSafe[c] {
				i++
				continue
			}
			buf = append(buf, s[start:i]...)
			switch c {
			case '"', '\\':
				buf = append(buf, '\\', c)
			case '\n':
				buf = append(buf, '\\', 'n')
			case '\r':
				buf = append(buf, '\\', 'r')
			case '\t':
				buf = append(buf, '\\', 't')
			case '\b':
				buf = append(buf, '\\', 'b')
			case '\f':
				buf = append(buf, '\\', 'f')
			default:
				buf = append(buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return buf, invalidUTF8(s)
		}
		if r == '\u2028' || r == '\u2029' {
			buf = append(buf, s[start:i]...)
			buf = append(buf, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			start = i + size
		}
		i += size
	}
	buf = append(buf, s[start:]...)
	return append(buf, '"'), nil
}
