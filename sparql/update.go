package sparql

import (
	"context"
	"fmt"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/internal/bnodes"
	"github.com/tggo/goRDFlib/term"
)

// Loader fetches RDF data from a URI and parses it into a graph.
// Implementations must handle URI scheme dispatch (e.g. file://, http(s)://).
type Loader interface {
	Load(ctx context.Context, g *graph.Graph, uri string) error
}

// Dataset holds the default graph and named graphs for update evaluation.
type Dataset struct {
	Default     *rdflibgo.Graph
	NamedGraphs map[string]*rdflibgo.Graph
	Loader      Loader // optional; if nil, LOAD returns error (backward compat)
}

// EvalUpdate evaluates a parsed SPARQL Update request against a dataset.
//
// It cannot be cancelled; EvalUpdateContext can.
func EvalUpdate(ds *Dataset, u *ParsedUpdate) error {
	return EvalUpdateContext(context.Background(), ds, u)
}

// EvalUpdateContext evaluates a parsed SPARQL Update request against a
// dataset, and stops once ctx is done. A stopped request returns an error that
// matches ErrQueryCancelled and ctx.Err() under errors.Is.
//
// What a cancellation can leave behind:
//
//   - One operation is all or nothing with respect to cancellation. DELETE
//     WHERE and DELETE/INSERT … WHERE evaluate their WHERE clause and
//     instantiate their templates, both interruptible, before they change
//     anything, and check the context once more just before. Once the first
//     triple is removed or added, the operation runs to the end. The one trace
//     a stopped operation can leave is an empty entry in Dataset.NamedGraphs
//     for a template graph that did not exist yet, because template graphs
//     are resolved during instantiation.
//   - A request is not. Its operations run in order and the context is checked
//     before each one; the operations that completed before the cancellation
//     stay applied. There is no rollback, so a request whose later operations
//     depend on its earlier ones can be left half done.
//   - INSERT DATA, DELETE DATA, CLEAR, DROP, CREATE, ADD, COPY and MOVE are not
//     interrupted once started. Their cost is the size of the request or of a
//     graph copy, not a join.
//   - LOAD passes ctx to Dataset.Loader. What a load cancelled halfway leaves
//     in the target graph is up to the Loader.
//
// None of this is isolation: other goroutines writing to the same graphs see,
// and can interleave with, the changes as they are applied.
func EvalUpdateContext(ctx context.Context, ds *Dataset, u *ParsedUpdate) error {
	ec := newEvalCtx(ctx)
	prefixes := u.Prefixes
	if prefixes == nil {
		prefixes = make(map[string]string)
	}
	if u.BaseURI != "" {
		prefixes[baseURIKey] = u.BaseURI
	}

	for _, op := range u.Operations {
		if ec.poll() {
			return ec.failure()
		}
		err := evalUpdateOp(ec, ds, op, prefixes)
		// An operation that noticed the cancellation returns nil without
		// changing anything; a LOAD whose Loader gave up returns the Loader's
		// error. Either way the request reports the cancellation.
		if ec.poll() {
			return ec.failure()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func evalUpdateOp(ec *evalCtx, ds *Dataset, op UpdateOperation, prefixes map[string]string) error {
	switch o := op.(type) {
	case *InsertDataOp:
		return evalInsertData(ec, ds, o, prefixes)
	case *DeleteDataOp:
		return evalDeleteData(ec, ds, o, prefixes)
	case *DeleteWhereOp:
		return evalDeleteWhere(ec, ds, o, prefixes)
	case *ModifyOp:
		return evalModify(ec, ds, o, prefixes)
	case *GraphMgmtOp:
		return evalGraphMgmt(ec, ds, o, prefixes)
	default:
		return fmt.Errorf("unknown update operation type: %T", op)
	}
}

func evalInsertData(ec *evalCtx, ds *Dataset, op *InsertDataOp, prefixes map[string]string) error {
	// SPARQL 1.1 Update §3.1.1: blank nodes in INSERT DATA are new nodes. A
	// label names the same node throughout the operation, and a different one
	// in every other operation and request.
	scope := bnodes.New(false)
	for _, qp := range op.Quads {
		g := graphForQuad(ec, ds, qp.Graph)
		for _, t := range qp.Triples {
			s := resolveTemplateValue(t.Subject, nil, prefixes, scope)
			p := resolveTemplateValue(t.Predicate, nil, prefixes, scope)
			o := resolveTemplateValue(t.Object, nil, prefixes, scope)
			if s == nil || p == nil || o == nil {
				continue
			}
			subj, ok := s.(term.Subject)
			if !ok {
				continue
			}
			pred, ok := p.(term.URIRef)
			if !ok {
				continue
			}
			g.Add(subj, pred, o)
		}
	}
	return nil
}

func evalDeleteData(ec *evalCtx, ds *Dataset, op *DeleteDataOp, prefixes map[string]string) error {
	scope := bnodes.New(false) // DELETE DATA may not contain blank nodes (§3.1.2)
	for _, qp := range op.Quads {
		g := graphForQuad(ec, ds, qp.Graph)
		for _, t := range qp.Triples {
			s := resolveTemplateValue(t.Subject, nil, prefixes, scope)
			p := resolveTemplateValue(t.Predicate, nil, prefixes, scope)
			o := resolveTemplateValue(t.Object, nil, prefixes, scope)
			if s == nil || p == nil || o == nil {
				continue
			}
			subj, ok := s.(term.Subject)
			if !ok {
				continue
			}
			pred, ok := p.(term.URIRef)
			if !ok {
				continue
			}
			g.Remove(subj, &pred, o)
		}
	}
	return nil
}

func evalDeleteWhere(ec *evalCtx, ds *Dataset, op *DeleteWhereOp, prefixes map[string]string) error {
	// Build a WHERE pattern from the quads and use the same quads as template
	pattern := quadsToPattern(op.Quads, prefixes)
	namedGraphs, _ := ec.bindNamed(ds.NamedGraphs)

	solutions := evalPattern(ec, ec.bind(ds.Default), pattern, prefixes, namedGraphs)

	// Instantiate every removal before applying any, so a cancellation during
	// the WHERE clause or the instantiation leaves the dataset untouched (see
	// EvalUpdateContext).
	type removal struct {
		graph *rdflibgo.Graph
		subj  term.Subject
		pred  term.URIRef
		obj   rdflibgo.Term
	}
	var removals []removal
	for _, sol := range solutions {
		if ec.stop() {
			return nil
		}
		scope := bnodes.New(false)
		for _, qp := range op.Quads {
			g := graphForQuadSolution(ec, ds, qp.Graph, sol)
			if g == nil {
				continue
			}
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes, scope)
				p := resolveTemplateValue(t.Predicate, sol, prefixes, scope)
				o := resolveTemplateValue(t.Object, sol, prefixes, scope)
				if s == nil || p == nil || o == nil {
					continue
				}
				subj, ok := s.(term.Subject)
				if !ok {
					continue
				}
				pred, ok := p.(term.URIRef)
				if !ok {
					continue
				}
				removals = append(removals, removal{g, subj, pred, o})
			}
		}
	}
	if ec.poll() {
		return nil
	}
	for _, r := range removals {
		r.graph.Remove(r.subj, &r.pred, r.obj)
	}
	return nil
}

func evalModify(ec *evalCtx, ds *Dataset, op *ModifyOp, prefixes map[string]string) error {
	// Determine the query graph
	queryGraph := ec.bind(ds.Default)
	namedGraphs, _ := ec.bindNamed(ds.NamedGraphs)

	if op.With != "" {
		// WITH <g> makes the named graph the default for pattern matching
		if ng, ok := ds.NamedGraphs[op.With]; ok {
			queryGraph = ec.bind(ng)
		} else {
			queryGraph = rdflibgo.NewGraph()
		}
	}

	// USING clauses define the query dataset
	if len(op.Using) > 0 {
		merged := rdflibgo.NewGraph()
		usedNamed := make(map[string]*rdflibgo.Graph)
		for _, uc := range op.Using {
			if uc.Named {
				if ng, ok := ds.NamedGraphs[uc.IRI]; ok {
					usedNamed[uc.IRI] = ec.bind(ng)
				}
			} else {
				// Merge into default graph
				if ng, ok := ds.NamedGraphs[uc.IRI]; ok {
					for tr := range ec.bind(ng).Triples(nil, nil, nil) {
						if ec.stop() {
							return nil
						}
						merged.Add(tr.Subject, tr.Predicate, tr.Object)
					}
				}
			}
		}
		queryGraph = merged
		// USING defines the complete dataset: named graphs are only those from USING NAMED
		namedGraphs = usedNamed
	}

	solutions := evalPattern(ec, queryGraph, op.Where, prefixes, namedGraphs)

	// Collect all deletions/insertions first (snapshot semantics)
	type tripleAction struct {
		graph *rdflibgo.Graph
		subj  term.Subject
		pred  term.URIRef
		obj   rdflibgo.Term
	}
	var deletes, inserts []tripleAction

	for _, sol := range solutions {
		if ec.stop() {
			return nil
		}
		// Update §3.1.3: template blank nodes are fresh for each solution.
		scope := bnodes.New(false)
		for _, qp := range op.Delete {
			g := resolveModifyGraph(ec, ds, qp.Graph, op.With, sol)
			if g == nil {
				continue
			}
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes, scope)
				p := resolveTemplateValue(t.Predicate, sol, prefixes, scope)
				o := resolveTemplateValue(t.Object, sol, prefixes, scope)
				if s == nil || p == nil || o == nil {
					continue
				}
				subj, ok := s.(term.Subject)
				if !ok {
					continue
				}
				pred, ok := p.(term.URIRef)
				if !ok {
					continue
				}
				deletes = append(deletes, tripleAction{g, subj, pred, o})
			}
		}
		for _, qp := range op.Insert {
			g := resolveModifyGraph(ec, ds, qp.Graph, op.With, sol)
			if g == nil {
				continue
			}
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes, scope)
				p := resolveTemplateValue(t.Predicate, sol, prefixes, scope)
				o := resolveTemplateValue(t.Object, sol, prefixes, scope)
				if s == nil || p == nil || o == nil {
					continue
				}
				subj, ok := s.(term.Subject)
				if !ok {
					continue
				}
				pred, ok := p.(term.URIRef)
				if !ok {
					continue
				}
				inserts = append(inserts, tripleAction{g, subj, pred, o})
			}
		}
	}

	// Apply deletions then insertions. The last chance to stop without having
	// changed anything is here (see EvalUpdateContext).
	if ec.poll() {
		return nil
	}
	for _, d := range deletes {
		d.graph.Remove(d.subj, &d.pred, d.obj)
	}
	for _, i := range inserts {
		i.graph.Add(i.subj, i.pred, i.obj)
	}

	return nil
}

func evalGraphMgmt(ec *evalCtx, ds *Dataset, op *GraphMgmtOp, prefixes map[string]string) error {
	switch op.Op {
	case "CLEAR", "DROP":
		switch op.Target {
		case "DEFAULT":
			clearGraph(ec.bind(ds.Default))
		case "NAMED":
			for k := range ds.NamedGraphs {
				clearGraph(ec.bind(ds.NamedGraphs[k]))
				if op.Op == "DROP" {
					delete(ds.NamedGraphs, k)
				}
			}
		case "ALL":
			clearGraph(ec.bind(ds.Default))
			for k := range ds.NamedGraphs {
				clearGraph(ec.bind(ds.NamedGraphs[k]))
				if op.Op == "DROP" {
					delete(ds.NamedGraphs, k)
				}
			}
		default:
			if g, ok := ds.NamedGraphs[op.Target]; ok {
				clearGraph(ec.bind(g))
				if op.Op == "DROP" {
					delete(ds.NamedGraphs, op.Target)
				}
			} else if !op.Silent {
				// Graph doesn't exist - silent means no error
			}
		}

	case "CREATE":
		// No-op for in-memory; graph created on first add
		if _, exists := ds.NamedGraphs[op.Target]; exists && !op.Silent {
			// Graph already exists - not an error for silent
		}

	case "LOAD":
		if ds.Loader == nil {
			if !op.Silent {
				return fmt.Errorf("LOAD not supported: no Loader configured on Dataset")
			}
			return nil
		}
		target := getOrCreateGraph(ec, ds, func() string {
			if op.Into != "" {
				return op.Into
			}
			return "DEFAULT"
		}())
		if err := ds.Loader.Load(ec.context(), target, op.Source); err != nil {
			if !op.Silent {
				return fmt.Errorf("LOAD <%s>: %w", op.Source, err)
			}
		}

	case "ADD":
		return transferGraphs(ec, ds, op.Source, op.Target, false, op.Silent)

	case "COPY":
		return transferGraphs(ec, ds, op.Source, op.Target, true, op.Silent)

	case "MOVE":
		if op.Source == op.Target {
			break // no-op
		}
		if err := transferGraphs(ec, ds, op.Source, op.Target, true, op.Silent); err != nil {
			return err
		}
		// Clear source
		src := getGraph(ec, ds, op.Source)
		if src != nil {
			clearGraph(src)
			if op.Source != "DEFAULT" {
				delete(ds.NamedGraphs, op.Source)
			}
		}
	}

	return nil
}

func transferGraphs(ec *evalCtx, ds *Dataset, srcName, dstName string, replace bool, silent bool) error {
	src := getGraph(ec, ds, srcName)
	if src == nil {
		if silent {
			return nil
		}
		return fmt.Errorf("source graph %s not found", srcName)
	}

	// Collect triples first to avoid deadlock when src and dst share a store
	type triple struct {
		s term.Subject
		p term.URIRef
		o rdflibgo.Term
	}
	var triples []triple
	for tr := range src.Triples(nil, nil, nil) {
		triples = append(triples, triple{tr.Subject, tr.Predicate, tr.Object})
	}

	dst := getOrCreateGraph(ec, ds, dstName)
	if replace {
		clearGraph(dst)
	}

	for _, t := range triples {
		dst.Add(t.s, t.p, t.o)
	}
	return nil
}

func getGraph(ec *evalCtx, ds *Dataset, name string) *rdflibgo.Graph {
	if name == "DEFAULT" {
		return ec.bind(ds.Default)
	}
	if g, ok := ds.NamedGraphs[name]; ok {
		return ec.bind(g)
	}
	return nil
}

// getOrCreateGraph returns the named graph, bound to the evaluation's context.
// A graph it creates is stored in ds unbound, so the caller's dataset never
// holds a view tied to one request's context.
func getOrCreateGraph(ec *evalCtx, ds *Dataset, name string) *rdflibgo.Graph {
	if name == "DEFAULT" {
		return ec.bind(ds.Default)
	}
	if g, ok := ds.NamedGraphs[name]; ok {
		return ec.bind(g)
	}
	g := rdflibgo.NewGraph()
	if ds.NamedGraphs == nil {
		ds.NamedGraphs = make(map[string]*rdflibgo.Graph)
	}
	ds.NamedGraphs[name] = g
	return ec.bind(g)
}

func clearGraph(g *rdflibgo.Graph) {
	g.Remove(nil, nil, nil)
}

func graphForQuad(ec *evalCtx, ds *Dataset, graphName string) *rdflibgo.Graph {
	if graphName == "" {
		return ec.bind(ds.Default)
	}
	return getOrCreateGraph(ec, ds, graphName)
}

// graphForVar returns the graph named by a GRAPH variable in an update
// template, or nil when the variable is unbound or bound to something that is
// not an IRI. SPARQL 1.1 Update §3.1.3: a template triple that contains an
// unbound variable or an illegal RDF construct is not included, so the caller
// must skip the quad rather than fall back to the default graph.
func graphForVar(ec *evalCtx, ds *Dataset, name string, sol map[string]rdflibgo.Term) *rdflibgo.Graph {
	u, ok := sol[name].(term.URIRef)
	if !ok {
		return nil
	}
	return getOrCreateGraph(ec, ds, u.Value())
}

// graphForQuadSolution returns the graph a DELETE WHERE quad applies to, or nil
// when the quad must be skipped (see graphForVar).
func graphForQuadSolution(ec *evalCtx, ds *Dataset, graphName string, sol map[string]rdflibgo.Term) *rdflibgo.Graph {
	if graphName == "" {
		return ec.bind(ds.Default)
	}
	if strings.HasPrefix(graphName, "?") {
		return graphForVar(ec, ds, graphName[1:], sol)
	}
	return getOrCreateGraph(ec, ds, graphName)
}

// resolveModifyGraph returns the graph a DELETE/INSERT template quad applies
// to, or nil when the quad must be skipped (see graphForVar).
func resolveModifyGraph(ec *evalCtx, ds *Dataset, graphName, with string, sol map[string]rdflibgo.Term) *rdflibgo.Graph {
	if graphName == "" {
		if with != "" {
			return getOrCreateGraph(ec, ds, with)
		}
		return ec.bind(ds.Default)
	}
	if strings.HasPrefix(graphName, "?") {
		return graphForVar(ec, ds, graphName[1:], sol)
	}
	return getOrCreateGraph(ec, ds, graphName)
}

// quadsToPattern converts QuadPattern slice to a Pattern for WHERE evaluation.
func quadsToPattern(quads []QuadPattern, prefixes map[string]string) Pattern {
	var result Pattern
	for _, qp := range quads {
		var bgp Pattern = &BGP{Triples: qp.Triples}
		if qp.Graph != "" {
			name := qp.Graph
			if !strings.HasPrefix(name, "?") && !strings.HasPrefix(name, "<") {
				name = "<" + name + ">"
			}
			bgp = &GraphPattern{Name: name, Pattern: bgp}
		}
		if result == nil {
			result = bgp
		} else {
			result = &JoinPattern{Left: result, Right: bgp}
		}
	}
	if result == nil {
		result = &BGP{}
	}
	return result
}
