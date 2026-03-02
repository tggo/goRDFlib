package shacl

import (
	"testing"
)

func TestIsValidInteger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"+123", true},
		{"-123", true},
		{"0", true},
		{"", false},
		{"+", false},
		{"-", false},
		{"12.3", false},
		{"abc", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidInteger(tc.input); got != tc.want {
				t.Errorf("isValidInteger(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidDecimal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"1.0", true},
		{".5", true},
		{"1.", true},
		{"+1.0", true},
		{"-1.0", true},
		{"1.2.3", false},
		{"", false},
		{"+", false},
		{"123", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidDecimal(tc.input); got != tc.want {
				t.Errorf("isValidDecimal(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"1.0", true},
		{"INF", true},
		{"-INF", true},
		{"+INF", true},
		{"NaN", true},
		{"1.5e3", true},
		{"1.5E-3", true},
		{"", false},
		{"abc", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidFloat(tc.input); got != tc.want {
				t.Errorf("isValidFloat(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-01-15", true},
		{"2024-1-1", false},
		{"abcdefghij", false},
		{"2024-12-31Z", true},
		{"short", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidDate(tc.input); got != tc.want {
				t.Errorf("isValidDate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidDateTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-01-15T10:30:00", true},
		{"2024-01-15T10:30:00Z", true},
		{"2024-01-15", false},
		{"noThere", true}, // contains 'T'
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidDateTime(tc.input); got != tc.want {
				t.Errorf("isValidDateTime(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsWellFormedLiteral(t *testing.T) {
	t.Parallel()

	xsd := XSD

	tests := []struct {
		name string
		term Term
		want bool
	}{
		// integer types
		{"integer_valid", Literal("42", xsd+"integer", ""), true},
		{"integer_invalid", Literal("abc", xsd+"integer", ""), false},
		{"int_valid", Literal("100", xsd+"int", ""), true},
		{"int_overflow", Literal("9999999999999", xsd+"int", ""), false},
		{"long_valid", Literal("100", xsd+"long", ""), true},
		{"short_valid", Literal("100", xsd+"short", ""), true},
		{"short_overflow", Literal("40000", xsd+"short", ""), false},
		{"byte_valid", Literal("100", xsd+"byte", ""), true},
		{"byte_overflow", Literal("200", xsd+"byte", ""), false},
		{"unsignedInt_valid", Literal("100", xsd+"unsignedInt", ""), true},
		{"unsignedInt_negative", Literal("-1", xsd+"unsignedInt", ""), false},
		{"unsignedLong_valid", Literal("100", xsd+"unsignedLong", ""), true},
		{"unsignedShort_valid", Literal("100", xsd+"unsignedShort", ""), true},
		{"unsignedByte_valid", Literal("200", xsd+"unsignedByte", ""), true},
		{"unsignedByte_overflow", Literal("300", xsd+"unsignedByte", ""), false},
		// nonNegativeInteger
		{"nonNegativeInteger_valid", Literal("0", xsd+"nonNegativeInteger", ""), true},
		{"nonNegativeInteger_negative", Literal("-1", xsd+"nonNegativeInteger", ""), false},
		// positiveInteger
		{"positiveInteger_valid", Literal("1", xsd+"positiveInteger", ""), true},
		{"positiveInteger_zero", Literal("0", xsd+"positiveInteger", ""), false},
		// nonPositiveInteger
		{"nonPositiveInteger_valid", Literal("0", xsd+"nonPositiveInteger", ""), true},
		{"nonPositiveInteger_positive", Literal("1", xsd+"nonPositiveInteger", ""), false},
		// negativeInteger
		{"negativeInteger_valid", Literal("-1", xsd+"negativeInteger", ""), true},
		{"negativeInteger_zero", Literal("0", xsd+"negativeInteger", ""), false},
		// decimal
		{"decimal_valid", Literal("1.5", xsd+"decimal", ""), true},
		{"decimal_invalid", Literal("abc", xsd+"decimal", ""), false},
		// float / double
		{"float_valid", Literal("1.5", xsd+"float", ""), true},
		{"float_NaN", Literal("NaN", xsd+"float", ""), true},
		{"float_invalid", Literal("abc", xsd+"float", ""), false},
		{"double_valid", Literal("1.5e3", xsd+"double", ""), true},
		{"double_invalid", Literal("abc", xsd+"double", ""), false},
		// boolean
		{"boolean_true", Literal("true", xsd+"boolean", ""), true},
		{"boolean_false", Literal("false", xsd+"boolean", ""), true},
		{"boolean_1", Literal("1", xsd+"boolean", ""), true},
		{"boolean_0", Literal("0", xsd+"boolean", ""), true},
		{"boolean_invalid", Literal("yes", xsd+"boolean", ""), false},
		// date
		{"date_valid", Literal("2024-01-15", xsd+"date", ""), true},
		{"date_invalid", Literal("not-a-date", xsd+"date", ""), false},
		// dateTime
		{"dateTime_valid", Literal("2024-01-15T10:30:00", xsd+"dateTime", ""), true},
		{"dateTime_invalid", Literal("2024-01-15", xsd+"dateTime", ""), false},
		// string (always well-formed)
		{"string_valid", Literal("hello", xsd+"string", ""), true},
		// non-literal returns false
		{"non_literal", IRI("http://example.org/x"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isWellFormedLiteral(tc.term); got != tc.want {
				t.Errorf("isWellFormedLiteral(%v) = %v, want %v", tc.term, got, tc.want)
			}
		})
	}
}

func TestCompareLiterals(t *testing.T) {
	t.Parallel()

	xsd := XSD

	tests := []struct {
		name    string
		a, b    Term
		wantCmp int
		wantOk  bool
	}{
		{
			"numeric_less",
			Literal("1", xsd+"integer", ""),
			Literal("2", xsd+"integer", ""),
			-1, true,
		},
		{
			"numeric_equal",
			Literal("5", xsd+"integer", ""),
			Literal("5", xsd+"decimal", ""),
			0, true,
		},
		{
			"numeric_greater",
			Literal("10", xsd+"integer", ""),
			Literal("2", xsd+"integer", ""),
			1, true,
		},
		{
			"date_less",
			Literal("2024-01-01", xsd+"date", ""),
			Literal("2024-12-31", xsd+"date", ""),
			-1, true,
		},
		{
			"string_compare",
			Literal("abc", xsd+"string", ""),
			Literal("def", xsd+"string", ""),
			-1, true,
		},
		{
			"different_datatypes",
			Literal("1", xsd+"integer", ""),
			Literal("1", xsd+"string", ""),
			0, false,
		},
		{
			"non_literal_a",
			IRI("http://example.org/x"),
			Literal("1", xsd+"integer", ""),
			0, false,
		},
		{
			"non_literal_b",
			Literal("1", xsd+"integer", ""),
			IRI("http://example.org/x"),
			0, false,
		},
		{
			"dateTime_tz_incomparable",
			Literal("2024-01-15T10:30:00Z", xsd+"dateTime", ""),
			Literal("2024-01-15T10:30:00", xsd+"dateTime", ""),
			0, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmp, ok := compareLiterals(tc.a, tc.b)
			if ok != tc.wantOk {
				t.Fatalf("compareLiterals ok = %v, want %v", ok, tc.wantOk)
			}
			if ok && cmp != tc.wantCmp {
				t.Errorf("compareLiterals cmp = %d, want %d", cmp, tc.wantCmp)
			}
		})
	}
}

func TestMatchesNodeKind(t *testing.T) {
	t.Parallel()

	iri := IRI("http://example.org/x")
	blank := BlankNode("b0")
	lit := Literal("hello", "", "")

	tests := []struct {
		name string
		term Term
		nk   string
		want bool
	}{
		{"IRI_IRI", iri, SH + "IRI", true},
		{"IRI_BlankNode", iri, SH + "BlankNode", false},
		{"IRI_Literal", iri, SH + "Literal", false},
		{"Blank_BlankNode", blank, SH + "BlankNode", true},
		{"Blank_IRI", blank, SH + "IRI", false},
		{"Lit_Literal", lit, SH + "Literal", true},
		{"Lit_IRI", lit, SH + "IRI", false},
		{"IRI_BlankNodeOrIRI", iri, SH + "BlankNodeOrIRI", true},
		{"Blank_BlankNodeOrIRI", blank, SH + "BlankNodeOrIRI", true},
		{"Lit_BlankNodeOrIRI", lit, SH + "BlankNodeOrIRI", false},
		{"Blank_BlankNodeOrLiteral", blank, SH + "BlankNodeOrLiteral", true},
		{"Lit_BlankNodeOrLiteral", lit, SH + "BlankNodeOrLiteral", true},
		{"IRI_BlankNodeOrLiteral", iri, SH + "BlankNodeOrLiteral", false},
		{"IRI_IRIOrLiteral", iri, SH + "IRIOrLiteral", true},
		{"Lit_IRIOrLiteral", lit, SH + "IRIOrLiteral", true},
		{"Blank_IRIOrLiteral", blank, SH + "IRIOrLiteral", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchesNodeKind(tc.term, tc.nk); got != tc.want {
				t.Errorf("matchesNodeKind(%v, %q) = %v, want %v", tc.term, tc.nk, got, tc.want)
			}
		})
	}
}
