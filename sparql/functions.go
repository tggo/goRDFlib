package sparql

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
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
			switch v := vals[0].(type) {
			case rdflibgo.URIRef:
				return rdflibgo.NewLiteral(v.Value())
			case rdflibgo.Literal:
				return rdflibgo.NewLiteral(v.Lexical())
			default:
				return rdflibgo.NewLiteral(vals[0].String())
			}
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
				return stringResult("", vals[0])
			}
			if len(vals) >= 3 {
				length := int(toFloat64(vals[2]))
				end := start + length
				if end > len(runes) {
					end = len(runes)
				}
				return stringResult(string(runes[start:end]), vals[0])
			}
			return stringResult(string(runes[start:]), vals[0])
		}
	case "UCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return stringResult(strings.ToUpper(termString(vals[0])), vals[0])
		}
	case "LCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return stringResult(strings.ToLower(termString(vals[0])), vals[0])
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
			// REPLACE requires string literal input
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				if isNumericDatatype(l.Datatype()) {
					return nil // type error
				}
			} else {
				return nil // non-literal
			}
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
			return stringResult(re.ReplaceAllString(termString(vals[0]), replacement), vals[0])
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
			if cond == nil {
				return nil // error in condition propagates
			}
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
	case "LANGMATCHES":
		vals := evalArgs()
		if len(vals) == 2 {
			tag := strings.ToLower(termString(vals[0]))
			range_ := strings.ToLower(termString(vals[1]))
			if range_ == "*" {
				return rdflibgo.NewLiteral(tag != "")
			}
			return rdflibgo.NewLiteral(tag == range_ || strings.HasPrefix(tag, range_+"-"))
		}
		return rdflibgo.NewLiteral(false)
	case "SAMETERM":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			return rdflibgo.NewLiteral(vals[0].N3() == vals[1].N3())
		}
		return rdflibgo.NewLiteral(false)

	// String constructors
	case "STRLANG":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil {
			// STRLANG requires a simple literal (no language, no datatype other than xsd:string)
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				if l.Language() != "" {
					return nil // type error
				}
				dt := l.Datatype()
				if dt != rdflibgo.XSDString && dt.Value() != "" {
					return nil // type error
				}
			}
			return rdflibgo.NewLiteral(termString(vals[0]), rdflibgo.WithLang(termString(vals[1])))
		}
	case "STRDT":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil {
			// STRDT requires a simple literal
			if l, ok := vals[0].(rdflibgo.Literal); ok {
				if l.Language() != "" {
					return nil // type error
				}
			}
			if u, ok := vals[1].(rdflibgo.URIRef); ok {
				return rdflibgo.NewLiteral(termString(vals[0]), rdflibgo.WithDatatype(u))
			}
		}
	case "STRBEFORE":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			if !isStringLiteral(vals[0]) || !isStringLiteral(vals[1]) {
				return nil
			}
			if !strArgCompatible(vals[0], vals[1]) {
				return nil
			}
			s := termString(vals[0])
			arg := termString(vals[1])
			if arg == "" {
				return stringResult("", vals[0])
			}
			idx := strings.Index(s, arg)
			if idx < 0 {
				return rdflibgo.NewLiteral("")
			}
			return stringResult(s[:idx], vals[0])
		}
	case "STRAFTER":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			if !isStringLiteral(vals[0]) || !isStringLiteral(vals[1]) {
				return nil
			}
			if !strArgCompatible(vals[0], vals[1]) {
				return nil
			}
			s := termString(vals[0])
			arg := termString(vals[1])
			if arg == "" {
				return stringResult(s, vals[0])
			}
			idx := strings.Index(s, arg)
			if idx < 0 {
				return rdflibgo.NewLiteral("")
			}
			return stringResult(s[idx+len(arg):], vals[0])
		}
	case "ENCODE_FOR_URI":
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewLiteral(encodeForURI(termString(vals[0])))
		}
	case "IRI", "URI":
		vals := evalArgs()
		if len(vals) == 1 && vals[0] != nil {
			var s string
			if u, ok := vals[0].(rdflibgo.URIRef); ok {
				s = u.Value()
			} else {
				s = termString(vals[0])
			}
			// Resolve relative URI against base
			if base, ok := prefixes["__base__"]; ok && !strings.Contains(s, ":") {
				s = base + s
			}
			return rdflibgo.NewURIRefUnsafe(s)
		}
	case "BNODE":
		if len(args) == 0 {
			return rdflibgo.NewBNode("")
		}
		vals := evalArgs()
		if len(vals) == 1 {
			return rdflibgo.NewBNode(termString(vals[0]))
		}

	// Date/time functions
	case "NOW":
		return rdflibgo.NewLiteral(timeNow(), rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime")))
	case "YEAR":
		vals := evalArgs()
		if len(vals) == 1 {
			if y, ok := extractDatePart(termString(vals[0]), "year"); ok {
				return rdflibgo.NewLiteral(y, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
	case "MONTH":
		vals := evalArgs()
		if len(vals) == 1 {
			if m, ok := extractDatePart(termString(vals[0]), "month"); ok {
				return rdflibgo.NewLiteral(m, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
	case "DAY":
		vals := evalArgs()
		if len(vals) == 1 {
			if d, ok := extractDatePart(termString(vals[0]), "day"); ok {
				return rdflibgo.NewLiteral(d, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
	case "HOURS":
		vals := evalArgs()
		if len(vals) == 1 {
			if h, ok := extractDatePart(termString(vals[0]), "hours"); ok {
				return rdflibgo.NewLiteral(h, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
	case "MINUTES":
		vals := evalArgs()
		if len(vals) == 1 {
			if m, ok := extractDatePart(termString(vals[0]), "minutes"); ok {
				return rdflibgo.NewLiteral(m, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
	case "SECONDS":
		vals := evalArgs()
		if len(vals) == 1 {
			if s, ok := extractDatePart(termString(vals[0]), "seconds"); ok {
				return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
			}
		}
	case "TIMEZONE":
		vals := evalArgs()
		if len(vals) == 1 {
			if tz, ok := extractTimezone(termString(vals[0])); ok {
				return rdflibgo.NewLiteral(tz, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dayTimeDuration")))
			}
		}
	case "TZ":
		vals := evalArgs()
		if len(vals) == 1 {
			if tz, ok := extractTZ(termString(vals[0])); ok {
				return rdflibgo.NewLiteral(tz)
			}
		}

	// Hash
	case "SHA384":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha512.Sum384([]byte(termString(vals[0])))
			return rdflibgo.NewLiteral(fmt.Sprintf("%x", h))
		}
	case "SHA512":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha512.Sum512([]byte(termString(vals[0])))
			return rdflibgo.NewLiteral(fmt.Sprintf("%x", h))
		}

	// Random/UUID
	case "RAND":
		return rdflibgo.NewLiteral(randFloat(), rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	case "UUID":
		return rdflibgo.NewURIRefUnsafe("urn:uuid:" + newUUID())
	case "STRUUID":
		return rdflibgo.NewLiteral(newUUID())

	// Cast functions
	case "XSD:BOOLEAN", "XSD:INTEGER", "XSD:FLOAT", "XSD:DOUBLE", "XSD:DECIMAL", "XSD:STRING":
		vals := evalArgs()
		if len(vals) == 1 && vals[0] != nil {
			return castXSD(name, vals[0])
		}
	}

	// Try cast with full IRI
	if strings.HasPrefix(name, "HTTP://WWW.W3.ORG/2001/XMLSCHEMA#") {
		vals := evalArgs()
		if len(vals) == 1 && vals[0] != nil {
			localName := strings.ToUpper(name[len("HTTP://WWW.W3.ORG/2001/XMLSCHEMA#"):])
			return castXSD("XSD:"+localName, vals[0])
		}
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

// stringResult creates a literal preserving language/datatype from the source term.
func stringResult(s string, source rdflibgo.Term) rdflibgo.Literal {
	if l, ok := source.(rdflibgo.Literal); ok {
		if lang := l.Language(); lang != "" {
			return rdflibgo.NewLiteral(s, rdflibgo.WithLang(lang))
		}
		if dt := l.Datatype(); dt != rdflibgo.XSDString {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(dt))
		}
	}
	return rdflibgo.NewLiteral(s)
}

// isStringLiteral checks if a term is a string-type literal (plain, xsd:string, or lang-tagged).
func isStringLiteral(t rdflibgo.Term) bool {
	l, ok := t.(rdflibgo.Literal)
	if !ok {
		return false
	}
	dt := l.Datatype()
	return dt == rdflibgo.XSDString || l.Language() != "" || dt.Value() == ""
}

// strArgCompatible checks if two string arguments are type-compatible for
// STRBEFORE/STRAFTER per SPARQL spec. Compatible if:
// - Both are simple/xsd:string literals (no lang)
// - Both have the same language tag
// - Second arg is simple/xsd:string (no lang)
func strArgCompatible(a, b rdflibgo.Term) bool {
	la, aLit := a.(rdflibgo.Literal)
	lb, bLit := b.(rdflibgo.Literal)
	if !aLit || !bLit {
		return false
	}
	aLang := la.Language()
	bLang := lb.Language()
	// If second arg has a language, first must have the same language
	if bLang != "" {
		return strings.EqualFold(aLang, bLang)
	}
	// Second arg is simple — compatible with anything
	return true
}

func encodeForURI(s string) string {
	return url.QueryEscape(s)
}

func timeNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func extractDatePart(dt, part string) (string, bool) {
	// Parse ISO 8601 datetime: 2011-01-10T14:45:13.815-05:00
	t, err := time.Parse(time.RFC3339, dt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", dt)
		if err != nil {
			t, err = time.Parse("2006-01-02", dt)
			if err != nil {
				return "", false
			}
		}
	}
	switch part {
	case "year":
		return strconv.Itoa(t.Year()), true
	case "month":
		return strconv.Itoa(int(t.Month())), true
	case "day":
		return strconv.Itoa(t.Day()), true
	case "hours":
		return strconv.Itoa(t.Hour()), true
	case "minutes":
		return strconv.Itoa(t.Minute()), true
	case "seconds":
		sec := float64(t.Second()) + float64(t.Nanosecond())/1e9
		if t.Nanosecond() == 0 {
			return fmt.Sprintf("%d", t.Second()), true
		}
		return fmt.Sprintf("%g", sec), true
	}
	return "", false
}

func extractTimezone(dt string) (string, bool) {
	t, err := time.Parse(time.RFC3339, dt)
	if err != nil {
		return "", false
	}
	_, offset := t.Zone()
	if offset == 0 {
		return "PT0S", true
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	sign := ""
	if hours < 0 {
		sign = "-"
		hours = -hours
		minutes = -minutes
	}
	if minutes == 0 {
		return fmt.Sprintf("%sPT%dH", sign, hours), true
	}
	return fmt.Sprintf("%sPT%dH%dM", sign, hours, minutes), true
}

func extractTZ(dt string) (string, bool) {
	// Return timezone string like "Z", "-05:00", etc.
	if strings.HasSuffix(dt, "Z") {
		return "Z", true
	}
	// Look for +HH:MM or -HH:MM at end
	if len(dt) >= 6 {
		tz := dt[len(dt)-6:]
		if (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' {
			return tz, true
		}
	}
	return "", true // no timezone info
}

func randFloat() float64 {
	return rand.Float64()
}

func newUUID() string {
	return uuid.New().String()
}

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
			if s == "true" {
				return rdflibgo.NewLiteral(1, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
			return rdflibgo.NewLiteral(0, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		}
		if isNumericDatatype(dt) {
			// From numeric: truncate to integer
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return rdflibgo.NewLiteral(int(f), rdflibgo.WithDatatype(rdflibgo.XSDInteger))
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
			if s == "true" {
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
			if s == "true" {
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
				return rdflibgo.NewLiteral("1.0", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
			}
			return rdflibgo.NewLiteral("0.0", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		// Reject scientific notation strings (not valid xsd:decimal)
		if !isNumericDatatype(dt) && strings.ContainsAny(s, "eE") {
			return nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return rdflibgo.NewLiteral(formatDecimal(f), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return nil

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
		} else if dt == rdflibgo.XSDInteger || dt == rdflibgo.XSDInt || dt == rdflibgo.XSDLong {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				s = strconv.FormatInt(v, 10)
			}
		} else if dt == rdflibgo.XSDDecimal {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if f == float64(int64(f)) {
					s = strconv.FormatInt(int64(f), 10)
				} else {
					s = strconv.FormatFloat(f, 'f', -1, 64)
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
