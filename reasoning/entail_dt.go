package reasoning

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Datatype value spaces for the entailment checker (RDF 1.1 Semantics §7,
// RDF 1.2 Semantics). A recognized datatype maps each well-typed literal to a
// value; two literals denote the same thing exactly when their values are
// equal, which the checker implements by interning literals under a canonical
// value key. Only the datatypes below can be recognized: a datatype whose
// lexical-to-value mapping is not implemented here must not be silently
// treated as opaque, because that would turn "recognized" into a lie.

const (
	nsXSD  = "http://www.w3.org/2001/XMLSchema#"
	nsRDF  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS = "http://www.w3.org/2000/01/rdf-schema#"
)

// family names the value space a datatype's values live in. Datatypes of one
// family share values (xsd:integer 1 and xsd:decimal 1.0 are the same value);
// different families are disjoint (XSD 1.1 Part 2 §3.3: float, double and
// decimal have disjoint value spaces).
type family string

const (
	famString family = "string"
	famLang   family = "lang"
	famDirLng family = "dirlang"
	famBool   family = "boolean"
	famDec    family = "decimal"
	famFloat  family = "float"
	famDouble family = "double"
	famXML    family = "xml"
	famJSON   family = "json"
)

// dtSpec describes one recognizable datatype.
type dtSpec struct {
	iri      string
	fam      family
	integral bool     // decimal family: only whole numbers
	lo, hi   *big.Int // decimal family: inclusive bounds, nil = unbounded
}

var dtSpecs = buildDatatypeSpecs()

func buildDatatypeSpecs() map[string]*dtSpec {
	m := make(map[string]*dtSpec, 32)
	add := func(s *dtSpec) { m[s.iri] = s }
	add(&dtSpec{iri: nsXSD + "string", fam: famString})
	add(&dtSpec{iri: nsRDF + "langString", fam: famLang})
	add(&dtSpec{iri: nsRDF + "dirLangString", fam: famDirLng})
	add(&dtSpec{iri: nsXSD + "boolean", fam: famBool})
	add(&dtSpec{iri: nsXSD + "float", fam: famFloat})
	add(&dtSpec{iri: nsXSD + "double", fam: famDouble})
	add(&dtSpec{iri: nsRDF + "XMLLiteral", fam: famXML})
	add(&dtSpec{iri: nsRDF + "JSON", fam: famJSON})
	add(&dtSpec{iri: nsXSD + "decimal", fam: famDec})

	pow := func(base, exp int64) *big.Int {
		return new(big.Int).Exp(big.NewInt(base), big.NewInt(exp), nil)
	}
	neg := func(x *big.Int) *big.Int { return new(big.Int).Neg(x) }
	dec1 := func(x *big.Int) *big.Int { return new(big.Int).Sub(x, big.NewInt(1)) }
	ints := []struct {
		name   string
		lo, hi *big.Int
	}{
		{"integer", nil, nil},
		{"long", neg(pow(2, 63)), dec1(pow(2, 63))},
		{"int", neg(pow(2, 31)), dec1(pow(2, 31))},
		{"short", neg(pow(2, 15)), dec1(pow(2, 15))},
		{"byte", neg(pow(2, 7)), dec1(pow(2, 7))},
		{"nonNegativeInteger", big.NewInt(0), nil},
		{"positiveInteger", big.NewInt(1), nil},
		{"nonPositiveInteger", nil, big.NewInt(0)},
		{"negativeInteger", nil, big.NewInt(-1)},
		{"unsignedLong", big.NewInt(0), dec1(pow(2, 64))},
		{"unsignedInt", big.NewInt(0), dec1(pow(2, 32))},
		{"unsignedShort", big.NewInt(0), dec1(pow(2, 16))},
		{"unsignedByte", big.NewInt(0), dec1(pow(2, 8))},
	}
	for _, d := range ints {
		add(&dtSpec{iri: nsXSD + d.name, fam: famDec, integral: true, lo: d.lo, hi: d.hi})
	}
	return m
}

// supportedDatatypes lists the recognizable datatype IRIs, sorted, for error
// messages.
func supportedDatatypes() []string {
	out := make([]string, 0, len(dtSpecs))
	for iri := range dtSpecs {
		out = append(out, iri)
	}
	sort.Strings(out)
	return out
}

// bounds returns the integer range of s as inclusive big.Ints; nil is open.
func (s *dtSpec) contains(r *big.Rat) bool {
	if s.integral && !r.IsInt() {
		return false
	}
	if s.lo != nil && r.Cmp(new(big.Rat).SetInt(s.lo)) < 0 {
		return false
	}
	if s.hi != nil && r.Cmp(new(big.Rat).SetInt(s.hi)) > 0 {
		return false
	}
	return true
}

// subsumes reports whether every value of b is a value of a.
func (a *dtSpec) subsumes(b *dtSpec) bool {
	if a.fam != b.fam {
		return false
	}
	if a.fam != famDec {
		return true
	}
	if a.integral && !b.integral {
		return false
	}
	if a.lo != nil && (b.lo == nil || b.lo.Cmp(a.lo) < 0) {
		return false
	}
	if a.hi != nil && (b.hi == nil || b.hi.Cmp(a.hi) > 0) {
		return false
	}
	return true
}

// overlaps reports whether the two value spaces share a value.
func (a *dtSpec) overlaps(b *dtSpec) bool {
	if a.fam != b.fam {
		return false
	}
	if a.fam != famDec {
		return true
	}
	// Integral ranges: [lo,hi] intersect. A non-integral decimal space always
	// overlaps an integral one inside the same bounds, so bounds decide.
	if a.hi != nil && b.lo != nil && a.hi.Cmp(b.lo) < 0 {
		return false
	}
	if b.hi != nil && a.lo != nil && b.hi.Cmp(a.lo) < 0 {
		return false
	}
	return true
}

// value is the denotation of a well-typed literal of a recognized datatype.
type value struct {
	fam family
	key string   // canonical key; equal keys, equal values
	rat *big.Rat // decimal family only
}

var (
	reInteger = regexp.MustCompile(`^[+-]?[0-9]+$`)
	reDecimal = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)$`)
	reFloat   = regexp.MustCompile(`^([+-]?(([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?|INF)|NaN)$`)
)

// valueOf maps a lexical form to the value it denotes under datatype s, or
// reports that the form is not in the lexical space (the literal is
// ill-typed). The lexical space is exactly the XSD one with no whitespace
// handling: RDF 1.1 Concepts §5.1 takes the lexical space as given, so
// " 3 "^^xsd:int is ill-typed.
func (s *dtSpec) valueOf(lex string) (value, bool) {
	switch s.fam {
	case famString:
		return value{fam: famString, key: "S" + lex}, true
	case famBool:
		switch lex {
		case "true", "1":
			return value{fam: famBool, key: "B1"}, true
		case "false", "0":
			return value{fam: famBool, key: "B0"}, true
		}
		return value{}, false
	case famDec:
		re := reDecimal
		if s.integral {
			re = reInteger
		}
		if !re.MatchString(lex) {
			return value{}, false
		}
		r, ok := new(big.Rat).SetString(strings.TrimPrefix(lex, "+"))
		if !ok || !s.contains(r) {
			return value{}, false
		}
		return value{fam: famDec, key: "D" + r.String(), rat: r}, true
	case famFloat, famDouble:
		return floatValue(s.fam, lex)
	case famXML:
		if !wellFormedXMLContent(lex) {
			return value{}, false
		}
		// Value identity is the lexical form: canonicalising XML (RDF 1.1
		// Concepts §5.2, exclusive c14n) is not implemented, so two literals
		// that differ only in canonical-equivalent markup are kept apart.
		return value{fam: famXML, key: "X" + lex}, true
	case famJSON:
		k, ok := canonicalJSON(lex)
		if !ok {
			return value{}, false
		}
		return value{fam: famJSON, key: "J" + k}, true
	}
	return value{}, false
}

func floatValue(f family, lex string) (value, bool) {
	if !reFloat.MatchString(lex) {
		return value{}, false
	}
	bits := 32
	prefix := "F"
	if f == famDouble {
		bits = 64
		prefix = "P"
	}
	var v float64
	switch strings.TrimPrefix(lex, "+") {
	case "INF":
		v = math.Inf(1)
	case "-INF":
		v = math.Inf(-1)
	case "NaN":
		v = math.NaN()
	default:
		var err error
		v, err = strconv.ParseFloat(lex, bits)
		// A range error still yields the correctly signed infinity, which is
		// what XSD 1.1 Part 2 §3.3.4.2 specifies for a too-large magnitude.
		if err != nil && !math.IsInf(v, 0) {
			return value{}, false
		}
	}
	// The key keeps the sign of zero: XSD 1.1 §3.3.4 makes 0 and -0 two values.
	return value{fam: f, key: prefix + strconv.FormatFloat(v, 'g', -1, bits)}, true
}

// wellFormedXMLContent reports whether s is a well-formed XML content
// fragment (RDF 1.1 Concepts §5.2: the lexical space of rdf:XMLLiteral).
func wellFormedXMLContent(s string) bool {
	dec := xml.NewDecoder(strings.NewReader("<r>" + s + "</r>"))
	dec.Strict = true
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return depth == 0
		}
		if err != nil {
			return false
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

// canonicalJSON returns a canonical form of a JSON text such that two texts
// have the same form exactly when they denote the same rdf:JSON value
// (RDF 1.2 Concepts: object member order is irrelevant, array order is
// significant, numbers are compared as IEEE 754 doubles, so 1E400 and 1E401
// are both +Infinity while 0 and -0 differ).
func canonicalJSON(s string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", false
	}
	var b bytes.Buffer
	writeCanonicalJSON(&b, v)
	return b.String(), true
}

func writeCanonicalJSON(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case json.Number:
		f, _ := strconv.ParseFloat(string(x), 64) // range error: ±Inf, as specified
		b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	case string:
		b.WriteString(strconv.Quote(x))
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonicalJSON(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Quote(k))
			b.WriteByte(':')
			writeCanonicalJSON(b, x[k])
		}
		b.WriteByte('}')
	}
}
