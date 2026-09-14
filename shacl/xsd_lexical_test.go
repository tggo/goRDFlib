package shacl

import (
	"testing"
)

// Ill-typed literals passed sh:datatype (pySHACL #151, #167). SHACL 1.0 §4.1.2
// requires a literal of the right datatype to also be in its lexical space.

func TestXSDLexicalSpace(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		valid, invalid []string
	}{
		"dateTime": {
			valid: []string{
				"2024-01-15T10:30:00", "2024-01-15T10:30:00Z", "2024-01-15T10:30:00.5+05:30",
				"2024-01-15T10:30:00.123456789-14:00", "2024-01-15T24:00:00", "2024-01-15T24:00:00.000",
				"2024-02-29T00:00:00", "2000-02-29T00:00:00", "-0001-12-31T23:59:59", "0000-01-01T00:00:00",
				"12024-01-15T10:30:00",
			},
			invalid: []string{
				"", "noThere", "abcT", "2024-01-15", "2024-01-15T", "2024-01-15T10:30", "2024-01-15T10:30:00.",
				"2022-07-08T25:99:99", "2024-01-15T24:00:01", "2024-01-15T24:00:00.1", "2024-01-15T23:60:00",
				"2024-01-15T23:59:60", "2024-13-01T00:00:00", "2024-00-01T00:00:00", "2024-01-00T00:00:00",
				"2024-04-31T00:00:00", "2023-02-29T00:00:00", "1900-02-29T00:00:00", "2024-01-32T00:00:00",
				"024-01-15T10:30:00", "02024-01-15T10:30:00", "2024-1-15T10:30:00", "2024-01-15T1:30:00",
				"2024-01-15T10:30:00+15:00", "2024-01-15T10:30:00+14:01", "2024-01-15T10:30:00+05",
				"2024-01-15T10:30:00z", "2024-01-15T10:30:00Zjunk", " 2024-01-15T10:30:00", "2024-01-15 10:30:00",
			},
		},
		"dateTimeStamp": {
			valid:   []string{"2022-11-17T09:37:25.626789Z", "2022-11-17T09:37:25-05:00"},
			invalid: []string{"2022-11-17T09:37:25.626789", "2022-11-17", "2022-11-31T09:37:25Z"},
		},
		"date": {
			valid:   []string{"2024-01-15", "2024-12-31Z", "2024-02-29+01:00", "-0044-03-15"},
			invalid: []string{"2022-07-08T06:48:22.159262", "2022-13-45", "2022-07-08garbage", "2023-02-29", "2024-06-31", "2024-1-1"},
		},
		"time": {
			valid:   []string{"10:30:00", "10:30:00.25Z", "24:00:00", "00:00:00-01:00"},
			invalid: []string{"10:30", "24:00:01", "25:00:00", "10:30:00+14:30", "T10:30:00"},
		},
		"gYear":      {valid: []string{"2024", "-0044", "2024Z"}, invalid: []string{"24", "2024-01", "02024"}},
		"gYearMonth": {valid: []string{"2024-02", "2024-12+02:00"}, invalid: []string{"2024-13", "2024"}},
		"gMonthDay":  {valid: []string{"--02-29", "--12-31Z"}, invalid: []string{"--02-30", "--04-31", "02-28"}},
		"gDay":       {valid: []string{"---01", "---31Z"}, invalid: []string{"---32", "--01"}},
		"gMonth":     {valid: []string{"--01", "--12Z"}, invalid: []string{"--13", "-01"}},
		"duration": {
			valid:   []string{"P1Y", "P1Y2M3DT4H5M6.7S", "-P3D", "PT0S", "PT.5S", "PT1.S", "P0Y"},
			invalid: []string{"P", "PT", "P1YT", "1Y", "P1H", "PT1D", "P-1Y", "P1.5Y"},
		},
		"yearMonthDuration": {valid: []string{"P1Y", "-P2M", "P1Y2M"}, invalid: []string{"P1D", "P", "P1YT1H"}},
		"dayTimeDuration":   {valid: []string{"P1D", "PT1H", "-P1DT2M"}, invalid: []string{"P1Y", "P1M", "PT", "P"}},
		"decimal": {
			valid:   []string{"1", "1.0", "1.", ".5", "+1.5", "-0"},
			invalid: []string{"", ".", "1.095E3", "1.2.3", "+", "INF", " 1"},
		},
		"float": {
			valid:   []string{"1", "1.5", "1.", ".5e3", "1.5E-3", "INF", "+INF", "-INF", "NaN"},
			invalid: []string{"", "1.5.3", "1e", "e5", "inf", "-NaN", "1e1.5", "abc"},
		},
		"double":  {valid: []string{"1.5e3", "-INF"}, invalid: []string{"1e", "1.5.3"}},
		"boolean": {valid: []string{"true", "false", "1", "0"}, invalid: []string{"yes", "TRUE", "01", ""}},
		"integer": {valid: []string{"0", "-12", "+12", "0012"}, invalid: []string{"", "1.0", "+", "1e3"}},
		"long": {
			valid:   []string{"9223372036854775807", "-9223372036854775808"},
			invalid: []string{"9223372036854775808", "-9223372036854775809"},
		},
		"unsignedLong":       {valid: []string{"18446744073709551615"}, invalid: []string{"18446744073709551616", "-1"}},
		"int":                {valid: []string{"-2147483648"}, invalid: []string{"2147483648"}},
		"byte":               {valid: []string{"-128", "+127"}, invalid: []string{"128"}},
		"positiveInteger":    {valid: []string{"1"}, invalid: []string{"0", "-1"}},
		"nonPositiveInteger": {valid: []string{"0", "-5"}, invalid: []string{"1"}},
		"hexBinary":          {valid: []string{"", "0FB7", "0fb7"}, invalid: []string{"0FB", "0G"}},
		"base64Binary":       {valid: []string{"", "YQ==", "YWI=", "YWJj", "YW Jj"}, invalid: []string{"YQ=", "Y===", "YWJ"}},
	}
	for dt, tt := range tests {
		for _, v := range tt.valid {
			if !isWellFormedLiteral(Literal(v, XSD+dt, "")) {
				t.Errorf("%q^^xsd:%s rejected, but it is in the lexical space", v, dt)
			}
		}
		for _, v := range tt.invalid {
			if isWellFormedLiteral(Literal(v, XSD+dt, "")) {
				t.Errorf("%q^^xsd:%s accepted, but it is ill-typed", v, dt)
			}
		}
	}
}

func TestXSDLexicalSpace_UnknownDatatypeAccepted(t *testing.T) {
	t.Parallel()
	if !isWellFormedLiteral(Literal("anything at all", "http://example.org/myType", "")) {
		t.Error("a datatype with no known lexical space must not be reported as ill-typed")
	}
}

func TestDatatype_IllTypedLiteralsViolate(t *testing.T) {
	t.Parallel()
	report, _ := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetClass ex:W ;
  sh:property [ sh:path ex:date ; sh:datatype xsd:date ] ;
  sh:property [ sh:path ex:dts ; sh:datatype xsd:dateTimeStamp ] ;
  sh:property [ sh:path ex:dt ; sh:datatype xsd:dateTime ] ;
  sh:property [ sh:path ex:dec ; sh:datatype xsd:decimal ] .
`, `
ex:bad a ex:W ;
  ex:date "2022-07-08T06:48:22.159262"^^xsd:date ;
  ex:dts "2022-11-17T09:37:25.626789"^^xsd:dateTimeStamp ;
  ex:dt "2022-07-08T25:99:99"^^xsd:dateTime ;
  ex:dec "1.095E3"^^xsd:decimal .
ex:ok a ex:W ;
  ex:date "2022-07-08"^^xsd:date ;
  ex:dts "2022-11-17T09:37:25.626789Z"^^xsd:dateTimeStamp ;
  ex:dt "2022-07-08T23:59:59"^^xsd:dateTime ;
  ex:dec "1."^^xsd:decimal .
`)
	bad := 0
	for _, r := range report.Results {
		if r.FocusNode.Value() != "http://example.org/bad" {
			t.Errorf("unexpected result on %s for value %s", r.FocusNode, r.Value)
			continue
		}
		if r.SourceConstraintComponent.Value() != SH+"DatatypeConstraintComponent" {
			t.Errorf("unexpected component %s", r.SourceConstraintComponent)
		}
		bad++
	}
	if bad != 4 {
		t.Errorf("got %d sh:datatype results on ex:bad, want 4", bad)
	}
}

func TestDatatype_SubtypeIsNotTheDatatype(t *testing.T) {
	t.Parallel()
	// pySHACL #167: sh:datatype compares the datatype IRI exactly, so a
	// well-formed xsd:dateTimeStamp is not an xsd:dateTime.
	report, _ := validateTTL(t, `
ex:S a sh:NodeShape ; sh:targetNode ex:w ; sh:property [ sh:path ex:d ; sh:datatype xsd:dateTime ] .
`, `ex:w ex:d "2022-11-17T09:37:25Z"^^xsd:dateTimeStamp .`)
	if len(report.Results) != 1 {
		t.Errorf("got %d results, want 1", len(report.Results))
	}
}
