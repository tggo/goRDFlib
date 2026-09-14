package term

// Turtle's numeric shorthand grammar (Turtle 1.1 [19]-[21], [154]).
//
// Literal.N3 writes an xsd:decimal or xsd:double without quotes only when the
// lexical form matches these productions exactly. The three productions are
// disjoint, so a shorthand token always parses back to the datatype it was
// written from, and TermKey (built from N3) stays injective. Checking with
// strconv.ParseFloat instead accepted "1.5e3" as a decimal and "0x1ep3" as a
// double; the first came back as an xsd:double and shared a store key with the
// genuine double, so the store kept only one of the two.

// isTurtleInteger reports whether s matches INTEGER ::= [+-]? [0-9]+.
func isTurtleInteger(s string) bool {
	i := skipSign(s)
	j := skipDigits(s, i)
	return j > i && j == len(s)
}

// isTurtleDecimal reports whether s matches DECIMAL ::= [+-]? [0-9]* '.' [0-9]+.
func isTurtleDecimal(s string) bool {
	i := skipSign(s)
	i = skipDigits(s, i)
	if i >= len(s) || s[i] != '.' {
		return false
	}
	j := skipDigits(s, i+1)
	return j > i+1 && j == len(s)
}

// isTurtleDouble reports whether s matches
// DOUBLE ::= [+-]? ([0-9]+ '.' [0-9]* EXPONENT | '.' [0-9]+ EXPONENT | [0-9]+ EXPONENT).
func isTurtleDouble(s string) bool {
	i := skipSign(s)
	intEnd := skipDigits(s, i)
	intDigits := intEnd - i
	i = intEnd
	if i < len(s) && s[i] == '.' {
		fracEnd := skipDigits(s, i+1)
		if intDigits == 0 && fracEnd == i+1 {
			return false // '.' EXPONENT needs fraction digits
		}
		i = fracEnd
	} else if intDigits == 0 {
		return false
	}
	return isExponent(s[i:])
}

// isExponent reports whether s matches EXPONENT ::= [eE] [+-]? [0-9]+.
func isExponent(s string) bool {
	if len(s) == 0 || (s[0] != 'e' && s[0] != 'E') {
		return false
	}
	i := skipSign(s[1:]) + 1
	j := skipDigits(s, i)
	return j > i && j == len(s)
}

func skipSign(s string) int {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		return 1
	}
	return 0
}

func skipDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
