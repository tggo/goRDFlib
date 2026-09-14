package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// builtin is a SPARQL built-in function whose arguments are all evaluated
// before the call. Every argument it receives is bound: evalFunc raises the
// error for an unbound or failed argument before dispatching (§17.4.1.3), so
// an implementation only checks types. It returns nil for a type error.
type builtin struct {
	minArgs, maxArgs int // maxArgs < 0: unlimited
	fn               func(args []rdflibgo.Term, prefixes map[string]string) rdflibgo.Term
}

// builtins maps upper-cased function names to their implementations. The
// signatures follow SPARQL 1.1 §17.4 and SPARQL 1.2 for the triple term and
// base direction functions.
var builtins = map[string]builtin{
	// §17.4.2 functions on RDF terms
	"ISIRI": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return boolTerm(isIRI(a[0])) }},
	"ISURI": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return boolTerm(isIRI(a[0])) }},
	"ISBLANK": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
		_, ok := a[0].(rdflibgo.BNode)
		return boolTerm(ok)
	}},
	"ISLITERAL": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
		_, ok := a[0].(rdflibgo.Literal)
		return boolTerm(ok)
	}},
	// §17.4.2.5: false for an ill-typed numeric literal such as "1200"^^xsd:byte.
	"ISNUMERIC": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return boolTerm(isNumericTerm(a[0])) }},
	"SAMETERM":  {2, 2, fnSameTerm},
	"STR":       {1, 1, fnStr},
	"LANG":      {1, 1, fnLang},
	"DATATYPE":  {1, 1, fnDatatype},
	"IRI":       {1, 1, fnIRI},
	"URI":       {1, 1, fnIRI},
	"STRDT":     {2, 2, fnStrDT},
	"STRLANG":   {2, 2, fnStrLang},

	// §17.4.3 functions on strings
	"STRLEN":         {1, 1, fnStrLen},
	"SUBSTR":         {2, 3, fnSubstr},
	"UCASE":          {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return mapString(a[0], strings.ToUpper) }},
	"LCASE":          {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return mapString(a[0], strings.ToLower) }},
	"STRSTARTS":      {2, 2, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return stringTest(a, strings.HasPrefix) }},
	"STRENDS":        {2, 2, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return stringTest(a, strings.HasSuffix) }},
	"CONTAINS":       {2, 2, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return stringTest(a, strings.Contains) }},
	"STRBEFORE":      {2, 2, fnStrBefore},
	"STRAFTER":       {2, 2, fnStrAfter},
	"ENCODE_FOR_URI": {1, 1, fnEncodeForURI},
	"CONCAT":         {0, -1, fnConcat},
	"LANGMATCHES":    {2, 2, fnLangMatches},
	"REGEX":          {2, 3, fnRegex},
	"REPLACE":        {3, 4, fnReplace},

	// §17.4.4 functions on numerics
	"ABS":   {1, 1, fnAbs},
	"ROUND": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return roundLike(a[0], xpathRound) }},
	"CEIL":  {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return roundLike(a[0], mathCeil) }},
	"FLOOR": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return roundLike(a[0], mathFloor) }},

	// §17.4.5 functions on dates and times
	"YEAR":     {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "year") }},
	"MONTH":    {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "month") }},
	"DAY":      {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "day") }},
	"HOURS":    {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "hours") }},
	"MINUTES":  {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "minutes") }},
	"SECONDS":  {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return datePart(a[0], "seconds") }},
	"TIMEZONE": {1, 1, fnTimezone},
	"TZ":       {1, 1, fnTZ},

	// §17.4.6 hash functions
	"MD5":    {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return hashString(a[0], "MD5") }},
	"SHA1":   {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return hashString(a[0], "SHA1") }},
	"SHA256": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return hashString(a[0], "SHA256") }},
	"SHA384": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return hashString(a[0], "SHA384") }},
	"SHA512": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return hashString(a[0], "SHA512") }},

	// SPARQL 1.2 triple terms and base direction
	"ISTRIPLE": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
		_, ok := a[0].(rdflibgo.TripleTerm)
		return boolTerm(ok)
	}},
	"TRIPLE":    {3, 3, fnTriple},
	"SUBJECT":   {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return triplePart(a[0], 0) }},
	"PREDICATE": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return triplePart(a[0], 1) }},
	"OBJECT":    {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term { return triplePart(a[0], 2) }},
	"LANGDIR":   {1, 1, fnLangDir},
	"HASLANG": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
		l, ok := a[0].(rdflibgo.Literal)
		return boolTerm(ok && l.Language() != "")
	}},
	"HASLANGDIR": {1, 1, func(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
		l, ok := a[0].(rdflibgo.Literal)
		return boolTerm(ok && l.Dir() != "")
	}},
	"STRLANGDIR": {3, 3, fnStrLangDir},
}

func boolTerm(b bool) rdflibgo.Term { return rdflibgo.NewLiteral(b) }

func isIRI(t rdflibgo.Term) bool {
	_, ok := t.(rdflibgo.URIRef)
	return ok
}

// simpleString returns the lexical form of a simple literal or xsd:string,
// the argument type of e.g. MD5, STRDT and IRI.
func simpleString(t rdflibgo.Term) (string, bool) {
	l, ok := t.(rdflibgo.Literal)
	if !ok || !isSimpleOrXSDString(l) {
		return "", false
	}
	return l.Lexical(), true
}

// stringLiteral returns t as a "string literal" of §17.4.3: a simple literal,
// an xsd:string or a language-tagged literal.
func stringLiteral(t rdflibgo.Term) (rdflibgo.Literal, bool) {
	l, ok := t.(rdflibgo.Literal)
	if !ok || !isStringLiteral(l) {
		return rdflibgo.Literal{}, false
	}
	return l, true
}

func fnSameTerm(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	return boolTerm(a[0].N3() == a[1].N3())
}

func fnStr(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	switch v := a[0].(type) {
	case rdflibgo.URIRef:
		return rdflibgo.NewLiteral(v.Value())
	case rdflibgo.Literal:
		return rdflibgo.NewLiteral(v.Lexical())
	}
	return nil // §17.4.2.4: STR takes a literal or an IRI
}

func fnLang(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	if l, ok := a[0].(rdflibgo.Literal); ok {
		return rdflibgo.NewLiteral(l.Language())
	}
	return nil
}

func fnLangDir(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	if l, ok := a[0].(rdflibgo.Literal); ok {
		return rdflibgo.NewLiteral(l.Dir())
	}
	return nil
}

func fnDatatype(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, ok := a[0].(rdflibgo.Literal)
	if !ok {
		return nil
	}
	if isSimpleOrXSDString(l) {
		return rdflibgo.XSDString // a simple literal has datatype xsd:string (RDF 1.1)
	}
	return l.Datatype()
}

func fnIRI(a []rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	var s string
	if u, ok := a[0].(rdflibgo.URIRef); ok {
		s = u.Value()
	} else if str, ok := simpleString(a[0]); ok {
		s = str
	} else {
		return nil // §17.4.2.8: a simple literal, xsd:string or IRI
	}
	if base, ok := prefixes[baseURIKey]; ok && !strings.Contains(s, ":") {
		s = base + s
	}
	return rdflibgo.NewURIRefUnsafe(s)
}

func fnStrDT(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	lex, ok := simpleString(a[0])
	if !ok {
		return nil
	}
	dt, ok := a[1].(rdflibgo.URIRef)
	if !ok {
		return nil
	}
	return rdflibgo.NewLiteral(lex, rdflibgo.WithDatatype(dt))
}

func fnStrLang(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	lex, ok := simpleString(a[0])
	if !ok {
		return nil
	}
	lang, ok := simpleString(a[1])
	if !ok || lang == "" {
		return nil
	}
	return rdflibgo.NewLiteral(lex, rdflibgo.WithLang(lang))
}

func fnStrLangDir(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	lex, ok := simpleString(a[0])
	if !ok {
		return nil
	}
	lang, ok := simpleString(a[1])
	if !ok || lang == "" { // RDF 1.2: a base direction requires a language tag
		return nil
	}
	dir, ok := simpleString(a[2])
	if !ok || (dir != "ltr" && dir != "rtl") {
		return nil
	}
	return rdflibgo.NewLiteral(lex, rdflibgo.WithLang(lang), rdflibgo.WithDir(dir))
}

func fnTriple(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	subj, ok := a[0].(rdflibgo.Subject)
	if !ok {
		return nil
	}
	pred, ok := a[1].(rdflibgo.URIRef)
	if !ok {
		return nil
	}
	return rdflibgo.NewTripleTerm(subj, pred, a[2])
}

func triplePart(t rdflibgo.Term, i int) rdflibgo.Term {
	tt, ok := t.(rdflibgo.TripleTerm)
	if !ok {
		return nil
	}
	switch i {
	case 0:
		return tt.Subject()
	case 1:
		return tt.Predicate()
	}
	return tt.Object()
}
