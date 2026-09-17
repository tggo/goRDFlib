package sparqlstore

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// SPARQLStore implements store.Store by communicating with a remote SPARQL
// endpoint over HTTP. It translates Store method calls into SPARQL queries
// and updates sent via the W3C SPARQL 1.1 Protocol.
//
// All methods are safe for concurrent use.
//
// It implements store.ContextBinder: BindContext returns a view whose HTTP
// requests are made with the given context, so they carry its deadline and
// cancellation, and whatever a tracing http.RoundTripper reads from it. The
// SPARQL engine binds its evaluation context this way. Without a bound
// context, requests use context.Background and WithTimeout's limit.
type SPARQLStore struct {
	queryURL  string
	updateURL string
	client    *http.Client

	// ctx is the context requests are made with; nil means Background.
	ctx context.Context

	// ns is the local namespace cache (there is no standard SPARQL way to
	// query prefixes). It is shared with every view BindContext returns.
	ns *namespaces
}

type namespaces struct {
	mu     sync.RWMutex
	prefix map[string]term.URIRef // prefix → namespace
	uri    map[string]string      // namespace → prefix
}

// Option configures a SPARQLStore.
type Option func(*SPARQLStore)

// WithUpdate sets the SPARQL Update endpoint URL.
// If not set, write operations (Add, Remove, etc.) will return silently.
func WithUpdate(url string) Option {
	return func(s *SPARQLStore) { s.updateURL = url }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(s *SPARQLStore) { s.client = c }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(s *SPARQLStore) { s.client.Timeout = d }
}

// New creates a SPARQLStore that queries the given endpoint URL.
func New(queryURL string, opts ...Option) *SPARQLStore {
	s := &SPARQLStore{
		queryURL: queryURL,
		client:   &http.Client{Timeout: 30 * time.Second},
		ns: &namespaces{
			prefix: make(map[string]term.URIRef),
			uri:    make(map[string]string),
		},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// BindContext returns a view of the store whose requests are made with ctx. The
// view shares the endpoint, HTTP client and namespace cache with s.
func (s *SPARQLStore) BindContext(ctx context.Context) store.Store {
	v := *s
	v.ctx = ctx
	return &v
}

// requestContext is the context an HTTP request is made with.
func (s *SPARQLStore) requestContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// ContextAware reports true — remote SPARQL stores typically support named graphs.
func (s *SPARQLStore) ContextAware() bool { return true }

// TransactionAware reports false — SPARQL Protocol does not guarantee transactions.
func (s *SPARQLStore) TransactionAware() bool { return false }

// Add inserts a triple into the store.
func (s *SPARQLStore) Add(t term.Triple, ctx term.Term) {
	if s.updateURL == "" {
		return
	}
	stmt := "INSERT DATA { " + wrapGraph(ctx, tripleToSPARQL(t)) + " }"
	// Error intentionally ignored: store.Store interface does not return errors
	// from write operations. Use execUpdate directly for error handling.
	_ = s.execUpdate(s.requestContext(), stmt)
}

// AddN batch-inserts multiple quads, grouping them by context for efficient
// INSERT DATA operations. Quads with the same graph context are combined into
// a single SPARQL update request.
func (s *SPARQLStore) AddN(quads []term.Quad) {
	if s.updateURL == "" || len(quads) == 0 {
		return
	}
	// Group by context for efficient INSERT DATA.
	groups := make(map[string][]term.Quad)
	for _, q := range quads {
		key := termKeyOrDefault(q.Graph)
		groups[key] = append(groups[key], q)
	}
	for _, qs := range groups {
		var sb strings.Builder
		for _, q := range qs {
			sb.WriteString(tripleToSPARQL(q.Triple))
			sb.WriteByte(' ')
		}
		stmt := "INSERT DATA { " + wrapGraph(qs[0].Graph, sb.String()) + " }"
		// Error intentionally ignored: store.Store interface does not return errors
		// from write operations. Use execUpdate directly for error handling.
		_ = s.execUpdate(s.requestContext(), stmt)
	}
}

// Remove deletes triples matching the pattern.
func (s *SPARQLStore) Remove(pattern term.TriplePattern, ctx term.Term) {
	if s.updateURL == "" {
		return
	}
	sp := patternToSPARQL(pattern)
	stmt := "DELETE WHERE { " + wrapGraph(ctx, sp) + " }"
	// Error intentionally ignored: store.Store interface does not return errors
	// from write operations. Use execUpdate directly for error handling.
	_ = s.execUpdate(s.requestContext(), stmt)
}

// Set atomically replaces triples matching (s, p, *) with the new triple.
func (s *SPARQLStore) Set(t term.Triple, ctx term.Term) {
	if s.updateURL == "" {
		return
	}
	// DELETE then INSERT in a single update request.
	delPattern := fmt.Sprintf("%s %s ?__o .", termToSPARQL(t.Subject), termToSPARQL(t.Predicate))
	del := "DELETE WHERE { " + wrapGraph(ctx, delPattern) + " }"
	ins := "INSERT DATA { " + wrapGraph(ctx, tripleToSPARQL(t)) + " }"
	// Error intentionally ignored: store.Store interface does not return errors
	// from write operations. Use execUpdate directly for error handling.
	_ = s.execUpdate(s.requestContext(), del+" ;\n"+ins)
}

// Triples returns an iterator over matching triples.
func (s *SPARQLStore) Triples(pattern term.TriplePattern, ctx term.Term) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		sv, pv, ov := "?s", "?p", "?o"
		if pattern.Subject != nil {
			sv = termToSPARQL(pattern.Subject)
		}
		if pattern.Predicate != nil {
			pv = termToSPARQL(*pattern.Predicate)
		}
		if pattern.Object != nil {
			ov = termToSPARQL(pattern.Object)
		}
		body := fmt.Sprintf("%s %s %s .", sv, pv, ov)
		query := "SELECT ?s ?p ?o WHERE { " + wrapGraph(ctx, body) + " }"

		result, err := s.execQuery(s.requestContext(), query)
		if err != nil {
			return
		}

		for _, row := range result.Bindings {
			subj, okS := resolveVar("s", pattern.Subject, row).(term.Subject)
			pred, okP := resolveVar("p", predToTerm(pattern.Predicate), row).(term.URIRef)
			obj := resolveVar("o", pattern.Object, row)
			if !okS || !okP || obj == nil {
				continue
			}
			if !yield(term.Triple{Subject: subj, Predicate: pred, Object: obj}) {
				return
			}
		}
	}
}

// Len returns the triple count.
func (s *SPARQLStore) Len(ctx term.Term) int {
	body := "?s ?p ?o ."
	query := "SELECT (COUNT(*) AS ?c) WHERE { " + wrapGraph(ctx, body) + " }"
	result, err := s.execQuery(s.requestContext(), query)
	if err != nil || len(result.Bindings) == 0 {
		return 0
	}
	row := result.Bindings[0]
	if c, ok := row["c"]; ok {
		if lit, ok := c.(term.Literal); ok {
			n, err := strconv.Atoi(lit.Lexical())
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}

// Contexts returns an iterator over named graph URIs.
func (s *SPARQLStore) Contexts(triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term) bool) {
		body := "?s ?p ?o"
		if triple != nil {
			body = tripleToSPARQL(*triple)
			body = body[:len(body)-2] // trim " ."
		}
		query := "SELECT DISTINCT ?g WHERE { GRAPH ?g { " + body + " } }"
		result, err := s.execQuery(s.requestContext(), query)
		if err != nil {
			return
		}
		for _, row := range result.Bindings {
			if g, ok := row["g"]; ok {
				if !yield(g) {
					return
				}
			}
		}
	}
}

// Bind associates a prefix with a namespace (local cache only).
func (s *SPARQLStore) Bind(prefix string, namespace term.URIRef) {
	s.ns.mu.Lock()
	defer s.ns.mu.Unlock()
	s.ns.prefix[prefix] = namespace
	s.ns.uri[namespace.Value()] = prefix
}

// Namespace returns the namespace URI for a prefix.
func (s *SPARQLStore) Namespace(prefix string) (term.URIRef, bool) {
	s.ns.mu.RLock()
	defer s.ns.mu.RUnlock()
	ns, ok := s.ns.prefix[prefix]
	return ns, ok
}

// Prefix returns the prefix for a namespace URI.
func (s *SPARQLStore) Prefix(namespace term.URIRef) (string, bool) {
	s.ns.mu.RLock()
	defer s.ns.mu.RUnlock()
	p, ok := s.ns.uri[namespace.Value()]
	return p, ok
}

// Namespaces returns an iterator over all namespace bindings.
func (s *SPARQLStore) Namespaces() store.NamespaceIterator {
	return func(yield func(string, term.URIRef) bool) {
		s.ns.mu.RLock()
		defer s.ns.mu.RUnlock()
		for prefix, ns := range s.ns.prefix {
			if !yield(prefix, ns) {
				return
			}
		}
	}
}

// predToTerm converts *URIRef to Term (returns nil if p is nil).
func predToTerm(p *term.URIRef) term.Term {
	if p == nil {
		return nil
	}
	return *p
}

// resolveVar returns the bound value for a variable, or the fixed value if the
// pattern position was not a variable.
func resolveVar(varName string, fixed term.Term, row map[string]term.Term) term.Term {
	if fixed != nil {
		return fixed
	}
	return row[varName]
}

// termKeyOrDefault returns a string key for context grouping.
func termKeyOrDefault(ctx term.Term) string {
	if store.IsDefaultGraph(ctx) {
		return ""
	}
	return ctx.N3()
}

// Compile-time check that SPARQLStore implements store.Store.
var _ store.Store = (*SPARQLStore)(nil)
