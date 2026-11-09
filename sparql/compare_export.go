package sparql

import rdflibgo "github.com/tggo/goRDFlib"

// CompareTerms orders two terms as SPARQL ORDER BY does (§15.1, with the
// numeric and temporal value comparisons of the engine): blank nodes, then
// IRIs, then literals, then triple terms, and an unbound value (nil) before
// any term. It returns a negative number, zero or a positive number.
//
// It is the order SPARQL MIN and MAX use, which is what other packages (the
// SHACL node expressions shnex:min, shnex:max and shnex:orderBy) need. Safe
// for concurrent use.
func CompareTerms(a, b rdflibgo.Term) int { return compareTermValues(a, b) }
