package rdflibgo

// Collection provides a high-level API for RDF lists (rdf:first/rdf:rest/rdf:nil).
// Ported from: rdflib.collection.Collection
type Collection struct {
	graph *Graph
	head  Subject
}

// NewCollection creates a Collection for an existing list head node in the graph.
func NewCollection(g *Graph, head Subject) *Collection {
	return &Collection{graph: g, head: head}
}

// NewEmptyCollection creates a new empty Collection with a fresh blank node head.
// The head initially points to rdf:nil.
func NewEmptyCollection(g *Graph) *Collection {
	head := NewBNode()
	return &Collection{graph: g, head: head}
}

// Head returns the head node of the list.
func (c *Collection) Head() Subject {
	return c.head
}

// Append adds an item to the end of the list.
// Ported from: rdflib.collection.Collection.append
func (c *Collection) Append(item Term) {
	end := c.end()
	if end == nil {
		// Empty list: set first item on head
		c.graph.Add(c.head, RDF.First, item)
		c.graph.Add(c.head, RDF.Rest, RDF.Nil)
		return
	}

	// Check if end already has rdf:first
	_, hasFirst := c.graph.Value(end, &RDF.First, nil)
	if !hasFirst {
		c.graph.Add(end, RDF.First, item)
		c.graph.Add(end, RDF.Rest, RDF.Nil)
		return
	}

	// Create new node, link from end
	newNode := NewBNode()
	rest := RDF.Rest
	c.graph.Remove(end, &rest, nil)
	c.graph.Add(end, RDF.Rest, newNode)
	c.graph.Add(newNode, RDF.First, item)
	c.graph.Add(newNode, RDF.Rest, RDF.Nil)
}

// Len returns the number of items in the list.
// Ported from: rdflib.collection.Collection.__len__
func (c *Collection) Len() int {
	count := 0
	c.Iter()(func(Term) bool {
		count++
		return true
	})
	return count
}

// Get returns the item at the given index.
// Ported from: rdflib.collection.Collection.__getitem__
func (c *Collection) Get(index int) (Term, bool) {
	node := c.getContainer(index)
	if node == nil {
		return nil, false
	}
	first := RDF.First
	val, ok := c.graph.Value(node, &first, nil)
	return val, ok
}

// Index returns the position of an item in the list, or -1 if not found.
// Ported from: rdflib.collection.Collection.index
func (c *Collection) Index(item Term) int {
	i := 0
	node := c.head
	firstKey := RDF.First
	restKey := RDF.Rest
	itemN3 := item.N3()

	for node != nil {
		val, ok := c.graph.Value(node, &firstKey, nil)
		if ok && val.N3() == itemN3 {
			return i
		}
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok || next.N3() == RDF.Nil.N3() {
			break
		}
		if subj, ok := next.(Subject); ok {
			node = subj
		} else {
			break
		}
		i++
	}
	return -1
}

// Iter iterates over the items in the list.
// Ported from: rdflib.collection.Collection.__iter__
func (c *Collection) Iter() TermIterator {
	return func(yield func(Term) bool) {
		node := c.head
		firstKey := RDF.First
		restKey := RDF.Rest

		for node != nil {
			val, ok := c.graph.Value(node, &firstKey, nil)
			if !ok {
				return
			}
			if !yield(val) {
				return
			}
			next, ok := c.graph.Value(node, &restKey, nil)
			if !ok || next.N3() == RDF.Nil.N3() {
				return
			}
			if subj, ok := next.(Subject); ok {
				node = subj
			} else {
				return
			}
		}
	}
}

// Clear removes all triples belonging to this list from the graph.
// Ported from: rdflib.collection.Collection.clear
func (c *Collection) Clear() {
	node := c.head
	firstKey := RDF.First
	restKey := RDF.Rest

	for node != nil {
		c.graph.Remove(node, &firstKey, nil)
		next, ok := c.graph.Value(node, &restKey, nil)
		c.graph.Remove(node, &restKey, nil)
		if !ok || next.N3() == RDF.Nil.N3() {
			return
		}
		if subj, ok := next.(Subject); ok {
			node = subj
		} else {
			return
		}
	}
}

// end returns the last node in the list (the one whose rdf:rest is rdf:nil).
// Ported from: rdflib.collection.Collection._end
func (c *Collection) end() Subject {
	node := c.head
	restKey := RDF.Rest

	for {
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok {
			return node
		}
		if next.N3() == RDF.Nil.N3() {
			return node
		}
		if subj, ok := next.(Subject); ok {
			node = subj
		} else {
			return node
		}
	}
}

// getContainer navigates to the nth node in the list.
// Ported from: rdflib.collection.Collection._get_container
func (c *Collection) getContainer(index int) Subject {
	node := c.head
	restKey := RDF.Rest

	for i := 0; i < index; i++ {
		next, ok := c.graph.Value(node, &restKey, nil)
		if !ok || next.N3() == RDF.Nil.N3() {
			return nil
		}
		if subj, ok := next.(Subject); ok {
			node = subj
		} else {
			return nil
		}
	}
	return node
}
