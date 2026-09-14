package sparql

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// String functions of SPARQL 1.1 §17.4.3 and the hash functions of §17.4.6.

func fnStrLen(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, ok := stringLiteral(a[0])
	if !ok {
		return nil
	}
	return rdflibgo.NewLiteral(utf8.RuneCountInString(l.Lexical()))
}

// fnSubstr follows XPath fn:substring: it returns the characters at positions p
// with round(start) <= p < round(start) + round(length), so NaN or infinite
// bounds give an empty string rather than an error.
func fnSubstr(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, ok := stringLiteral(a[0])
	if !ok {
		return nil
	}
	start, ok := numericOf(a[1])
	if !ok {
		return nil
	}
	first := xpathRound(start.f)
	last := math.Inf(1)
	if len(a) == 3 {
		length, ok := numericOf(a[2])
		if !ok {
			return nil
		}
		last = first + xpathRound(length.f)
	}
	runes := []rune(l.Lexical())
	var sb strings.Builder
	for i, r := range runes {
		p := float64(i + 1)
		if p >= first && p < last {
			sb.WriteRune(r)
		}
	}
	return stringResult(sb.String(), l)
}

func mapString(t rdflibgo.Term, f func(string) string) rdflibgo.Term {
	l, ok := stringLiteral(t)
	if !ok {
		return nil
	}
	return stringResult(f(l.Lexical()), l)
}

// compatibleStrings checks the two string literal arguments of STRSTARTS,
// STRENDS, CONTAINS, STRBEFORE and STRAFTER for argument compatibility
// (§17.4.3.1.1).
func compatibleStrings(a []rdflibgo.Term) (rdflibgo.Literal, rdflibgo.Literal, bool) {
	l0, ok0 := stringLiteral(a[0])
	l1, ok1 := stringLiteral(a[1])
	if !ok0 || !ok1 || !strArgCompatible(l0, l1) {
		return l0, l1, false
	}
	return l0, l1, true
}

func stringTest(a []rdflibgo.Term, test func(s, sub string) bool) rdflibgo.Term {
	l0, l1, ok := compatibleStrings(a)
	if !ok {
		return nil
	}
	return boolTerm(test(l0.Lexical(), l1.Lexical()))
}

func fnStrBefore(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l0, l1, ok := compatibleStrings(a)
	if !ok {
		return nil
	}
	s, sub := l0.Lexical(), l1.Lexical()
	if sub == "" {
		return stringResult("", l0)
	}
	idx := strings.Index(s, sub)
	if idx < 0 {
		return rdflibgo.NewLiteral("")
	}
	return stringResult(s[:idx], l0)
}

func fnStrAfter(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l0, l1, ok := compatibleStrings(a)
	if !ok {
		return nil
	}
	s, sub := l0.Lexical(), l1.Lexical()
	if sub == "" {
		return stringResult(s, l0)
	}
	idx := strings.Index(s, sub)
	if idx < 0 {
		return rdflibgo.NewLiteral("")
	}
	return stringResult(s[idx+len(sub):], l0)
}

func fnEncodeForURI(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, ok := stringLiteral(a[0])
	if !ok {
		return nil
	}
	return rdflibgo.NewLiteral(encodeForURI(l.Lexical()))
}

// fnConcat keeps a language tag (and base direction) only when every argument
// has the same one (§17.4.3.12).
func fnConcat(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	var sb strings.Builder
	lang, dir := "", ""
	for i, v := range a {
		l, ok := stringLiteral(v)
		if !ok {
			return nil
		}
		sb.WriteString(l.Lexical())
		if i == 0 {
			lang, dir = l.Language(), l.Dir()
		} else if l.Language() != lang || l.Dir() != dir {
			lang, dir = "", ""
		}
	}
	switch {
	case lang != "" && dir != "":
		return rdflibgo.NewLiteral(sb.String(), rdflibgo.WithLang(lang), rdflibgo.WithDir(dir))
	case lang != "":
		return rdflibgo.NewLiteral(sb.String(), rdflibgo.WithLang(lang))
	}
	return rdflibgo.NewLiteral(sb.String())
}

func fnLangMatches(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	tag, ok := simpleString(a[0])
	if !ok {
		return nil
	}
	rng, ok := simpleString(a[1])
	if !ok {
		return nil
	}
	tag, rng = strings.ToLower(tag), strings.ToLower(rng)
	if rng == "*" {
		return boolTerm(tag != "")
	}
	return boolTerm(tag == rng || strings.HasPrefix(tag, rng+"-"))
}

// xpathRegexp compiles an XPath regular expression with its flags
// (XPath Functions §7.6.1.1: s, m, i, x, q). An invalid pattern or an unknown
// flag is an error.
func xpathRegexp(pattern, flags string) (*regexp.Regexp, bool) {
	var goFlags strings.Builder
	for _, f := range flags {
		switch f {
		case 'i', 's', 'm':
			goFlags.WriteRune(f)
		case 'x':
			pattern = stripRegexWhitespace(pattern)
		case 'q':
			pattern = regexp.QuoteMeta(pattern)
		default:
			return nil, false
		}
	}
	if goFlags.Len() > 0 {
		pattern = "(?" + goFlags.String() + ")" + pattern
	}
	re, err := cachedRegexpCompile(pattern)
	return re, err == nil
}

// stripRegexWhitespace implements the x flag: whitespace outside character
// classes is removed from the pattern.
func stripRegexWhitespace(p string) string {
	var sb strings.Builder
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '\\' && i+1 < len(p):
			sb.WriteByte(c)
			i++
			sb.WriteByte(p[i])
			continue
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case !inClass && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// regexArgs checks the text, pattern and optional flags arguments of REGEX
// and REPLACE: a string literal and simple literals.
func regexArgs(text, pattern rdflibgo.Term, flags []rdflibgo.Term) (rdflibgo.Literal, *regexp.Regexp, bool) {
	l, ok := stringLiteral(text)
	if !ok {
		return l, nil, false
	}
	pat, ok := simpleString(pattern)
	if !ok {
		return l, nil, false
	}
	fl := ""
	if len(flags) > 0 {
		if fl, ok = simpleString(flags[0]); !ok {
			return l, nil, false
		}
	}
	re, ok := xpathRegexp(pat, fl)
	return l, re, ok
}

func fnRegex(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, re, ok := regexArgs(a[0], a[1], a[2:])
	if !ok {
		return nil
	}
	return boolTerm(re.MatchString(l.Lexical()))
}

func fnReplace(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	l, re, ok := regexArgs(a[0], a[1], a[3:])
	if !ok {
		return nil
	}
	repl, ok := simpleString(a[2])
	if !ok {
		return nil
	}
	// XPath fn:replace raises FORX0003 when the pattern matches "".
	if re.MatchString("") {
		return nil
	}
	tmpl, ok := xpathReplacement(repl, re.NumSubexp())
	if !ok {
		return nil
	}
	return stringResult(re.ReplaceAllString(l.Lexical(), tmpl), l)
}

// xpathReplacement converts an XPath replacement string ($N references, \$ and
// \\ escapes) into a Go regexp template. A lone $ or \ is an error (FORX0004).
func xpathReplacement(r string, groups int) (string, bool) {
	var sb strings.Builder
	for i := 0; i < len(r); i++ {
		switch c := r[i]; c {
		case '\\':
			if i+1 >= len(r) || (r[i+1] != '\\' && r[i+1] != '$') {
				return "", false
			}
			i++
			if r[i] == '$' {
				sb.WriteString("$$")
			} else {
				sb.WriteByte('\\')
			}
		case '$':
			j := i + 1
			if j >= len(r) || r[j] < '0' || r[j] > '9' {
				return "", false
			}
			// Take the longest digit run that still names an existing group.
			n := int(r[j] - '0')
			j++
			for j < len(r) && r[j] >= '0' && r[j] <= '9' && n*10+int(r[j]-'0') <= groups {
				n = n*10 + int(r[j]-'0')
				j++
			}
			sb.WriteString("${" + strconv.Itoa(n) + "}")
			i = j - 1
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String(), true
}

func hashString(t rdflibgo.Term, alg string) rdflibgo.Term {
	s, ok := simpleString(t)
	if !ok {
		return nil // §17.4.6: the argument is a simple literal or xsd:string
	}
	var sum []byte
	switch alg {
	case "MD5":
		h := md5.Sum([]byte(s))
		sum = h[:]
	case "SHA1":
		h := sha1.Sum([]byte(s))
		sum = h[:]
	case "SHA256":
		h := sha256.Sum256([]byte(s))
		sum = h[:]
	case "SHA384":
		h := sha512.Sum384([]byte(s))
		sum = h[:]
	default:
		h := sha512.Sum512([]byte(s))
		sum = h[:]
	}
	return rdflibgo.NewLiteral(hex.EncodeToString(sum))
}
