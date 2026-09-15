package endpoint

import (
	"strings"

	"github.com/tggo/goRDFlib/internal/iri"
)

// queryShape is what the handler needs to know about a request before the
// parser sees it, and what the parser does not keep: the query form, and the
// dataset clauses (the engine skips FROM and FROM NAMED).
type queryShape struct {
	// form is the first keyword after the prologue, upper-cased: SELECT, ASK,
	// CONSTRUCT, DESCRIBE, or an update keyword (INSERT, DELETE, LOAD, …).
	form      string
	from      []string
	fromNamed []string
}

// scanQuery reads the prologue, the query form and the dataset clauses of a
// query lexically. It skips strings, comments and IRIs, tracks brace depth,
// and resolves prefixed names and relative IRIs in FROM clauses with the
// PREFIX and BASE declarations it has seen (base is the service base IRI).
//
// It is not a parser. A FROM it cannot resolve (an undeclared prefix) is left
// out, and the query then fails to parse or runs without that graph. Dataset
// clauses only occur at brace depth 0, where no other construct uses the word
// FROM, so a whole-word FROM there is a dataset clause.
func scanQuery(text, base string) queryShape {
	var sh queryShape
	prefixes := map[string]string{}
	s := scanner{src: text}
	depth := 0
	for {
		tok := s.next()
		if tok.kind == tokEOF {
			return sh
		}
		switch tok.kind {
		case tokPunct:
			switch tok.text {
			case "{":
				depth++
			case "}":
				depth--
			}
			continue
		case tokWord:
		default:
			continue
		}
		if depth != 0 {
			continue
		}
		word := strings.ToUpper(tok.text)
		switch {
		case word == "PREFIX" && sh.form == "":
			name := s.next()
			ref := s.next()
			if name.kind == tokPName && strings.HasSuffix(name.text, ":") && ref.kind == tokIRI {
				prefixes[strings.TrimSuffix(name.text, ":")] = resolveRef(base, ref.text)
			}
		case word == "BASE" && sh.form == "":
			if ref := s.next(); ref.kind == tokIRI {
				base = resolveRef(base, ref.text)
			}
		case sh.form == "" && word != "VERSION":
			sh.form = word
		case word == "FROM":
			named := false
			tok := s.next()
			if tok.kind == tokWord && strings.EqualFold(tok.text, "NAMED") {
				named = true
				tok = s.next()
			}
			var ref string
			switch tok.kind {
			case tokIRI:
				ref = resolveRef(base, tok.text)
			case tokPName:
				p, local, _ := strings.Cut(tok.text, ":")
				ns, ok := prefixes[p]
				if !ok {
					continue
				}
				ref = ns + local
			default:
				continue
			}
			if named {
				sh.fromNamed = append(sh.fromNamed, ref)
			} else {
				sh.from = append(sh.from, ref)
			}
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
