package sparqlstore

import (
	"context"
	"errors"
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
// Every request is made with the context of the method that causes it, so it
// carries that context's deadline and cancellation, and whatever a tracing
// http.RoundTripper reads from it. WithTimeout's limit applies on top. A
// network failure, a non-2xx answer or an unreadable result is the method's
// error: nothing is swallowed.
type SPARQLStore struct {
	queryURL  string
	updateURL string
	client    *http.Client

	// ns is the local namespace cache (there is no standard SPARQL way to
	// query prefixes).
	ns *namespaces
}

// ErrNoUpdateEndpoint is returned by every write when the store was created
// without WithUpdate: a SPARQL query endpoint cannot be written to.
var ErrNoUpdateEndpoint = errors.New("sparqlstore: no update endpoint configured (set one with WithUpdate)")

type namespaces struct {
	mu     sync.RWMutex
	prefix map[string]term.URIRef // prefix → namespace
	uri    map[string]string      // namespace → prefix
}

// Option configures a SPARQLStore.
type Option func(*SPARQLStore)

// WithUpdate sets the SPARQL Update endpoint URL.
// If not set, write operations (Add, Remove, etc.) return ErrNoUpdateEndpoint.
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

// ContextAware reports true — remote SPARQL stores typically support named graphs.
func (s *SPARQLStore) ContextAware() bool { return true }

// TransactionAware reports false — SPARQL Protocol does not guarantee transactions.
func (s *SPARQLStore) TransactionAware() bool { return false }

// writable refuses a write that cannot be sent: a done context, or a store
// without an update endpoint.
func (s *SPARQLStore) writable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.updateURL == "" {
		return ErrNoUpdateEndpoint
	}
	return nil
}

// Add inserts a triple into the store.
func (s *SPARQLStore) Add(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := s.writable(ctx); err != nil {
		return err
	}
	stmt := "INSERT DATA { " + wrapGraph(graph, tripleToSPARQL(t)) + " }"
	return s.execUpdate(ctx, stmt)
}

// AddN batch-inserts multiple quads, grouping them by context for efficient
// INSERT DATA operations. Quads with the same graph context are combined into
// a single SPARQL update request. A failure stops the batch; groups sent
// before it stay inserted.
func (s *SPARQLStore) AddN(ctx context.Context, quads []term.Quad) error {
	if err := s.writable(ctx); err != nil {
		return err
	}
	if len(quads) == 0 {
		return nil
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
		if err := s.execUpdate(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Remove deletes triples matching the pattern.
func (s *SPARQLStore) Remove(ctx context.Context, pattern term.TriplePattern, graph term.Term) error {
	if err := s.writable(ctx); err != nil {
		return err
	}
	sp := patternToSPARQL(pattern)
	stmt := "DELETE WHERE { " + wrapGraph(graph, sp) + " }"
	return s.execUpdate(ctx, stmt)
}

// Set replaces triples matching (s, p, *) with the new triple, in a single
// update request.
func (s *SPARQLStore) Set(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := s.writable(ctx); err != nil {
		return err
	}
	// DELETE then INSERT in a single update request.
	delPattern := fmt.Sprintf("%s %s ?__o .", termToSPARQL(t.Subject), termToSPARQL(t.Predicate))
	del := "DELETE WHERE { " + wrapGraph(graph, delPattern) + " }"
	ins := "INSERT DATA { " + wrapGraph(graph, tripleToSPARQL(t)) + " }"
	return s.execUpdate(ctx, del+" ;\n"+ins)
}

// Triples returns an iterator over matching triples. The query is sent when
// iteration starts; a failure is yielded once as the iterator's error.
func (s *SPARQLStore) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(term.Triple{}, err)
			return
		}
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
		query := "SELECT ?s ?p ?o WHERE { " + wrapGraph(graph, body) + " }"

		result, err := s.execQuery(ctx, query)
		if err != nil {
			yield(term.Triple{}, err)
			return
		}

		for _, row := range result.Bindings {
			subj, okS := resolveVar("s", pattern.Subject, row).(term.Subject)
			pred, okP := resolveVar("p", predToTerm(pattern.Predicate), row).(term.URIRef)
			obj := resolveVar("o", pattern.Object, row)
			if !okS || !okP || obj == nil {
				continue
			}
			if !yield(term.Triple{Subject: subj, Predicate: pred, Object: obj}, nil) {
				return
			}
		}
	}
}

// Len returns the triple count.
func (s *SPARQLStore) Len(ctx context.Context, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	body := "?s ?p ?o ."
	query := "SELECT (COUNT(*) AS ?c) WHERE { " + wrapGraph(graph, body) + " }"
	result, err := s.execQuery(ctx, query)
	if err != nil {
		return 0, err
	}
	if len(result.Bindings) == 0 {
		return 0, fmt.Errorf("sparqlstore: count query returned no row")
	}
	lit, ok := result.Bindings[0]["c"].(term.Literal)
	if !ok {
		return 0, fmt.Errorf("sparqlstore: count query returned no ?c value")
	}
	n, err := strconv.Atoi(lit.Lexical())
	if err != nil {
		return 0, fmt.Errorf("sparqlstore: count %q is not an integer: %w", lit.Lexical(), err)
	}
	return n, nil
}

// Contexts returns an iterator over named graph URIs.
func (s *SPARQLStore) Contexts(ctx context.Context, triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		body := "?s ?p ?o"
		if triple != nil {
			body = tripleToSPARQL(*triple)
			body = body[:len(body)-2] // trim " ."
		}
		query := "SELECT DISTINCT ?g WHERE { GRAPH ?g { " + body + " } }"
		result, err := s.execQuery(ctx, query)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, row := range result.Bindings {
			if g, ok := row["g"]; ok {
				if !yield(g, nil) {
					return
				}
			}
		}
	}
}

// Bind associates a prefix with a namespace (local cache only).
func (s *SPARQLStore) Bind(ctx context.Context, prefix string, namespace term.URIRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.ns.mu.Lock()
	defer s.ns.mu.Unlock()
	s.ns.prefix[prefix] = namespace
	s.ns.uri[namespace.Value()] = prefix
	return nil
}

// Namespace returns the namespace URI for a prefix.
func (s *SPARQLStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if err := ctx.Err(); err != nil {
		return term.URIRef{}, false, err
	}
	s.ns.mu.RLock()
	defer s.ns.mu.RUnlock()
	ns, ok := s.ns.prefix[prefix]
	return ns, ok, nil
}

// Prefix returns the prefix for a namespace URI.
func (s *SPARQLStore) Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	s.ns.mu.RLock()
	defer s.ns.mu.RUnlock()
	p, ok := s.ns.uri[namespace.Value()]
	return p, ok, nil
}

// Namespaces returns an iterator over all namespace bindings.
func (s *SPARQLStore) Namespaces(ctx context.Context) store.NamespaceIterator {
	return func(yield func(store.NamespaceBinding, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(store.NamespaceBinding{}, err)
			return
		}
		s.ns.mu.RLock()
		defer s.ns.mu.RUnlock()
		for prefix, ns := range s.ns.prefix {
			if !yield(store.NamespaceBinding{Prefix: prefix, Namespace: ns}, nil) {
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
