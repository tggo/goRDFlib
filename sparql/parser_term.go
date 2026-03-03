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
		parts := splitTripleTermPartsParser(inner)
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
		// Check for prefixed datatype (^^prefix:local)
		if idx := strings.Index(s, "^^"); idx >= 0 {
			dtPart := s[idx+2:]
			if !strings.HasPrefix(dtPart, "<") {
				// It's a prefixed name datatype
				if cidx := strings.Index(dtPart, ":"); cidx >= 0 {
					prefix := dtPart[:cidx]
					local := dtPart[cidx+1:]
					if ns, ok := p.prefixes[prefix]; ok {
						lit := parseLiteralString(s[:idx])
						return rdflibgo.NewLiteral(lit.Lexical(), rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(ns+local)))
					}
				}
			}
		}
		return parseLiteralString(s)
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

func parseLiteralString(s string) rdflibgo.Literal {
	// Simplified literal parsing from N3 form
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote

	var lexEnd int
	if long {
		q3 := string([]byte{quote, quote, quote})
		lexEnd = strings.Index(s[3:], q3)
		if lexEnd < 0 {
			return rdflibgo.NewLiteral(s)
		}
		lexEnd += 3
	} else {
		lexEnd = strings.Index(s[1:], string(quote))
		if lexEnd < 0 {
			return rdflibgo.NewLiteral(s)
		}
		lexEnd += 1
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

// splitTripleTermPartsParser splits the inner part of a triple term into 3 components.
func splitTripleTermPartsParser(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1 < len(s) && s[i+1] == '<' {
			depth++
			i++
		} else if s[i] == '>' && i+1 < len(s) && s[i+1] == '>' {
			depth--
			i++
		} else if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			i++
			for i < len(s) && s[i] != q {
				if s[i] == '\\' {
					i++
				}
				i++
			}
		} else if s[i] == ' ' && depth == 0 {
			part := strings.TrimSpace(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		part := strings.TrimSpace(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
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

// preprocessCodepointEscapes processes \uHHHH and \UHHHHHHHH escapes everywhere in the input.
// Per SPARQL 1.2 spec, codepoint escapes are processed at the lexical level before any other processing.
// Returns the input with escapes replaced by the actual Unicode characters.
func preprocessCodepointEscapes(input string) string {
	if !strings.ContainsRune(input, '\\') {
		return input
	}

	var sb strings.Builder
	sb.Grow(len(input))

	for i := 0; i < len(input); i++ {
		ch := input[i]

		// Process \u and \U escapes everywhere
		if ch == '\\' && i+1 < len(input) {
			if input[i+1] == 'u' && i+5 < len(input) {
				hex := input[i+2 : i+6]
				if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
					// Don't convert surrogates — they remain as-is for later validation
					if cp >= 0xD800 && cp <= 0xDFFF {
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
