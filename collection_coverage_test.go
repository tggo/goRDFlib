package rdflibgo

import "testing"

func TestNewCollectionExisting(t *testing.T) {
	g := NewGraph()
	head := NewBNode("list")
	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, RDF.Nil)

	col := NewCollection(g, head)
	if col.Head() != head {
		t.Error("wrong head")
	}
	if col.Len() != 1 {
		t.Errorf("expected 1, got %d", col.Len())
	}
}

func TestCollectionGetOutOfBounds(t *testing.T) {
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("a"))
	_, ok := col.Get(5)
	if ok {
		t.Error("expected out of bounds")
	}
}

func TestCollectionIterEarlyStop(t *testing.T) {
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("a"))
	col.Append(NewLiteral("b"))
	col.Append(NewLiteral("c"))

	count := 0
	col.Iter()(func(Term) bool {
		count++
		return count < 2 // stop after 2
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}
