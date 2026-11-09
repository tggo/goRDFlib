package shacl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tggo/goRDFlib/sparql"
)

// SPARQL-RL expressions are the SPARQL expression grammar restricted to the
// built-in calls listed in the SPARQL-RL grammar (production BuiltInCall):
// no aggregates, no EXISTS, no BOUND, COALESCE or RAND. The parser does not
// re-implement that grammar. It finds the extent of an expression (balanced
// parentheses, aware of strings, IRIs and comments), checks the function
// names against the SPARQL-RL list, and hands the text to the sparql
// package's parser to prove it is a well-formed expression. Evaluation is the
// sparql engine's (srl_sparql.go), so a FILTER or SET sees exactly SPARQL's
// value semantics: numeric promotion, language matching, errors as unbound.

// srlBuiltins are the function names of the SPARQL-RL BuiltInCall production,
// upper-cased, plus the operator words that are followed by a parenthesis.
var srlBuiltins = map[string]bool{
	"STR": true, "LANG": true, "LANGMATCHES": true, "LANGDIR": true, "DATATYPE": true,
	"IRI": true, "URI": true, "BNODE": true, "ABS": true, "CEIL": true, "FLOOR": true,
	"ROUND": true, "CONCAT": true, "SUBSTR": true, "STRLEN": true, "REPLACE": true,
	"UCASE": true, "LCASE": true, "ENCODE_FOR_URI": true, "CONTAINS": true,
	"STRSTARTS": true, "STRENDS": true, "STRBEFORE": true, "STRAFTER": true,
	"YEAR": true, "MONTH": true, "DAY": true, "HOURS": true, "MINUTES": true,
	"SECONDS": true, "TIMEZONE": true, "TZ": true, "NOW": true, "UUID": true,
	"STRUUID": true, "IF": true, "STRLANG": true, "STRLANGDIR": true, "STRDT": true,
	"SAMETERM": true, "ISIRI": true, "ISURI": true, "ISBLANK": true, "ISLITERAL": true,
	"ISNUMERIC": true, "HASLANG": true, "HASLANGDIR": true, "REGEX": true,
	"ISTRIPLE": true, "TRIPLE": true, "SUBJECT": true, "PREDICATE": true, "OBJECT": true,
	// not functions, but words that precede a parenthesised list
	"IN": true, "NOT": true,
}

// exprScanner finds the extent of expression text.
type exprScanner struct {
	in string
}

var iriInExpr = regexp.MustCompile("^<[^<>\"{}|^`\\\\\\x00-\\x20]*>")

// skipAtom returns the end of a string literal, IRI or comment starting at
// i, or i if there is none.
func (s exprScanner) skipAtom(i int) int {
	in := s.in
	switch c := in[i]; c {
	case '"', '\'':
		if strings.HasPrefix(in[i:], strings.Repeat(string(c), 3)) {
			j := i + 3
			for j < len(in) {
				if in[j] == '\\' {
					j += 2
					continue
				}
				if strings.HasPrefix(in[j:], strings.Repeat(string(c), 3)) {
					return j + 3
				}
				j++
			}
			return len(in)
		}
		j := i + 1
		for j < len(in) {
			if in[j] == '\\' {
				j += 2
				continue
			}
			if in[j] == c {
				return j + 1
			}
			j++
		}
		return len(in)
	case '<':
		if m := iriInExpr.FindString(in[i:]); m != "" {
			return i + len(m)
		}
	case '#':
		j := i
		for j < len(in) && in[j] != '\n' {
			j++
		}
		return j
	}
	return i
}

// closeParen returns the index just past the parenthesis that closes the one
// opened before in[from] (depth starts at 1), or -1.
func (s exprScanner) closeParen(from int) int {
	depth := 1
	for i := from; i < len(s.in); {
		if j := s.skipAtom(i); j != i {
			i = j
			continue
		}
		switch s.in[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return -1
}

// scanConstraint reads a FILTER constraint (Constraint ::= BrackettedExpression
// | BuiltInCall | FunctionCall) at the lexer position and returns its source
// text.
func (l *srlLexer) scanConstraint() (string, error) {
	l.skipWS()
	start := l.pos
	sc := exprScanner{in: l.input}
	i := start
	if i >= len(l.input) {
		return "", fmt.Errorf("expected a constraint after FILTER at position %d", start)
	}
	if l.input[i] != '(' {
		// BuiltInCall or FunctionCall: a name, then a parenthesised argument list.
		if l.input[i] == '<' {
			m := iriInExpr.FindString(l.input[i:])
			if m == "" {
				return "", fmt.Errorf("expected a constraint after FILTER at position %d", start)
			}
			i += len(m)
		} else {
			for i < len(l.input) {
				r := rune(l.input[i])
				if r == '_' || r == ':' || r == '-' || r == '.' || r >= 0x80 || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					i++
					continue
				}
				break
			}
			if i == start {
				return "", fmt.Errorf("expected a constraint after FILTER at position %d: a FILTER takes ( expression ), a built-in call or a function call", start)
			}
		}
		for i < len(l.input) && (l.input[i] == ' ' || l.input[i] == '\t' || l.input[i] == '\n' || l.input[i] == '\r') {
			i++
		}
		if i >= len(l.input) || l.input[i] != '(' {
			return "", fmt.Errorf("expected '(' after %q at position %d", l.input[start:i], i)
		}
	}
	end := sc.closeParen(i + 1)
	if end < 0 {
		return "", fmt.Errorf("unterminated expression starting at position %d: missing ')'", start)
	}
	l.pos = end
	return l.input[start:end], nil
}

// scanAssignedExpr reads the expression of SET ( ?v := expr ): everything up
// to the parenthesis that closes the SET. The lexer is just past ":=".
func (l *srlLexer) scanAssignedExpr() (string, error) {
	start := l.pos
	end := exprScanner{in: l.input}.closeParen(start)
	if end < 0 {
		return "", fmt.Errorf("unterminated SET starting at position %d: missing ')'", start)
	}
	l.pos = end
	expr := strings.TrimSpace(l.input[start : end-1])
	if expr == "" {
		return "", fmt.Errorf("empty expression in SET at position %d", start)
	}
	return expr, nil
}

// blankLiterals returns text with strings, IRIs and comments replaced by
// spaces, so that scanning it for names and variables cannot be fooled by
// their content.
func blankLiterals(text string) string {
	sc := exprScanner{in: text}
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		if j := sc.skipAtom(i); j != i {
			b.WriteByte(' ')
			i = j
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

var (
	exprCallRe = regexp.MustCompile(`([A-Za-z_][A-Za-z_0-9:.\-]*)\s*\(`)
	exprVarRe  = regexp.MustCompile(`[?$]([\p{L}\p{N}_]+)`)
)

// exprVars returns the distinct variables of an expression, without sigils,
// in order of first appearance.
func exprVars(text string) []string {
	clean := blankLiterals(text)
	var out []string
	seen := map[string]bool{}
	for _, m := range exprVarRe.FindAllStringSubmatch(clean, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// checkExpr proves text is a SPARQL-RL expression. wrap turns the expression
// into the SPARQL query used to parse it ("FILTER %s" or "BIND(%s AS ?v)").
func checkExpr(prologue, wrap, text string) error {
	clean := blankLiterals(text)
	if strings.ContainsAny(clean, "{}") {
		return fmt.Errorf("an expression cannot contain a graph pattern (EXISTS and NOT EXISTS are not part of SPARQL-RL)")
	}
	for _, m := range exprCallRe.FindAllStringSubmatch(clean, -1) {
		name := m[1]
		if strings.Contains(name, ":") {
			continue // a function named by IRI: iriOrFunction
		}
		if !srlBuiltins[strings.ToUpper(name)] {
			return fmt.Errorf("%s is not a SPARQL-RL built-in function", name)
		}
	}
	if _, err := sparql.Parse(prologue + "SELECT * WHERE { " + fmt.Sprintf(wrap, text) + " }"); err != nil {
		return fmt.Errorf("invalid expression %q: %w", strings.TrimSpace(text), err)
	}
	return nil
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
