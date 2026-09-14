package sparql

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	rdflibgo "github.com/tggo/goRDFlib"
)

func (p *sparqlParser) resolveTermValue(s string) rdflibgo.Term {
	if s == "" {
		return rdflibgo.NewLiteral("")
	}
	// Triple term: <<( s p o )>>
	if strings.HasPrefix(s, "<<( ") && strings.HasSuffix(s, " )>>") {
		inner := s[4 : len(s)-4]
		parts := splitTripleTermParts(inner)
		if len(parts) == 3 {
			st := p.resolveTermValue(parts[0])
			pt := p.resolveTermValue(parts[1])
			ot := p.resolveTermValue(parts[2])
			if st == nil || pt == nil || ot == nil {
				return nil
			}
			// Validate: subject must not be a TripleTerm or Literal
			if _, ok := st.(rdflibgo.TripleTerm); ok {
				p.tripleTermError = fmt.Errorf("sparql parse error: triple term in subject position of triple term")
				return nil
			}
			if _, ok := st.(rdflibgo.Literal); ok {
				p.tripleTermError = fmt.Errorf("sparql parse error: literal in subject position of triple term")
				return nil
			}
			subj, ok := st.(rdflibgo.Subject)
			if !ok {
				return nil
			}
			pred, ok := pt.(rdflibgo.URIRef)
			if !ok {
				return nil
			}
			return rdflibgo.NewTripleTerm(subj, pred, ot)
		}
		return nil
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return rdflibgo.NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if strings.HasPrefix(s, "_:") {
		label := s[2:]
		return rdflibgo.NewBNode(label)
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralWithPrefixes(s, p.prefixes)
	}
	if s == "true" {
		return rdflibgo.NewLiteral(true)
	}
	if s == "false" {
		return rdflibgo.NewLiteral(false)
	}
	// Numeric
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '+' || s[0] == '-') {
		if strings.ContainsAny(s, "eE") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDouble))
		}
		if strings.Contains(s, ".") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	}
	// Prefixed name
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		// Unescape PN_LOCAL_ESC (backslash escapes) and percent encoding
		local = unescapePNLocal(local)
		if ns, ok := p.prefixes[prefix]; ok {
			return rdflibgo.NewURIRefUnsafe(ns + local)
		}
	}
	return rdflibgo.NewLiteral(s)
}

// parseLiteralWithPrefixes parses a literal token whose datatype may be a
// prefixed name ("a"^^xsd:string), resolving it against prefixes. The "^^" is
// looked for after the closing quote, so a "^^" inside the lexical form is
// not mistaken for the datatype separator.
func parseLiteralWithPrefixes(s string, prefixes map[string]string) rdflibgo.Literal {
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote
	end := closingQuoteIndex(s, quote, long)
	if end >= 0 {
		after := end + 1
		if long {
			after = end + 3
		}
		if rest := s[after:]; strings.HasPrefix(rest, "^^") && !strings.HasPrefix(rest, "^^<") {
			if prefix, local, ok := strings.Cut(rest[2:], ":"); ok {
				if ns, ok := prefixes[prefix]; ok {
					lit := parseLiteralString(s[:after])
					return rdflibgo.NewLiteral(lit.Lexical(), rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(ns+unescapePNLocal(local))))
				}
			}
		}
	}
	return parseLiteralString(s)
}

func parseLiteralString(s string) rdflibgo.Literal {
	// Simplified literal parsing from N3 form
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote

	// The closing delimiter has to be found with the escapes taken into
	// account. Searching for the first bare quote instead truncated every
	// literal that contained one: `"he said \"hi\""` ended at the quote inside
	// \" and came back as `he said \`.
	lexEnd := closingQuoteIndex(s, quote, long)
	if lexEnd < 0 {
		return rdflibgo.NewLiteral(s)
	}

	var lexical string
	if long {
		lexical = s[3:lexEnd]
	} else {
		lexical = s[1:lexEnd]
	}
	lexical = unescapeSPARQLString(lexical)

	rest := s[lexEnd+1:]
	if long {
		rest = s[lexEnd+3:]
	}

	var opts []rdflibgo.LiteralOption
	if strings.HasPrefix(rest, "@") {
		langDir := rest[1:]
		if idx := strings.Index(langDir, "--"); idx >= 0 {
			// Directional language tag: lang--dir (e.g., "en--ltr")
			opts = append(opts, rdflibgo.WithLang(langDir[:idx]))
			opts = append(opts, rdflibgo.WithDir(langDir[idx+2:]))
		} else {
			opts = append(opts, rdflibgo.WithLang(langDir))
		}
	} else if strings.HasPrefix(rest, "^^") {
		dt := rest[2:]
		if strings.HasPrefix(dt, "<") && strings.HasSuffix(dt, ">") {
			opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt[1:len(dt)-1])))
		}
	}
	return rdflibgo.NewLiteral(lexical, opts...)
}

// closingQuoteIndex returns the index in s of the delimiter that closes the
// string literal starting at s[0], or -1 if the literal is unterminated.
//
// A backslash escapes the next byte, so `\"` and `\\` never close the literal.
// For a long literal the delimiter is three quotes, and the same escape rule
// applies to each of them.
func closingQuoteIndex(s string, quote byte, long bool) int {
	i := 1
	if long {
		i = 3
	}
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == quote {
			if !long {
				return i
			}
			if i+2 < len(s) && s[i+1] == quote && s[i+2] == quote {
				return i
			}
		}
		i++
	}
	return -1
}

// sparqlStringUnescaper is a package-level replacer for SPARQL string escape sequences.
var sparqlStringUnescaper = strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t")

func unescapeSPARQLString(s string) string {
	return sparqlStringUnescaper.Replace(s)
}

// validateLangDir checks that a directional language tag (lang--dir) has a valid direction.
func validateLangDir(s string) error {
	// Find the @lang part at the end
	// s is a raw token like `"foo"@en--ltr`
	atIdx := -1
	// Find @ after the closing quote
	inQuote := false
	q := byte(0)
	for i := 0; i < len(s); i++ {
		if !inQuote {
			if s[i] == '"' || s[i] == '\'' {
				inQuote = true
				q = s[i]
			}
		} else {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == q {
				inQuote = false
			}
		}
		if !inQuote && s[i] == '@' {
			atIdx = i
		}
	}
	if atIdx < 0 {
		return nil
	}
	langDir := s[atIdx+1:]
	if idx := strings.Index(langDir, "--"); idx >= 0 {
		dir := strings.ToLower(langDir[idx+2:])
		if dir != "ltr" && dir != "rtl" {
			return fmt.Errorf("invalid base direction %q in language tag (must be ltr or rtl)", langDir[idx+2:])
		}
	}
	return nil
}

// validateStringEscapes checks for invalid escape sequences in string literals.
func validateStringEscapes(s string) error {
	if s == "" {
		return nil
	}
	// Find the string content (between quotes)
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote
	var content string
	if long {
		q3 := string([]byte{quote, quote, quote})
		end := strings.Index(s[3:], q3)
		if end >= 0 {
			content = s[3 : 3+end]
		}
	} else {
		end := strings.Index(s[1:], string(quote))
		if end >= 0 {
			content = s[1 : 1+end]
		}
	}

	for i := 0; i < len(content); i++ {
		if content[i] == '\\' && i+1 < len(content) {
			next := content[i+1]
			switch next {
			case 't', 'n', 'r', '\\', '"', '\'':
				i++ // valid escape
			case 'u':
				if i+5 < len(content) {
					hex := content[i+2 : i+6]
					if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
						if cp >= 0xD800 && cp <= 0xDFFF {
							return fmt.Errorf("invalid unicode surrogate U+%04X in string literal", cp)
						}
					}
					i += 5
				} else {
					return fmt.Errorf("invalid \\u escape in string literal")
				}
			case 'U':
				if i+9 < len(content) {
					hex := content[i+2 : i+10]
					if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
						if cp >= 0xD800 && cp <= 0xDFFF {
							return fmt.Errorf("invalid unicode surrogate U+%08X in string literal", cp)
						}
					}
					i += 9
				} else {
					return fmt.Errorf("invalid \\U escape in string literal")
				}
			default:
				return fmt.Errorf("invalid escape sequence \\%c in string literal", next)
			}
		}
	}
	return nil
}

func (p *sparqlParser) skipWS() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else if ch == '#' {
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *sparqlParser) expect(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *sparqlParser) startsWith(s string) bool {
	return strings.HasPrefix(p.input[p.pos:], s)
}

func (p *sparqlParser) matchKeywordCI(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	if !strings.EqualFold(p.input[p.pos:p.pos+len(kw)], kw) {
		return false
	}
	after := p.pos + len(kw)
	if after < len(p.input) && isNameChar(rune(p.input[after])) {
		return false
	}
	return true
}

func (p *sparqlParser) isKeyword() bool {
	for _, kw := range []string{"ORDER", "LIMIT", "OFFSET", "GROUP", "HAVING", "VALUES"} {
		if p.matchKeywordCI(kw) {
			return true
		}
	}
	return false
}

func (p *sparqlParser) errorf(format string, args ...any) error {
	return fmt.Errorf("sparql parse error at pos %d: %s", p.pos, fmt.Sprintf(format, args...))
}

func isNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// parseVersion parses a VERSION directive. Accepts single-quoted or double-quoted strings.
// Triple-quoted strings are rejected.
func (p *sparqlParser) parseVersion() error {
	p.pos += 7 // skip "VERSION"
	p.skipWS()
	if p.pos >= len(p.input) {
		return p.errorf("expected version string after VERSION")
	}
	ch := p.input[p.pos]
	if ch != '"' && ch != '\'' {
		return p.errorf("expected quoted string after VERSION, got %c", ch)
	}
	// Reject triple-quoted strings
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == ch && p.input[p.pos+2] == ch {
		return p.errorf("triple-quoted strings not allowed in VERSION")
	}
	p.pos++ // skip opening quote
	for p.pos < len(p.input) && p.input[p.pos] != ch {
		p.pos++
	}
	if p.pos < len(p.input) {
		p.pos++ // skip closing quote
	}
	return nil
}

// preprocessCodepointEscapes processes \uHHHH and \UHHHHHHHH codepoint escapes in SPARQL input.
// Per SPARQL 1.2, codepoint escapes are processed at the character level before any other
// tokenization, including inside string literals.
func preprocessCodepointEscapes(input string) string {
	if strings.IndexByte(input, '\\') < 0 {
		return input
	}

	var sb strings.Builder
	sb.Grow(len(input))

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if ch == '\\' && i+1 < len(input) {
			if input[i+1] == 'u' && i+5 < len(input) {
				hex := input[i+2 : i+6]
				if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
					if cp >= 0xD800 && cp <= 0xDFFF {
						// Preserve surrogates for later validation
						sb.WriteString(input[i : i+6])
						i += 5
						continue
					}
					sb.WriteRune(rune(cp))
					i += 5
					continue
				}
			} else if input[i+1] == 'U' && i+9 < len(input) {
				hex := input[i+2 : i+10]
				if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
					if cp >= 0xD800 && cp <= 0xDFFF {
						sb.WriteString(input[i : i+10])
						i += 9
						continue
					}
					sb.WriteRune(rune(cp))
					i += 9
					continue
				}
			}
		}

		sb.WriteByte(ch)
	}
	return sb.String()
}
