package shacl

import (
	"math/big"
)

// ClassConstraint implements sh:class.
type ClassConstraint struct {
	Class Term
}

func (c *ClassConstraint) ComponentIRI() string {
	return SH + "ClassConstraintComponent"
}

func (c *ClassConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !ctx.dataGraph.HasType(vn, c.Class) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// DatatypeConstraint implements sh:datatype.
type DatatypeConstraint struct {
	Datatype Term
}

func (c *DatatypeConstraint) ComponentIRI() string {
	return SH + "DatatypeConstraintComponent"
}

func (c *DatatypeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	dt := c.Datatype.Value()
	for _, vn := range valueNodes {
		if !vn.IsLiteral() || vn.Datatype() != dt {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		} else if !isWellFormedLiteral(vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func isWellFormedLiteral(t Term) bool {
	if !t.IsLiteral() {
		return false
	}
	dt := t.Datatype()
	val := t.Value()
	switch dt {
	case XSD + "integer",
		XSD + "nonNegativeInteger", XSD + "positiveInteger",
		XSD + "nonPositiveInteger", XSD + "negativeInteger":
		return isValidInteger(val) && isInIntegerRange(val, dt)
	case XSD + "int":
		return isValidInteger(val) && isInRange(val, -2147483648, 2147483647)
	case XSD + "long":
		return isValidInteger(val) && isInRange(val, -9223372036854775808, 9223372036854775807)
	case XSD + "short":
		return isValidInteger(val) && isInRange(val, -32768, 32767)
	case XSD + "byte":
		return isValidInteger(val) && isInRange(val, -128, 127)
	case XSD + "unsignedInt":
		return isValidInteger(val) && isInRange(val, 0, 4294967295)
	case XSD + "unsignedLong":
		return isValidInteger(val) && isInBigRange(val, "0", "18446744073709551615")
	case XSD + "unsignedShort":
		return isValidInteger(val) && isInRange(val, 0, 65535)
	case XSD + "unsignedByte":
		return isValidInteger(val) && isInRange(val, 0, 255)
	case XSD + "boolean":
		return val == "true" || val == "false" || val == "1" || val == "0"
	case XSD + "decimal":
		return isValidDecimal(val)
	case XSD + "float", XSD + "double":
		return isValidFloat(val)
	case XSD + "date":
		return isValidDate(val)
	case XSD + "dateTime":
		return isValidDateTime(val)
	}
	return true
}

func isValidInteger(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '+' || s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isValidDecimal(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '+' || s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	dot := false
	digits := false
	for i := start; i < len(s); i++ {
		if s[i] == '.' {
			if dot {
				return false
			}
			dot = true
		} else if s[i] >= '0' && s[i] <= '9' {
			digits = true
		} else {
			return false
		}
	}
	return digits
}

func isValidFloat(s string) bool {
	if s == "INF" || s == "-INF" || s == "+INF" || s == "NaN" {
		return true
	}
	return isValidDecimal(s) || isValidFloatWithExponent(s)
}

func isValidFloatWithExponent(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 'e' || s[i] == 'E' {
			return isValidDecimal(s[:i]) && isValidInteger(s[i+1:])
		}
	}
	return false
}

func isValidDate(s string) bool {
	if len(s) < 10 {
		return false
	}
	for i, c := range s[:10] {
		if i == 4 || i == 7 {
			if c != '-' {
				return false
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isValidDateTime(s string) bool {
	for _, c := range s {
		if c == 'T' {
			return true
		}
	}
	return false
}

func isInRange(val string, min, max int64) bool {
	n, ok := new(big.Int).SetString(val, 10)
	if !ok {
		return false
	}
	return n.Cmp(big.NewInt(min)) >= 0 && n.Cmp(big.NewInt(max)) <= 0
}

func isInBigRange(val, minS, maxS string) bool {
	n, ok := new(big.Int).SetString(val, 10)
	if !ok {
		return false
	}
	minN, ok1 := new(big.Int).SetString(minS, 10)
	maxN, ok2 := new(big.Int).SetString(maxS, 10)
	if !ok1 || !ok2 {
		return false
	}
	return n.Cmp(minN) >= 0 && n.Cmp(maxN) <= 0
}

func isInIntegerRange(val, dt string) bool {
	n, ok := new(big.Int).SetString(val, 10)
	if !ok {
		return false
	}
	switch dt {
	case XSD + "nonNegativeInteger":
		return n.Sign() >= 0
	case XSD + "positiveInteger":
		return n.Sign() > 0
	case XSD + "nonPositiveInteger":
		return n.Sign() <= 0
	case XSD + "negativeInteger":
		return n.Sign() < 0
	}
	return true
}

// NodeKindConstraint implements sh:nodeKind.
type NodeKindConstraint struct {
	NodeKind Term
}

func (c *NodeKindConstraint) ComponentIRI() string {
	return SH + "NodeKindConstraintComponent"
}

func (c *NodeKindConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	nk := c.NodeKind.Value()
	for _, vn := range valueNodes {
		if !matchesNodeKind(vn, nk) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func matchesNodeKind(t Term, nk string) bool {
	switch nk {
	case SH + "IRI":
		return t.IsIRI()
	case SH + "BlankNode":
		return t.IsBlank()
	case SH + "Literal":
		return t.IsLiteral()
	case SH + "BlankNodeOrIRI":
		return t.IsBlank() || t.IsIRI()
	case SH + "BlankNodeOrLiteral":
		return t.IsBlank() || t.IsLiteral()
	case SH + "IRIOrLiteral":
		return t.IsIRI() || t.IsLiteral()
	}
	return false
}
