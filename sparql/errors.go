package sparql

import "errors"

// ErrUndeclaredPrefix is returned by Parse and ParseUpdate for a prefixed
// name whose prefix no PREFIX declaration introduced (SPARQL 1.1 §4.1.1).
// It used to be accepted and the term silently matched anything; declare the
// prefix with PREFIX, or write the full <IRI>.
var ErrUndeclaredPrefix = errors.New("sparql: undeclared prefix")
