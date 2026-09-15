package term

import "testing"

// A base direction needs a language tag (RDF 1.2 Concepts §3.3).
func TestNewLiteralDropsDirectionWithoutLanguage(t *testing.T) {
	if l := NewLiteral("x", WithDir("rtl")); l.Dir() != "" || l.Datatype() != XSDString {
		t.Errorf("direction without a language: Dir=%q datatype=%s, want no direction and xsd:string", l.Dir(), l.Datatype().Value())
	}
	if l := NewLiteral("x", WithLang("abcdefghi"), WithDir("rtl")); l.Dir() != "" {
		t.Errorf("an invalid language tag is dropped, so the direction must go too; Dir=%q", l.Dir())
	}
	if l := NewLiteral("x", WithLang("ar"), WithDir("rtl")); l.Dir() != "rtl" || l.Datatype() != RDFDirLangString {
		t.Errorf("valid language + direction: Dir=%q datatype=%s", l.Dir(), l.Datatype().Value())
	}
}
