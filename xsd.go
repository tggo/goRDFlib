package rdflibgo

import "fmt"

// XSD namespace base URI.
const XSDNamespace = "http://www.w3.org/2001/XMLSchema#"

// newURIRefUnsafe creates a URIRef without validation. Used for known-good constants.
func newURIRefUnsafe(value string) URIRef {
	return URIRef{value: value}
}

// NewURIRefUnsafe creates a URIRef without validation. Exported for use in init-time
// constants and tests. Do not use for user-provided input — use NewURIRef instead.
func NewURIRefUnsafe(value string) URIRef {
	return URIRef{value: value}
}

// XSD datatype constants.
// Ported from: rdflib.namespace.XSD
var (
	XSDString   = newURIRefUnsafe(XSDNamespace + "string")
	XSDInteger  = newURIRefUnsafe(XSDNamespace + "integer")
	XSDInt      = newURIRefUnsafe(XSDNamespace + "int")
	XSDLong     = newURIRefUnsafe(XSDNamespace + "long")
	XSDFloat    = newURIRefUnsafe(XSDNamespace + "float")
	XSDDouble   = newURIRefUnsafe(XSDNamespace + "double")
	XSDDecimal  = newURIRefUnsafe(XSDNamespace + "decimal")
	XSDBoolean  = newURIRefUnsafe(XSDNamespace + "boolean")
	XSDDateTime = newURIRefUnsafe(XSDNamespace + "dateTime")
	XSDDate     = newURIRefUnsafe(XSDNamespace + "date")
	XSDTime     = newURIRefUnsafe(XSDNamespace + "time")
	XSDAnyURI   = newURIRefUnsafe(XSDNamespace + "anyURI")
)

// RDF namespace constants used by Literal.
const RDFNamespace = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

// RDFLangString is the datatype for language-tagged literals per RDF 1.1.
var RDFLangString = newURIRefUnsafe(RDFNamespace + "langString")

// GoToLexical converts a Go value to its lexical form and XSD datatype.
// Ported from: rdflib.term — value-to-literal conversion
func GoToLexical(value any) (string, URIRef) {
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("%d", v), XSDInteger
	case int64:
		return fmt.Sprintf("%d", v), XSDInteger
	case float32:
		return fmt.Sprintf("%g", v), XSDFloat
	case float64:
		return fmt.Sprintf("%g", v), XSDDouble
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
