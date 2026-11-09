package shacl

import (
	"fmt"
	"sync"

	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// ErrPreBoundRestriction is wrapped by the failure of a run whose SHACL-SPARQL
// query breaks the restrictions on queries executed with pre-bound variables
// (SHACL-SPARQL, Appendix "Pre-binding of Variables in SPARQL Queries"): a
// MINUS clause, a VALUES clause that mentions a pre-bound variable, or an
// AS ?var that assigns one. Such a query is not evaluated: the spec requires
// the processor to report a failure (the error also matches ErrQueryFailed),
// which is what ValidateContext returns and
// what Validate puts in ValidationReport.Err. Rewrite the query to filter on
// the variable instead (FILTER NOT EXISTS replaces MINUS, FILTER (?x = ...)
// replaces VALUES, a fresh variable replaces AS).
var ErrPreBoundRestriction = sparql.ErrPreBindRestriction

// ErrQueryReportedFailure is wrapped by the failure of a run in which a
// SHACL-SPARQL constraint produced a solution that binds ?failure to true
// (SHACL-SPARQL, "Validation with SPARQL-based Constraints").
var ErrQueryReportedFailure = fmt.Errorf("%w: a solution binds ?failure to true", ErrQueryFailed)

type preBoundKey struct {
	query string
	mask  uint64
}

// preBoundChecks caches the verdict per query text and set of pre-bound names.
// The text is a template with the $-variables still in it; the mask is the XOR
// of the hashes of the names, which does not depend on map order and allocates
// nothing on a hit.
var preBoundChecks = struct {
	sync.RWMutex
	m map[preBoundKey]error
}{m: map[preBoundKey]error{}}

func hashName(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// checkPreBound parses query (a template, $-variables not yet substituted) and
// applies the pre-binding restrictions for the variables named by the keys of
// text and initial. A query that does not parse passes here: its parse error
// is reported where the query is run.
func checkPreBound(query string, text map[string]string, initial map[string]term.Term) error {
	if len(text)+len(initial) == 0 {
		return nil
	}
	var mask uint64
	for k := range text {
		mask ^= hashName(k)
	}
	for k := range initial {
		if _, dup := text[k]; !dup {
			mask ^= hashName(k)
		}
	}
	key := preBoundKey{query, mask}
	preBoundChecks.RLock()
	err, ok := preBoundChecks.m[key]
	preBoundChecks.RUnlock()
	if ok {
		return err
	}

	err = nil
	if pq, perr := parseShapesQuery(query, true); perr == nil {
		names := make([]string, 0, len(text)+len(initial))
		for k := range text {
			names = append(names, k)
		}
		for k := range initial {
			if _, dup := text[k]; !dup {
				names = append(names, k)
			}
		}
		if rerr := pq.PreBindRestrictions(names...); rerr != nil {
			err = fmt.Errorf("%w: %w", ErrQueryFailed, rerr)
		}
	}

	preBoundChecks.Lock()
	if len(preBoundChecks.m) >= maxParsedQueries {
		preBoundChecks.m = map[preBoundKey]error{}
	}
	preBoundChecks.m[key] = err
	preBoundChecks.Unlock()
	return err
}

// isFailureTrue reports whether a solution binds ?failure to true.
func isFailureTrue(row map[string]Term) bool {
	f, ok := row["failure"]
	return ok && f.IsLiteral() && f.Datatype() == XSD+"boolean" && (f.Value() == "true" || f.Value() == "1")
}

// failureError builds the run failure of a ?failure = true solution of the
// query of constraint node.
func failureError(node Term) error {
	return fmt.Errorf("%w: the SPARQL query of %s", ErrQueryReportedFailure, node)
}
