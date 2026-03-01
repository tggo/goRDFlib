package rdflibgo

import (
	"fmt"
	"strconv"
	"strings"
)

// Literal represents an RDF literal.
// Ported from: rdflib.term.Literal
type Literal struct {
	lexical  string
	lang     string
	datatype URIRef
}

func (l Literal) termType() string { return "Literal" }

// LiteralOption configures Literal construction.
type LiteralOption func(*Literal)

// WithLang sets the language tag.
func WithLang(lang string) LiteralOption {
	return func(l *Literal) {
		l.lang = strings.ToLower(lang)
	}
}

// WithDatatype sets the datatype.
func WithDatatype(dt URIRef) LiteralOption {
	return func(l *Literal) {
		l.datatype = dt
	}
}

// NewLiteral creates a new Literal from a Go value.
func NewLiteral(value any, opts ...LiteralOption) Literal {
	var lit Literal

	switch v := value.(type) {
	case string:
		lit.lexical = v
		lit.datatype = XSDString
	case int:
		lit.lexical = intToString(v)
		lit.datatype = XSDInteger
	case int64:
		lit.lexical = int64ToString(v)
		lit.datatype = XSDInteger
	case float32:
		lit.lexical = float32ToString(v)
		lit.datatype = XSDFloat
	case float64:
		lit.lexical = float64ToString(v)
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

	// Language-tagged literals get rdf:langString datatype
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

// Value attempts to convert the lexical form to a Go value based on datatype.
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

func (l Literal) String() string {
	return l.lexical
}

// N3 returns the N-Triples/N3 representation.
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

// Eq performs value-space comparison.
func (l Literal) Eq(other Literal) bool {
	// If datatypes differ, no value-space match
	if l.datatype != other.datatype {
		return false
	}
	// Compare parsed values
	v1 := l.Value()
	v2 := other.Value()
	return fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
}

func escapeLiteral(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return r.Replace(s)
}
