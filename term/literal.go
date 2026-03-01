package term

import (
	"strconv"
	"strings"
)

// Literal represents an RDF literal with a lexical form, optional language tag, and datatype.
// Ported from: rdflib.term.Literal
type Literal struct {
	lexical  string
	lang     string
	datatype URIRef
}

func (l Literal) termType() string { return "Literal" }

// LiteralOption configures Literal construction.
type LiteralOption func(*Literal)

// WithLang sets the language tag. The tag is normalized to lowercase.
func WithLang(lang string) LiteralOption {
	return func(l *Literal) {
		l.lang = strings.ToLower(lang)
	}
}

// WithDatatype sets the datatype IRI.
func WithDatatype(dt URIRef) LiteralOption {
	return func(l *Literal) {
		l.datatype = dt
	}
}

// NewLiteral creates a new Literal from a Go value.
// Supported types: string, int, int64, float32, float64, bool.
// Options WithLang and WithDatatype can override the defaults.
// If WithLang is set, the datatype is forced to rdf:langString.
// Ported from: rdflib.term.Literal.__new__
func NewLiteral(value any, opts ...LiteralOption) Literal {
	var lit Literal

	lit.lexical, lit.datatype = goToLexical(value)

	for _, opt := range opts {
		opt(&lit)
	}

	// Language-tagged literals get rdf:langString datatype per RDF 1.1.
	if lit.lang != "" {
		lit.datatype = RDFLangString
	}

	return lit
}

// Lexical returns the lexical form.
func (l Literal) Lexical() string { return l.lexical }

// Language returns the language tag (empty string if none).
func (l Literal) Language() string { return l.lang }

// Datatype returns the datatype URIRef.
func (l Literal) Datatype() URIRef { return l.datatype }

// Value converts the lexical form to a Go value based on datatype.
// Returns int64 for xsd:integer/int/long, float32 for xsd:float,
// float64 for xsd:double/decimal, bool for xsd:boolean, or the lexical string.
// Ported from: rdflib.term.Literal.toPython
func (l Literal) Value() any {
	switch l.datatype {
	case XSDInteger, XSDInt, XSDLong:
		if v, err := strconv.ParseInt(l.lexical, 10, 64); err == nil {
			return v
		}
	case XSDFloat:
		if v, err := strconv.ParseFloat(l.lexical, 32); err == nil {
			return float32(v)
		}
	case XSDDouble, XSDDecimal:
		if v, err := strconv.ParseFloat(l.lexical, 64); err == nil {
			return v
		}
	case XSDBoolean:
		if v, err := strconv.ParseBool(l.lexical); err == nil {
			return v
		}
	}
	return l.lexical
}

// String returns the lexical form of the literal.
func (l Literal) String() string {
	return l.lexical
}

// Equal returns true if other is a Literal with identical lexical form, language, and datatype.
func (l Literal) Equal(other Term) bool {
	if o, ok := other.(Literal); ok {
		return l == o
	}
	return false
}

// N3 returns the N-Triples/N3 representation.
// Uses shorthand forms for xsd:integer, xsd:double, xsd:decimal, and xsd:boolean.
// Ported from: rdflib.term.Literal.n3
func (l Literal) N3(ns ...NamespaceManager) string {
	// Shortcut forms
	switch l.datatype {
	case XSDInteger:
		if _, err := strconv.ParseInt(l.lexical, 10, 64); err == nil {
			return l.lexical
		}
	case XSDDouble:
		if strings.ContainsAny(l.lexical, "eE") {
			if _, err := strconv.ParseFloat(l.lexical, 64); err == nil {
				return l.lexical
			}
		}
	case XSDDecimal:
		if strings.Contains(l.lexical, ".") {
			if _, err := strconv.ParseFloat(l.lexical, 64); err == nil {
				return l.lexical
			}
		}
	case XSDBoolean:
		if l.lexical == "true" || l.lexical == "false" {
			return l.lexical
		}
	}

	// Quote the lexical value
	var quoted string
	if strings.Contains(l.lexical, "\n") {
		quoted = `"""` + escapeTripleQuotedLiteral(l.lexical) + `"""`
	} else {
		quoted = `"` + escapeLiteral(l.lexical) + `"`
	}

	if l.lang != "" {
		return quoted + "@" + l.lang
	}
	if l.datatype != (URIRef{}) && l.datatype != XSDString {
		return quoted + "^^" + l.datatype.N3()
	}
	return quoted
}

// ValueEqual performs value-space comparison: two literals are ValueEqual if
// they have the same datatype and their parsed Go values are equal.
// This differs from Equal (struct equality) which compares lexical forms exactly.
// Ported from: rdflib.term.Literal.eq
func (l Literal) ValueEqual(other Literal) bool {
	return l.valueEqual(other)
}

// Eq is an alias for ValueEqual.
//
// Deprecated: use ValueEqual for clarity.
func (l Literal) Eq(other Literal) bool {
	return l.valueEqual(other)
}

func (l Literal) valueEqual(other Literal) bool {
	if l.datatype != other.datatype {
		return false
	}
	v1 := l.Value()
	v2 := other.Value()
	switch a := v1.(type) {
	case int64:
		if b, ok := v2.(int64); ok {
			return a == b
		}
	case float32:
		if b, ok := v2.(float32); ok {
			return a == b
		}
	case float64:
		if b, ok := v2.(float64); ok {
			return a == b
		}
	case bool:
		if b, ok := v2.(bool); ok {
			return a == b
		}
	case string:
		if b, ok := v2.(string); ok {
			return a == b
		}
	}
	return false
}

// literalEscaper is a package-level replacer for escaping literal strings.
var literalEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)

func escapeLiteral(s string) string {
	return literalEscaper.Replace(s)
}

// tripleQuotedEscaper escapes for triple-quoted strings: only backslashes and
// sequences of 3+ consecutive quotes need escaping. Individual quotes are safe.
var tripleQuotedEscaper = strings.NewReplacer(
	`\`, `\\`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)

// escapeTripleQuotedLiteral escapes a string for use inside triple-quoted N3
// delimiters (""" ... """). Individual double-quotes are left alone; runs of
// 3 or more consecutive quotes are broken by inserting backslash escapes.
func escapeTripleQuotedLiteral(s string) string {
	s = tripleQuotedEscaper.Replace(s)
	// Break any run of 3+ consecutive double-quotes.
	var b strings.Builder
	consecutiveQuotes := 0
	for _, r := range s {
		if r == '"' {
			consecutiveQuotes++
			if consecutiveQuotes == 3 {
				// Insert escape before this quote to break the run.
				b.WriteString(`\"`)
				consecutiveQuotes = 1 // the escaped quote starts a new run
				continue
			}
		} else {
			consecutiveQuotes = 0
		}
		b.WriteRune(r)
	}
	return b.String()
}
