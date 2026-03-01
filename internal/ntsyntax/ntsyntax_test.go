package ntsyntax

import (
	"testing"
)

func TestUnescapeIRI_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://example.org", "http://example.org"},
		{`http://example.org/\u0041`, "http://example.org/A"},
		{`http://example.org/\U00000042`, "http://example.org/B"},
	}
	for _, tt := range tests {
		got, err := UnescapeIRI(tt.input)
		if err != nil {
			t.Errorf("UnescapeIRI(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("UnescapeIRI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUnescapeIRI_MalformedReturnsError(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"truncated \\u", `http://\u00`},
		{"truncated \\U", `http://\U0000`},
		{"invalid hex \\u", `http://\uZZZZ`},
		{"invalid hex \\U", `http://\UZZZZZZZZ`},
		{"unknown escape", `http://\x41`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnescapeIRI(tt.input)
			if err == nil {
				t.Errorf("UnescapeIRI(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestEscapeIRI_AngleBrackets(t *testing.T) {
	got := EscapeIRI("http://example.org/<test>")
	if got == "http://example.org/<test>" {
		t.Errorf("EscapeIRI should escape < and >, got: %s", got)
	}
	// Should contain \u003C and \u003E
	if got != `http://example.org/\u003Ctest\u003E` {
		t.Errorf("unexpected: %s", got)
	}
}

func TestEscapeString_FastPath(t *testing.T) {
	// Pure ASCII, no special chars -> should return same string (no alloc)
	input := "hello world 123"
	got := EscapeString(input)
	if got != input {
		t.Errorf("EscapeString fast path failed: %q", got)
	}
}

func TestEscapeString_SpecialChars(t *testing.T) {
	got := EscapeString("a\nb\tc\\d\"e")
	want := `a\nb\tc\\d\"e`
	if got != want {
		t.Errorf("EscapeString(%q) = %q, want %q", "a\nb\tc\\d\"e", got, want)
	}
}

func TestEscapeString_ControlChar(t *testing.T) {
	got := EscapeString("\x01")
	if got != `\u0001` {
		t.Errorf("got %q", got)
	}
}

func TestTerm_UnsupportedType(t *testing.T) {
	_, err := Term(nil)
	if err == nil {
		t.Error("expected error for nil term")
	}
}

func TestEscapeIRI_NoEscapeNeeded(t *testing.T) {
	input := "http://example.org/path"
	got := EscapeIRI(input)
	if got != input {
		t.Errorf("expected fast path, got %q", got)
	}
}
