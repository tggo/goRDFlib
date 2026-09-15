package results

import (
	"fmt"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// xmlDocStart opens every XML results document. The its namespace (SPARQL
// 1.2, base direction) is declared up front because a streaming writer cannot
// know whether a directional literal will follow.
const xmlDocStart = `<?xml version="1.0" encoding="UTF-8"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#" xmlns:its="http://www.w3.org/2005/11/its">
`

func (w *Writer) xmlRow(buf []byte, row map[string]rdflibgo.Term) ([]byte, error) {
	buf = append(buf, "    <result>\n"...)
	var err error
	for i, v := range w.vars {
		t := row[v]
		if t == nil {
			continue
		}
		buf = append(buf, "      "...)
		buf = append(buf, w.prefix[i]...)
		if buf, err = w.appendXMLTerm(buf, t); err != nil {
			return buf, fmt.Errorf("variable ?%s: %w", v, err)
		}
		buf = append(buf, "</binding>\n"...)
	}
	return append(buf, "    </result>\n"...), nil
}

func (w *Writer) appendXMLTerm(buf []byte, t rdflibgo.Term) ([]byte, error) {
	var err error
	switch t := t.(type) {
	case rdflibgo.URIRef:
		buf = append(buf, "<uri>"...)
		if buf, err = appendXMLText(buf, t.Value(), false); err != nil {
			return buf, err
		}
		return append(buf, "</uri>"...), nil
	case rdflibgo.BNode:
		buf = append(buf, "<bnode>"...)
		buf = w.appendBNodeLabel(buf, t)
		return append(buf, "</bnode>"...), nil
	case rdflibgo.Literal:
		buf = append(buf, "<literal"...)
		if t.Language() != "" {
			buf = append(buf, ` xml:lang="`...)
			if buf, err = appendXMLText(buf, t.Language(), true); err != nil {
				return buf, err
			}
			buf = append(buf, '"')
			if t.Dir() != "" {
				buf = append(buf, ` its:dir="`...)
				if buf, err = appendXMLText(buf, t.Dir(), true); err != nil {
					return buf, err
				}
				buf = append(buf, '"')
			}
		} else if hasDatatype(t) {
			buf = append(buf, ` datatype="`...)
			if buf, err = appendXMLText(buf, t.Datatype().Value(), true); err != nil {
				return buf, err
			}
			buf = append(buf, '"')
		}
		buf = append(buf, '>')
		if buf, err = appendXMLText(buf, t.Lexical(), false); err != nil {
			return buf, err
		}
		return append(buf, "</literal>"...), nil
	case rdflibgo.TripleTerm:
		if t.Subject() == nil || t.Object() == nil {
			return buf, fmt.Errorf("%w: triple term with a nil component", ErrUnsupportedTerm)
		}
		buf = append(buf, "<triple><subject>"...)
		if buf, err = w.appendXMLTerm(buf, t.Subject()); err != nil {
			return buf, err
		}
		buf = append(buf, "</subject><predicate>"...)
		if buf, err = w.appendXMLTerm(buf, t.Predicate()); err != nil {
			return buf, err
		}
		buf = append(buf, "</predicate><object>"...)
		if buf, err = w.appendXMLTerm(buf, t.Object()); err != nil {
			return buf, err
		}
		return append(buf, "</object></triple>"...), nil
	}
	return buf, fmt.Errorf("%w: %T", ErrUnsupportedTerm, t)
}

// appendXMLText appends s escaped for XML character data, or for a
// double-quoted attribute value when attr is true. CR is always written as a
// reference, and TAB and LF too inside attributes, because XML parsers
// normalise them otherwise (XML 1.0 §2.11, §3.3.3).
func appendXMLText(buf []byte, s string, attr bool) ([]byte, error) {
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				return buf, invalidUTF8(s)
			}
			if !isXMLChar(r) {
				return buf, unrepresentableXMLChar(r)
			}
			i += size
			continue
		}
		var esc string
		switch {
		case c == '&':
			esc = "&amp;"
		case c == '<':
			esc = "&lt;"
		case c == '>':
			esc = "&gt;"
		case c == '\r':
			esc = "&#xD;"
		case c == '"' && attr:
			esc = "&quot;"
		case c == '\t' && attr:
			esc = "&#x9;"
		case c == '\n' && attr:
			esc = "&#xA;"
		case c < 0x20 && c != '\t' && c != '\n':
			return buf, unrepresentableXMLChar(rune(c))
		default:
			i++
			continue
		}
		buf = append(buf, s[start:i]...)
		buf = append(buf, esc...)
		i++
		start = i
	}
	return append(buf, s[start:]...), nil
}
