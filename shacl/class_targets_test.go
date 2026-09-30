package shacl

import (
	"slices"
	"testing"
)

// targetsOf returns the focus nodes (local names, sorted) that shape ex:<name>
// selects in data.
func targetsOf(t *testing.T, shapesTTL, dataTTL, shape string) []string {
	t.Helper()
	shapes, err := LoadTurtleString(shapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(dataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	p := Prepare(data, shapes)
	ctx := p.evaluation(nil)
	s := ctx.shapesMap["<http://example.org/"+shape+">"]
	if s == nil {
		t.Fatalf("no shape %s", shape)
	}
	var out []string
	for _, n := range resolveTargets(ctx, s) {
		out = append(out, n.Value()[len("http://example.org/"):])
	}
	slices.Sort(out)
	return out
}

// TestClassTargets: sh:targetClass selects direct instances and instances of
// subclasses, terminates on an rdfs:subClassOf cycle, and a class that is itself
// a node is selected like any other instance.
func TestClassTargets(t *testing.T) {
	const prefixes = `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
`
	tests := []struct {
		name, shapes, data, shape string
		want                      []string
	}{
		{
			name:   "direct instances only",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:C .`,
			data:   prefixes + `ex:x a ex:C . ex:y a ex:D .`,
			shape:  "S",
			want:   []string{"x"},
		},
		{
			name:   "subclass and sub-subclass instances",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:C .`,
			data: prefixes + `ex:D rdfs:subClassOf ex:C . ex:E rdfs:subClassOf ex:D .
ex:x a ex:C . ex:y a ex:D . ex:z a ex:E . ex:w a ex:F .`,
			shape: "S",
			want:  []string{"x", "y", "z"},
		},
		{
			name:   "subclass cycle terminates",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:C .`,
			data: prefixes + `ex:C rdfs:subClassOf ex:D . ex:D rdfs:subClassOf ex:C .
ex:x a ex:C . ex:y a ex:D .`,
			shape: "S",
			want:  []string{"x", "y"},
		},
		{
			name:   "a class that is also a node",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:D .`,
			data: prefixes + `ex:C rdfs:subClassOf ex:D . ex:C a ex:C .
ex:D a ex:D .`,
			shape: "S",
			want:  []string{"C", "D"},
		},
		{
			name:   "a node listed as instance twice through two classes",
			shapes: prefixes + `ex:S a sh:NodeShape ; sh:targetClass ex:C .`,
			data:   prefixes + `ex:D rdfs:subClassOf ex:C . ex:x a ex:C, ex:D .`,
			shape:  "S",
			want:   []string{"x"},
		},
		{
			name:   "implicit class target",
			shapes: prefixes + `ex:K a sh:NodeShape, rdfs:Class .`,
			data:   prefixes + `ex:J rdfs:subClassOf ex:K . ex:x a ex:K . ex:y a ex:J . ex:z a ex:L .`,
			shape:  "K",
			want:   []string{"x", "y"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := targetsOf(t, tc.shapes, tc.data, tc.shape); !slices.Equal(got, tc.want) {
				t.Errorf("targets = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSubClassesCycle: subClasses returns each subclass once and never the class
// itself, however the subClassOf edges loop.
func TestSubClassesCycle(t *testing.T) {
	g, err := LoadTurtleString(`@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix ex: <http://example.org/> .
ex:B rdfs:subClassOf ex:A . ex:C rdfs:subClassOf ex:B . ex:A rdfs:subClassOf ex:C .
ex:D rdfs:subClassOf ex:A .`, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range subClasses(g, IRI("http://example.org/A")) {
		got = append(got, c.Value()[len("http://example.org/"):])
	}
	slices.Sort(got)
	if want := []string{"B", "C", "D"}; !slices.Equal(got, want) {
		t.Errorf("subClasses(A) = %v, want %v", got, want)
	}
}
