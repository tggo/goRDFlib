package term

import (
	"fmt"
	"math"
	"strconv"
)

// XSD namespace base URI.
const XSDNamespace = "http://www.w3.org/2001/XMLSchema#"

// XSD datatype constants.
// Ported from: rdflib.namespace.XSD
var (
	XSDString   = NewURIRefUnsafe(XSDNamespace + "string")
	XSDInteger  = NewURIRefUnsafe(XSDNamespace + "integer")
	XSDInt      = NewURIRefUnsafe(XSDNamespace + "int")
	XSDLong     = NewURIRefUnsafe(XSDNamespace + "long")
	XSDFloat    = NewURIRefUnsafe(XSDNamespace + "float")
	XSDDouble   = NewURIRefUnsafe(XSDNamespace + "double")
	XSDDecimal  = NewURIRefUnsafe(XSDNamespace + "decimal")
	XSDBoolean  = NewURIRefUnsafe(XSDNamespace + "boolean")
	XSDDateTime = NewURIRefUnsafe(XSDNamespace + "dateTime")
	XSDDate     = NewURIRefUnsafe(XSDNamespace + "date")
	XSDTime     = NewURIRefUnsafe(XSDNamespace + "time")
	XSDAnyURI   = NewURIRefUnsafe(XSDNamespace + "anyURI")
)

// RDF namespace constants used by Literal.
const RDFNamespace = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

// RDFLangString is the datatype for language-tagged literals per RDF 1.1.
var RDFLangString = NewURIRefUnsafe(RDFNamespace + "langString")

// RDFDirLangString is the datatype for directional language-tagged literals per RDF 1.2.
var RDFDirLangString = NewURIRefUnsafe(RDFNamespace + "dirLangString")

// RDFReifies is the rdf:reifies predicate for RDF 1.2 reification.
var RDFReifies = NewURIRefUnsafe(RDFNamespace + "reifies")

// goToLexical is the shared implementation for converting Go values to
// lexical form + XSD datatype. Used by both GoToLexical and NewLiteral.
func goToLexical(value any) (string, URIRef) {
	switch v := value.(type) {
	case string:
		return v, XSDString
	case int:
		return strconv.Itoa(v), XSDInteger
	case int64:
		return strconv.FormatInt(v, 10), XSDInteger
	case float32:
		return formatXSDFloat(float64(v), 32), XSDFloat
	case float64:
		return formatXSDFloat(v, 64), XSDDouble
	case bool:
		if v {
			return "true", XSDBoolean
		}
		return "false", XSDBoolean
	default:
		return fmt.Sprintf("%v", value), XSDString
	}
}

// formatXSDFloat writes a float in the xsd:float / xsd:double lexical space.
// strconv spells the special values "+Inf", "-Inf" and "NaN"; XSD 1.1 §3.3.4
// and §3.3.5 spell them "INF", "-INF" and "NaN", and "+Inf" is ill-typed.
func formatXSDFloat(v float64, bitSize int) string {
	switch {
	case math.IsInf(v, 1):
		return "INF"
	case math.IsInf(v, -1):
		return "-INF"
	case math.IsNaN(v):
		return "NaN"
	}
	return strconv.FormatFloat(v, 'g', -1, bitSize)
}

// GoToLexical converts a Go value to its lexical form and XSD datatype.
// Ported from: rdflib.term — value-to-literal conversion
func GoToLexical(value any) (string, URIRef) {
	return goToLexical(value)
}
