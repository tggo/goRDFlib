package shacl

import (
	"errors"
	"fmt"
	"sort"
)

// ErrUnsupportedEntailment is reported when the shapes graph declares an
// sh:entailment regime this validator does not apply.
//
// SHACL 1.0 §1.5: a processor that does not support entailment regime E MUST
// signal a failure when the shapes graph contains a triple with predicate
// sh:entailment and object E. Validate has no error return, so the failure
// reaches the caller through WithErrorHandler; validation still runs, on the
// graph without the requested entailments, and its report should not be
// trusted as the answer to what the shapes graph asked.
//
// The only regime supported is sh:Rules (SHACL-AF §7.4), and only with
// WithAdvancedFeatures. For RDFS or OWL entailment, materialise the
// inferences into the data graph first (for example with the reasoning
// package) and remove the sh:entailment triple from the shapes graph.
var ErrUnsupportedEntailment = errors.New("shacl: unsupported entailment regime")

// checkEntailment reports every sh:entailment regime in the shapes graph that
// this run will not apply.
func checkEntailment(shapesGraph *Graph, cfg *config) {
	pred := IRI(SH + "entailment")
	seen := make(map[string]bool)
	var regimes []Term
	for _, t := range shapesGraph.All(nil, &pred, nil) {
		if k := t.Object.TermKey(); !seen[k] {
			seen[k] = true
			regimes = append(regimes, t.Object)
		}
	}
	// Reported in a fixed order, so the handler sees the same sequence on
	// every run.
	sort.Slice(regimes, func(i, j int) bool { return regimes[i].TermKey() < regimes[j].TermKey() })

	for _, e := range regimes {
		if e.IsIRI() && e.Value() == SH+"Rules" {
			if !cfg.advanced {
				// SHACL-AF §7.4: an engine that will not run the rules must
				// not silently validate the un-inferred graph.
				cfg.report(fmt.Errorf("%w: the shapes graph requests the sh:Rules entailment regime, but advanced features are off; pass WithAdvancedFeatures to run the rules (%w)",
					ErrAdvancedFeatures, ErrUnsupportedEntailment))
			}
			continue
		}
		cfg.report(fmt.Errorf("%w: the shapes graph requests %s, which this validator does not apply, so the data graph is validated without those inferences; materialise them into the data graph and remove the sh:entailment triple",
			ErrUnsupportedEntailment, e))
	}
}
