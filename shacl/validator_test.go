package shacl

import (
	"fmt"
	"strings"
	"testing"
)

func mustParseWithPrefixes(t *testing.T, turtle string) *Graph {
	t.Helper()
	return mustGraph(t, shapesPrefixes+turtle)
}

func mustParseJsonLDWithPrefixes(t *testing.T, jsonld string) *Graph {
	t.Helper()
	return mustGraphJsonld(t, jsonld)
}

func TestValidate_Conforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:MyShape a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:nodeKind sh:IRI .
`)
	data := mustParseWithPrefixes(t, `
ex:Alice a ex:Person .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Errorf("expected conforming report, got %d violations", len(report.Results))
	}
}

func TestJsonLDValidate_Conforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLDWithPrefixes(t, `{
            "@context": {
                "ex": "http://example.org/" ,
                "sh": "http://www.w3.org/ns/shacl#" ,
                "rdf": "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
            },
            "@id": "ex:MyShape",
            "@type": "sh:NodeShape",
            "sh:targetNode": {
                "@id": "ex:Alice"
            },
            "sh:nodeKind": {
                "@id": "sh:IRI"
            }
        }`)
	data := mustParseJsonLDWithPrefixes(t, `{
            "@context": {
                "ex": "http://example.org/"
            },
            "@id": "ex:Alice",
            "@type": "ex:Person"
        }`)
	report := Validate(data, shapes)
	fmt.Println(report)
	if !report.Conforms {
		t.Errorf("expected conforming report, got %d violations", len(report.Results))
	}
}

func TestJsonLDValidate_NonConforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseJsonLDWithPrefixes(t, `{
  "@context": {
    "ex": "http://example.org/" ,
    "sh": "http://www.w3.org/ns/shacl#"
  },
  "@id": "ex:MyShape",
  "@type": "sh:NodeShape",
  "sh:targetNode": {
    "@id": "ex:Alice"
  },
  "sh:property": {
    "sh:path": {
      "@id": "ex:name"
    },
    "sh:nodeKind": {
      "@id": "sh:IRI"
    }
  }
}`)
	data := mustParseJsonLDWithPrefixes(t, `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "ex:Alice",
  "ex:name": {
    "@id": "http://example.org/AliceName"
  }
}`)
	data = mustParseJsonLDWithPrefixes(t, `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "ex:Alice",
  "ex:name": "Alice"
}`)
	report := Validate(data, shapes)

	if report.Conforms {
		t.Error("expected non-conforming report")
	}
	if len(report.Results) == 0 {
		t.Error("expected at least one violation")
	}
}

func TestValidate_NonConforming(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:MyShape a sh:NodeShape ;
    sh:targetNode ex:Alice ;
    sh:property [
        sh:path ex:name ;
        sh:nodeKind sh:IRI ;
    ] .
`)
	data := mustParseWithPrefixes(t, `
ex:Alice ex:name "Alice" .
`)
	report := Validate(data, shapes)
	if report.Conforms {
		t.Error("expected non-conforming report")
	}
	if len(report.Results) == 0 {
		t.Error("expected at least one violation")
	}
}

func TestValidate_DeactivatedSkipped(t *testing.T) {
	t.Parallel()
	shapes := mustParseWithPrefixes(t, `
ex:MyShape a sh:NodeShape ;
    sh:deactivated true ;
    sh:targetNode ex:Alice ;
    sh:nodeKind sh:Literal .
`)
	data := mustParseWithPrefixes(t, `
ex:Alice a ex:Person .
`)
	report := Validate(data, shapes)
	if !report.Conforms {
		t.Error("expected deactivated shape to be skipped, but got violations")
	}
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		shapesTTL  string
		dataTTL    string
		wantSubstr string // substring expected in one of the target values
	}{
		{
			"targetNode",
			`ex:S a sh:NodeShape ; sh:targetNode ex:Alice .`,
			`ex:Alice a ex:Person .`,
			"Alice",
		},
		{
			"targetClass",
			`ex:S a sh:NodeShape ; sh:targetClass ex:Person .`,
			`ex:Alice a ex:Person .`,
			"Alice",
		},
		{
			"targetSubjectsOf",
			`ex:S a sh:NodeShape ; sh:targetSubjectsOf ex:knows .`,
			`ex:Alice ex:knows ex:Bob .`,
			"Alice",
		},
		{
			"targetObjectsOf",
			`ex:S a sh:NodeShape ; sh:targetObjectsOf ex:knows .`,
			`ex:Alice ex:knows ex:Bob .`,
			"Bob",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sg := mustParseWithPrefixes(t, tc.shapesTTL)
			dg := mustParseWithPrefixes(t, tc.dataTTL)
			shapes := parseShapes(sg)
			ctx := &evalContext{dataGraph: dg, shapesGraph: sg, shapesMap: shapes, classInstances: buildClassIndex(dg)}

			var allTargets []Term
			for _, s := range shapes {
				allTargets = append(allTargets, resolveTargets(ctx, s)...)
			}
			if len(allTargets) == 0 {
				t.Fatal("expected at least one target")
			}
			var found bool
			for _, tgt := range allTargets {
				if strings.Contains(tgt.Value(), tc.wantSubstr) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected target containing %q", tc.wantSubstr)
			}
		})
	}
}
