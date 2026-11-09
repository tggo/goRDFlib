package ntsyntax

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// CanonicalTerm is Term in the canonical form of RDF 1.2 N-Triples: literals
// use the short escapes \b \t \n \f \r, \uXXXX (upper-case hex) for the other
// C0 controls, U+007F and the noncharacters U+FFFE and U+FFFF, and write every
// other character, supplementary planes included, as itself.
func CanonicalTerm(t rdflibgo.Term) (string, error) { return term(t, true) }

// escapeCanonical escapes a literal's lexical form as CanonicalTerm describes.
func escapeCanonical(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\\':
			sb.WriteString(`\\`)
		case r == '"':
			sb.WriteString(`\"`)
		case r == '\b':
			sb.WriteString(`\b`)
		case r == '\t':
			sb.WriteString(`\t`)
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\f':
			sb.WriteString(`\f`)
		case r == '\r':
			sb.WriteString(`\r`)
		case r < 0x20 || r == 0x7F || r == 0xFFFE || r == 0xFFFF:
			sb.WriteString(`\u`)
			sb.WriteString(padHex4(uint64(r)))
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
