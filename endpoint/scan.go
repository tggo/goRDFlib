package endpoint

import (
	"strings"

	"github.com/tggo/goRDFlib/internal/iri"
)

// scanForm reads the query form — the first keyword after the prologue —
// lexically, before the parser sees the text, so that a DESCRIBE query or an
// update sent to the query endpoint is answered with its own status instead of
// a syntax error. The returned keyword is upper-cased: SELECT, ASK, CONSTRUCT,
// DESCRIBE, or an update keyword (INSERT, DELETE, LOAD, …); it is "" when the
// text has no keyword at all.
//
// It is not a parser. It skips strings, comments and IRIs so that a word
// inside one is never mistaken for a keyword, and it steps over the prologue
// declarations, whose operands are not words.
func scanForm(text string) string {
	s := scanner{src: text}
	for {
		tok := s.next()
		switch {
		case tok.kind == tokEOF:
			return ""
		case tok.kind != tokWord:
			continue
		}
		switch word := strings.ToUpper(tok.text); word {
		case "PREFIX", "BASE", "VERSION":
			// Prologue: the operand is a prefixed name, an IRI or a string,
			// none of which the scanner reports as a word.
		default:
			return word
		}
	}
}

func resolveRef(base, ref string) string {
	if base == "" || iri.IsAbsolute(ref) {
		return ref
	}
	return iri.Resolve(base, ref)
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokWord
	tokPName
	tokIRI
	tokPunct
	tokOther
)

type token struct {
	kind tokKind
	text string // an IRI without its angle brackets
}

type scanner struct {
	src string
	pos int
}

func (s *scanner) next() token {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.pos++
		case c == '#':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
		case c == '"' || c == '\'':
			s.skipString(c)
			return token{kind: tokOther}
		case c == '<':
			if end, ok := s.iriEnd(); ok {
				t := token{kind: tokIRI, text: s.src[s.pos+1 : end]}
				s.pos = end + 1
				return t
			}
			s.pos++
			return token{kind: tokOther}
		case c == '?' || c == '$' || c == '@':
			// A variable or a language tag: never a keyword.
			s.pos++
			for s.pos < len(s.src) && (isWordByte(s.src[s.pos]) || s.src[s.pos] == '-') {
				s.pos++
			}
			return token{kind: tokOther}
		case c == '{' || c == '}':
			s.pos++
			return token{kind: tokPunct, text: string(c)}
		case isWordByte(c) || c == ':':
			start := s.pos
			pname := false
			for s.pos < len(s.src) {
				b := s.src[s.pos]
				if b == ':' {
					pname = true
				} else if !isWordByte(b) && b != '.' && b != '-' && b != '%' && b != '\\' {
					break
				}
				s.pos++
			}
			text := strings.TrimRight(s.src[start:s.pos], ".")
			s.pos = start + len(text)
			if text == "" {
				s.pos++
				return token{kind: tokOther}
			}
			if pname {
				return token{kind: tokPName, text: text}
			}
			return token{kind: tokWord, text: text}
		default:
			s.pos++
			return token{kind: tokOther}
		}
	}
	return token{kind: tokEOF}
}

// isWordByte accepts ASCII letters, digits and '_', and every byte of a
// multi-byte UTF-8 sequence (PN_CHARS allows most of Unicode).
func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c >= 0x80
}

// iriEnd reports the index of the '>' closing an IRIREF that starts at s.pos.
// An IRIREF cannot contain spaces, quotes or braces, which is what tells it
// apart from a less-than comparison.
func (s *scanner) iriEnd() (int, bool) {
	for i := s.pos + 1; i < len(s.src); i++ {
		switch c := s.src[i]; {
		case c == '>':
			return i, true
		case c <= ' ' || c == '<' || c == '"' || c == '{' || c == '}' || c == '|' || c == '^' || c == '`' || c == '\\':
			return 0, false
		}
	}
	return 0, false
}

// skipString moves past a short or long string literal starting at s.pos.
func (s *scanner) skipString(q byte) {
	triple := string([]byte{q, q, q})
	long := strings.HasPrefix(s.src[s.pos:], triple)
	if long {
		s.pos += 3
	} else {
		s.pos++
	}
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\\':
			s.pos += 2
		case long && strings.HasPrefix(s.src[s.pos:], triple):
			s.pos += 3
			return
		case !long && c == q:
			s.pos++
			return
		case !long && (c == '\n' || c == '\r'):
			return
		default:
			s.pos++
		}
	}
}
