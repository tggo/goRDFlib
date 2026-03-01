package rdflibgo

// Dataset extends ConjunctiveGraph with explicit named graph management.
// Ported from: rdflib.graph.Dataset
type Dataset struct {
	*ConjunctiveGraph
	graphs map[string]*Graph // identifier key → Graph
}

// NewDataset creates a new Dataset.
// Ported from: rdflib.graph.Dataset.__init__
func NewDataset(opts ...GraphOption) *Dataset {
	cg := NewConjunctiveGraph(opts...)
	ds := &Dataset{
		ConjunctiveGraph: cg,
		graphs:           make(map[string]*Graph),
	}
	// Register default graph
	ds.graphs[termKey(cg.defaultContext.identifier)] = cg.defaultContext
	return ds
}

// Graph returns or creates a named graph with the given identifier.
// Ported from: rdflib.graph.Dataset.graph
func (ds *Dataset) Graph(id Term) *Graph {
	if id == nil {
		return ds.DefaultContext()
	}
	k := termKey(id)
	if g, ok := ds.graphs[k]; ok {
		return g
	}
	g := &Graph{store: ds.store, identifier: id}
	ds.graphs[k] = g
	return g
}

// AddGraph registers a graph in the dataset.
// Ported from: rdflib.graph.Dataset.add_graph
func (ds *Dataset) AddGraph(g *Graph) {
	ds.graphs[termKey(g.identifier)] = g
}

// RemoveGraph removes a named graph from the dataset.
// Ported from: rdflib.graph.Dataset.remove_graph
func (ds *Dataset) RemoveGraph(id Term) {
	k := termKey(id)
	// Don't remove default graph
	if k == termKey(ds.defaultContext.identifier) {
		return
	}
	if g, ok := ds.graphs[k]; ok {
		// Remove all triples in that context
		ds.store.Remove(TriplePattern{}, g.identifier)
		delete(ds.graphs, k)
	}
}

// Graphs returns all named graphs in the dataset.
// Ported from: rdflib.graph.Dataset.graphs
func (ds *Dataset) Graphs() func(yield func(*Graph) bool) {
	return func(yield func(*Graph) bool) {
		for _, g := range ds.graphs {
			if !yield(g) {
				return
			}
		}
	}
}
