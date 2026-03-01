package rdflibgo

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// NTriplesParser parses N-Triples format RDF.
// Ported from: rdflib.plugins.parsers.ntriples.NTriplesParser
type NTriplesParser struct{}

func init() {
	RegisterParser("nt", func() Parser { return &NTriplesParser{} })
	RegisterParser("ntriples", func() Parser { return &NTriplesParser{} })
}

func (p *NTriplesParser) Parse(g *Graph, r io.Reader, base string) error {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if err := parseNTLine(g, line, lineNum); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func parseNTLine(g *Graph, line string, lineNum int) error {
	p := &ntLineParser{line: line, pos: 0, lineNum: lineNum}

	subj, err := p.readNTSubject()
	if err != nil {
		return err
	}
	p.skipSpaces()

	pred, err := p.readNTIRI()
	if err != nil {
		return fmt.Errorf("line %d: predicate: %w", lineNum, err)
	}
	p.skipSpaces()

	obj, err := p.readNTObject()
	if err != nil {
		return err
	}
	p.skipSpaces()

	if !p.expect('.') {
		return fmt.Errorf("line %d: expected '.'", lineNum)
	}

	g.Add(subj, NewURIRefUnsafe(pred), obj)
	return nil
}

type ntLineParser struct {
	line    string
	pos     int
	lineNum int
}

func (p *ntLineParser) skipSpaces() {
	for p.pos < len(p.line) && (p.line[p.pos] == ' ' || p.line[p.pos] == '\t') {
		p.pos++
	}
}

func (p *ntLineParser) expect(ch byte) bool {
	if p.pos < len(p.line) && p.line[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *ntLineParser) readNTSubject() (Subject, error) {
	p.skipSpaces()
	if p.pos >= len(p.line) {
		return nil, fmt.Errorf("line %d: unexpected end", p.lineNum)
	}
	if p.line[p.pos] == '<' {
		iri, err := p.readNTIRI()
		if err != nil {
			return nil, fmt.Errorf("line %d: subject: %w", p.lineNum, err)
		}
		return NewURIRefUnsafe(iri), nil
	}
	if strings.HasPrefix(p.line[p.pos:], "_:") {
		return p.readNTBNode(), nil
	}
	return nil, fmt.Errorf("line %d: expected IRI or blank node for subject", p.lineNum)
}

func (p *ntLineParser) readNTObject() (Term, error) {
	p.skipSpaces()
	if p.pos >= len(p.line) {
		return nil, fmt.Errorf("line %d: unexpected end", p.lineNum)
	}
	if p.line[p.pos] == '<' {
		iri, err := p.readNTIRI()
		if err != nil {
			return nil, fmt.Errorf("line %d: object: %w", p.lineNum, err)
		}
		return NewURIRefUnsafe(iri), nil
	}
	if strings.HasPrefix(p.line[p.pos:], "_:") {
		return p.readNTBNode(), nil
	}
	if p.line[p.pos] == '"' {
		return p.readNTLiteral()
	}
	return nil, fmt.Errorf("line %d: expected IRI, blank node, or literal for object", p.lineNum)
}

func (p *ntLineParser) readNTIRI() (string, error) {
	if !p.expect('<') {
		return "", fmt.Errorf("expected '<'")
	}
	start := p.pos
	for p.pos < len(p.line) {
		if p.line[p.pos] == '>' {
			iri := p.line[start:p.pos]
			p.pos++
			return ntUnescapeString(iri), nil
		}
		if p.line[p.pos] == '\\' {
			p.pos += 2
			continue
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated IRI")
}

func (p *ntLineParser) readNTBNode() BNode {
	p.pos += 2 // skip "_:"
	start := p.pos
	for p.pos < len(p.line) {
		ch := p.line[p.pos]
		if ch == ' ' || ch == '\t' {
			break
		}
		// '.' is allowed inside labels but not as the last character.
		// Peek ahead: if '.' is followed by space/tab/EOL, it's the statement terminator.
		if ch == '.' {
			if p.pos+1 >= len(p.line) || p.line[p.pos+1] == ' ' || p.line[p.pos+1] == '\t' {
				break
			}
		}
		p.pos++
	}
	return NewBNode(p.line[start:p.pos])
}

// readNTLiteral parses "lexical"@lang or "lexical"^^<datatype>.
// Ported from: rdflib.plugins.parsers.ntriples — literal parsing
func (p *ntLineParser) readNTLiteral() (Literal, error) {
	p.pos++ // skip opening "
	var sb strings.Builder
	closed := false
	for p.pos < len(p.line) {
		ch := p.line[p.pos]
		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.line) {
				return Literal{}, fmt.Errorf("line %d: unterminated escape", p.lineNum)
			}
			esc := p.line[p.pos]
			p.pos++
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
				if p.pos+4 > len(p.line) {
					return Literal{}, fmt.Errorf("line %d: truncated \\u escape", p.lineNum)
				}
				code, err := strconv.ParseUint(p.line[p.pos:p.pos+4], 16, 32)
				if err != nil {
					return Literal{}, fmt.Errorf("line %d: invalid \\u escape", p.lineNum)
				}
				sb.WriteRune(rune(code))
				p.pos += 4
			case 'U':
				if p.pos+8 > len(p.line) {
					return Literal{}, fmt.Errorf("line %d: truncated \\U escape", p.lineNum)
				}
				code, err := strconv.ParseUint(p.line[p.pos:p.pos+8], 16, 32)
				if err != nil {
					return Literal{}, fmt.Errorf("line %d: invalid \\U escape", p.lineNum)
				}
				sb.WriteRune(rune(code))
				p.pos += 8
			default:
				return Literal{}, fmt.Errorf("line %d: unknown escape \\%c", p.lineNum, esc)
			}
			continue
		}
		if ch == '"' {
			p.pos++
			closed = true
			break
		}
		sb.WriteByte(ch)
		p.pos++
	}

	if !closed {
		return Literal{}, fmt.Errorf("line %d: unterminated string literal", p.lineNum)
	}

	lexical := sb.String()
	var opts []LiteralOption

	if p.pos < len(p.line) && p.line[p.pos] == '@' {
		p.pos++
		start := p.pos
		for p.pos < len(p.line) && p.line[p.pos] != ' ' && p.line[p.pos] != '\t' && p.line[p.pos] != '.' {
			p.pos++
		}
		opts = append(opts, WithLang(p.line[start:p.pos]))
	} else if p.pos+1 < len(p.line) && p.line[p.pos] == '^' && p.line[p.pos+1] == '^' {
		p.pos += 2
		dt, err := p.readNTIRI()
		if err != nil {
			return Literal{}, fmt.Errorf("line %d: datatype: %w", p.lineNum, err)
		}
		opts = append(opts, WithDatatype(NewURIRefUnsafe(dt)))
	}

	return NewLiteral(lexical, opts...), nil
}

func ntUnescapeString(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'u':
				if i+4 <= len(s) {
					code, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
					if err == nil {
						sb.WriteRune(rune(code))
						i += 5
						continue
					}
				}
			case 'U':
				if i+8 <= len(s) {
					code, err := strconv.ParseUint(s[i+1:i+9], 16, 32)
					if err == nil {
						sb.WriteRune(rune(code))
						i += 9
						continue
					}
				}
			}
			sb.WriteByte(s[i])
			i++
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}
