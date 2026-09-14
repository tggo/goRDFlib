package turtle

import (
	"regexp"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestInvalidLanguageTagIsASyntaxError: "x"@abcdefghi used to parse as the
// plain literal "x", because term.WithLang drops a tag it rejects. RDF 1.2
// Turtle §7.2 requires a well-formed tag, so it is a positioned syntax error.
func TestInvalidLanguageTagIsASyntaxError(t *testing.T) {
	for _, lit := range []string{
		`"x"@abcdefghi`,
		`"x"@en-abcdefghi`,
		`"x"@en-`,
		`"x"@en--ltr-x`,
		`"x"@abcdefghi--rtl`,
		`"x"@e1`,
	} {
		src := "<http://e/s> <http://e/p> " + lit + " ."
		err := Parse(rdflibgo.NewGraph(), strings.NewReader(src))
		if err == nil {
			t.Errorf("%s: accepted an invalid language tag", lit)
			continue
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: error has no position: %v", lit, err)
		}
	}
	for _, lit := range []string{`"x"@en`, `"x"@en-US`, `"x"@zh-Hant-TW`, `"x"@de-1996`, `"x"@ar--rtl`, `"x"@abcdefgh`} {
		src := "<http://e/s> <http://e/p> " + lit + " ."
		if err := Parse(rdflibgo.NewGraph(), strings.NewReader(src)); err != nil {
			t.Errorf("%s: rejected a valid language tag: %v", lit, err)
		}
	}
}

// TestIsValidLangTagMatchesTermRule keeps the parser's check in step with the
// rule term.WithLang applies, so nothing the parser accepts is dropped later.
func TestIsValidLangTagMatchesTermRule(t *testing.T) {
	termRule := regexp.MustCompile(`^[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*$`)
	for _, tag := range []string{
		"", "a", "en", "EN", "en-US", "abcdefgh", "abcdefghi", "en-abcdefgh", "en-abcdefghi",
		"1en", "e1", "en-1", "en--US", "-en", "en-", "en_US", "de-1996-x", "zh-Hant-TW", "é",
	} {
		if got, want := isValidLangTag(tag), termRule.MatchString(tag); got != want {
			t.Errorf("isValidLangTag(%q) = %v, term rule says %v", tag, got, want)
		}
	}
}
