package rdflibgo

import (
	"fmt"
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

	switch v := value.(type) {
	case string:
		lit.lexical = v
		lit.datatype = XSDString
	case int:
		lit.lexical = strconv.Itoa(v)
		lit.datatype = XSDInteger
	case int64:
		lit.lexical = strconv.FormatInt(v, 10)
		lit.datatype = XSDInteger
	case float32:
		lit.lexical = strconv.FormatFloat(float64(v), 'g', -1, 32)
		lit.datatype = XSDFloat
	case float64:
		lit.lexical = strconv.FormatFloat(v, 'g', -1, 64)
		lit.datatype = XSDDouble
	case bool:
		if v {
			lit.lexical = "true"
		} else {
			lit.lexical = "false"
		}
		lit.datatype = XSDBoolean
	default:
		lit.lexical = fmt.Sprintf("%v", value)
		lit.datatype = XSDString
	}

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
		quoted = `"""` + escapeLiteral(l.lexical) + `"""`
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

// Eq performs value-space comparison: two literals are Eq if they have the same
// datatype and their parsed Go values are equal.
// This differs from == (struct equality) which compares lexical forms exactly.
// Ported from: rdflib.term.Literal.eq
func (l Literal) Eq(other Literal) bool {
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
)

func escapeLiteral(s string) string {
	return literalEscaper.Replace(s)
}
