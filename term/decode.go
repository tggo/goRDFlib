package term

import (
	"fmt"
	"strings"
)

// TermFromKey reconstructs a Term from a TermKey string produced by TermKey().
// Supported prefixes: "U:" (URIRef), "B:" (BNode), "L:" (Literal), "T:" (TripleTerm).
func TermFromKey(key string) (Term, error) {
	if len(key) < 2 {
		return nil, fmt.Errorf("term: key too short: %q", key)
	}
	prefix := key[:2]
	rest := key[2:]
	switch prefix {
	case "U:":
		return NewURIRefUnsafe(rest), nil
	case "B:":
		return NewBNode(rest), nil
	case "L:":
		return literalFromN3(rest)
	case "T:":
		return tripleTermFromKey(rest)
	default:
		return nil, fmt.Errorf("term: unknown key prefix: %q", prefix)
	}
}

// literalFromN3 parses a literal from its N3 representation.
// Handles: "lex", "lex"@lang, "lex"@lang--dir, "lex"^^<datatype>,
// and shorthand forms (integers, decimals, booleans).
func literalFromN3(n3 string) (Literal, error) {
	// Numeric shorthand, recognised by the same Turtle productions N3 writes
	// it with. Testing for "." before the exponent read every double with a
	// fraction ("1.5e3") back as an xsd:decimal.
	switch {
	case isTurtleDouble(n3):
		return NewLiteral(n3, WithDatatype(XSDDouble)), nil
	case isTurtleDecimal(n3):
		return NewLiteral(n3, WithDatatype(XSDDecimal)), nil
	case isTurtleInteger(n3):
		return NewLiteral(n3, WithDatatype(XSDInteger)), nil
	}
	// Shorthand: boolean
	if n3 == "true" || n3 == "false" {
		return NewLiteral(n3, WithDatatype(XSDBoolean)), nil
	}

	// Quoted literal
	if len(n3) == 0 || n3[0] != '"' {
		return Literal{}, fmt.Errorf("term: invalid literal N3: %q", n3)
	}

	// Find closing quote(s)
	var lexical string
	var afterQuote string
	if strings.HasPrefix(n3, `"""`) {
		// Triple-quoted
		end := findClosingTripleQuote(n3[3:])
		if end < 0 {
			return Literal{}, fmt.Errorf("term: unterminated triple-quoted literal: %q", n3)
		}
		lexical = unescapeTripleQuotedLiteral(n3[3 : 3+end])
		afterQuote = n3[3+end+3:]
	} else {
		// Single-quoted
		end := findClosingQuote(n3[1:])
		if end < 0 {
			return Literal{}, fmt.Errorf("term: unterminated literal: %q", n3)
		}
		lexical = unescapeLiteral(n3[1 : 1+end])
		afterQuote = n3[1+end+1:]
	}

	// Parse suffix: @lang, @lang--dir, ^^<datatype>
	if strings.HasPrefix(afterQuote, "@") {
		langDir := afterQuote[1:]
		if idx := strings.Index(langDir, "--"); idx >= 0 {
			lang := langDir[:idx]
			dir := langDir[idx+2:]
			return NewLiteral(lexical, WithLang(lang), WithDir(dir)), nil
		}
		return NewLiteral(lexical, WithLang(langDir)), nil
	}
	if strings.HasPrefix(afterQuote, "^^") {
		dtStr := afterQuote[2:]
		if len(dtStr) >= 2 && dtStr[0] == '<' && dtStr[len(dtStr)-1] == '>' {
			dt := NewURIRefUnsafe(dtStr[1 : len(dtStr)-1])
			return NewLiteral(lexical, WithDatatype(dt)), nil
		}
		return Literal{}, fmt.Errorf("term: invalid datatype in literal: %q", afterQuote)
	}

	// Plain string literal (xsd:string)
	return NewLiteral(lexical), nil
}

// findClosingQuote finds the index of the closing unescaped quote in s.
func findClosingQuote(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}

// findClosingTripleQuote finds the index of the closing """ in s, skipping
// escaped characters. A plain strings.Index stopped at an escaped quote that
// sits next to the delimiter (`\""""`), truncating the lexical form.
func findClosingTripleQuote(s string) int {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' && s[i+1] == '"' && s[i+2] == '"' {
			return i
		}
	}
	return -1
}

// tripleTermFromKey parses a TripleTerm from the body of its TermKey — the
// three component keys joined by NUL, as NewTripleTerm builds them.
//
// It is deliberately not the N3 parser below: TermKey("T:") emits component
// keys, not N3, and reading the key back as N3 fails for every triple term.
// That mattered, because a store decodes rows by TermKey and skips rows it
// cannot decode, so triple terms used to vanish silently on read from any
// backend that persists keys.
//
// Splitting into exactly three parts is what makes nesting work. Only the
// object may itself be a triple term, and it is last, so any further NULs
// belong to it.
func tripleTermFromKey(key string) (TripleTerm, error) {
	// Accept the N3 body as well. TermKey never produces it, but hand-built
	// keys and older callers do, and rejecting them would break them for no
	// gain — the two forms are unambiguous.
	if strings.HasPrefix(key, "<<(") {
		return tripleTermFromN3(key)
	}

	parts := strings.SplitN(key, "\x00", 3)
	if len(parts) != 3 {
		return TripleTerm{}, fmt.Errorf("term: invalid triple term key: %q", key)
	}

	subj, err := TermFromKey(parts[0])
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term subject: %w", err)
	}
	subjTerm, ok := subj.(Subject)
	if !ok {
		return TripleTerm{}, fmt.Errorf("term: triple term subject must be URIRef or BNode, got %q", parts[0])
	}

	pred, err := TermFromKey(parts[1])
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term predicate: %w", err)
	}
	predURI, ok := pred.(URIRef)
	if !ok {
		return TripleTerm{}, fmt.Errorf("term: triple term predicate must be URIRef, got %q", parts[1])
	}

	obj, err := TermFromKey(parts[2])
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term object: %w", err)
	}

	return NewTripleTerm(subjTerm, predURI, obj), nil
}

// tripleTermFromN3 parses a TripleTerm from its N3 representation.
// Format: <<( <s> <p> <o> )>>
func tripleTermFromN3(n3 string) (TripleTerm, error) {
	// The key format is "T:" + N3, and N3 is "<<( <s> <p> <o> )>>"
	s := strings.TrimSpace(n3)
	if !strings.HasPrefix(s, "<<(") || !strings.HasSuffix(s, ")>>") {
		return TripleTerm{}, fmt.Errorf("term: invalid triple term N3: %q", n3)
	}
	inner := strings.TrimSpace(s[3 : len(s)-3])

	// Parse subject
	subj, rest, err := parseOneTermN3(inner)
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term subject: %w", err)
	}
	subjTerm, ok := subj.(Subject)
	if !ok {
		return TripleTerm{}, fmt.Errorf("term: triple term subject must be URIRef or BNode")
	}

	// Parse predicate
	pred, rest, err := parseOneTermN3(strings.TrimSpace(rest))
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term predicate: %w", err)
	}
	predURI, ok := pred.(URIRef)
	if !ok {
		return TripleTerm{}, fmt.Errorf("term: triple term predicate must be URIRef")
	}

	// Parse object
	obj, _, err := parseOneTermN3(strings.TrimSpace(rest))
	if err != nil {
		return TripleTerm{}, fmt.Errorf("term: triple term object: %w", err)
	}

	return NewTripleTerm(subjTerm, predURI, obj), nil
}

// parseOneTermN3 parses a single term from the beginning of an N3 string,
// returning the term and the remaining string.
func parseOneTermN3(s string) (Term, string, error) {
	if len(s) == 0 {
		return nil, "", fmt.Errorf("empty input")
	}

	switch {
	case s[0] == '<':
		// Check for triple term <<(
		if strings.HasPrefix(s, "<<(") {
			// Find matching )>>
			depth := 1
			i := 3
			for i < len(s) && depth > 0 {
				if strings.HasPrefix(s[i:], "<<(") {
					depth++
					i += 3
				} else if strings.HasPrefix(s[i:], ")>>") {
					depth--
					if depth == 0 {
						i += 3
						break
					}
					i += 3
				} else {
					i++
				}
			}
			tt, err := tripleTermFromN3(s[:i])
			return tt, s[i:], err
		}
		// URIRef: <iri>
		end := strings.IndexByte(s, '>')
		if end < 0 {
			return nil, "", fmt.Errorf("unterminated URI: %q", s)
		}
		return NewURIRefUnsafe(s[1:end]), s[end+1:], nil

	case s[0] == '_' && len(s) > 1 && s[1] == ':':
		// BNode: _:id
		end := strings.IndexAny(s[2:], " \t\n\r")
		if end < 0 {
			return NewBNode(s[2:]), "", nil
		}
		return NewBNode(s[2 : 2+end]), s[2+end:], nil

	case s[0] == '"':
		// Literal
		lit, err := literalFromN3(s)
		if err != nil {
			return nil, "", err
		}
		// Need to find where literal ends to get remaining string
		consumed := consumeLiteralN3(s)
		return lit, s[consumed:], nil

	default:
		// Bare value (integer, boolean, decimal)
		end := strings.IndexAny(s, " \t\n\r")
		if end < 0 {
			lit, err := literalFromN3(s)
			return lit, "", err
		}
		lit, err := literalFromN3(s[:end])
		return lit, s[end:], err
	}
}

// unescapeLiteral reverses escapeLiteral: converts \", \\, \n, \r, \t back.
func unescapeLiteral(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				b.WriteByte('"')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// unescapeTripleQuotedLiteral reverses escapeTripleQuotedLiteral.
func unescapeTripleQuotedLiteral(s string) string {
	return unescapeLiteral(s)
}

// consumeLiteralN3 returns the number of bytes consumed by a literal N3 at the start of s.
func consumeLiteralN3(s string) int {
	if len(s) == 0 || s[0] != '"' {
		return 0
	}
	i := 0
	if strings.HasPrefix(s, `"""`) {
		end := findClosingTripleQuote(s[3:])
		if end < 0 {
			return len(s)
		}
		i = 3 + end + 3
	} else {
		end := findClosingQuote(s[1:])
		if end < 0 {
			return len(s)
		}
		i = 1 + end + 1
	}
	// Consume @lang or @lang--dir or ^^<dt>
	if i < len(s) && s[i] == '@' {
		i++
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			i++
		}
	} else if i+1 < len(s) && s[i] == '^' && s[i+1] == '^' {
		i += 2
		if i < len(s) && s[i] == '<' {
			end := strings.IndexByte(s[i:], '>')
			if end >= 0 {
				i += end + 1
			}
		}
	}
	return i
}
