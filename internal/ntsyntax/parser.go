// Package ntsyntax provides shared N-Triples/N-Quads parsing and serialization helpers.
package ntsyntax

import (
	"fmt"
	"strconv"
	"strings"

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
		validated, verr := rdflibgo.NewURIRef(iri)
		if verr != nil {
			return nil, fmt.Errorf("line %d: subject: %w", p.LineNum, verr)
		}
		return validated, nil
	}
	if strings.HasPrefix(p.Line[p.Pos:], "_:") {
		return p.ReadBNode(), nil
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
		validated, verr := rdflibgo.NewURIRef(iri)
		if verr != nil {
			return nil, fmt.Errorf("line %d: object: %w", p.LineNum, verr)
		}
		return validated, nil
	}
	if strings.HasPrefix(p.Line[p.Pos:], "_:") {
		return p.ReadBNode(), nil
	}
	if p.Line[p.Pos] == '"' {
		return p.ReadLiteral()
	}
	return nil, fmt.Errorf("line %d: expected IRI, blank node, or literal for object", p.LineNum)
}

// ReadPredicate parses a predicate IRI with validation.
func (p *LineParser) ReadPredicate() (rdflibgo.URIRef, error) {
	iri, err := p.ReadIRI()
	if err != nil {
		return rdflibgo.URIRef{}, fmt.Errorf("line %d: predicate: %w", p.LineNum, err)
	}
	validated, verr := rdflibgo.NewURIRef(iri)
	if verr != nil {
		return rdflibgo.URIRef{}, fmt.Errorf("line %d: predicate: %w", p.LineNum, verr)
	}
	return validated, nil
}

// ReadIRI parses an IRI enclosed in < >.
func (p *LineParser) ReadIRI() (string, error) {
	if !p.Expect('<') {
		return "", fmt.Errorf("expected '<'")
	}
	start := p.Pos
	for p.Pos < len(p.Line) {
		if p.Line[p.Pos] == '>' {
			iri := p.Line[start:p.Pos]
			p.Pos++
			unescaped, err := UnescapeIRI(iri)
			if err != nil {
				return "", fmt.Errorf("line %d: %w", p.LineNum, err)
			}
			return unescaped, nil
		}
		if p.Line[p.Pos] == '\\' {
			if p.Pos+1 >= len(p.Line) {
				return "", fmt.Errorf("line %d: unterminated escape in IRI", p.LineNum)
			}
			p.Pos += 2
			continue
		}
		p.Pos++
	}
	return "", fmt.Errorf("unterminated IRI")
}

// ReadBNode parses a blank node (_:label).
func (p *LineParser) ReadBNode() rdflibgo.BNode {
	p.Pos += 2 // skip "_:"
	start := p.Pos
	for p.Pos < len(p.Line) {
		ch := p.Line[p.Pos]
		if ch == ' ' || ch == '\t' {
			break
		}
		if ch == '.' {
			if p.Pos+1 >= len(p.Line) || p.Line[p.Pos+1] == ' ' || p.Line[p.Pos+1] == '\t' {
				break
			}
		}
		p.Pos++
	}
	return rdflibgo.NewBNode(p.Line[start:p.Pos])
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
		opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt)))
	}

	return rdflibgo.NewLiteral(lexical, opts...), nil
}

// UnescapeIRI unescapes \uXXXX and \UXXXXXXXX in an IRI string.
// Returns an error for malformed escape sequences.
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
