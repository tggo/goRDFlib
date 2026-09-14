package sparql

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	rdflibgo "github.com/tggo/goRDFlib"
)

// queryStartTimeKey is used to pass the query start time through the prefixes map.
const queryStartTimeKey = "__query_start_time__"

// regexCache caches compiled regular expressions to avoid recompilation per row.
var regexCache sync.Map // pattern string → *regexp.Regexp

func cachedRegexpCompile(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// evalFunc evaluates a SPARQL built-in function call. It returns nil for an
// error: an unknown function, a wrong number of arguments, an argument that is
// unbound or failed to evaluate, or an argument of the wrong type.
// Ported from: rdflib.plugins.sparql.operators
func evalFunc(name string, args []Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	return evalFuncWithGraph(name, args, bindings, prefixes, nil, nil)
}

// evalFuncWithGraph is evalFunc with a graph for EXISTS inside the arguments.
func evalFuncWithGraph(name string, args []Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string, g *rdflibgo.Graph, namedGraphs map[string]*rdflibgo.Graph) rdflibgo.Term {
	evalArg := func(a Expr) rdflibgo.Term { return evalExprWithGraph(a, bindings, prefixes, g, namedGraphs) }
	// Functional forms (§17.4.1) evaluate their arguments themselves.
	switch name {
	case "BOUND":
		if len(args) == 1 {
			if v, ok := args[0].(*VarExpr); ok {
				_, exists := bindings[v.Name]
				return rdflibgo.NewLiteral(exists)
			}
		}
		return nil
	case "IF":
		if len(args) != 3 {
			return nil
		}
		cond, ok := ebv(evalArg(args[0]))
		if !ok {
			return nil // an error in the condition propagates (§17.4.1.2)
		}
		if cond {
			return evalArg(args[1])
		}
		return evalArg(args[2])
	case "COALESCE":
		for _, a := range args {
			if v := evalArg(a); v != nil {
				return v
			}
		}
		return nil
	case "BNODE":
		if len(args) == 0 {
			return rdflibgo.NewBNode("") // unique each call
		}
	case "RAND", "UUID", "STRUUID", "NOW":
		if len(args) != 0 {
			return nil
		}
		return evalNullary(name, prefixes)
	}

	vals := make([]rdflibgo.Term, len(args))
	for i, a := range args {
		v := evalArg(a)
		if v == nil {
			// §17.4.1.3 / §17.3: evaluating an unbound variable, or an
			// expression that raised an error, makes the call an error. No
			// function below accepts an error as an argument.
			return nil
		}
		vals[i] = v
	}

	if b, ok := builtins[name]; ok {
		if len(vals) < b.minArgs || (b.maxArgs >= 0 && len(vals) > b.maxArgs) {
			return nil
		}
		return b.fn(vals, prefixes)
	}

	switch name {
	case "BNODE":
		if len(vals) != 1 {
			return nil
		}
		key, ok := simpleString(vals[0])
		if !ok {
			return nil // §17.4.2.9: BNODE takes a simple literal or xsd:string
		}
		return rdflibgo.NewBNode("bnode_" + key)
	case "XSD:BOOLEAN", "XSD:INTEGER", "XSD:FLOAT", "XSD:DOUBLE", "XSD:DECIMAL", "XSD:STRING":
		if len(vals) == 1 {
			return castXSD(name, vals[0])
		}
		return nil
	}
	if strings.HasPrefix(name, "HTTP://WWW.W3.ORG/2001/XMLSCHEMA#") && len(vals) == 1 {
		return castXSD("XSD:"+name[len("HTTP://WWW.W3.ORG/2001/XMLSCHEMA#"):], vals[0])
	}
	return nil
}

// evalNullary evaluates the functions that take no arguments.
func evalNullary(name string, prefixes map[string]string) rdflibgo.Term {
	switch name {
	case "RAND":
		return rdflibgo.NewLiteral(randFloat(), rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	case "UUID":
		return rdflibgo.NewURIRefUnsafe("urn:uuid:" + newUUID())
	case "STRUUID":
		return rdflibgo.NewLiteral(newUUID())
	}
	// Per SPARQL 1.1 §17.4.5.1, NOW() must return the same value throughout a
	// single query evaluation.
	nowStr := prefixes[queryStartTimeKey]
	if nowStr == "" {
		nowStr = timeNow()
	}
	return rdflibgo.NewLiteral(nowStr, rdflibgo.WithDatatype(rdflibgo.XSDDateTime))
}

// --- Helpers ---

// toFloat64 returns the value of a numeric literal, or 0 when t is not a
// well-formed numeric literal. Only for callers that have already checked t
// with numericOf, or that need a sort key rather than a value.
func toFloat64(t rdflibgo.Term) float64 {
	n, _ := numericOf(t)
	return n.f
}

// isIntegral reports whether t is a literal of xsd:integer or a type derived
// from it.
func isIntegral(t rdflibgo.Term) bool {
	if l, ok := t.(rdflibgo.Literal); ok {
		k, ok := numericKind(l.Datatype())
		return ok && k == numInteger
	}
	return false
}

// termString returns the string form of a term, "" for nil. It is not an
// argument check: built-in functions validate their arguments with
// simpleString / stringLiteral and treat nil as an error before reaching it.
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

// encodeForURI implements SPARQL ENCODE_FOR_URI per §17.4.3.14.
// Only RFC 3986 unreserved characters (A-Z a-z 0-9 - _ . ~) are left unencoded.
func encodeForURI(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 3) // worst case
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
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
	return rand.Float64() // math/rand/v2: goroutine-safe global source
}

func newUUID() string {
	return uuid.New().String()
}
