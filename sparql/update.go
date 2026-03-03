package sparql

import (
	"context"
	"fmt"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
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
func EvalUpdate(ds *Dataset, u *ParsedUpdate) error {
	prefixes := u.Prefixes
	if prefixes == nil {
		prefixes = make(map[string]string)
	}
	if u.BaseURI != "" {
		prefixes[baseURIKey] = u.BaseURI
	}

	for i, op := range u.Operations {
		// Scope bnode labels per operation by adding operation index prefix
		scopedPrefixes := make(map[string]string, len(prefixes)+1)
		for k, v := range prefixes {
			scopedPrefixes[k] = v
		}
		scopedPrefixes["__bnode_scope__"] = fmt.Sprintf("_op%d_", i)
		if err := evalUpdateOp(ds, op, scopedPrefixes); err != nil {
			return err
		}
	}
	return nil
}

func evalUpdateOp(ds *Dataset, op UpdateOperation, prefixes map[string]string) error {
	switch o := op.(type) {
	case *InsertDataOp:
		return evalInsertData(ds, o, prefixes)
	case *DeleteDataOp:
		return evalDeleteData(ds, o, prefixes)
	case *DeleteWhereOp:
		return evalDeleteWhere(ds, o, prefixes)
	case *ModifyOp:
		return evalModify(ds, o, prefixes)
	case *GraphMgmtOp:
		return evalGraphMgmt(ds, o, prefixes)
	default:
		return fmt.Errorf("unknown update operation type: %T", op)
	}
}

func evalInsertData(ds *Dataset, op *InsertDataOp, prefixes map[string]string) error {
	for _, qp := range op.Quads {
		g := graphForQuad(ds, qp.Graph)
		for _, t := range qp.Triples {
			s := resolveTemplateValue(t.Subject, nil, prefixes)
			p := resolveTemplateValue(t.Predicate, nil, prefixes)
			o := resolveTemplateValue(t.Object, nil, prefixes)
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

func evalDeleteData(ds *Dataset, op *DeleteDataOp, prefixes map[string]string) error {
	for _, qp := range op.Quads {
		g := graphForQuad(ds, qp.Graph)
		for _, t := range qp.Triples {
			s := resolveTemplateValue(t.Subject, nil, prefixes)
			p := resolveTemplateValue(t.Predicate, nil, prefixes)
			o := resolveTemplateValue(t.Object, nil, prefixes)
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

func evalDeleteWhere(ds *Dataset, op *DeleteWhereOp, prefixes map[string]string) error {
	// Build a WHERE pattern from the quads and use the same quads as template
	pattern := quadsToPattern(op.Quads, prefixes)
	namedGraphs := ds.NamedGraphs

	solutions := evalPattern(ds.Default, pattern, prefixes, namedGraphs)

	for _, sol := range solutions {
		for _, qp := range op.Quads {
			g := graphForQuadSolution(ds, qp.Graph, sol)
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes)
				p := resolveTemplateValue(t.Predicate, sol, prefixes)
				o := resolveTemplateValue(t.Object, sol, prefixes)
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
	}
	return nil
}

func evalModify(ds *Dataset, op *ModifyOp, prefixes map[string]string) error {
	// Determine the query graph
	queryGraph := ds.Default
	namedGraphs := ds.NamedGraphs

	if op.With != "" {
		// WITH <g> makes the named graph the default for pattern matching
		if ng, ok := ds.NamedGraphs[op.With]; ok {
			queryGraph = ng
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
					usedNamed[uc.IRI] = ng
				}
			} else {
				// Merge into default graph
				if ng, ok := ds.NamedGraphs[uc.IRI]; ok {
					for tr := range ng.Triples(nil, nil, nil) {
						merged.Add(tr.Subject, tr.Predicate, tr.Object)
					}
				}
			}
		}
		queryGraph = merged
		// USING defines the complete dataset: named graphs are only those from USING NAMED
		namedGraphs = usedNamed
	}

	solutions := evalPattern(queryGraph, op.Where, prefixes, namedGraphs)

	// Collect all deletions/insertions first (snapshot semantics)
	type tripleAction struct {
		graph *rdflibgo.Graph
		subj  term.Subject
		pred  term.URIRef
		obj   rdflibgo.Term
	}
	var deletes, inserts []tripleAction

	for _, sol := range solutions {
		for _, qp := range op.Delete {
			g := resolveModifyGraph(ds, qp.Graph, op.With, sol)
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes)
				p := resolveTemplateValue(t.Predicate, sol, prefixes)
				o := resolveTemplateValue(t.Object, sol, prefixes)
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
			g := resolveModifyGraph(ds, qp.Graph, op.With, sol)
			for _, t := range qp.Triples {
				s := resolveTemplateValue(t.Subject, sol, prefixes)
				p := resolveTemplateValue(t.Predicate, sol, prefixes)
				o := resolveTemplateValue(t.Object, sol, prefixes)
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

	// Apply deletions then insertions
	for _, d := range deletes {
		d.graph.Remove(d.subj, &d.pred, d.obj)
	}
	for _, i := range inserts {
		i.graph.Add(i.subj, i.pred, i.obj)
	}

	return nil
}

func evalGraphMgmt(ds *Dataset, op *GraphMgmtOp, prefixes map[string]string) error {
	switch op.Op {
	case "CLEAR", "DROP":
		switch op.Target {
		case "DEFAULT":
			clearGraph(ds.Default)
		case "NAMED":
			for k := range ds.NamedGraphs {
				clearGraph(ds.NamedGraphs[k])
				if op.Op == "DROP" {
					delete(ds.NamedGraphs, k)
				}
			}
		case "ALL":
			clearGraph(ds.Default)
			for k := range ds.NamedGraphs {
				clearGraph(ds.NamedGraphs[k])
				if op.Op == "DROP" {
					delete(ds.NamedGraphs, k)
				}
			}
		default:
			if g, ok := ds.NamedGraphs[op.Target]; ok {
				clearGraph(g)
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
		target := getOrCreateGraph(ds, func() string {
			if op.Into != "" {
				return op.Into
			}
			return "DEFAULT"
		}())
		if err := ds.Loader.Load(context.Background(), target, op.Source); err != nil {
			if !op.Silent {
				return fmt.Errorf("LOAD <%s>: %w", op.Source, err)
			}
		}

	case "ADD":
		return transferGraphs(ds, op.Source, op.Target, false, op.Silent)

	case "COPY":
		return transferGraphs(ds, op.Source, op.Target, true, op.Silent)

	case "MOVE":
		if op.Source == op.Target {
			break // no-op
		}
		if err := transferGraphs(ds, op.Source, op.Target, true, op.Silent); err != nil {
			return err
		}
		// Clear source
		src := getGraph(ds, op.Source)
		if src != nil {
			clearGraph(src)
			if op.Source != "DEFAULT" {
				delete(ds.NamedGraphs, op.Source)
			}
		}
	}

	return nil
}

func transferGraphs(ds *Dataset, srcName, dstName string, replace bool, silent bool) error {
	src := getGraph(ds, srcName)
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

	dst := getOrCreateGraph(ds, dstName)
	if replace {
		clearGraph(dst)
	}

	for _, t := range triples {
		dst.Add(t.s, t.p, t.o)
	}
	return nil
}

func getGraph(ds *Dataset, name string) *rdflibgo.Graph {
	if name == "DEFAULT" {
		return ds.Default
	}
	if g, ok := ds.NamedGraphs[name]; ok {
		return g
	}
	return nil
}

func getOrCreateGraph(ds *Dataset, name string) *rdflibgo.Graph {
	if name == "DEFAULT" {
		return ds.Default
	}
	if g, ok := ds.NamedGraphs[name]; ok {
		return g
	}
	g := rdflibgo.NewGraph()
	if ds.NamedGraphs == nil {
		ds.NamedGraphs = make(map[string]*rdflibgo.Graph)
	}
	ds.NamedGraphs[name] = g
	return g
}

func clearGraph(g *rdflibgo.Graph) {
	g.Remove(nil, nil, nil)
}

func graphForQuad(ds *Dataset, graphName string) *rdflibgo.Graph {
	if graphName == "" {
		return ds.Default
	}
	return getOrCreateGraph(ds, graphName)
}

func graphForQuadSolution(ds *Dataset, graphName string, sol map[string]rdflibgo.Term) *rdflibgo.Graph {
	if graphName == "" {
		return ds.Default
	}
	if strings.HasPrefix(graphName, "?") {
		if v, ok := sol[graphName[1:]]; ok {
			if u, ok := v.(term.URIRef); ok {
				return getOrCreateGraph(ds, u.Value())
			}
		}
		return ds.Default
	}
	return getOrCreateGraph(ds, graphName)
}

func resolveModifyGraph(ds *Dataset, graphName, with string, sol map[string]rdflibgo.Term) *rdflibgo.Graph {
	if graphName == "" {
		if with != "" {
			return getOrCreateGraph(ds, with)
		}
		return ds.Default
	}
	if strings.HasPrefix(graphName, "?") {
		if v, ok := sol[graphName[1:]]; ok {
			if u, ok := v.(term.URIRef); ok {
				return getOrCreateGraph(ds, u.Value())
			}
		}
		return ds.Default
	}
	return getOrCreateGraph(ds, graphName)
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
