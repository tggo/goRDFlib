package trig

import rdflibgo "github.com/tggo/goRDFlib"

// nodeKey is the map key the TriG serializer uses for a term.
func nodeKey(t rdflibgo.Term) string { return t.N3() }

// Collections. This mirrors turtle/serializer_lists.go.
//
// A chain of rdf:first / rdf:rest nodes may be written as ( ... ) only when the
// collection syntax reproduces exactly those triples. Turtle 1.1 §7.2 expands
// ( a b ) into fresh blank nodes, one per item, each with one rdf:first and one
// rdf:rest, ending in rdf:nil. So every node of the chain must be
//
//   - a blank node (an IRI node would be replaced by a fresh blank node),
//   - the subject of exactly one rdf:first and one rdf:rest and nothing else,
//   - referenced exactly once as an object: by the rdf:rest of the previous
//     node, or, for the first node, by the triple the collection is written
//     into. A second reference cannot be expressed without a label, and a node
//     with no reference cannot be written as a collection at all, because a
//     collection in subject position needs a predicate-object list,
//
// and the chain must end in rdf:nil without a cycle.
//
// Such a node is a "list cell". Any cell starts a writable collection, not
// only the first: if a head is referenced twice it keeps its label and its tail
// is still written as ( ... ). Cells are never written as top-level subjects;
// the single reference writes them. Earlier versions accepted IRI nodes,
// shared tails and unreferenced lists, skipped them as subjects, and dropped
// their triples (rdflib #802, #3542).

// isListCellShape reports whether k has the local shape of a list cell. It
// does not look at the rest of the chain.
func (ts *trigState) isListCellShape(k string) bool {
	if _, ok := ts.subjectMap[k].(rdflibgo.BNode); !ok {
		return false
	}
	if ts.refs[k] != 1 {
		return false
	}
	preds := ts.spoMap[k]
	return len(preds) == 2 && len(preds[ts.firstKey]) == 1 && len(preds[ts.restKey]) == 1
}

// detectLists marks every list cell. Each chain is walked once: the verdict
// found at its end (rdf:nil, a node that is not a cell, a cycle, or a node
// already decided) is assigned to every node on the path. The walk is
// iterative, so a long list cannot exhaust the stack.
func (ts *trigState) detectLists() {
	const (
		unknown = iota
		good
		bad
	)
	verdict := make(map[string]int8)
	visitedBy := make(map[string]int)
	var path []string
	walk := 0
	for k := range ts.spoMap {
		if verdict[k] != unknown || !ts.isListCellShape(k) {
			continue
		}
		walk++
		path = path[:0]
		result := int8(bad)
		for node := k; ; {
			if node == ts.nilKey {
				result = good
				break
			}
			if v := verdict[node]; v != unknown {
				result = v
				break
			}
			if visitedBy[node] == walk || !ts.isListCellShape(node) {
				break
			}
			visitedBy[node] = walk
			path = append(path, node)
			node = nodeKey(ts.spoMap[node][ts.restKey][0])
		}
		for _, n := range path {
			verdict[n] = result
		}
	}
	for k, v := range verdict {
		if v == good {
			ts.listCells[k] = true
		}
	}
}

// canWriteList reports whether the collection starting at cell k can still be
// written: none of its nodes has been emitted yet. A cell that is part of a
// cycle through rdf:first is reachable only from inside its own collection, so
// the final pass in write may emit it as a subject first; its collection must
// then not be written a second time as fresh blank nodes.
func (ts *trigState) canWriteList(k string) bool {
	for node := k; node != ts.nilKey; node = nodeKey(ts.spoMap[node][ts.restKey][0]) {
		if ts.serialized[node] {
			return false
		}
	}
	return true
}
