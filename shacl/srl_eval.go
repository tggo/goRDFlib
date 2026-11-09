package shacl

import (
	"context"
	"fmt"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// EvalRuleSet evaluates a SPARQL-RL rule set against a base graph and returns
// the inference graph: the triples the rule set infers (including its DATA
// blocks) that are not in the base graph. dataGraph is not modified.
//
// It uses context.Background; see EvalRuleSetContext.
func EvalRuleSet(rs *RuleSet, dataGraph *Graph) (*Graph, error) {
	return EvalRuleSetContext(context.Background(), rs, dataGraph)
}

// EvalRuleSetContext is EvalRuleSet reading dataGraph with ctx: a store error
// or a done context is returned, never an evaluation over part of the graph.
//
// The rule set must be well-formed (ErrSRLNotWellFormed) and stratifiable
// (ErrSRLNotStratifiable), and have no IMPORTS (ErrSRLImports). Evaluation
// follows the specification: strata in order; in each, the run-once rules
// once, then the general rules until nothing new is inferred. A rule body is
// run by the sparql engine, so expressions have SPARQL semantics.
func EvalRuleSetContext(ctx context.Context, rs *RuleSet, dataGraph *Graph) (*Graph, error) {
	if len(rs.Imports) > 0 {
		return nil, fmt.Errorf("%w: %d IMPORTS declared (first: <%s>)", ErrSRLImports, len(rs.Imports), rs.Imports[0])
	}
	if err := buildIndexes(ctx, dataGraph); err != nil {
		return nil, err
	}
	if err := CheckWellformed(rs); err != nil {
		return nil, err
	}
	layers, err := stratify(rs)
	if err != nil {
		return nil, err
	}

	base, err := dataGraph.rdf(ctx)
	if err != nil {
		return nil, err
	}
	ev := &srlEvaluator{
		base: base,
		eval: graph.NewGraph(),
		inf:  graph.NewGraph(),
	}
	// GE starts as the base graph plus the DATA blocks, GI as the DATA
	// triples that are not in the base graph.
	for t, err := range base.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return nil, err
		}
		if err := ev.eval.Add(ctx, t.Subject, t.Predicate, t.Object); err != nil {
			return nil, err
		}
	}
	dataBnodes := map[string]term.BNode{}
	for _, block := range rs.DataBlocks {
		for _, t := range block {
			tr, ok := instantiateTriple(t, nil, dataBnodes)
			if !ok {
				continue
			}
			if _, err := ev.add(ctx, tr); err != nil {
				return nil, err
			}
		}
	}

	compiled := make([]*srlCompiledRule, len(rs.Rules))
	for i, r := range rs.Rules {
		pq, err := sparql.Parse(buildRuleQuery(r))
		if err != nil {
			return nil, fmt.Errorf("shacl: SPARQL-RL rule %d could not be translated to a query: %w", i+1, err)
		}
		pq.NamedGraphs = map[string]*graph.Graph{srlBaseGraphIRI: base}
		compiled[i] = &srlCompiledRule{rule: r, pq: pq, n: i + 1}
	}

	for _, layer := range layers {
		for _, i := range layer.once {
			if _, err := ev.run(ctx, compiled[i]); err != nil {
				return nil, err
			}
		}
		for changed := len(layer.general) > 0; changed; {
			changed = false
			for _, i := range layer.general {
				added, err := ev.run(ctx, compiled[i])
				if err != nil {
					return nil, err
				}
				if added > 0 {
					changed = true
				}
			}
		}
	}
	return NewGraphFromRDF(ctx, ev.inf, dataGraph.BaseURI())
}

type srlCompiledRule struct {
	rule SRLRule
	pq   *sparql.ParsedQuery
	n    int // 1-based position, for messages
}

type srlEvaluator struct {
	base *graph.Graph // G0: the input
	eval *graph.Graph // GE: base + DATA + inferred
	inf  *graph.Graph // GI: the result
}

// add puts t into the evaluation graph and the inference graph unless the
// evaluation graph (base, DATA and inferred) already has it. It reports whether
// t was new.
func (ev *srlEvaluator) add(ctx context.Context, t term.Triple) (bool, error) {
	if has, err := ev.eval.Contains(ctx, t.Subject, t.Predicate, t.Object); err != nil || has {
		return false, err
	}
	if err := ev.eval.Add(ctx, t.Subject, t.Predicate, t.Object); err != nil {
		return false, err
	}
	return true, ev.inf.Add(ctx, t.Subject, t.Predicate, t.Object)
}

// run evaluates a rule once and returns how many triples it added.
func (ev *srlEvaluator) run(ctx context.Context, cr *srlCompiledRule) (int, error) {
	if err := stopped(ctx); err != nil {
		return 0, err
	}
	g := ev.eval
	if cr.rule.BaseGraphOnly {
		g = ev.base
	}
	res, err := sparql.EvalQueryContext(ctx, g, cr.pq, nil)
	if err != nil {
		return 0, fmt.Errorf("shacl: SPARQL-RL rule %d: %w", cr.n, err)
	}
	added := 0
	for _, sol := range res.Bindings {
		// A blank node of the head is fresh for each solution.
		bnodes := map[string]term.BNode{}
		for _, tt := range cr.rule.Head {
			tr, ok := instantiateTriple(tt, sol, bnodes)
			if !ok {
				continue // not an RDF triple under this solution
			}
			isNew, err := ev.add(ctx, tr)
			if err != nil {
				return added, err
			}
			if isNew {
				added++
			}
		}
	}
	return added, nil
}

// instantiateTriple substitutes a solution into a triple template. It reports
// false if a variable is unbound or the result is not an RDF triple (a literal
// subject, a predicate that is not an IRI): a template that cannot produce a
// triple for a solution produces none for it (evalRule's subst).
func instantiateTriple(t SRLTriple, sol map[string]term.Term, bnodes map[string]term.BNode) (term.Triple, bool) {
	s, ok := instantiateTerm(t.Subject, sol, bnodes)
	if !ok {
		return term.Triple{}, false
	}
	p, ok := instantiateTerm(t.Predicate, sol, bnodes)
	if !ok {
		return term.Triple{}, false
	}
	o, ok := instantiateTerm(t.Object, sol, bnodes)
	if !ok {
		return term.Triple{}, false
	}
	subj, ok := s.(term.Subject)
	if !ok {
		return term.Triple{}, false
	}
	pred, ok := p.(term.URIRef)
	if !ok {
		return term.Triple{}, false
	}
	return term.Triple{Subject: subj, Predicate: pred, Object: o}, true
}

func instantiateTerm(t SRLTerm, sol map[string]term.Term, bnodes map[string]term.BNode) (term.Term, bool) {
	switch t.Kind {
	case SRLTermIRI:
		u, err := term.NewURIRef(t.Value)
		return u, err == nil
	case SRLTermVariable:
		v, ok := sol[t.Value]
		return v, ok && v != nil
	case SRLTermBlankNode:
		b, ok := bnodes[t.Value]
		if !ok {
			b = term.NewBNode()
			bnodes[t.Value] = b
		}
		return b, true
	case SRLTermLiteral:
		return srlLiteral(t), true
	case SRLTermTripleTerm:
		s, ok := instantiateTerm(*t.TTSubject, sol, bnodes)
		if !ok {
			return nil, false
		}
		p, ok := instantiateTerm(*t.TTPredicate, sol, bnodes)
		if !ok {
			return nil, false
		}
		o, ok := instantiateTerm(*t.TTObject, sol, bnodes)
		if !ok {
			return nil, false
		}
		subj, ok := s.(term.Subject)
		pred, ok2 := p.(term.URIRef)
		if !ok || !ok2 {
			return nil, false
		}
		return term.NewTripleTerm(subj, pred, o), true
	}
	return nil, false
}

// srlLiteral builds the literal of a template. A plain string is a plain
// literal, as every parser builds it.
func srlLiteral(t SRLTerm) term.Literal {
	if t.Language != "" {
		lang, dir, _ := strings.Cut(t.Language, "--")
		if dir != "" {
			return term.NewLiteral(t.Value, term.WithLang(lang), term.WithDir(dir))
		}
		return term.NewLiteral(t.Value, term.WithLang(lang))
	}
	if t.Datatype != "" && t.Datatype != XSD+"string" {
		return term.NewLiteral(t.Value, term.WithDatatype(term.NewURIRefUnsafe(t.Datatype)))
	}
	return term.NewLiteral(t.Value)
}
