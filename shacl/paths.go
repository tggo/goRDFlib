package shacl

// PathKind distinguishes the different SHACL property path types.
type PathKind int

const (
	// PathPredicate is a simple predicate path.
	PathPredicate PathKind = iota
	// PathInverse is sh:inversePath.
	PathInverse
	// PathSequence is a sequence path (RDF list).
	PathSequence
	// PathAlternative is sh:alternativePath.
	PathAlternative
	// PathZeroOrMore is sh:zeroOrMorePath.
	PathZeroOrMore
	// PathOneOrMore is sh:oneOrMorePath.
	PathOneOrMore
	// PathZeroOrOne is sh:zeroOrOnePath.
	PathZeroOrOne
)

// String returns a human-readable name for the path kind.
func (k PathKind) String() string {
	switch k {
	case PathPredicate:
		return "predicate"
	case PathInverse:
		return "inverse"
	case PathSequence:
		return "sequence"
	case PathAlternative:
		return "alternative"
	case PathZeroOrMore:
		return "zeroOrMore"
	case PathOneOrMore:
		return "oneOrMore"
	case PathZeroOrOne:
		return "zeroOrOne"
	}
	return "unknown"
}

// PropertyPath represents a parsed SHACL property path.
type PropertyPath struct {
	Kind     PathKind
	Pred     Term            // for PathPredicate
	Sub      *PropertyPath   // for PathInverse, PathZeroOrMore, PathOneOrMore, PathZeroOrOne
	Elements []*PropertyPath // for PathSequence, PathAlternative
	Node     Term            // original RDF node (for complex path result reporting)
}

func parsePath(g *Graph, node Term) *PropertyPath {
	if node.IsIRI() {
		return &PropertyPath{Kind: PathPredicate, Pred: node, Node: node}
	}

	// Check for sequence path first (RDF list: node has rdf:first)
	firstPred := IRI(RDFFirst)
	if g.Has(&node, &firstPred, nil) {
		items := g.RDFList(node)
		if len(items) > 0 {
			paths := make([]*PropertyPath, len(items))
			for i, item := range items {
				paths[i] = parsePath(g, item)
			}
			return &PropertyPath{Kind: PathSequence, Elements: paths, Node: node}
		}
	}

	if inv := g.Objects(node, IRI(SH+"inversePath")); len(inv) > 0 {
		return &PropertyPath{Kind: PathInverse, Sub: parsePath(g, inv[0]), Node: node}
	}

	if alt := g.Objects(node, IRI(SH+"alternativePath")); len(alt) > 0 {
		items := g.RDFList(alt[0])
		paths := make([]*PropertyPath, len(items))
		for i, item := range items {
			paths[i] = parsePath(g, item)
		}
		return &PropertyPath{Kind: PathAlternative, Elements: paths, Node: node}
	}

	if zom := g.Objects(node, IRI(SH+"zeroOrMorePath")); len(zom) > 0 {
		return &PropertyPath{Kind: PathZeroOrMore, Sub: parsePath(g, zom[0]), Node: node}
	}

	if oom := g.Objects(node, IRI(SH+"oneOrMorePath")); len(oom) > 0 {
		return &PropertyPath{Kind: PathOneOrMore, Sub: parsePath(g, oom[0]), Node: node}
	}

	if zoo := g.Objects(node, IRI(SH+"zeroOrOnePath")); len(zoo) > 0 {
		return &PropertyPath{Kind: PathZeroOrOne, Sub: parsePath(g, zoo[0]), Node: node}
	}

	// Fallback: treat as predicate
	return &PropertyPath{Kind: PathPredicate, Pred: node, Node: node}
}

// evalPath evaluates a property path starting from a focus node, returning all reachable values.
func evalPath(g *Graph, path *PropertyPath, focus Term) []Term {
	if path == nil {
		return nil
	}
	switch path.Kind {
	case PathPredicate:
		return g.Objects(focus, path.Pred)
	case PathInverse:
		return evalInversePath(g, path.Sub, focus)
	case PathSequence:
		return evalSequencePath(g, path.Elements, focus)
	case PathAlternative:
		return evalAlternativePath(g, path.Elements, focus)
	case PathZeroOrMore:
		return transitiveClose(g, path.Sub, focus, true)
	case PathOneOrMore:
		return transitiveClose(g, path.Sub, focus, false)
	case PathZeroOrOne:
		return evalZeroOrOnePath(g, path.Sub, focus)
	}
	return nil
}

func evalInversePath(g *Graph, sub *PropertyPath, focus Term) []Term {
	if sub.Kind == PathPredicate {
		return g.Subjects(sub.Pred, focus)
	}
	// General inverse: find all nodes that can reach focus via sub.
	// Collect all unique nodes (subjects and objects) as candidates.
	candidates := make(map[string]Term)
	for _, t := range g.Triples() {
		candidates[t.Subject.TermKey()] = t.Subject
		candidates[t.Object.TermKey()] = t.Object
	}
	var result []Term
	seen := make(map[string]bool)
	for _, node := range candidates {
		vals := evalPath(g, sub, node)
		for _, v := range vals {
			key := node.TermKey()
			if v.Equal(focus) && !seen[key] {
				seen[key] = true
				result = append(result, node)
			}
		}
	}
	return result
}

func evalSequencePath(g *Graph, elements []*PropertyPath, focus Term) []Term {
	current := []Term{focus}
	for _, elem := range elements {
		var next []Term
		seen := make(map[string]bool)
		for _, node := range current {
			for _, v := range evalPath(g, elem, node) {
				key := v.TermKey()
				if !seen[key] {
					seen[key] = true
					next = append(next, v)
				}
			}
		}
		current = next
	}
	return current
}

func evalAlternativePath(g *Graph, elements []*PropertyPath, focus Term) []Term {
	var result []Term
	seen := make(map[string]bool)
	for _, elem := range elements {
		for _, v := range evalPath(g, elem, focus) {
			key := v.TermKey()
			if !seen[key] {
				seen[key] = true
				result = append(result, v)
			}
		}
	}
	return result
}

// transitiveClose performs BFS over a path. If includeSelf is true, the focus node
// is included in the result (zero-or-more). Otherwise only nodes reachable via one
// or more steps are included (one-or-more).
func transitiveClose(g *Graph, sub *PropertyPath, focus Term, includeSelf bool) []Term {
	visited := make(map[string]bool)
	var result []Term
	var queue []Term

	if includeSelf {
		visited[focus.TermKey()] = true
		result = append(result, focus)
		queue = append(queue, focus)
	} else {
		// Seed with first step only
		visited[focus.TermKey()] = true // don't revisit focus
		for _, v := range evalPath(g, sub, focus) {
			key := v.TermKey()
			if !visited[key] {
				visited[key] = true
				result = append(result, v)
				queue = append(queue, v)
			}
		}
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, v := range evalPath(g, sub, node) {
			key := v.TermKey()
			if !visited[key] {
				visited[key] = true
				result = append(result, v)
				queue = append(queue, v)
			}
		}
	}
	return result
}

func evalZeroOrOnePath(g *Graph, sub *PropertyPath, focus Term) []Term {
	result := []Term{focus}
	seen := map[string]bool{focus.TermKey(): true}
	for _, v := range evalPath(g, sub, focus) {
		key := v.TermKey()
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result
}

// pathToTerm returns the RDF term representing this path (for result reporting).
func pathToTerm(p *PropertyPath) Term {
	if p == nil {
		return Term{}
	}
	if p.Kind == PathPredicate {
		return p.Pred
	}
	return p.Node
}
