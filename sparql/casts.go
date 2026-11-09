package sparql

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

func castXSD(name string, val rdflibgo.Term) rdflibgo.Term {
	lit, isLit := val.(rdflibgo.Literal)
	_, isURI := val.(rdflibgo.URIRef)

	switch name {
	case "XSD:BOOLEAN":
		if isURI {
			return nil // can't cast URI to boolean
		}
		if !isLit {
			return nil
		}
		s := lit.Lexical()
		dt := lit.Datatype()
		if dt == rdflibgo.XSDBoolean {
			// Normalize: "0"/"1" → "false"/"true"
			return rdflibgo.NewLiteral(effectiveBooleanValue(val))
		}
		if isNumericDatatype(dt) {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil
			}
			return rdflibgo.NewLiteral(f != 0)
		}
		// String/plain literal
		switch strings.ToLower(s) {
		case "true", "1":
			return rdflibgo.NewLiteral(true)
		case "false", "0":
			return rdflibgo.NewLiteral(false)
		default:
			return nil // can't cast arbitrary string to boolean
		}

	case "XSD:INTEGER":
		if !isLit {
			return nil
		}
		s := lit.Lexical()
		dt := lit.Datatype()
		if dt == rdflibgo.XSDBoolean {
			if s == "true" || s == "1" {
				return rdflibgo.NewLiteral(1, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
			return rdflibgo.NewLiteral(0, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		}
		if isNumericDatatype(dt) {
			// From numeric: truncate to integer
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if math.IsNaN(f) || math.IsInf(f, 0) || f > float64(math.MaxInt64) || f < float64(math.MinInt64) {
					return nil
				}
				return rdflibgo.NewLiteral(int64(f), rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
		// From string/plain: must be a valid integer lexical form
		if _, err := strconv.ParseInt(strings.TrimLeft(s, "+"), 10, 64); err == nil {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		}
		return nil

	case "XSD:FLOAT":
		if !isLit {
			return nil
		}
		s := lit.Lexical()
		if lit.Datatype() == rdflibgo.XSDBoolean {
			if s == "true" || s == "1" {
				s = "1.0"
			} else {
				s = "0.0"
			}
		}
		if f, err := strconv.ParseFloat(s, 32); err == nil {
			return rdflibgo.NewLiteral(strconv.FormatFloat(float64(float32(f)), 'E', -1, 32), rdflibgo.WithDatatype(rdflibgo.XSDFloat))
		}
		return nil

	case "XSD:DOUBLE":
		if !isLit {
			return nil
		}
		s := lit.Lexical()
		if lit.Datatype() == rdflibgo.XSDBoolean {
			if s == "true" || s == "1" {
				s = "1.0"
			} else {
				s = "0.0"
			}
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return rdflibgo.NewLiteral(strconv.FormatFloat(f, 'E', -1, 64), rdflibgo.WithDatatype(rdflibgo.XSDDouble))
		}
		return nil

	case "XSD:DECIMAL":
		if !isLit {
			return nil
		}
		s := lit.Lexical()
		dt := lit.Datatype()
		if dt == rdflibgo.XSDBoolean {
			if effectiveBooleanValue(val) {
				return rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
			}
			return rdflibgo.NewLiteral("0", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		// Reject scientific notation strings (not valid xsd:decimal)
		if !isNumericDatatype(dt) && strings.ContainsAny(s, "eE") {
			return nil
		}
		// An integer, decimal or decimal-shaped string converts exactly; a
		// float or double through its shortest representation.
		if (!isNumericDatatype(dt) || dt == rdflibgo.XSDDecimal || isIntegral(val)) && isDecimalLexical(strings.TrimSpace(s)) {
			if r, ok := new(big.Rat).SetString(decimalForRat(strings.TrimSpace(s))); ok {
				return rdflibgo.NewLiteral(formatRat(r), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
			}
		}
		// A float or double converts through its shortest representation;
		// NaN and INF have no decimal value (XPath casting: FOCA0002). A
		// string must be in the decimal lexical space, which ParseFloat's
		// "1_0", "0x10" and "Inf" are not.
		if dt == rdflibgo.XSDDouble || dt == rdflibgo.XSDFloat {
			if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
				return rdflibgo.NewLiteral(formatDecimal(f), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
			}
		}
		return nil

	case "XSD:DATETIME":
		// SPARQL 1.0 section 11.2 / 1.1 section 17.5, casting table: an
		// xsd:dateTime stays itself, a simple literal or xsd:string becomes
		// one when its lexical form is valid, anything else is an error.
		if !isLit {
			return nil
		}
		if lit.Datatype() == rdflibgo.XSDDateTime {
			if _, ok := parseDateTime(lit.Lexical(), rdflibgo.XSDDateTime); !ok {
				return nil
			}
			return lit
		}
		if !isSimpleOrXSDString(lit) {
			return nil
		}
		if _, ok := parseDateTime(lit.Lexical(), rdflibgo.XSDDateTime); !ok {
			return nil
		}
		return rdflibgo.NewLiteral(lit.Lexical(), rdflibgo.WithDatatype(rdflibgo.XSDDateTime))

	case "XSD:STRING":
		if isURI {
			u := val.(rdflibgo.URIRef)
			return rdflibgo.NewLiteral(u.Value(), rdflibgo.WithDatatype(rdflibgo.XSDString))
		}
		if !isLit {
			return nil
		}
		// Canonical string representation per datatype
		dt := lit.Datatype()
		s := lit.Lexical()
		if dt == rdflibgo.XSDBoolean {
			if effectiveBooleanValue(val) {
				s = "true"
			} else {
				s = "false"
			}
		} else if isIntegral(val) || dt == rdflibgo.XSDDecimal {
			// The canonical form of the exact value (XPath casting to
			// xs:string), which is also what STR() of a computed one is.
			if n, ok := numericOf(val); ok {
				if n.kind == numInteger {
					s = n.i.String()
				} else {
					s = formatRat(n.exact())
				}
			}
		} else if dt == rdflibgo.XSDDouble || dt == rdflibgo.XSDFloat {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if f == float64(int64(f)) && f != 0 {
					s = strconv.FormatInt(int64(f), 10)
				} else if f == 0 {
					s = "0"
				} else {
					s = strconv.FormatFloat(f, 'f', -1, 64)
				}
			}
		}
		return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDString))
	}
	return nil
}
