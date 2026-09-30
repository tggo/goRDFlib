package shacl

import (
	"slices"
	"testing"
)

// targetStrings returns the N3-ish form of the focus nodes shape ex:S selects,
// in selection order.
func targetStrings(t *testing.T, shapesTTL, dataTTL string) []string {
	t.Helper()
	shapes, err := LoadTurtleString(shapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(dataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := Prepare(data, shapes).evaluation(nil)
	s := ctx.shapesMap["<http://example.org/S>"]
	if s == nil {
		t.Fatal("no shape ex:S")
	}
	var out []string
	for _, n := range resolveTargets(ctx, s) {
		out = append(out, n.String())
	}
	return out
}

// TestResolveTargetsDeduplicates: a node reached through several targets, or
// several times through one, is a focus node once; terms that merely look alike
// stay separate.
func TestResolveTargetsDeduplicates(t *testing.T) {
	const prefixes = `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`
	tests := []struct {
		name, shapes, data string
		want               []string // sorted
	}{
		{
			name:   "targetNode twice and as a class instance",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetNode ex:a, ex:a ; sh:targetClass ex:C .`,
			data:   prefixes + `ex:a a ex:C . ex:b a ex:C .`,
			want:   []string{"<http://example.org/a>", "<http://example.org/b>"},
		},
		{
			name:   "literals, an IRI and a language tag that share a lexical form",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetNode "x", "x", "x"@en, "x"^^xsd:token, <x>, <x> .`,
			data:   prefixes + `ex:a a ex:C .`,
			want:   []string{`"x"`, `"x"@en`, `"x"^^<http://www.w3.org/2001/XMLSchema#token>`, "<x>"},
		},
		{
			name:   "subjects and objects of a predicate, each node once",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetSubjectsOf ex:p ; sh:targetObjectsOf ex:p .`,
			data:   prefixes + `ex:a ex:p ex:b, ex:c . ex:b ex:p ex:c .`,
			want:   []string{"<http://example.org/a>", "<http://example.org/b>", "<http://example.org/c>"},
		},
		{
			name:   "sh:shape declared by the data, also a class instance",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:C .`,
			data: prefixes + `ex:a a ex:C ; sh:shape ex:S . ex:m sh:shape ex:S . ex:n sh:shape ex:Other .
ex:m sh:shape ex:S .`,
			want: []string{"<http://example.org/a>", "<http://example.org/m>"},
		},
		{
			name:   "no target at all",
			shapes: prefixes + `ex:S a sh:NodeShape .`,
			data:   prefixes + `ex:a a ex:C .`,
			want:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := targetStrings(t, tc.shapes, tc.data)
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("targets = %v, want %v", got, tc.want)
			}
		})
	}
}
