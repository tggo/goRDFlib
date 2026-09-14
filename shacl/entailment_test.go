package shacl

import (
	"errors"
	"strings"
	"testing"
)

// pySHACL #221: an sh:entailment regime the validator does not apply was
// ignored, so a shapes graph that relies on RDFS inferences validated the
// raw graph with no sign that anything was missing. SHACL 1.0 §1.5 requires a
// failure.

const rdfsEntailmentShapes = `
<http://example.org/shapes> sh:entailment <http://www.w3.org/ns/entailment/RDFS> .
ex:S a sh:NodeShape ; sh:targetClass ex:Super ; sh:property [ sh:path ex:p ; sh:minCount 1 ] .
`

func TestEntailment_UnsupportedRegimeIsReported(t *testing.T) {
	t.Parallel()
	data := `ex:Sub rdfs:subClassOf ex:Super . ex:p rdfs:domain ex:Super . ex:x ex:p 1 .`
	for name, opts := range map[string][]Option{"core": nil, "advanced": {WithAdvancedFeatures()}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, errs := validateTTL(t, rdfsEntailmentShapes, data, opts...)
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want one", errs)
			}
			if !errors.Is(errs[0], ErrUnsupportedEntailment) {
				t.Errorf("error %q does not match ErrUnsupportedEntailment", errs[0])
			}
			if !strings.Contains(errs[0].Error(), "http://www.w3.org/ns/entailment/RDFS") {
				t.Errorf("the error should name the regime, got %q", errs[0])
			}
		})
	}
}

func TestEntailment_EachRegimeReportedOnce(t *testing.T) {
	t.Parallel()
	_, errs := validateTTL(t, `
ex:a sh:entailment <http://www.w3.org/ns/entailment/OWL-RDF-Based> , <http://www.w3.org/ns/entailment/RDFS> .
ex:b sh:entailment <http://www.w3.org/ns/entailment/RDFS> .
`, `ex:x ex:p 1 .`)
	if len(errs) != 2 {
		t.Fatalf("errors = %v, want one per distinct regime", errs)
	}
	if !strings.Contains(errs[0].Error(), "OWL-RDF-Based") || !strings.Contains(errs[1].Error(), "RDFS") {
		t.Errorf("regimes should be reported in a stable order, got %v", errs)
	}
}

func TestEntailment_RulesRegime(t *testing.T) {
	t.Parallel()
	shapes := `<http://example.org/shapes> sh:entailment sh:Rules .`
	_, errs := validateTTL(t, shapes, `ex:x ex:p 1 .`)
	if len(errs) != 1 || !errors.Is(errs[0], ErrAdvancedFeatures) || !errors.Is(errs[0], ErrUnsupportedEntailment) {
		t.Errorf("without advanced features: errors = %v, want one matching both ErrAdvancedFeatures and ErrUnsupportedEntailment", errs)
	}
	if _, errs := validateTTL(t, shapes, `ex:x ex:p 1 .`, WithAdvancedFeatures()); len(errs) != 0 {
		t.Errorf("sh:Rules is supported with advanced features, got errors %v", errs)
	}
}

func TestEntailment_NoDeclarationNoError(t *testing.T) {
	t.Parallel()
	if _, errs := validateTTL(t, `ex:S a sh:NodeShape ; sh:targetNode ex:x .`, `ex:x ex:p 1 .`); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
