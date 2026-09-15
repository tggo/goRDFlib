// Package endpoint is an http.Handler for the SPARQL 1.1 Protocol and the
// SPARQL 1.1 Graph Store HTTP Protocol over a dataset of this module, using
// only the standard library and the module's own engine, parsers, serializers
// and result writers.
//
// It is meant to be embedded in Go services and wrapped by a server binary.
// Listening, TLS, authentication, CORS, metrics and access logs are left to
// the caller; WithAuthorizer, WithRequestHook, WithLogger and WithRequestIRI
// are the hooks for them.
//
//	h := endpoint.New(&sparql.Dataset{Default: g}, endpoint.WithQueryTimeout(30*time.Second))
//	http.Handle("/sparql", h)
//	http.Handle("/rdf-graphs/", http.StripPrefix("/rdf-graphs", h.GraphStoreHandler()))
//
// Specifications: https://www.w3.org/TR/sparql11-protocol/,
// https://www.w3.org/TR/sparql11-http-rdf-update/,
// https://www.w3.org/TR/sparql11-service-description/.
//
// # Datasets
//
// New takes a *sparql.Dataset, the type the query and update engines
// evaluate against: a default graph and named graphs keyed by IRI, each of
// which may live in its own store. NewForStore takes a store-backed
// *graph.Dataset, which is what a persistent store provides; there every graph
// is a context of one store, and graphs an update creates are written into it.
// NewForGraph serves a single graph as the default graph.
//
// Blank-node graph names are not addressable: GRAPH, FROM, the protocol
// parameters and ?graph= all take IRIs. store.DefaultGraphIRI
// (urn:x-rdflib:default) names the service's default graph wherever a graph
// IRI is accepted.
//
// # Query dataset
//
// SPARQL 1.1 Protocol §2.1.4: default-graph-uri and named-graph-uri define the
// dataset of a query and override FROM and FROM NAMED; without them the
// query's own FROM and FROM NAMED apply; without either the service dataset
// applies. The default graph of a specified dataset is the union of the listed
// graphs (copied into a temporary graph when there is more than one), and its
// named graphs are the listed ones. Graph IRIs are resolved against the
// service's own graphs only and never fetched; an IRI the service has no
// graph for is an empty graph.
//
// The query parser skips FROM clauses, so the handler reads them with a
// lexical scanner (strings, comments, IRIs, prefixed names, PREFIX and BASE).
// A FROM with an undeclared prefix is ignored rather than guessed.
//
// Updates: using-graph-uri and using-named-graph-uri become USING and USING
// NAMED of every DELETE/INSERT operation; with USING or WITH in the request
// they are a 400 (§2.2.3).
//
// Relative IRIs in a query or update without BASE resolve against
// WithBaseIRI, or the request IRI of the endpoint (§2.3 allows the endpoint as
// the base).
//
// # Status codes
//
//	200  query result, graph read, service description, GSP POST to an existing graph, GSP DELETE
//	201  GSP PUT or POST that created the graph (POST to the store root adds Location)
//	204  update, GSP PUT that replaced an existing graph, OPTIONS
//	400  malformed query or update (text/plain, with the parser's byte offset and the line and column),
//	     malformed form or payload, conflicting or repeated parameters, a relative graph IRI,
//	     an update that fails during evaluation (COPY from a missing graph)
//	401  Authorizer returned ErrUnauthenticated
//	403  Authorizer refused, read-only endpoint, LOAD without WithLoader
//	404  GSP request for a named graph that does not exist
//	405  method not allowed (with Allow), update via GET
//	406  no acceptable representation (the body lists the available ones)
//	413  body over WithMaxRequestBytes or WithMaxGraphBytes
//	415  unsupported Content-Type, or a charset other than UTF-8
//	422  result over WithMaxResultRows
//	500  panic (the value is logged, not sent), result that cannot be encoded in the negotiated format
//	501  DESCRIBE, which the engine does not implement
//	503  query or update exceeded its time limit, including time spent waiting for the lock
//
// A timeout is 503 without Retry-After: the same request would most likely
// time out again, so no retry is suggested; 504 is reserved for gateways. A
// client that disconnects gets nothing; the RequestHook sees
// StatusClientClosedRequest (499). Every error body is text/plain; charset=utf-8.
//
// # Streaming and failures
//
// The engine materializes solutions, so evaluation always ends before the
// response starts and every evaluation error has a proper status. SELECT and
// ASK results are written with sparql/results.Write, which checks every value
// before writing the first byte and then streams rows through a bounded
// buffer, so a value the negotiated format cannot carry (a control character
// in XML) is a 500 with a message, never a 200 followed by garbage. After the
// first byte only the connection can fail; the handler then stops writing.
// Should any other failure occur after the header was sent, the handler
// aborts the connection (http.ErrAbortHandler) so the client sees a truncated
// response instead of a complete-looking one.
//
// CONSTRUCT results, graph reads and the service description are serialized
// into memory first and sent with Content-Length.
//
// # Concurrency and isolation
//
// A readers-writer lock guards the dataset. Queries and graph reads hold it
// shared for their evaluation (not while the response is sent: results are
// values), updates and graph writes hold it exclusively for the whole request.
// Waiting writers keep new readers out, so updates are not starved, and lock
// acquisition gives up with the request's context or timeout.
//
// Through the handler this is serializable: a query never observes a
// half-applied update, and two updates never interleave. It is not atomic:
// an update request whose third operation fails, or that times out, leaves its
// first two applied (see sparql.EvalUpdateContext), and there is no rollback.
// Writes that bypass the handler are not covered; make them inside
// Handler.Write.
//
// # Security defaults
//
//   - LOAD is disabled until WithLoader, since it makes the server fetch URLs
//     chosen by the client (SSRF). Dataset.Loader is ignored.
//   - JSON-LD payloads never fetch remote contexts (ErrRemoteContext).
//   - Bodies are limited: 10 MiB for queries and updates, 64 MiB for graph
//     payloads. Payloads are parsed into a separate graph before the lock is
//     taken, so a malformed or oversized body changes nothing.
//   - Panics are recovered (500, logged with stack); a panic in a payload
//     parser is a 400.
//   - Every response carries X-Content-Type-Options: nosniff; errors and write
//     responses carry Cache-Control: no-store.
//
// # Limits
//
//   - WithMaxResultRows fails (422) rather than truncates. The WHERE clause is
//     still evaluated in full, so it bounds response size, not evaluation
//     memory; use WithQueryTimeout for that.
//   - A single store call (a transitive path pushed down to the store) and an
//     extension function are not interrupted by a timeout.
//   - DESCRIBE is not supported (501).
//   - Graph Store PATCH is not supported (405). Empty named graphs survive in a
//     New dataset but not in a NewForStore one.
package endpoint
