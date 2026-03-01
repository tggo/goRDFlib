package rdflibgo

import "testing"

func TestGoToLexical(t *testing.T) {
	tests := []struct {
		input    any
		lexical  string
		datatype URIRef
	}{
		{42, "42", XSDInteger},
		{int64(99), "99", XSDInteger},
		{float32(1.5), "1.5", XSDFloat},
		{3.14, "3.14", XSDDouble},
		{true, "true", XSDBoolean},
		{false, "false", XSDBoolean},
		{"hello", "hello", XSDString},
	}
	for _, tt := range tests {
		lex, dt := GoToLexical(tt.input)
		if lex != tt.lexical {
			t.Errorf("GoToLexical(%v) lexical = %q, want %q", tt.input, lex, tt.lexical)
		}
		if dt != tt.datatype {
			t.Errorf("GoToLexical(%v) datatype = %v, want %v", tt.input, dt, tt.datatype)
		}
	}
}

func TestGoToLexicalDefault(t *testing.T) {
	type custom struct{}
	lex, dt := GoToLexical(custom{})
	if dt != XSDString {
		t.Errorf("expected XSDString, got %v", dt)
	}
	if lex != "{}" {
		t.Errorf("expected {}, got %q", lex)
	}
}

func TestXSDConstants(t *testing.T) {
	if XSDString.Value() != "http://www.w3.org/2001/XMLSchema#string" {
		t.Error("XSDString wrong")
	}
	if XSDInteger.Value() != "http://www.w3.org/2001/XMLSchema#integer" {
		t.Error("XSDInteger wrong")
	}
	if XSDBoolean.Value() != "http://www.w3.org/2001/XMLSchema#boolean" {
		t.Error("XSDBoolean wrong")
	}
	if XSDDateTime.Value() != "http://www.w3.org/2001/XMLSchema#dateTime" {
		t.Error("XSDDateTime wrong")
	}
}
