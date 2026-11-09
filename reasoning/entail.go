package reasoning

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// ErrUnsupportedDatatype is returned by Entails and Satisfiable when
// WithRecognizedDatatypes names a datatype whose lexical-to-value mapping is
// not implemented. Treating it as unrecognized would silently weaken the
// semantics the caller asked for. Recognize one of the datatypes listed in the
// error, or leave it out.
var ErrUnsupportedDatatype = errors.New("reasoning: datatype cannot be recognized")

type entailConfig struct {
	sem        Semantics
	recognized []string
}

// EntailOption configures Entails and Satisfiable.
type EntailOption func(*entailConfig)

// WithSemantics selects the entailment semantics; the default is
// SimpleEntailment.
func WithSemantics(s Semantics) EntailOption {
	return func(c *entailConfig) { c.sem = s }
}

// WithRecognizedDatatypes makes the checker interpret literals of the given
// datatype IRIs by value (D-entailment, RDF 1.1 Semantics §7): "010"^^xsd:integer
// and "10.0"^^xsd:decimal are then the same thing, and a literal outside the
// datatype's lexical space makes the graph inconsistent. xsd:string,
// rdf:langString and rdf:dirLangString are always recognized (RDF 1.1 Concepts
// §3.3). Supported: xsd:string, xsd:boolean, xsd:decimal and the integer types
// derived from it, xsd:float, xsd:double, rdf:XMLLiteral and rdf:JSON.
func WithRecognizedDatatypes(iris ...string) EntailOption {
	return func(c *entailConfig) { c.recognized = append(c.recognized, iris...) }
}

func newEntailModel(cfg *entailConfig) (*model, error) {
	rec := make(map[string]*dtSpec, len(cfg.recognized)+3)
	for _, iri := range []string{nsXSD + "string", nsRDF + "langString", nsRDF + "dirLangString"} {
		rec[iri] = dtSpecs[iri]
	}
	for _, iri := range cfg.recognized {
		spec, ok := dtSpecs[iri]
		if !ok {
			return nil, fmt.Errorf("%w: %s (recognizable: %s)", ErrUnsupportedDatatype, iri, strings.Join(supportedDatatypes(), ", "))
		}
		rec[iri] = spec
	}
	return newModel(rec), nil
}

func newEntailConfig(opts []EntailOption) *entailConfig {
	cfg := &entailConfig{}
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

// Satisfiable reports whether g has a model under the chosen semantics. Simple
// and RDF semantics have inconsistencies only through recognized datatypes
// (an ill-typed literal); RDFS adds datatype clashes such as a string literal
// in the range xsd:integer.
//
// Safe for concurrent use as long as g is not modified meanwhile.
func Satisfiable(ctx context.Context, g *graph.Graph, opts ...EntailOption) (bool, error) {
	cfg := newEntailConfig(opts)
	m, err := newEntailModel(cfg)
	if err != nil {
		return false, err
	}
	if err := m.loadPremise(ctx, g); err != nil {
		return false, err
	}
	if err := m.saturate(ctx, cfg.sem); err != nil {
		return false, err
	}
	if cfg.sem >= RDFSEntailment {
		m.checkConsistent()
	}
	return m.inconsistent == "", nil
}

// Entails reports whether premise entails conclusion under the chosen
// semantics. An inconsistent premise entails everything. It is meant for
// graphs of test-suite size: it saturates the premise in memory.
//
// Blank nodes of the conclusion are existential variables; blank nodes of the
// premise are constants. The two graphs need not use distinct labels.
//
// Safe for concurrent use as long as the graphs are not modified meanwhile.
func Entails(ctx context.Context, premise, conclusion *graph.Graph, opts ...EntailOption) (bool, error) {
	cfg := newEntailConfig(opts)
	m, err := newEntailModel(cfg)
	if err != nil {
		return false, err
	}
	if err := m.loadPremise(ctx, premise); err != nil {
		return false, err
	}
	var concl []term.Triple
	for t, err := range conclusion.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return false, err
		}
		concl = append(concl, t)
	}
	q := newQuery(m)
	for _, t := range concl {
		q.addTriple(t)
	}
	if err := m.saturate(ctx, cfg.sem); err != nil {
		return false, err
	}
	if cfg.sem >= RDFSEntailment {
		m.checkConsistent()
	}
	if m.inconsistent != "" {
		return true, nil
	}
	return q.solve(ctx)
}
