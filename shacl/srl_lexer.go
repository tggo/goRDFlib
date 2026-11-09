package shacl

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	iriutil "github.com/tggo/goRDFlib/internal/iri"
	"github.com/tggo/goRDFlib/term"
)

// srlTokenKind identifies types of SRL tokens.
type srlTokenKind int

const (
	srlTokEOF srlTokenKind = iota
	srlTokPrefix
	srlTokBase
	srlTokData
	srlTokRule
	srlTokWhere
	srlTokNot
	srlTokFilter
	srlTokSet      // SET
	srlTokBind     // BIND (the earlier spelling of SET, accepted for old rule files)
	srlTokGroup    // GROUP
	srlTokBy       // BY
	srlTokVersion  // VERSION
	srlTokImports  // IMPORTS
	srlTokDistinct // DISTINCT
	srlTokAgg      // COUNT, MIN, MAX, SUM (val holds the upper-cased name)
	srlTokStar     // *
	srlTokSlash    // /
	srlTokCaret    // ^ (inverse path)
	srlTokAssign   // :=
	srlTokLang     // @en, @en-GB, @ar--rtl (val holds the tag without @)
	srlTokA        // 'a' (rdf:type shorthand)
	srlTokLBrace
	srlTokRBrace
	srlTokLParen
	srlTokRParen
	srlTokLBrack
	srlTokRBrack
	srlTokDot
	srlTokSemicolon
	srlTokComma
	srlTokHat     // ^^
	srlTokTilde   // ~
	srlTokLTriple // << or <<(
	srlTokRTriple // >> or )>>
	srlTokLAnnot  // {|
	srlTokRAnnot  // |}
	srlTokIRI     // <http://...>
	srlTokPName   // prefix:local
	srlTokVar     // ?name
	srlTokBNode   // _:label
	srlTokString  // "..." or '...' or """...""" or '''...'''
	srlTokInteger // 123, -123, +123
	srlTokDecimal // 123.45
	srlTokDouble  // 123e10
	srlTokTrue    // true
	srlTokFalse   // false
)

type srlToken struct {
	kind srlTokenKind
	val  string
	pos  int
}

type srlLexer struct {
	input string
	pos   int
	base  string
}

func newSRLLexer(input string) *srlLexer {
	return &srlLexer{input: input}
}

func (l *srlLexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *srlLexer) peekRune() (rune, int) {
	if l.pos >= len(l.input) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(l.input[l.pos:])
}

func (l *srlLexer) advance() {
	l.pos++
}

func (l *srlLexer) skipWS() {
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if c == '#' {
			// Skip comment to end of line
			for l.pos < len(l.input) && l.input[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			l.pos++
			continue
		}
		break
	}
}

func (l *srlLexer) next() (srlToken, error) {
	l.skipWS()
	if l.pos >= len(l.input) {
		return srlToken{kind: srlTokEOF, pos: l.pos}, nil
	}

	start := l.pos
	c := l.input[l.pos]

	switch c {
	case '{':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '|' {
			l.pos += 2
			return srlToken{kind: srlTokLAnnot, pos: start}, nil
		}
		l.pos++
		return srlToken{kind: srlTokLBrace, pos: start}, nil
	case '}':
		l.pos++
		return srlToken{kind: srlTokRBrace, pos: start}, nil
	case '(':
		l.pos++
		return srlToken{kind: srlTokLParen, pos: start}, nil
	case ')':
		if l.pos+2 < len(l.input) && l.input[l.pos+1] == '>' && l.input[l.pos+2] == '>' {
			l.pos += 3
			return srlToken{kind: srlTokRTriple, val: ")>>", pos: start}, nil
		}
		l.pos++
		return srlToken{kind: srlTokRParen, pos: start}, nil
	case '[':
		l.pos++
		return srlToken{kind: srlTokLBrack, pos: start}, nil
	case ']':
		l.pos++
		return srlToken{kind: srlTokRBrack, pos: start}, nil
	case '.':
		// Check if followed by digit (could be decimal starting with .)
		if l.pos+1 < len(l.input) && l.input[l.pos+1] >= '0' && l.input[l.pos+1] <= '9' {
			return l.scanNumber()
		}
		l.pos++
		return srlToken{kind: srlTokDot, pos: start}, nil
	case ';':
		l.pos++
		return srlToken{kind: srlTokSemicolon, pos: start}, nil
	case ',':
		l.pos++
		return srlToken{kind: srlTokComma, pos: start}, nil
	case '~':
		l.pos++
		return srlToken{kind: srlTokTilde, pos: start}, nil
	case '@':
		return l.scanLangTag()
	case '^':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '^' {
			l.pos += 2
			return srlToken{kind: srlTokHat, pos: start}, nil
		}
		l.pos++
		return srlToken{kind: srlTokCaret, pos: start}, nil
	case '*':
		l.pos++
		return srlToken{kind: srlTokStar, pos: start}, nil
	case '/':
		l.pos++
		return srlToken{kind: srlTokSlash, pos: start}, nil
	case ':':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
			l.pos += 2
			return srlToken{kind: srlTokAssign, val: ":=", pos: start}, nil
		}
		return l.scanKeywordOrPName()
	case '$':
		return l.scanVariable()
	case '|':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '}' {
			l.pos += 2
			return srlToken{kind: srlTokRAnnot, pos: start}, nil
		}
		return srlToken{}, fmt.Errorf("unexpected '|' at position %d", l.pos)
	case '<':
		return l.scanAngleOrTriple()
	case '>':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {
			l.pos += 2
			return srlToken{kind: srlTokRTriple, val: ">>", pos: start}, nil
		}
		return srlToken{}, fmt.Errorf("unexpected '>' at position %d", l.pos)
	case '"', '\'':
		return l.scanString()
	case '?':
		return l.scanVariable()
	case '_':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == ':' {
			return l.scanBNode()
		}
		return l.scanKeywordOrPName()
	case '+', '-':
		if l.pos+1 < len(l.input) && (l.input[l.pos+1] >= '0' && l.input[l.pos+1] <= '9' || l.input[l.pos+1] == '.') {
			return l.scanNumber()
		}
		// Could be part of expression
		return srlToken{}, fmt.Errorf("unexpected '%c' at position %d", c, l.pos)
	default:
		if c >= '0' && c <= '9' {
			return l.scanNumber()
		}
		return l.scanKeywordOrPName()
	}
}

func (l *srlLexer) scanAngleOrTriple() (srlToken, error) {
	start := l.pos
	l.pos++ // skip <

	if l.pos < len(l.input) && l.input[l.pos] == '<' {
		l.pos++ // skip second <
		// Check for <<(
		l.skipWS()
		if l.pos < len(l.input) && l.input[l.pos] == '(' {
			l.pos++
			return srlToken{kind: srlTokLTriple, val: "<<(", pos: start}, nil
		}
		return srlToken{kind: srlTokLTriple, val: "<<", pos: start}, nil
	}

	// IRI: <...>
	var sb strings.Builder
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if c == '>' {
			l.pos++
			iri := sb.String()
			if !term.ValidIRI(iri) {
				return srlToken{}, fmt.Errorf("invalid IRI <%s> at position %d: an IRI cannot contain spaces, control characters or any of < > \" { } | \\ ^ `", iri, start)
			}
			if l.base != "" && !iriutil.IsAbsolute(iri) {
				iri = iriutil.Resolve(l.base, iri)
			}
			return srlToken{kind: srlTokIRI, val: iri, pos: start}, nil
		}
		if c == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			esc := l.input[l.pos]
			switch esc {
			case 'u':
				r, n := l.scanUnicodeEscape(4)
				if n == 0 {
					return srlToken{}, fmt.Errorf("invalid \\u escape at position %d", l.pos)
				}
				sb.WriteRune(r)
				continue
			case 'U':
				r, n := l.scanUnicodeEscape(8)
				if n == 0 {
					return srlToken{}, fmt.Errorf("invalid \\U escape at position %d", l.pos)
				}
				sb.WriteRune(r)
				continue
			default:
				sb.WriteByte(esc)
				l.pos++
				continue
			}
		}
		sb.WriteByte(c)
		l.pos++
	}
	return srlToken{}, fmt.Errorf("unterminated IRI at position %d", start)
}

// scanLangTag scans @lang or @lang--dir. The direction is ltr or rtl, in
// lower case, as in RDF 1.2 (an upper-case "--LTR" is a syntax error).
func (l *srlLexer) scanLangTag() (srlToken, error) {
	start := l.pos
	l.pos++ // skip @
	tagStart := l.pos
	isAlpha := func(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
	for l.pos < len(l.input) && isAlpha(l.input[l.pos]) {
		l.pos++
	}
	if l.pos == tagStart {
		return srlToken{}, fmt.Errorf("empty language tag at position %d", start)
	}
	for l.pos+1 < len(l.input) && l.input[l.pos] == '-' && l.input[l.pos+1] != '-' {
		l.pos++
		subStart := l.pos
		for l.pos < len(l.input) && (isAlpha(l.input[l.pos]) || (l.input[l.pos] >= '0' && l.input[l.pos] <= '9')) {
			l.pos++
		}
		if l.pos == subStart {
			return srlToken{}, fmt.Errorf("empty language subtag at position %d", subStart)
		}
	}
	lang := l.input[tagStart:l.pos]
	dir := ""
	if l.pos+1 < len(l.input) && l.input[l.pos] == '-' && l.input[l.pos+1] == '-' {
		l.pos += 2
		dirStart := l.pos
		for l.pos < len(l.input) && isAlpha(l.input[l.pos]) {
			l.pos++
		}
		dir = l.input[dirStart:l.pos]
		if dir != "ltr" && dir != "rtl" {
			return srlToken{}, fmt.Errorf("invalid base direction %q at position %d: it must be ltr or rtl in lower case", dir, dirStart)
		}
	}
	if !term.ValidLanguageTag(lang) {
		return srlToken{}, fmt.Errorf("invalid language tag %q at position %d (RFC 5646)", lang, tagStart)
	}
	val := lang
	if dir != "" {
		val = lang + "--" + dir
	}
	return srlToken{kind: srlTokLang, val: val, pos: start}, nil
}

func (l *srlLexer) scanUnicodeEscape(digits int) (rune, int) {
	l.pos++ // skip 'u' or 'U'
	if l.pos+digits > len(l.input) {
		return 0, 0
	}
	hex := l.input[l.pos : l.pos+digits]
	var val rune
	for _, c := range hex {
		val <<= 4
		switch {
		case c >= '0' && c <= '9':
			val |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			val |= rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			val |= rune(c-'A') + 10
		default:
			return 0, 0
		}
	}
	l.pos += digits
	return val, digits
}

func (l *srlLexer) scanString() (srlToken, error) {
	start := l.pos
	delim := l.input[l.pos]
	l.pos++

	// Check for triple-quoted string
	triple := false
	if l.pos+1 < len(l.input) && l.input[l.pos] == delim && l.input[l.pos+1] == delim {
		triple = true
		l.pos += 2
	}

	var sb strings.Builder
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if triple {
			if c == delim && l.pos+2 < len(l.input) && l.input[l.pos+1] == delim && l.input[l.pos+2] == delim {
				l.pos += 3
				return srlToken{kind: srlTokString, val: sb.String(), pos: start}, nil
			}
		} else if c == delim {
			l.pos++
			return srlToken{kind: srlTokString, val: sb.String(), pos: start}, nil
		}
		if c == '\\' {
			l.pos++
			if l.pos >= len(l.input) {
				return srlToken{}, fmt.Errorf("unterminated string at position %d", start)
			}
			esc := l.input[l.pos]
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '\'':
				sb.WriteByte('\'')
			case 'u':
				r, n := l.scanUnicodeEscape(4)
				if n == 0 {
					return srlToken{}, fmt.Errorf("invalid \\u escape at position %d", l.pos)
				}
				sb.WriteRune(r)
				continue
			case 'U':
				r, n := l.scanUnicodeEscape(8)
				if n == 0 {
					return srlToken{}, fmt.Errorf("invalid \\U escape at position %d", l.pos)
				}
				sb.WriteRune(r)
				continue
			default:
				sb.WriteByte(esc)
			}
			l.pos++
			continue
		}
		if !triple && (c == '\n' || c == '\r') {
			return srlToken{}, fmt.Errorf("unterminated string at position %d", start)
		}
		sb.WriteByte(c)
		l.pos++
	}
	return srlToken{}, fmt.Errorf("unterminated string at position %d", start)
}

func (l *srlLexer) scanVariable() (srlToken, error) {
	start := l.pos
	l.pos++ // skip ?
	nameStart := l.pos
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if isVarCharRune(r) {
			l.pos += size
		} else {
			break
		}
	}
	if l.pos == nameStart {
		return srlToken{}, fmt.Errorf("empty variable name at position %d", start)
	}
	return srlToken{kind: srlTokVar, val: l.input[nameStart:l.pos], pos: start}, nil
}

func (l *srlLexer) scanBNode() (srlToken, error) {
	start := l.pos
	l.pos += 2 // skip _:
	nameStart := l.pos
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if isPnChar(r) || r == '.' {
			// Don't end with '.'
			if r == '.' {
				// Look ahead
				next := l.pos + size
				if next >= len(l.input) {
					break
				}
				nr, _ := utf8.DecodeRuneInString(l.input[next:])
				if !isPnChar(nr) {
					break
				}
			}
			l.pos += size
		} else {
			break
		}
	}
	if l.pos == nameStart {
		return srlToken{}, fmt.Errorf("empty blank node label at position %d", start)
	}
	return srlToken{kind: srlTokBNode, val: l.input[nameStart:l.pos], pos: start}, nil
}

func (l *srlLexer) scanNumber() (srlToken, error) {
	start := l.pos
	kind := srlTokInteger

	// Optional sign
	if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
		l.pos++
	}

	// Integer part
	for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
		l.pos++
	}

	// Decimal part
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		next := l.pos + 1
		if next < len(l.input) && l.input[next] >= '0' && l.input[next] <= '9' {
			kind = srlTokDecimal
			l.pos++
			for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
				l.pos++
			}
		}
	}

	// Exponent
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		kind = srlTokDouble
		l.pos++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
			l.pos++
		}
	}

	return srlToken{kind: kind, val: l.input[start:l.pos], pos: start}, nil
}

func (l *srlLexer) scanKeywordOrPName() (srlToken, error) {
	start := l.pos

	// Read the first word
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if isPnChar(r) || r == ':' || r == '.' || r == '-' {
			l.pos += size
		} else {
			break
		}
	}

	word := l.input[start:l.pos]

	// Remove trailing dots not followed by valid PN chars
	for strings.HasSuffix(word, ".") {
		r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
		if !isPnChar(r) && r != ':' {
			word = word[:len(word)-1]
			l.pos--
		} else {
			break
		}
	}

	// Check keywords
	switch strings.ToUpper(word) {
	case "PREFIX":
		return srlToken{kind: srlTokPrefix, val: word, pos: start}, nil
	case "BASE":
		return srlToken{kind: srlTokBase, val: word, pos: start}, nil
	case "DATA":
		return srlToken{kind: srlTokData, val: word, pos: start}, nil
	case "RULE":
		return srlToken{kind: srlTokRule, val: word, pos: start}, nil
	case "WHERE":
		return srlToken{kind: srlTokWhere, val: word, pos: start}, nil
	case "NOT":
		return srlToken{kind: srlTokNot, val: word, pos: start}, nil
	case "FILTER":
		return srlToken{kind: srlTokFilter, val: word, pos: start}, nil
	case "SET":
		return srlToken{kind: srlTokSet, val: word, pos: start}, nil
	case "BIND":
		return srlToken{kind: srlTokBind, val: word, pos: start}, nil
	case "GROUP":
		return srlToken{kind: srlTokGroup, val: word, pos: start}, nil
	case "BY":
		return srlToken{kind: srlTokBy, val: word, pos: start}, nil
	case "VERSION":
		return srlToken{kind: srlTokVersion, val: word, pos: start}, nil
	case "IMPORTS":
		return srlToken{kind: srlTokImports, val: word, pos: start}, nil
	case "DISTINCT":
		return srlToken{kind: srlTokDistinct, val: word, pos: start}, nil
	case "COUNT", "MIN", "MAX", "SUM":
		return srlToken{kind: srlTokAgg, val: strings.ToUpper(word), pos: start}, nil
	case "TRUE":
		return srlToken{kind: srlTokTrue, val: word, pos: start}, nil
	case "FALSE":
		return srlToken{kind: srlTokFalse, val: word, pos: start}, nil
	}

	// 'a' keyword (rdf:type shorthand)
	if word == "a" {
		return srlToken{kind: srlTokA, val: word, pos: start}, nil
	}

	// Must be a prefixed name
	if strings.Contains(word, ":") {
		return srlToken{kind: srlTokPName, val: word, pos: start}, nil
	}

	return srlToken{}, fmt.Errorf("unexpected token %q at position %d", word, start)
}

func isVarCharRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isPnChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' ||
		r == 0xB7 || (r >= 0x0300 && r <= 0x036F) || (r >= 0x203F && r <= 0x2040)
}
