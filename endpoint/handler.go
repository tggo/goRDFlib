package endpoint

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
)

// Handler serves the SPARQL 1.1 Protocol and the SPARQL 1.1 Graph Store HTTP
// Protocol over one dataset. Create it with New, NewForGraph or NewForStore.
//
// It is safe for concurrent use. Queries run concurrently; updates and Graph
// Store writes run one at a time and never while a query is being evaluated.
// Code that changes the dataset outside the handler while it serves requests
// must do so inside Write (and read inside Read), or it races with them.
type Handler struct {
	cfg     config
	backend backend
	lock    *rwLock
}

// New returns a handler over a sparql.Dataset, the dataset type the query and
// update engines evaluate against. Each named graph is an entry of
// ds.NamedGraphs and may live in its own store; a nil map is replaced by an
// empty one, and a nil default graph by an empty in-memory graph.
// ds.Loader is ignored, see WithLoader.
func New(ds *sparql.Dataset, opts ...Option) *Handler {
	if ds.Default == nil {
		ds.Default = graph.NewGraph()
	}
	if ds.NamedGraphs == nil {
		ds.NamedGraphs = make(map[string]*graph.Graph)
	}
	return newHandler(&mapBackend{ds: ds}, opts)
}

// NewForGraph returns a handler whose dataset is g as the default graph and
// no named graphs. Graphs created by updates or Graph Store writes are kept in
// memory.
func NewForGraph(g *graph.Graph, opts ...Option) *Handler {
	return New(&sparql.Dataset{Default: g}, opts...)
}

// NewForStore returns a handler over a store-backed graph.Dataset, which is
// what a persistent backend (Badger, SQLite) provides. Every graph is a
// context of ds's store: the named graphs are its IRI contexts, and graphs
// created by an update are written into the store. A named graph exists while
// the store holds a triple in it, so an emptied graph disappears. Blank node
// contexts are not reachable through the protocol.
func NewForStore(ds *graph.Dataset, opts ...Option) *Handler {
	return newHandler(&storeBackend{ds: ds}, opts)
}

func newHandler(b backend, opts []Option) *Handler {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	return &Handler{cfg: cfg, backend: b, lock: newRWLock()}
}

// Read runs fn while holding the handler's read lock, so fn sees the dataset
// as no update is changing it. fn must not change the dataset. It returns
// ctx.Err() without running fn if ctx is done before the lock is acquired.
func (h *Handler) Read(ctx context.Context, fn func()) error {
	if err := h.lock.rlock(ctx); err != nil {
		return err
	}
	defer h.lock.runlock()
	fn()
	return nil
}

// Write runs fn while holding the handler's write lock: no query, update or
// Graph Store request is evaluated at the same time. Use it to change the
// dataset from outside the handler.
func (h *Handler) Write(ctx context.Context, fn func()) error {
	if err := h.lock.lock(ctx); err != nil {
		return err
	}
	defer h.lock.unlock()
	fn()
	return nil
}

// ServeHTTP serves the SPARQL endpoint:
//
//   - GET/HEAD with ?query=, POST application/x-www-form-urlencoded with
//     query=, POST application/sparql-query: a query.
//   - POST form with update=, POST application/sparql-update: an update.
//   - GET/HEAD without query: the service description.
//   - any method with ?graph=<iri> or ?default: the Graph Store HTTP Protocol
//     with indirect graph identification (unless WithoutGraphStore).
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, routeAll)
}

// QueryHandler returns a handler that serves only queries and the service
// description, for deployments that separate the query and update URLs.
// Updates sent to it are answered with 400.
func (h *Handler) QueryHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.serve(w, r, routeQuery) })
}

// UpdateHandler returns a handler that serves only updates (POST). Any other
// request is answered with 405.
func (h *Handler) UpdateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.serve(w, r, routeUpdate) })
}

// GraphStoreHandler returns a handler that serves only the Graph Store HTTP
// Protocol, with indirect identification (?graph=, ?default) and direct
// identification: a request without either parameter addresses the graph
// whose IRI is the request IRI (see WithRequestIRI).
//
// Mount it under a prefix with http.StripPrefix. A request for the prefix
// itself (an empty or "/" path after stripping) addresses the graph store as a
// whole: POST there creates a new graph and answers 201 with its IRI in
// Location; other methods without ?graph= or ?default are answered with 400.
// The graph IRI is computed from the unstripped request URI, so a graph
// created there is readable at its Location.
func (h *Handler) GraphStoreHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.serve(w, r, routeGraphStore) })
}

type route int

const (
	routeAll route = iota
	routeQuery
	routeUpdate
	routeGraphStore
)

// exchange is the state of one request.
type exchange struct {
	h     *Handler
	w     *responseWriter
	r     *http.Request
	op    Operation
	rows  int
	err   error
	start time.Time
}

func (h *Handler) serve(rw http.ResponseWriter, r *http.Request, rt route) {
	x := &exchange{h: h, w: &responseWriter{ResponseWriter: rw}, r: r, start: time.Now()}
	hdr := rw.Header()
	hdr.Set("X-Content-Type-Options", "nosniff")
	defer x.finish()
	defer x.recoverPanic()

	var err error
	switch rt {
	case routeGraphStore:
		err = x.graphStore(true)
	case routeUpdate:
		err = x.sparql(false, true)
	case routeQuery:
		err = x.sparql(true, false)
	default:
		if h.cfg.graphStore && isGraphStoreRequest(r) {
			err = x.graphStore(false)
		} else {
			err = x.sparql(true, true)
		}
	}
	if err != nil {
		x.fail(err)
	}
}

func isGraphStoreRequest(r *http.Request) bool {
	q := r.URL.Query()
	_, isDefault := q["default"]
	_, isGraph := q["graph"]
	return isDefault || isGraph
}

// authorize runs the Authorizer for op.
func (x *exchange) authorize(op Operation) error {
	x.op = op
	a := x.h.cfg.authorize
	if a == nil {
		return nil
	}
	err := a(x.r, op)
	if err == nil {
		return nil
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return err
	}
	if errors.Is(err, ErrUnauthenticated) {
		return httpErr(http.StatusUnauthorized, err)
	}
	return httpErr(http.StatusForbidden, err)
}

// fail answers the request with err. If the response has already started it
// cannot change the status; it aborts the connection instead, so the client
// sees a truncated response rather than a complete-looking one.
func (x *exchange) fail(err error) {
	x.err = err
	if x.r.Context().Err() != nil && !errors.Is(err, ErrTimeout) {
		// The client is gone; nobody reads a response.
		x.w.status = StatusClientClosedRequest
		x.h.cfg.logger.Debug("sparql endpoint: client went away", "op", x.op.String(), "err", err)
		return
	}
	status := http.StatusInternalServerError
	var he *HTTPError
	if errors.As(err, &he) {
		status = he.Status
	}
	if x.w.wroteHeader {
		x.h.cfg.logger.Error("sparql endpoint: failure after the response started; aborting the connection",
			"op", x.op.String(), "err", err)
		panic(http.ErrAbortHandler)
	}
	switch {
	case status >= 500 && errors.Is(err, ErrTimeout):
		x.h.cfg.logger.Warn("sparql endpoint: time limit exceeded", "op", x.op.String(), "err", err)
	case status >= 500:
		x.h.cfg.logger.Error("sparql endpoint: request failed", "op", x.op.String(), "status", status, "err", err)
	default:
		x.h.cfg.logger.Debug("sparql endpoint: request rejected", "op", x.op.String(), "status", status, "err", err)
	}
	hdr := x.w.Header()
	for _, k := range []string{"Content-Length", "Content-Disposition", "ETag", "Last-Modified", "Vary"} {
		hdr.Del(k)
	}
	if he != nil {
		for k, vs := range he.Header {
			hdr[k] = append([]string(nil), vs...)
		}
	}
	hdr.Set("Content-Type", "text/plain; charset=utf-8")
	hdr.Set("Cache-Control", "no-store")
	x.w.WriteHeader(status)
	msg := err.Error()
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	_, _ = x.w.Write([]byte(msg))
}

// recoverPanic turns a panic into a 500, or into an aborted connection when
// the response has already started. finish still runs: it is deferred before.
func (x *exchange) recoverPanic() {
	v := recover()
	if v == nil {
		return
	}
	if v == http.ErrAbortHandler {
		if x.w.status == 0 || x.w.status < 400 {
			x.w.status = http.StatusInternalServerError
		}
		panic(v)
	}
	x.h.cfg.logger.Error("sparql endpoint: panic", "op", x.op.String(), "panic", fmt.Sprint(v),
		"stack", string(debug.Stack()))
	err := fmt.Errorf("internal error: %v", v)
	if x.w.wroteHeader {
		x.err = err
		x.w.status = http.StatusInternalServerError
		panic(http.ErrAbortHandler)
	}
	// The panic value is logged, not sent: it can carry internal details.
	x.fail(httpErr(http.StatusInternalServerError, errors.New("internal server error")))
	x.err = err
}

func (x *exchange) finish() {
	hook := x.h.cfg.hook
	if hook == nil {
		return
	}
	status := x.w.status
	if status == 0 {
		status = http.StatusOK
	}
	info := RequestInfo{
		Op:       x.op,
		Method:   x.r.Method,
		Status:   status,
		Duration: time.Since(x.start),
		Rows:     x.rows,
		Bytes:    x.w.bytes,
	}
	if status >= 400 {
		info.Err = x.err
	}
	hook(x.r, info)
}

// responseWriter records what was sent.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	bytes       int64
}

func (w *responseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

// Flush implements http.Flusher when the underlying writer does.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if !w.wroteHeader {
			w.WriteHeader(http.StatusOK)
		}
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// requestIRI returns the absolute IRI of the request without its query.
func (h *Handler) requestIRI(r *http.Request) string {
	if h.cfg.requestIRI != nil {
		return h.cfg.requestIRI(r)
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	path := r.URL.EscapedPath()
	if r.RequestURI != "" && !strings.HasPrefix(r.RequestURI, "*") {
		// RequestURI survives http.StripPrefix; URL.Path does not.
		p, _, _ := strings.Cut(r.RequestURI, "?")
		if strings.HasPrefix(p, "/") {
			path = p
		} else if i := strings.Index(p, "://"); i >= 0 {
			// absolute-form request target
			if j := strings.IndexByte(p[i+3:], '/'); j >= 0 {
				path = p[i+3+j:]
			} else {
				path = "/"
			}
		}
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + path
}

// baseIRI is the base for queries and updates without BASE.
func (h *Handler) baseIRI(r *http.Request) string {
	if h.cfg.baseIRI != "" {
		return h.cfg.baseIRI
	}
	return h.requestIRI(r)
}

// withTimeout derives the evaluation context.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeoutCause(ctx, d, fmt.Errorf("%w (%s)", ErrTimeout, d))
}

// evalError maps an evaluation or lock error to an HTTP error. A deadline
// that is ours is a 503; a client cancellation needs no status (fail skips
// writing when the request context is done).
func evalError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(context.Cause(ctx), ErrTimeout) {
		if cause := context.Cause(ctx); errors.Is(cause, ErrTimeout) {
			return httpErr(http.StatusServiceUnavailable, cause)
		}
		return httpErr(http.StatusServiceUnavailable, fmt.Errorf("%w: %w", ErrTimeout, err))
	}
	return err
}
