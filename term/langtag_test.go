package term

import "testing"

func TestValidLanguageTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"en": true, "EN-gb": true, "zh-Hant-TW": true, "de-1996": true, "x-private1": true,
		"abcdefgh": true, "abcdefghi": false, "en-abcdefghi": false,
		"": false, "1en": false, "en-": false, "-en": false, "en--ltr": false, "en_GB": false, "é": false,
	} {
		if got := ValidLanguageTag(tag); got != want {
			t.Errorf("ValidLanguageTag(%q) = %v, want %v", tag, got, want)
		}
	}
}

// WithLang drops a tag ValidLanguageTag rejects; the documented contract is
// that callers check first.
func TestWithLangAgreesWithValidLanguageTag(t *testing.T) {
	for _, tag := range []string{"en-GB", "abcdefghi", "123-bad!"} {
		l := NewLiteral("x", WithLang(tag))
		if kept := l.Language() != ""; kept != ValidLanguageTag(tag) {
			t.Errorf("WithLang(%q) kept=%v, ValidLanguageTag=%v", tag, kept, ValidLanguageTag(tag))
		}
	}
}
