package rdflibgo

import "strings"

// termTypeOrder defines ordering between term types.
func termTypeOrder(t Term) int {
	switch t.(type) {
	case BNode:
		return 0
	case URIRef:
		return 1
	case Literal:
		return 2
	case Variable:
		return 3
	default:
		return 4
	}
}

// CompareTerm compares two terms for deterministic ordering.
// Returns -1, 0, or 1.
func CompareTerm(a, b Term) int {
	ta, tb := termTypeOrder(a), termTypeOrder(b)
	if ta != tb {
		if ta < tb {
			return -1
		}
		return 1
	}
	return strings.Compare(a.N3(), b.N3())
}
