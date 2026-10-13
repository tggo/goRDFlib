package sparql

import "errors"

// ErrUndeclaredPrefix is returned by Parse and ParseUpdate for a prefixed
// name whose prefix no PREFIX declaration introduced (SPARQL 1.1 §4.1.1).
// It used to be accepted and the term silently matched anything; declare the
// prefix with PREFIX, or write the full <IRI>.
var ErrUndeclaredPrefix = errors.New("sparql: undeclared prefix")

// ErrUpdateRejected marks an update the engine refuses for what it asks, not
// for a failure of the store: LOAD without a Loader or with a source that
// cannot be loaded, ADD/COPY/MOVE from a graph that does not exist. A store
// error is returned as it is and does not match it, so a service can answer
// the first with a client error and the second with a server error.
var ErrUpdateRejected = errors.New("sparql: update rejected")
