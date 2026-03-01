package sparql

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// evalFunc evaluates a SPARQL built-in function.
// Ported from: rdflib.plugins.sparql.operators
func evalFunc(name string, args []Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	evalArgs := func() []rdflibgo.Term {
		var vals []rdflibgo.Term
		for _, a := range args {
			vals = append(vals, evalExpr(a, bindings, prefixes))
		}
		return vals
	}

	switch name {
	// Term constructors
	case "BOUND":
		if len(args) == 1 {
			if v, ok := args[0].(*VarExpr); ok {
				_, exists := bindings[v.Name]
				return rdflibgo.NewLiteral(exists)
			}
		}
		return rdflibgo.NewLiteral(false)

	case "ISIRI", "ISURI":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(rdflibgo.URIRef)
			return rdflibgo.NewLiteral(ok)
		}
	case "ISBLANK":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(rdflibgo.BNode)
			return rdflibgo.NewLiteral(ok)
		}
	case "ISLITERAL":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(rdflibgo.Literal)
			return rdflibgo.NewLiteral(ok)
		}
	case "ISNUMERIC":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				dt := l.Datatype()
				return rdflibgo.NewLiteral(dt == rdflibgo.XSDInteger || dt == rdflibgo.XSDFloat || dt == rdflibgo.XSDDouble || dt == rdflibgo.XSDDecimal)
			}
		}
		return rdflibgo.NewLiteral(false)

	// String functions
	case "STR":
		vals := evalArgs()
		if len(vals) == 1 && vals[0] != nil {
			return rdflibgo.NewLiteral(vals[0].String())
		}
	case "STRLEN":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(utf8.RuneCountInString(termString(vals[0])))
		}
	case "SUBSTR":
		vals := evalArgs()
		if len(vals) < 1 {
			return nil
		}
		s := termString(vals[0])
		runes := []rune(s)
		if len(vals) >= 2 {
			start := int(toFloat64(vals[1])) - 1 // SPARQL is 1-based
			if start < 0 {
				start = 0
			}
			if start >= len(runes) {
				return rdflibgo.NewLiteral("")
			}
			if len(vals) >= 3 {
				length := int(toFloat64(vals[2]))
				end := start + length
				if end > len(runes) {
					end = len(runes)
				}
				return rdflibgo.NewLiteral(string(runes[start:end]))
			}
			return rdflibgo.NewLiteral(string(runes[start:]))
		}
	case "UCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(strings.ToUpper(termString(vals[0])))
		}
	case "LCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(strings.ToLower(termString(vals[0])))
		}
	case "STRSTARTS":
		vals := evalArgs()
		if len(vals) == 2 {
			return rdflibgo.NewLiteral(strings.HasPrefix(termString(vals[0]), termString(vals[1])))
		}
	case "STRENDS":
		vals := evalArgs()
		if len(vals) == 2 {
			return rdflibgo.NewLiteral(strings.HasSuffix(termString(vals[0]), termString(vals[1])))
		}
	case "CONTAINS":
		vals := evalArgs()
		if len(vals) == 2 {
			return rdflibgo.NewLiteral(strings.Contains(termString(vals[0]), termString(vals[1])))
		}
	case "CONCAT":
		vals := evalArgs()
		var sb strings.Builder
		for _, v := range vals {
			sb.WriteString(termString(v))
		}
		return rdflibgo.NewLiteral(sb.String())
	case "REGEX":
		vals := evalArgs()
		if len(vals) >= 2 {
			pattern := termString(vals[1])
			flags := ""
			if len(vals) >= 3 {
				flags = termString(vals[2])
			}
			if strings.Contains(flags, "i") {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return rdflibgo.NewLiteral(false)
			}
			return rdflibgo.NewLiteral(re.MatchString(termString(vals[0])))
		}
	case "REPLACE":
		vals := evalArgs()
		if len(vals) >= 3 {
			pattern := termString(vals[1])
			replacement := termString(vals[2])
			flags := ""
			if len(vals) >= 4 {
				flags = termString(vals[3])
			}
			if strings.Contains(flags, "i") {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return vals[0]
			}
			return rdflibgo.NewLiteral(re.ReplaceAllString(termString(vals[0]), replacement))
		}

	// Term accessors
	case "LANG":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				return rdflibgo.NewLiteral(l.Language())
			}
		}
		return rdflibgo.NewLiteral("")
	case "DATATYPE":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				return l.Datatype()
			}
		}

	// Numeric
	case "ABS":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(math.Abs(toFloat64(vals[0])))
		}
	case "ROUND":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(math.Round(toFloat64(vals[0])))
		}
	case "CEIL":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(math.Ceil(toFloat64(vals[0])))
		}
	case "FLOOR":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(math.Floor(toFloat64(vals[0])))
		}

	// Hash
	case "MD5":
		vals := evalArgs()
		if len(vals) == 1 {
			h := md5.Sum([]byte(termString(vals[0])))
			return rdflibgo.NewLiteral(fmt.Sprintf("%x", h))
		}
	case "SHA1":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha1.Sum([]byte(termString(vals[0])))
			return rdflibgo.NewLiteral(fmt.Sprintf("%x", h))
		}
	case "SHA256":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha256.Sum256([]byte(termString(vals[0])))
			return rdflibgo.NewLiteral(fmt.Sprintf("%x", h))
		}

	// Conditional
	case "IF":
		if len(args) == 3 {
			cond := evalExpr(args[0], bindings, prefixes)
			if effectiveBooleanValue(cond) {
				return evalExpr(args[1], bindings, prefixes)
			}
			return evalExpr(args[2], bindings, prefixes)
		}
	case "COALESCE":
		for _, a := range args {
			v := evalExpr(a, bindings, prefixes)
			if v != nil {
				return v
			}
		}
		return nil
	case "SAMETERM":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			return rdflibgo.NewLiteral(vals[0].N3() == vals[1].N3())
		}
		return rdflibgo.NewLiteral(false)
	}

	return nil
}

// --- Helpers ---

func effectiveBooleanValue(t rdflibgo.Term) bool {
	if t == nil {
		return false
	}
	if l, ok := t.(rdflibgo.Literal); ok {
		switch l.Datatype() {
		case rdflibgo.XSDBoolean:
			return l.Lexical() == "true"
		case rdflibgo.XSDInteger, rdflibgo.XSDInt, rdflibgo.XSDLong:
			v, _ := strconv.ParseInt(l.Lexical(), 10, 64)
			return v != 0
		case rdflibgo.XSDFloat, rdflibgo.XSDDouble, rdflibgo.XSDDecimal:
			v, _ := strconv.ParseFloat(l.Lexical(), 64)
			return v != 0
		case rdflibgo.XSDString:
			return l.Lexical() != ""
		default:
			return l.Lexical() != ""
		}
	}
	return true
}

func toFloat64(t rdflibgo.Term) float64 {
	if t == nil {
		return 0
	}
	if l, ok := t.(rdflibgo.Literal); ok {
		f, _ := strconv.ParseFloat(l.Lexical(), 64)
		return f
	}
	return 0
}

func isIntegral(t rdflibgo.Term) bool {
	if l, ok := t.(rdflibgo.Literal); ok {
		return l.Datatype() == rdflibgo.XSDInteger || l.Datatype() == rdflibgo.XSDInt || l.Datatype() == rdflibgo.XSDLong
	}
	return false
}

func termString(t rdflibgo.Term) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func compareTermValues(a, b rdflibgo.Term) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	la, okA := a.(rdflibgo.Literal)
	lb, okB := b.(rdflibgo.Literal)
	if okA && okB {
		fa, errA := strconv.ParseFloat(la.Lexical(), 64)
		fb, errB := strconv.ParseFloat(lb.Lexical(), 64)
		if errA == nil && errB == nil {
			if fa < fb {
				return -1
			}
			if fa > fb {
				return 1
			}
			return 0
		}
	}
	return strings.Compare(a.N3(), b.N3())
}
