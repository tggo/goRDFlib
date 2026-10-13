package graph

import (
	"context"

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
func (r *Resource) Add(ctx context.Context, p term.URIRef, o term.Term) error {
	return r.graph.Add(ctx, r.identifier, p, o)
}

// Remove removes matching predicate-object pairs.
// Ported from: rdflib.resource.Resource.remove
func (r *Resource) Remove(ctx context.Context, p term.URIRef, o term.Term) error {
	pp := p
	return r.graph.Remove(ctx, r.identifier, &pp, o)
}

// Set removes all values for the predicate, then adds the new one.
// Ported from: rdflib.resource.Resource.set
func (r *Resource) Set(ctx context.Context, p term.URIRef, o term.Term) error {
	return r.graph.Set(ctx, r.identifier, p, o)
}

// Objects returns all objects for the given predicate.
// Ported from: rdflib.resource.Resource.objects
func (r *Resource) Objects(ctx context.Context, p term.URIRef) store.TermIterator {
	pp := p
	return r.graph.Objects(ctx, r.identifier, &pp)
}

// Subjects returns all subjects where this resource is the object of the given predicate.
// Ported from: rdflib.resource.Resource.subjects
func (r *Resource) Subjects(ctx context.Context, p term.URIRef) store.TermIterator {
	pp := p
	return r.graph.Subjects(ctx, &pp, r.identifier)
}

// Value returns a single object for the given predicate.
// Ported from: rdflib.resource.Resource.value
func (r *Resource) Value(ctx context.Context, p term.URIRef) (term.Term, bool, error) {
	pp := p
	return r.graph.Value(ctx, r.identifier, &pp, nil)
}

// PredicateObjects returns all (predicate, object) pairs for this resource.
// Ported from: rdflib.resource.Resource.predicate_objects
func (r *Resource) PredicateObjects(ctx context.Context) store.TermPairIterator {
	return r.graph.PredicateObjects(ctx, r.identifier)
}
