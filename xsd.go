package rdflibgo

import "fmt"

// XSD namespace constants.
// Ported from: rdflib.namespace.XSD
const XSDNamespace = "http://www.w3.org/2001/XMLSchema#"

// NewURIRefUnsafe creates a URIRef without validation. Used for known-good constants.
func NewURIRefUnsafe(value string) URIRef {
	return URIRef{value: value}
}

// XSD datatype constants.
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

	// RDF namespace constants used by Literal
	RDFNamespace = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFLangString = NewURIRefUnsafe(RDFNamespace + "langString")
)

// LexicalToGo converts a lexical form + datatype to a Go value.
func LexicalToGo(lexical string, datatype URIRef) (any, error) {
	// Deferred to Story 1.5 — basic types only
	return lexical, nil
}

// GoToLexical converts a Go value to lexical form + datatype.
func GoToLexical(value any) (string, URIRef) {
	switch v := value.(type) {
	case int:
		return intToString(v), XSDInteger
	case int64:
		return int64ToString(v), XSDInteger
	case float32:
		return float32ToString(v), XSDFloat
	case float64:
		return float64ToString(v), XSDDouble
	case bool:
		if v {
			return "true", XSDBoolean
		}
		return "false", XSDBoolean
	case string:
		return v, XSDString
	default:
		return fmt.Sprintf("%v", value), XSDString
	}
}
