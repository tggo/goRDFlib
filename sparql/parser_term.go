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
		opts = append(opts, rdflibgo.WithLang(rest[1:]))
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

// validateStringEscapes checks for invalid \u/\U unicode escape sequences (surrogates).
func validateStringEscapes(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			if s[i+1] == 'u' && i+5 < len(s) {
				hex := s[i+2 : i+6]
				if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
					if cp >= 0xD800 && cp <= 0xDFFF {
						return fmt.Errorf("invalid unicode surrogate U+%04X in string literal", cp)
					}
				}
				i += 5
			} else if s[i+1] == 'U' && i+9 < len(s) {
				hex := s[i+2 : i+10]
				if cp, err := strconv.ParseUint(hex, 16, 32); err == nil {
					if cp >= 0xD800 && cp <= 0xDFFF {
						return fmt.Errorf("invalid unicode surrogate U+%08X in string literal", cp)
					}
				}
				i += 9
			} else {
				i++
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
