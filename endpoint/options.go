package endpoint

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

// Operation is the kind of work a request asks for. It is what an Authorizer
// decides on and what a RequestHook reports.
type Operation int

// The operations. OpUnknown is reported for requests rejected before their
// operation could be determined (a wrong method, a body over the limit, an
// unsupported media type).
const (
	OpUnknown    Operation = iota
	OpQuery                // SPARQL query, including the service description
	OpUpdate               // SPARQL update
	OpGraphRead            // Graph Store Protocol GET and HEAD
	OpGraphWrite           // Graph Store Protocol PUT, POST and DELETE
)

// String returns "query", "update", "graph-read", "graph-write" or "unknown".
func (o Operation) String() string {
	switch o {
	case OpQuery:
		return "query"
	case OpUpdate:
		return "update"
	case OpGraphRead:
		return "graph-read"
	case OpGraphWrite:
		return "graph-write"
	}
	return "unknown"
}

// Authorizer decides whether a request may perform op. It runs after the
// request has been read and classified, and before anything is evaluated or
// changed. Returning nil allows the request.
//
// An error matching ErrUnauthenticated under errors.Is is answered with 401,
// an *HTTPError with its own status and headers (use that to send
// WWW-Authenticate), and any other error with 403. The error text is sent to
// the client, so do not put secrets in it.
type Authorizer func(r *http.Request, op Operation) error

// RequestInfo describes one finished request for a RequestHook.
type RequestInfo struct {
	Op     Operation
	Method string
	// Status is the HTTP status sent, or StatusClientClosedRequest (499) when
	// the client went away before a response could be written.
	Status   int
	Duration time.Duration
	// Rows is the number of solutions of a SELECT, 1 for an ASK, the number of
	// triples of a CONSTRUCT result or of a graph read, and the number of
	// triples parsed from a Graph Store PUT or POST body. It is 0 otherwise.
	Rows int
	// Bytes is the number of response body bytes written.
	Bytes int64
	// Err is the error behind a status >= 400, nil otherwise.
	Err error
}

// StatusClientClosedRequest is the status a RequestHook sees when the client
// disconnected before the response was written. Nothing is sent with it; the
// number follows the nginx convention so that it can be counted separately
// from server errors.
const StatusClientClosedRequest = 499

// RequestHook is called once per request after the response is complete, on
// the request's goroutine. Keep it fast; it is meant for metrics and access
// logs.
type RequestHook func(r *http.Request, info RequestInfo)

// Option configures a Handler.
type Option func(*config)

type config struct {
	readOnly        bool
	queryTimeout    time.Duration
	updateTimeout   time.Duration
	maxRequestBytes int64
	maxGraphBytes   int64
	maxResultRows   int
	logger          *slog.Logger
	authorize       Authorizer
	hook            RequestHook
	loader          sparql.Loader
	baseIRI         string
	requestIRI      func(*http.Request) string
	resultFormats   []results.Format
	graphStore      bool
}

// Defaults.
const (
	// DefaultMaxRequestBytes bounds a query or update request body.
	DefaultMaxRequestBytes = 10 << 20
	// DefaultMaxGraphBytes bounds a Graph Store Protocol PUT or POST body.
	DefaultMaxGraphBytes = 64 << 20
)

func defaultConfig() config {
	return config{
		maxRequestBytes: DefaultMaxRequestBytes,
		maxGraphBytes:   DefaultMaxGraphBytes,
		logger:          slog.New(slog.DiscardHandler),
		resultFormats:   []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV},
		graphStore:      true,
	}
}

// WithReadOnly rejects every SPARQL update and Graph Store write with 403,
// and leaves sd:SPARQL11Update out of the service description.
func WithReadOnly() Option { return func(c *config) { c.readOnly = true } }

// WithQueryTimeout bounds the evaluation of one query, including the time
// spent waiting for an update to finish. A query that runs out of time is
// stopped and answered with 503 Service Unavailable. Zero (the default) means
// no limit beyond the client's own connection.
func WithQueryTimeout(d time.Duration) Option { return func(c *config) { c.queryTimeout = d } }

// WithUpdateTimeout bounds one update request or Graph Store write, including
// the time spent waiting for the write lock. See the package documentation for
// what a stopped update leaves applied.
func WithUpdateTimeout(d time.Duration) Option { return func(c *config) { c.updateTimeout = d } }

// WithMaxRequestBytes bounds the body of a query or update request (default
// DefaultMaxRequestBytes). A larger body is answered with 413. n <= 0 removes
// the limit.
func WithMaxRequestBytes(n int64) Option { return func(c *config) { c.maxRequestBytes = n } }

// WithMaxGraphBytes bounds the body of a Graph Store PUT or POST (default
// DefaultMaxGraphBytes). A larger body is answered with 413. n <= 0 removes
// the limit.
func WithMaxGraphBytes(n int64) Option { return func(c *config) { c.maxGraphBytes = n } }

// WithMaxResultRows makes a query whose result has more than n solutions (or a
// CONSTRUCT or graph read with more than n triples) fail with 422 instead of
// being sent. The result is never truncated: a silently incomplete answer is
// indistinguishable from a complete one. n <= 0 (the default) means no limit.
//
// For SELECT and CONSTRUCT the limit is pushed into the query as LIMIT n+1
// when the query has no smaller LIMIT, so projection and template
// instantiation stop early; the WHERE clause is still evaluated in full,
// because the engine materializes solutions.
func WithMaxResultRows(n int) Option { return func(c *config) { c.maxResultRows = n } }

// WithLogger sets the logger. Server errors and recovered panics are logged at
// Error, timeouts at Warn, client errors and disconnects at Debug. The default
// discards everything.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithAuthorizer installs an authorization check; see Authorizer.
func WithAuthorizer(a Authorizer) Option { return func(c *config) { c.authorize = a } }

// WithRequestHook installs a per-request callback for metrics; see RequestHook.
func WithRequestHook(h RequestHook) Option { return func(c *config) { c.hook = h } }

// WithLoader enables the SPARQL Update LOAD operation with l. LOAD is disabled
// by default: a LOAD <url> makes the server fetch an arbitrary URL (or, with
// rdfloader.DefaultLoader, read an arbitrary local file), which is a
// server-side request forgery risk on any endpoint reachable by untrusted
// clients. Give it a Loader that only accepts the sources you intend.
//
// Dataset.Loader of the dataset passed to New is ignored for the same reason.
func WithLoader(l sparql.Loader) Option { return func(c *config) { c.loader = l } }

// WithBaseIRI sets the base IRI for relative IRIs in queries and updates that
// have no BASE of their own, and in Graph Store PUT and POST bodies. Without
// it, queries and updates resolve against the request IRI of the endpoint and
// Graph Store bodies against the IRI of the graph they are written to.
func WithBaseIRI(base string) Option { return func(c *config) { c.baseIRI = base } }

// WithRequestIRI replaces how the absolute IRI of a request is computed. The
// default is scheme://Host/path from the request, with https when r.TLS is set.
// Behind a reverse proxy that rewrites Host or terminates TLS, supply a
// function that reconstructs the public IRI; it decides the default base IRI
// and the graph IRI of direct graph identification.
func WithRequestIRI(f func(r *http.Request) string) Option {
	return func(c *config) { c.requestIRI = f }
}

// WithResultFormats sets which SPARQL result formats are offered and in which
// order of preference. The first one is sent to clients that accept anything.
// The default is JSON, XML, CSV, TSV.
func WithResultFormats(formats ...results.Format) Option {
	return func(c *config) {
		if len(formats) > 0 {
			c.resultFormats = append([]results.Format(nil), formats...)
		}
	}
}

// WithoutGraphStore turns off the Graph Store HTTP Protocol on the endpoint
// URL: requests with ?graph= or ?default are then treated as SPARQL Protocol
// requests. GraphStoreHandler is not affected.
func WithoutGraphStore() Option { return func(c *config) { c.graphStore = false } }
