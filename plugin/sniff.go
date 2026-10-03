package plugin

import "strings"

// sniffLimit is how many bytes FormatFromContent looks at.
const sniffLimit = 4096

// FormatFromContent detects the RDF format by sniffing the first bytes.
//
// Heuristic, and limited to the first 4 KiB:
//   - a UTF-8 BOM, whitespace and leading '#' comment lines are skipped;
//   - "<?xml" or "<rdf:RDF" is RDF/XML;
//   - '{', or '[' followed by '{', or a lone "[]", is JSON-LD (a Turtle document may
//     also start with '[', as an anonymous blank-node subject);
//   - anything else is scanned token by token, skipping IRIs, string literals
//     and comments. Any token N-Triples does not have (a directive, prefixed
//     name, 'a', number, ';', ',', '[', '(', a long or single-quoted string)
//     makes it Turtle. Otherwise it is N-Triples, or N-Quads when any
//     statement has four terms.
//
// Limits: TriG is never reported (its graph blocks look like Turtle until the
// '{', and a document without them is Turtle anyway); a Turtle document whose
// first 4 KiB happen to be plain N-Triples is reported as "nt", which still
// parses correctly up to the first Turtle-only token.
// Ported from: rdflib.plugin — content-based detection
func FormatFromContent(data []byte) (string, bool) {
	if len(data) > sniffLimit {
		data = data[:sniffLimit]
	}
	s := strings.TrimPrefix(string(data), "\xEF\xBB\xBF")
	s = skipSpaceAndComments(s)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "<?xml") || strings.HasPrefix(s, "<rdf:RDF") {
		return "xml", true
	}
	if s[0] == '{' {
		return "json-ld", true
	}
	if s[0] == '[' {
		rest := strings.TrimLeft(s[1:], " \t\r\n")
		if rest == "" || rest[0] == '{' {
			return "json-ld", true
		}
		if rest[0] == ']' && strings.TrimSpace(rest[1:]) == "" {
			return "json-ld", true // an empty JSON-LD array; "[] <p> <o>" is Turtle
		}
	}
	return sniffTriples(s)
}

func skipSpaceAndComments(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if !strings.HasPrefix(s, "#") {
			return s
		}
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
}

// sniffTriples tells Turtle from N-Triples/N-Quads. It reports turtle as soon
// as it meets syntax N-Triples lacks. A statement cut off by the sniff limit
// ends the scan without a verdict on the rest.
func sniffTriples(s string) (string, bool) {
	terms, maxTerms := 0, 0
	var outer []int // term counts of the statements enclosing open triple terms
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '#':
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				i = len(s)
			} else {
				i += j
			}
		case c == '<':
			if strings.HasPrefix(s[i:], "<<(") {
				// An RDF 1.2 triple term, in both syntaxes: one term,
				// whose components are scanned like any other.
				outer = append(outer, terms)
				terms = 0
				i += 3
				continue
			}
			if strings.HasPrefix(s[i:], "<<") {
				return "turtle", true // reified triple
			}
			j := strings.IndexAny(s[i+1:], "> \n")
			if j < 0 {
				i = len(s)
				continue
			}
			if s[i+1+j] != '>' {
				return "turtle", true // not an IRI: a '<' that is not N-Triples
			}
			i += j + 2
			terms++
		case c == '"':
			if strings.HasPrefix(s[i:], `"""`) {
				return "turtle", true
			}
			i = skipString(s, i)
			terms++
		case c == '@':
			j := i + 1
			for j < len(s) && (isAlnum(s[j]) || s[j] == '-') {
				j++
			}
			if w := s[i+1 : j]; w == "prefix" || w == "base" || w == "version" {
				return "turtle", true
			}
			i = j // language tag on the preceding literal
		case c == '^' && strings.HasPrefix(s[i:], "^^"):
			i += 2
			terms-- // the datatype IRI that follows belongs to the literal
		case c == '_' && strings.HasPrefix(s[i:], "_:"):
			j := i + 2
			for j < len(s) && !strings.ContainsRune(" \t\r\n<\"#)", rune(s[j])) {
				j++
			}
			for j > i+2 && s[j-1] == '.' { // a label never ends in '.'
				j--
			}
			i = j
			terms++
		case c == '.':
			maxTerms = max(maxTerms, terms)
			terms = 0
			i++
		case c == ')' && strings.HasPrefix(s[i:], ")>>") && len(outer) > 0:
			terms = outer[len(outer)-1] + 1
			outer = outer[:len(outer)-1]
			i += 3
		default:
			// ';' ',' '[' '(' prefixed names, keywords, numbers, booleans.
			return "turtle", true
		}
	}
	maxTerms = max(maxTerms, terms)
	if maxTerms == 0 {
		return "", false
	}
	if maxTerms >= 4 {
		return "nquads", true
	}
	return "nt", true
}

// skipString returns the index after the double-quoted literal starting at i,
// or len(s) if it is cut off.
func skipString(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(s)
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
