package graph

import (
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Resource provides a node-centric view of a graph.
// Ported from: rdflib.resource.Resource
type Resource struct {
	graph      *Graph
	identifier term.Subject
}

// NewResource creates a Resource wrapping a node in a graph.
// Ported from: rdflib.resource.Resource.__init__
func NewResource(g *Graph, id term.Subject) *Resource {
	return &Resource{graph: g, identifier: id}
}

// Graph returns the backing graph.
func (r *Resource) Graph() *Graph { return r.graph }

// Identifier returns the resource's node.
func (r *Resource) Identifier() term.Subject { return r.identifier }

// Add adds a predicate-object pair for this resource.
// Ported from: rdflib.resource.Resource.add
func (r *Resource) Add(p term.URIRef, o term.Term) {
	r.graph.Add(r.identifier, p, o)
}

// Remove removes matching predicate-object pairs.
// Ported from: rdflib.resource.Resource.remove
func (r *Resource) Remove(p term.URIRef, o term.Term) {
	pp := p
	r.graph.Remove(r.identifier, &pp, o)
}

// Set removes all values for the predicate, then adds the new one.
// Ported from: rdflib.resource.Resource.set
func (r *Resource) Set(p term.URIRef, o term.Term) {
	r.graph.Set(r.identifier, p, o)
}

// Objects returns all objects for the given predicate.
// Ported from: rdflib.resource.Resource.objects
func (r *Resource) Objects(p term.URIRef) store.TermIterator {
	pp := p
	return r.graph.Objects(r.identifier, &pp)
}

// Subjects returns all subjects where this resource is the object of the given predicate.
// Ported from: rdflib.resource.Resource.subjects
func (r *Resource) Subjects(p term.URIRef) store.TermIterator {
	pp := p
	return r.graph.Subjects(&pp, r.identifier)
}

// Value returns a single object for the given predicate.
// Ported from: rdflib.resource.Resource.value
func (r *Resource) Value(p term.URIRef) (term.Term, bool) {
	pp := p
	return r.graph.Value(r.identifier, &pp, nil)
}

// PredicateObjects returns all (predicate, object) pairs for this resource.
// Ported from: rdflib.resource.Resource.predicate_objects
func (r *Resource) PredicateObjects() store.TermPairIterator {
	return r.graph.PredicateObjects(r.identifier)
}
