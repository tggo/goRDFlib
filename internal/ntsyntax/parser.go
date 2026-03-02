// Package ntsyntax provides shared N-Triples/N-Quads parsing and serialization helpers.
package ntsyntax

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// LineParser holds state for parsing a single N-Triples/N-Quads line.
type LineParser struct {
	Line    string
	Pos     int
	LineNum int
}

// SkipSpaces advances past spaces and tabs.
func (p *LineParser) SkipSpaces() {
	for p.Pos < len(p.Line) && (p.Line[p.Pos] == ' ' || p.Line[p.Pos] == '\t') {
		p.Pos++
	}
}

// Expect consumes a specific byte if present, returns true if consumed.
func (p *LineParser) Expect(ch byte) bool {
	if p.Pos < len(p.Line) && p.Line[p.Pos] == ch {
		p.Pos++
		return true
	}
	return false
}

// ReadSubject parses an IRI or blank node subject.
func (p *LineParser) ReadSubject() (rdflibgo.Subject, error) {
	p.SkipSpaces()
	if p.Pos >= len(p.Line) {
		return nil, fmt.Errorf("line %d: unexpected end", p.LineNum)
	}
	if p.Line[p.Pos] == '<' {
		iri, err := p.ReadIRI()
		if err != nil {
			return nil, fmt.Errorf("line %d: subject: %w", p.LineNum, err)
		}
		if !isAbsoluteIRI(iri) {
			return nil, fmt.Errorf("line %d: subject: relative IRI not allowed in N-Triples", p.LineNum)
		}
		validated, verr := rdflibgo.NewURIRef(iri)
		if verr != nil {
			return nil, fmt.Errorf("line %d: subject: %w", p.LineNum, verr)
		}
		return validated, nil
	}
	if strings.HasPrefix(p.Line[p.Pos:], "_:") {
		return p.ReadBNode()
	}
	return nil, fmt.Errorf("line %d: expected IRI or blank node for subject", p.LineNum)
}

// ReadObject parses an IRI, blank node, or literal object.
func (p *LineParser) ReadObject() (rdflibgo.Term, error) {
	p.SkipSpaces()
	if p.Pos >= len(p.Line) {
		return nil, fmt.Errorf("line %d: unexpected end", p.LineNum)
	}
	if p.Line[p.Pos] == '<' {
		iri, err := p.ReadIRI()
		if err != nil {
			return nil, fmt.Errorf("line %d: object: %w", p.LineNum, err)
		}
		if !isAbsoluteIRI(iri) {
			return nil, fmt.Errorf("line %d: object: relative IRI not allowed in N-Triples", p.LineNum)
		}
		validated, verr := rdflibgo.NewURIRef(iri)
		if verr != nil {
			return nil, fmt.Errorf("line %d: object: %w", p.LineNum, verr)
		}
		return validated, nil
	}
	if strings.HasPrefix(p.Line[p.Pos:], "_:") {
		return p.ReadBNode()
	}
	if p.Line[p.Pos] == '"' {
		return p.ReadLiteral()
	}
	return nil, fmt.Errorf("line %d: expected IRI, blank node, or literal for object", p.LineNum)
}

// ReadPredicate parses a predicate IRI with validation.
func (p *LineParser) ReadPredicate() (rdflibgo.URIRef, error) {
	p.SkipSpaces()
	iri, err := p.ReadIRI()
	if err != nil {
		return rdflibgo.URIRef{}, fmt.Errorf("line %d: predicate: %w", p.LineNum, err)
	}
	if !isAbsoluteIRI(iri) {
		return rdflibgo.URIRef{}, fmt.Errorf("line %d: predicate: relative IRI not allowed in N-Triples", p.LineNum)
	}
	validated, verr := rdflibgo.NewURIRef(iri)
	if verr != nil {
		return rdflibgo.URIRef{}, fmt.Errorf("line %d: predicate: %w", p.LineNum, verr)
	}
	return validated, nil
}

// ReadGraphLabel parses an optional graph IRI or blank node for N-Quads.
func (p *LineParser) ReadGraphLabel() (rdflibgo.Term, error) {
	p.SkipSpaces()
	if p.Pos >= len(p.Line) || p.Line[p.Pos] == '.' {
		return nil, nil
	}
	if p.Line[p.Pos] == '<' {
		iri, err := p.ReadIRI()
		if err != nil {
			return nil, fmt.Errorf("line %d: graph: %w", p.LineNum, err)
		}
		if !isAbsoluteIRI(iri) {
			return nil, fmt.Errorf("line %d: graph: relative IRI not allowed in N-Quads", p.LineNum)
		}
		validated, verr := rdflibgo.NewURIRef(iri)
		if verr != nil {
			return nil, fmt.Errorf("line %d: graph: %w", p.LineNum, verr)
		}
		return validated, nil
	}
	if strings.HasPrefix(p.Line[p.Pos:], "_:") {
		bn, err := p.ReadBNode()
		return bn, err
	}
	return nil, fmt.Errorf("line %d: expected IRI or blank node for graph label", p.LineNum)
}

// ReadIRI parses an IRI enclosed in < >.
func (p *LineParser) ReadIRI() (string, error) {
	if !p.Expect('<') {
		return "", fmt.Errorf("expected '<'")
	}
	start := p.Pos
	for p.Pos < len(p.Line) {
		ch := p.Line[p.Pos]
		if ch == '>' {
			iri := p.Line[start:p.Pos]
			p.Pos++
			unescaped, err := UnescapeIRI(iri)
			if err != nil {
				return "", fmt.Errorf("line %d: %w", p.LineNum, err)
			}
			return unescaped, nil
		}
		if ch == '\\' {
			if p.Pos+1 >= len(p.Line) {
				return "", fmt.Errorf("line %d: unterminated escape in IRI", p.LineNum)
			}
			p.Pos += 2
			continue
		}
		// Reject invalid IRI characters.
		if ch <= 0x20 {
			return "", fmt.Errorf("line %d: invalid character in IRI", p.LineNum)
		}
		p.Pos++
	}
	return "", fmt.Errorf("unterminated IRI")
}

// ReadBNode parses a blank node (_:label).
func (p *LineParser) ReadBNode() (rdflibgo.BNode, error) {
	p.Pos += 2 // skip "_:"
	start := p.Pos
	if p.Pos >= len(p.Line) {
		return rdflibgo.BNode{}, fmt.Errorf("line %d: empty blank node label", p.LineNum)
	}
	// First char: PN_CHARS_U | [0-9]
	r, size := utf8.DecodeRuneInString(p.Line[p.Pos:])
	if !isPNCharsU(r) && !(r >= '0' && r <= '9') {
		return rdflibgo.BNode{}, fmt.Errorf("line %d: invalid blank node label start: %c", p.LineNum, r)
	}
	p.Pos += size
	// Subsequent chars: PN_CHARS | '.'
	for p.Pos < len(p.Line) {
		r, size = utf8.DecodeRuneInString(p.Line[p.Pos:])
		if isPNChar(r) || r == '.' {
			p.Pos += size
		} else {
			break
		}
	}
	// Trim trailing dots.
	for p.Pos > start && p.Line[p.Pos-1] == '.' {
		p.Pos--
	}
	label := p.Line[start:p.Pos]
	if label == "" {
		return rdflibgo.BNode{}, fmt.Errorf("line %d: empty blank node label", p.LineNum)
	}
	return rdflibgo.NewBNode(label), nil
}

// ReadLiteral parses "lexical"@lang or "lexical"^^<datatype>.
func (p *LineParser) ReadLiteral() (rdflibgo.Literal, error) {
	p.Pos++ // skip opening "
	var sb strings.Builder
	closed := false
	for p.Pos < len(p.Line) {
		ch := p.Line[p.Pos]
		if ch == '\\' {
			p.Pos++
			if p.Pos >= len(p.Line) {
				return rdflibgo.Literal{}, fmt.Errorf("line %d: unterminated escape", p.LineNum)
			}
			esc := p.Line[p.Pos]
			p.Pos++
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case 'b':
				sb.WriteByte('\b')
			case 'f':
				sb.WriteByte('\f')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case 'u':
				if p.Pos+4 > len(p.Line) {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: truncated \\u escape", p.LineNum)
				}
				code, err := strconv.ParseUint(p.Line[p.Pos:p.Pos+4], 16, 32)
				if err != nil {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: invalid \\u escape", p.LineNum)
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: invalid surrogate in \\u escape", p.LineNum)
				}
				sb.WriteRune(rune(code))
				p.Pos += 4
			case 'U':
				if p.Pos+8 > len(p.Line) {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: truncated \\U escape", p.LineNum)
				}
				code, err := strconv.ParseUint(p.Line[p.Pos:p.Pos+8], 16, 32)
				if err != nil {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: invalid \\U escape", p.LineNum)
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return rdflibgo.Literal{}, fmt.Errorf("line %d: invalid surrogate in \\U escape", p.LineNum)
				}
				sb.WriteRune(rune(code))
				p.Pos += 8
			default:
				return rdflibgo.Literal{}, fmt.Errorf("line %d: unknown escape \\%c", p.LineNum, esc)
			}
			continue
		}
		if ch == '"' {
			p.Pos++
			closed = true
			break
		}
		sb.WriteByte(ch)
		p.Pos++
	}

	if !closed {
		return rdflibgo.Literal{}, fmt.Errorf("line %d: unterminated string literal", p.LineNum)
	}

	lexical := sb.String()
	var opts []rdflibgo.LiteralOption

	if p.Pos < len(p.Line) && p.Line[p.Pos] == '@' {
		p.Pos++
		start := p.Pos
		// First char must be a letter.
		if p.Pos >= len(p.Line) || !isLetter(p.Line[p.Pos]) {
			return rdflibgo.Literal{}, fmt.Errorf("line %d: invalid language tag: must start with a letter", p.LineNum)
		}
		for p.Pos < len(p.Line) && p.Line[p.Pos] != ' ' && p.Line[p.Pos] != '\t' && p.Line[p.Pos] != '.' {
			p.Pos++
		}
		opts = append(opts, rdflibgo.WithLang(p.Line[start:p.Pos]))
	} else if p.Pos+1 < len(p.Line) && p.Line[p.Pos] == '^' && p.Line[p.Pos+1] == '^' {
		p.Pos += 2
		dt, err := p.ReadIRI()
		if err != nil {
			return rdflibgo.Literal{}, fmt.Errorf("line %d: datatype: %w", p.LineNum, err)
		}
		if !isAbsoluteIRI(dt) {
			return rdflibgo.Literal{}, fmt.Errorf("line %d: datatype: relative IRI not allowed", p.LineNum)
		}
		opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt)))
	}

	return rdflibgo.NewLiteral(lexical, opts...), nil
}

// UnescapeIRI unescapes \uXXXX and \UXXXXXXXX in an IRI string.
func UnescapeIRI(s string) (string, error) {
	if !strings.ContainsRune(s, '\\') {
		return s, nil
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'u':
				if i+5 > len(s) {
					return "", fmt.Errorf("truncated \\u escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid \\u escape in IRI: %w", err)
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return "", fmt.Errorf("invalid surrogate in IRI escape")
				}
				sb.WriteRune(rune(code))
				i += 5
			case 'U':
				if i+9 > len(s) {
					return "", fmt.Errorf("truncated \\U escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+9], 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid \\U escape in IRI: %w", err)
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return "", fmt.Errorf("invalid surrogate in IRI escape")
				}
				sb.WriteRune(rune(code))
				i += 9
			default:
				return "", fmt.Errorf("unknown escape \\%c in IRI", s[i])
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String(), nil
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAbsoluteIRI(s string) bool {
	colon := strings.Index(s, ":")
	if colon <= 0 {
		return false
	}
	for i := 0; i < colon; i++ {
		ch := s[i]
		if i == 0 {
			if !isLetter(ch) {
				return false
			}
		} else {
			if !isLetter(ch) && !(ch >= '0' && ch <= '9') && ch != '+' && ch != '-' && ch != '.' {
				return false
			}
		}
	}
	return true
}

// isPNCharsBase matches PN_CHARS_BASE from the grammar.
func isPNCharsBase(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0x00C0 && r <= 0x00D6) ||
		(r >= 0x00D8 && r <= 0x00F6) ||
		(r >= 0x00F8 && r <= 0x02FF) ||
		(r >= 0x0370 && r <= 0x037D) ||
		(r >= 0x037F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

func isPNCharsU(r rune) bool {
	return r == '_' || isPNCharsBase(r)
}

func isPNChar(r rune) bool {
	return isPNCharsU(r) ||
		r == '-' ||
		(r >= '0' && r <= '9') ||
		r == 0x00B7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}
