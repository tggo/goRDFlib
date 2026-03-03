package graph

import (
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

// Append adds an item to the end of the list.
func (c *Collection) Append(item term.Term) {
	end := c.end()
	if end == nil {
		// Empty list: set first item on head
		c.graph.Add(c.head, namespace.RDF.First, item)
		c.graph.Add(c.head, namespace.RDF.Rest, namespace.RDF.Nil)
		return
	}

	// end has rdf:first — create new node and link from end
	newNode := term.NewBNode()
	rest := namespace.RDF.Rest
	c.graph.Remove(end, &rest, nil)
	c.graph.Add(end, namespace.RDF.Rest, newNode)
	c.graph.Add(newNode, namespace.RDF.First, item)
	c.graph.Add(newNode, namespace.RDF.Rest, namespace.RDF.Nil)
}

// Len returns the number of items in the list.
func (c *Collection) Len() int {
	count := 0
	c.Iter()(func(term.Term) bool {
		count++
		return true
	})
	return count
}

// Get returns the item at the given index.
func (c *Collection) Get(index int) (term.Term, bool) {
	node := c.getContainer(index)
	if node == nil {
		return nil, false
	}
	first := namespace.RDF.First
	val, ok := c.graph.Value(node, &first, nil)
	return val, ok
}

// Index returns the position of an item in the list, or -1 if not found.
// Cycle-safe: uses a visited set to prevent infinite loops on malformed data.
func (c *Collection) Index(item term.Term) int {
	i := 0
	node := c.head
	firstKey := namespace.RDF.First
	restKey := namespace.RDF.Rest
	visited := map[string]struct{}{term.TermKey(node): {}}

	for node != nil {
		val, ok := c.graph.Value(node, &firstKey, nil)
		if ok && val.Equal(item) {
			return i
		}
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok || isRDFNil(next) {
			break
		}
		subj, ok := next.(term.Subject)
		if !ok {
			break
		}
		k := term.TermKey(subj)
		if _, seen := visited[k]; seen {
			break // cycle detected
		}
		visited[k] = struct{}{}
		node = subj
		i++
	}
	return -1
}

// Iter iterates over the items in the list.
// Cycle-safe: uses a visited set to prevent infinite loops on malformed data.
func (c *Collection) Iter() store.TermIterator {
	return func(yield func(term.Term) bool) {
		node := c.head
		firstKey := namespace.RDF.First
		restKey := namespace.RDF.Rest
		visited := map[string]struct{}{term.TermKey(node): {}}

		for node != nil {
			val, ok := c.graph.Value(node, &firstKey, nil)
			if !ok {
				return
			}
			if !yield(val) {
				return
			}
			next, ok := c.graph.Value(node, &restKey, nil)
			if !ok || isRDFNil(next) {
				return
			}
			subj, ok := next.(term.Subject)
			if !ok {
				return
			}
			k := term.TermKey(subj)
			if _, seen := visited[k]; seen {
				return // cycle detected
			}
			visited[k] = struct{}{}
			node = subj
		}
	}
}

// Clear removes all triples belonging to this list from the graph.
// Cycle-safe: uses a visited set to prevent infinite loops on malformed data.
func (c *Collection) Clear() {
	node := c.head
	firstKey := namespace.RDF.First
	restKey := namespace.RDF.Rest
	visited := map[string]struct{}{term.TermKey(node): {}}

	for node != nil {
		c.graph.Remove(node, &firstKey, nil)
		next, ok := c.graph.Value(node, &restKey, nil)
		c.graph.Remove(node, &restKey, nil)
		if !ok || isRDFNil(next) {
			return
		}
		subj, ok := next.(term.Subject)
		if !ok {
			return
		}
		k := term.TermKey(subj)
		if _, seen := visited[k]; seen {
			return // cycle detected
		}
		visited[k] = struct{}{}
		node = subj
	}
}

// end returns the last node in the list (the one whose rdf:rest is rdf:nil),
// or nil if the list is empty (head has no rdf:first).
// Cycle-safe: uses a visited set to prevent infinite loops on malformed data.
func (c *Collection) end() term.Subject {
	node := c.head
	firstKey := namespace.RDF.First
	restKey := namespace.RDF.Rest

	// If head has no rdf:first, the list is empty.
	if _, ok := c.graph.Value(node, &firstKey, nil); !ok {
		return nil
	}

	visited := map[string]struct{}{term.TermKey(node): {}}

	for {
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok || isRDFNil(next) {
			return node
		}
		subj, ok := next.(term.Subject)
		if !ok {
			return node
		}
		k := term.TermKey(subj)
		if _, seen := visited[k]; seen {
			return node // cycle detected, return current
		}
		visited[k] = struct{}{}
		node = subj
	}
}

// getContainer navigates to the nth node in the list.
// Cycle-safe: uses a visited set to prevent infinite loops on malformed data.
func (c *Collection) getContainer(index int) term.Subject {
	node := c.head
	restKey := namespace.RDF.Rest
	visited := map[string]struct{}{term.TermKey(node): {}}

	for i := 0; i < index; i++ {
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok || isRDFNil(next) {
			return nil
		}
		subj, ok := next.(term.Subject)
		if !ok {
			return nil
		}
		k := term.TermKey(subj)
		if _, seen := visited[k]; seen {
			return nil // cycle detected
		}
		visited[k] = struct{}{}
		node = subj
	}
	return node
}
