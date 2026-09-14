package sparql

import (
	"math"
	"math/big"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Operators of SPARQL 1.1 §17.3. A nil rdflibgo.Term stands for an evaluation
// error (a type error, or an unbound variable, §17.4.1.3) throughout: an
// operator given an error returns an error, except where §17.2 says otherwise
// (the logical connectives).

func evalBinaryOp(op string, left, right rdflibgo.Term) rdflibgo.Term {
	switch op {
	case "=", "!=":
		eq, ok := rdfTermEqual(left, right)
		if !ok {
			return nil
		}
		if op == "!=" {
			eq = !eq
		}
		return rdflibgo.NewLiteral(eq)
	case "<", ">", "<=", ">=":
		c, ok := valueCompare(left, right)
		if !ok {
			return nil
		}
		if c == cmpUnordered {
			// NaN compares false with everything (XPath op:numeric-less-than).
			return rdflibgo.NewLiteral(false)
		}
		switch op {
		case "<":
			return rdflibgo.NewLiteral(c < 0)
		case ">":
			return rdflibgo.NewLiteral(c > 0)
		case "<=":
			return rdflibgo.NewLiteral(c <= 0)
		default:
			return rdflibgo.NewLiteral(c >= 0)
		}
	case "&&", "||":
		return logicalOp(op, left, right)
	case "+", "-", "*", "/":
		return arithmetic(op, left, right)
	}
	return nil
}

// logicalOp implements the §17.2 truth table for || and &&: an error on one
// side is masked when the other side decides the result on its own.
func logicalOp(op string, left, right rdflibgo.Term) rdflibgo.Term {
	l, lok := ebv(left)
	r, rok := ebv(right)
	if op == "||" {
		if (lok && l) || (rok && r) {
			return rdflibgo.NewLiteral(true)
		}
		if lok && rok {
			return rdflibgo.NewLiteral(false)
		}
		return nil
	}
	if (lok && !l) || (rok && !r) {
		return rdflibgo.NewLiteral(false)
	}
	if lok && rok {
		return rdflibgo.NewLiteral(true)
	}
	return nil
}

func arithmetic(op string, left, right rdflibgo.Term) rdflibgo.Term {
	a, ok := numericOf(left)
	if !ok {
		return nil
	}
	b, ok := numericOf(right)
	if !ok {
		return nil
	}
	return numericArithmetic(op, a, b)
}

func evalUnaryOp(op string, arg rdflibgo.Term) rdflibgo.Term {
	switch op {
	case "!":
		v, ok := ebv(arg)
		if !ok {
			return nil
		}
		return rdflibgo.NewLiteral(!v)
	case "-":
		n, ok := numericOf(arg)
		if !ok {
			return nil
		}
		if n.kind == numInteger {
			n.i = new(big.Int).Neg(n.i)
		}
		n.f = -n.f
		return numericLiteral(n)
	}
	return nil
}

// ebv returns the effective boolean value of t (SPARQL 1.1 §17.2.2). The second
// result is false when EBV is an error: t is unbound, or is not a boolean,
// numeric or string literal.
//
// An ill-typed numeric literal ("abc"^^xsd:integer) has EBV false, as §17.2.2
// states. An ill-typed xsd:boolean is an error: the SPARQL 1.2 test
// expression/not-not expects !!"z"^^xsd:boolean to be unbound.
func ebv(t rdflibgo.Term) (bool, bool) {
	l, ok := t.(rdflibgo.Literal)
	if !ok {
		return false, false
	}
	dt := l.Datatype()
	switch {
	case dt == rdflibgo.XSDBoolean:
		if !isBooleanLexical(l.Lexical()) {
			return false, false
		}
		return booleanValue(l), true
	case isNumericDatatype(dt):
		n, ok := numericOf(l)
		if !ok {
			return false, true
		}
		return n.f != 0 && !math.IsNaN(n.f), true
	case isSimpleOrXSDString(l):
		return l.Lexical() != "", true
	}
	return false, false
}

// effectiveBooleanValue is EBV with an error read as false, which is what a
// FILTER does with it (SPARQL 1.1 §17.2: a FILTER eliminates any solution for
// which the expression raises an error).
func effectiveBooleanValue(t rdflibgo.Term) bool {
	v, ok := ebv(t)
	return ok && v
}

// isSimpleOrXSDString reports whether l is a simple literal or has datatype
// xsd:string (no language tag).
func isSimpleOrXSDString(l rdflibgo.Literal) bool {
	if l.Language() != "" {
		return false
	}
	dt := l.Datatype()
	return dt == rdflibgo.XSDString || dt.Value() == ""
}

// termValuesEqual is rdfTermEqual with an error read as false.
func termValuesEqual(a, b rdflibgo.Term) bool {
	eq, ok := rdfTermEqual(a, b)
	return ok && eq
}

// rdfTermEqual implements the = operator: the operator mapping of SPARQL 1.1
// §17.3 for numerics, strings, booleans and date/times, falling back to
// RDFterm-equal (§17.4.1.7). The second result is false for a type error.
//
// RDFterm-equal raises a type error for two literals that are not the same
// term, unless their values can be shown to differ. Values are known to differ
// when both literals are well-formed in known, distinct value spaces, and a
// language-tagged literal differs from every literal but an equal
// language-tagged one (the approved DAWG open-eq-08/10/12 tests).
func rdfTermEqual(a, b rdflibgo.Term) (bool, bool) {
	if a == nil || b == nil {
		return false, false
	}
	la, aIsLit := a.(rdflibgo.Literal)
	lb, bIsLit := b.(rdflibgo.Literal)
	if aIsLit && bIsLit {
		return literalsEqual(la, lb)
	}
	ttA, aIsTT := a.(rdflibgo.TripleTerm)
	ttB, bIsTT := b.(rdflibgo.TripleTerm)
	if aIsTT && bIsTT {
		for _, pair := range [3][2]rdflibgo.Term{
			{ttA.Subject(), ttB.Subject()},
			{ttA.Predicate(), ttB.Predicate()},
			{ttA.Object(), ttB.Object()},
		} {
			eq, ok := rdfTermEqual(pair[0], pair[1])
			if !ok || !eq {
				return eq, ok
			}
		}
		return true, true
	}
	if aIsLit || bIsLit || aIsTT || bIsTT {
		return false, true
	}
	return a.N3() == b.N3(), true
}

func literalsEqual(a, b rdflibgo.Literal) (bool, bool) {
	na, aNum := numericOf(a)
	nb, bNum := numericOf(b)
	if aNum && bNum {
		if na.kind == numInteger && nb.kind == numInteger {
			return na.i.Cmp(nb.i) == 0, true
		}
		return na.f == nb.f, true // NaN != NaN
	}
	if a.Language() != "" || b.Language() != "" {
		if a.Language() == "" || b.Language() == "" {
			return false, true
		}
		return a.Lexical() == b.Lexical() && strings.EqualFold(a.Language(), b.Language()) && a.Dir() == b.Dir(), true
	}
	if sameLiteral(a, b) {
		return true, true
	}
	sa, sb := valueSpace(a), valueSpace(b)
	if sa == spaceUnknown || sb == spaceUnknown {
		return false, false
	}
	if sa != sb {
		return false, true
	}
	switch sa {
	case spaceString:
		return false, true // same space, not the same lexical form
	case spaceBoolean:
		return booleanValue(a) == booleanValue(b), true
	case spaceDateTime, spaceDate, spaceTime:
		if ta, tb, ok := parseDatePair(a, b); ok {
			return ta.Equal(tb), true
		}
	}
	return false, false
}

// sameLiteral reports whether a and b are the same RDF term, reading a simple
// literal as xsd:string (RDF 1.1).
func sameLiteral(a, b rdflibgo.Literal) bool {
	if a.Lexical() != b.Lexical() || a.Language() != b.Language() || a.Dir() != b.Dir() {
		return false
	}
	if isSimpleOrXSDString(a) && isSimpleOrXSDString(b) {
		return true
	}
	return a.Datatype() == b.Datatype()
}

type literalSpace int

const (
	spaceUnknown literalSpace = iota
	spaceString
	spaceNumeric
	spaceBoolean
	spaceDateTime
	spaceDate
	spaceTime
)

// valueSpace classifies a literal without a language tag by the value space of
// a supported datatype. An ill-typed literal, or one of a datatype the engine
// does not implement, is spaceUnknown: nothing can be said about its value.
func valueSpace(l rdflibgo.Literal) literalSpace {
	switch {
	case isSimpleOrXSDString(l):
		return spaceString
	case isNumericDatatype(l.Datatype()):
		if _, ok := numericOf(l); ok {
			return spaceNumeric
		}
	case l.Datatype() == rdflibgo.XSDBoolean:
		if isBooleanLexical(l.Lexical()) {
			return spaceBoolean
		}
	case isDateDatatype(l.Datatype()):
		// xsd:dateTime, xsd:date and xsd:time are distinct value spaces: XPath
		// has no operator comparing a dateTime with a date (DAWG open-cmp-01).
		if _, ok := parseDateTime(l.Lexical(), l.Datatype()); ok {
			switch l.Datatype() {
			case rdflibgo.XSDDate:
				return spaceDate
			case rdflibgo.XSDTime:
				return spaceTime
			}
			return spaceDateTime
		}
	}
	return spaceUnknown
}

func isBooleanLexical(s string) bool {
	return s == "true" || s == "false" || s == "1" || s == "0"
}

func booleanValue(l rdflibgo.Literal) bool {
	return l.Lexical() == "true" || l.Lexical() == "1"
}

// cmpUnordered is returned by valueCompare when a NaN is involved.
const cmpUnordered = 2

// valueCompare orders two operands of a relational operator (<, >, <=, >=)
// per the SPARQL 1.1 §17.3 operator mapping: numerics, simple literals /
// xsd:string, booleans and date/times. Anything else — including an ill-typed
// literal, IRIs and language-tagged literals — has no mapping and is a type
// error (second result false). ORDER BY uses compareTermValues instead, which
// is total.
func valueCompare(a, b rdflibgo.Term) (int, bool) {
	la, ok := a.(rdflibgo.Literal)
	if !ok {
		return 0, false
	}
	lb, ok := b.(rdflibgo.Literal)
	if !ok {
		return 0, false
	}
	sa, sb := literalSpace(spaceUnknown), literalSpace(spaceUnknown)
	if la.Language() == "" {
		sa = valueSpace(la)
	}
	if lb.Language() == "" {
		sb = valueSpace(lb)
	}
	if sa == spaceUnknown || sa != sb {
		return 0, false
	}
	switch sa {
	case spaceNumeric:
		na, _ := numericOf(la)
		nb, _ := numericOf(lb)
		if na.kind == numInteger && nb.kind == numInteger {
			return na.i.Cmp(nb.i), true
		}
		if math.IsNaN(na.f) || math.IsNaN(nb.f) {
			return cmpUnordered, true
		}
		return cmpFloat(na.f, nb.f), true
	case spaceString:
		return strings.Compare(la.Lexical(), lb.Lexical()), true
	case spaceBoolean:
		return cmpBool(booleanValue(la), booleanValue(lb)), true
	case spaceDateTime, spaceDate, spaceTime:
		ta, tb, ok := parseDatePair(la, lb)
		if !ok {
			return 0, false
		}
		return ta.Compare(tb), true
	}
	return 0, false
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	}
	return 1
}
