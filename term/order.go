package term

import (
	"sort"
	"strconv"
	"strings"
)

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
// For Literals, uses type-aware comparison (numeric values compared numerically).
func CompareTerm(a, b Term) int {
	ta, tb := termTypeOrder(a), termTypeOrder(b)
	if ta != tb {
		if ta < tb {
			return -1
		}
		return 1
	}
	// Same type — compare within type.
	if la, ok := a.(Literal); ok {
		lb := b.(Literal)
		return compareLiterals(la, lb)
	}
	return strings.Compare(a.N3(), b.N3())
}

// compareLiterals performs type-aware comparison of two literals.
// Numeric types are compared by value; everything else by N3 representation.
func compareLiterals(a, b Literal) int {
	// Try numeric comparison if both have the same numeric datatype.
	if a.datatype == b.datatype {
		switch a.datatype {
		case XSDInteger, XSDInt, XSDLong:
			ai, aErr := strconv.ParseInt(a.lexical, 10, 64)
			bi, bErr := strconv.ParseInt(b.lexical, 10, 64)
			if aErr == nil && bErr == nil {
				switch {
				case ai < bi:
					return -1
				case ai > bi:
					return 1
				default:
					return 0
				}
			}
		case XSDFloat, XSDDouble, XSDDecimal:
			af, aErr := strconv.ParseFloat(a.lexical, 64)
			bf, bErr := strconv.ParseFloat(b.lexical, 64)
			if aErr == nil && bErr == nil {
				switch {
				case af < bf:
					return -1
				case af > bf:
					return 1
				default:
					return 0
				}
			}
		}
	}
	return strings.Compare(a.N3(), b.N3())
}

// TermSlice attaches sort.Interface to a []Term using CompareTerm ordering.
type TermSlice []Term

func (s TermSlice) Len() int           { return len(s) }
func (s TermSlice) Less(i, j int) bool { return CompareTerm(s[i], s[j]) < 0 }
func (s TermSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// SortTerms sorts a slice of Terms in-place using CompareTerm ordering.
func SortTerms(terms []Term) {
	sort.Sort(TermSlice(terms))
}
