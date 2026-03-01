package rdflibgo

// Resource provides a node-centric view of a graph.
// Ported from: rdflib.resource.Resource
type Resource struct {
	graph      *Graph
	identifier Subject
}

// NewResource creates a Resource wrapping a node in a graph.
// Ported from: rdflib.resource.Resource.__init__
func NewResource(g *Graph, id Subject) *Resource {
	return &Resource{graph: g, identifier: id}
}

// Graph returns the backing graph.
func (r *Resource) Graph() *Graph { return r.graph }

// Identifier returns the resource's node.
func (r *Resource) Identifier() Subject { return r.identifier }

// Add adds a predicate-object pair for this resource.
// Ported from: rdflib.resource.Resource.add
func (r *Resource) Add(p URIRef, o Term) {
	r.graph.Add(r.identifier, p, o)
}

// Remove removes matching predicate-object pairs.
// Ported from: rdflib.resource.Resource.remove
func (r *Resource) Remove(p URIRef, o Term) {
	pp := p
	r.graph.Remove(r.identifier, &pp, o)
}

// Set removes all values for the predicate, then adds the new one.
// Ported from: rdflib.resource.Resource.set
func (r *Resource) Set(p URIRef, o Term) {
	r.graph.Set(r.identifier, p, o)
}

// Objects returns all objects for the given predicate.
// Ported from: rdflib.resource.Resource.objects
func (r *Resource) Objects(p URIRef) TermIterator {
	pp := p
	return r.graph.Objects(r.identifier, &pp)
}

// Subjects returns all subjects where this resource is the object of the given predicate.
// Ported from: rdflib.resource.Resource.subjects
func (r *Resource) Subjects(p URIRef) TermIterator {
	pp := p
	return r.graph.Subjects(&pp, r.identifier)
}

// Value returns a single object for the given predicate.
// Ported from: rdflib.resource.Resource.value
func (r *Resource) Value(p URIRef) (Term, bool) {
	pp := p
	return r.graph.Value(r.identifier, &pp, nil)
}

// PredicateObjects returns all (predicate, object) pairs for this resource.
// Ported from: rdflib.resource.Resource.predicate_objects
func (r *Resource) PredicateObjects() func(yield func(Term, Term) bool) {
	return r.graph.PredicateObjects(r.identifier)
}
