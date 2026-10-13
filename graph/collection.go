package graph

import (
	"context"
	"iter"

	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Collection provides a high-level API for RDF lists (rdf:first/rdf:rest/rdf:nil).
// Collection is not safe for concurrent use without external synchronization.
type Collection struct {
	graph *Graph
	head  term.Subject
}

// NewCollection creates a Collection for an existing list head node in the graph.
func NewCollection(g *Graph, head term.Subject) *Collection {
	return &Collection{graph: g, head: head}
}

// NewEmptyCollection creates a new empty Collection with a fresh blank node head.
func NewEmptyCollection(g *Graph) *Collection {
	head := term.NewBNode()
	return &Collection{graph: g, head: head}
}

// Head returns the head node of the list.
func (c *Collection) Head() term.Subject {
	return c.head
}

// isRDFNil returns true if the term represents rdf:nil.
func isRDFNil(t term.Term) bool {
	if u, ok := t.(term.URIRef); ok {
		return u.Equal(namespace.RDF.Nil)
	}
	return false
}

// nodes yields the list's nodes from the head, stopping at rdf:nil, at a
// rdf:rest that is missing or not a subject, and at a cycle (a malformed list
// must not loop forever).
func (c *Collection) nodes(ctx context.Context) iter.Seq2[term.Subject, error] {
	return func(yield func(term.Subject, error) bool) {
		restKey := namespace.RDF.Rest
		node := c.head
		visited := map[string]struct{}{term.TermKey(node): {}}
		for {
			if !yield(node, nil) {
				return
			}
			next, ok, err := c.graph.Value(ctx, node, &restKey, nil)
			if err != nil {
				yield(nil, err)
				return
			}
			if !ok || isRDFNil(next) {
				return
			}
			subj, ok := next.(term.Subject)
			if !ok {
				return
			}
			k := term.TermKey(subj)
			if _, seen := visited[k]; seen {
				return
			}
			visited[k] = struct{}{}
			node = subj
		}
	}
}

// first returns the rdf:first of node.
func (c *Collection) first(ctx context.Context, node term.Subject) (term.Term, bool, error) {
	firstKey := namespace.RDF.First
	return c.graph.Value(ctx, node, &firstKey, nil)
}

// Append adds an item to the end of the list.
func (c *Collection) Append(ctx context.Context, item term.Term) error {
	end, err := c.end(ctx)
	if err != nil {
		return err
	}
	g := c.graph
	if end == nil {
		// Empty list: set first item on head
		if err := g.Add(ctx, c.head, namespace.RDF.First, item); err != nil {
			return err
		}
		return g.Add(ctx, c.head, namespace.RDF.Rest, namespace.RDF.Nil)
	}

	// end has rdf:first. Build the new node first, then switch the old tail's
	// rdf:rest to it with one Set: a write that fails partway leaves an
	// unreachable node, never a truncated or dangling list.
	newNode := term.NewBNode()
	if err := g.Add(ctx, newNode, namespace.RDF.First, item); err != nil {
		return err
	}
	if err := g.Add(ctx, newNode, namespace.RDF.Rest, namespace.RDF.Nil); err != nil {
		return err
	}
	return g.Set(ctx, end, namespace.RDF.Rest, newNode)
}

// Len returns the number of items in the list.
func (c *Collection) Len(ctx context.Context) (int, error) {
	count := 0
	for _, err := range c.Iter(ctx) {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// Get returns the item at the given index.
func (c *Collection) Get(ctx context.Context, index int) (term.Term, bool, error) {
	i := 0
	for node, err := range c.nodes(ctx) {
		if err != nil {
			return nil, false, err
		}
		if i == index {
			return c.first(ctx, node)
		}
		i++
	}
	return nil, false, nil
}

// Index returns the position of an item in the list, or -1 if not found.
// Cycle-safe: a malformed list is walked once.
func (c *Collection) Index(ctx context.Context, item term.Term) (int, error) {
	i := 0
	for node, err := range c.nodes(ctx) {
		if err != nil {
			return -1, err
		}
		val, ok, err := c.first(ctx, node)
		if err != nil {
			return -1, err
		}
		if ok && val.Equal(item) {
			return i, nil
		}
		i++
	}
	return -1, nil
}

// Iter iterates over the items in the list.
// Cycle-safe: a malformed list is walked once.
func (c *Collection) Iter(ctx context.Context) store.TermIterator {
	return func(yield func(term.Term, error) bool) {
		for node, err := range c.nodes(ctx) {
			if err != nil {
				yield(nil, err)
				return
			}
			val, ok, err := c.first(ctx, node)
			if err != nil {
				yield(nil, err)
				return
			}
			if !ok || !yield(val, nil) {
				return
			}
		}
	}
}

// Clear removes all triples belonging to this list from the graph.
// Cycle-safe: a malformed list is walked once.
func (c *Collection) Clear(ctx context.Context) error {
	// Collect first: removing rdf:rest while walking it would cut the walk.
	var nodes []term.Subject
	for node, err := range c.nodes(ctx) {
		if err != nil {
			return err
		}
		nodes = append(nodes, node)
	}
	firstKey := namespace.RDF.First
	restKey := namespace.RDF.Rest
	for _, node := range nodes {
		if err := c.graph.Remove(ctx, node, &firstKey, nil); err != nil {
			return err
		}
		if err := c.graph.Remove(ctx, node, &restKey, nil); err != nil {
			return err
		}
	}
	return nil
}

// end returns the last node in the list (the one whose rdf:rest is rdf:nil),
// or nil if the list is empty (head has no rdf:first).
// Cycle-safe: on a cycle it returns the node that closes it.
func (c *Collection) end(ctx context.Context) (term.Subject, error) {
	// If head has no rdf:first, the list is empty.
	if _, ok, err := c.first(ctx, c.head); err != nil || !ok {
		return nil, err
	}
	var last term.Subject
	for node, err := range c.nodes(ctx) {
		if err != nil {
			return nil, err
		}
		last = node
	}
	return last, nil
}
