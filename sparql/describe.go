package sparql

import (
	"context"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// DESCRIBE (SPARQL 1.1 §16.4).
//
// The specification leaves open which triples describe a resource. The
// default here is the Concise Bounded Description (CBD, W3C Member
// Submission 2005): every triple whose subject is the resource, and,
// recursively, every triple whose subject is a blank node that appeared as the
// object of a triple already in the description. The recursion carries a
// visited set, so a cycle of blank nodes terminates. Triples in which the
// resource is the object are not part of it.
//
// RDF 1.2 reifiers are included: for every triple in the description, each
// `?r rdf:reifies <<( s p o )>>` of the graph adds ?r and its own CBD, so a
// description carries what is said about its statements. The lookup costs one
// index probe per emitted triple and is skipped altogether when the graph has
// no rdf:reifies triple.
//
// The result is a graph in Result.Graph with Result.Type "DESCRIBE" (the same
// field CONSTRUCT uses). Resources are the IRIs written in the query and the
// IRIs and blank nodes the variables are bound to (all variables in scope for
// DESCRIBE *); a literal describes nothing. Every triple built counts against
// WithMaxSolutions and the loops poll the context, so a huge blank-node tree
// ends with ErrTooManySolutions or the context's error, never a partial graph.

// Describer replaces the Concise Bounded Description. It receives the
// resources to describe (IRIs and blank nodes, without duplicates) and returns
// the description. Install it with WithDescriber. An error ends the query and
// is returned as it is.
type Describer func(ctx context.Context, g *rdflibgo.Graph, resources []rdflibgo.Subject) (*rdflibgo.Graph, error)

type describerKey struct{}

// WithDescriber returns a context whose queries answer DESCRIBE with d instead
// of the Concise Bounded Description.
func WithDescriber(ctx context.Context, d Describer) context.Context {
	return context.WithValue(ctx, describerKey{}, d)
}

// describeResources collects the resources a DESCRIBE query names, in first
// appearance order, without duplicates.
func describeResources(q *ParsedQuery, solutions []map[string]rdflibgo.Term) []rdflibgo.Subject {
	var out []rdflibgo.Subject
	seen := make(map[string]struct{}, len(q.Describe)+len(solutions))
	add := func(t rdflibgo.Term) {
		var s rdflibgo.Subject
		switch v := t.(type) {
		case rdflibgo.URIRef:
			s = v
		case rdflibgo.BNode:
			s = v
		default:
			return // a literal (or a triple term) describes nothing
		}
		k := term.TermKey(s)
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	if len(q.Describe) == 0 { // DESCRIBE *
		for _, sol := range solutions {
			for name, v := range sol {
				if !isInternalVar(name) {
					add(v)
				}
			}
		}
		return out
	}
	for _, t := range q.Describe {
		if len(t) > 1 && (t[0] == '?' || t[0] == '$') {
			for _, sol := range solutions {
				if v, ok := sol[t[1:]]; ok {
					add(v)
				}
			}
			continue
		}
		add(resolveTermRef(t, q.Prefixes))
	}
	return out
}

func evalDescribe(ec *evalCtx, g *rdflibgo.Graph, q *ParsedQuery, solutions []map[string]rdflibgo.Term) (*Result, error) {
	resources := describeResources(q, solutions)
	if ec.stop() {
		return nil, nil
	}
	if d, ok := ec.context().Value(describerKey{}).(Describer); ok && d != nil {
		out, err := d(ec.context(), g, resources)
		if err != nil {
			ec.fail(err)
			return nil, nil
		}
		if out == nil {
			out = rdflibgo.NewGraph()
		}
		return &Result{Type: "DESCRIBE", Graph: out}, nil
	}
	out, err := conciseBounded(ec, g, resources)
	if err != nil {
		ec.fail(err)
		return nil, nil
	}
	if ec.err != nil {
		return nil, nil
	}
	return &Result{Type: "DESCRIBE", Graph: out}, nil
}

// conciseBounded builds the union of the CBDs of resources (see above). A
// stop of the evaluation (context, budget) leaves ec.err set and the caller
// discards the graph; a store error is returned.
func conciseBounded(ec *evalCtx, g *rdflibgo.Graph, resources []rdflibgo.Subject) (*rdflibgo.Graph, error) {
	ctx := ec.context()
	out := rdflibgo.NewGraph()

	reifies := term.RDFReifies
	hasReifiers := false
	for _, err := range g.Triples(ctx, nil, &reifies, nil) {
		if err != nil {
			return nil, err
		}
		hasReifiers = true
		break
	}

	visited := make(map[string]struct{}, len(resources))
	queue := make([]rdflibgo.Subject, 0, len(resources))
	enqueue := func(s rdflibgo.Subject) {
		k := term.TermKey(s)
		if _, dup := visited[k]; dup {
			return
		}
		visited[k] = struct{}{}
		queue = append(queue, s)
	}
	for _, r := range resources {
		enqueue(r)
	}

	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for t, err := range g.Triples(ctx, s, nil, nil) {
			if err != nil {
				return nil, err
			}
			if ec.stop() || ec.produced(1) {
				return nil, nil
			}
			if err := out.Add(ctx, t.Subject, t.Predicate, t.Object); err != nil {
				return nil, err
			}
			if b, ok := t.Object.(rdflibgo.BNode); ok {
				enqueue(b)
			}
			if !hasReifiers {
				continue
			}
			tt := term.NewTripleTerm(t.Subject, t.Predicate, t.Object)
			for rt, err := range g.Triples(ctx, nil, &reifies, tt) {
				if err != nil {
					return nil, err
				}
				if ec.stop() {
					return nil, nil
				}
				enqueue(rt.Subject)
			}
		}
	}
	return out, nil
}
