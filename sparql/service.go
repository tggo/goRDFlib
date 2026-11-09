package sparql

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// SERVICE (SPARQL 1.1 Federated Query).
//
// The engine parses SERVICE but never makes a network call on its own. A
// query is untrusted input wherever an endpoint serves it, and an endpoint
// IRI chosen by the client is a server-side request forgery (the same reason
// LOAD is off unless a loader is configured). So evaluating a SERVICE clause
// goes through a ServiceHandler the caller installs in the context with
// WithServiceHandler; without one it fails with ErrServiceUnsupported, and
// SERVICE SILENT yields the single empty solution (Federated Query §4:
// SILENT turns a failed service call into one solution with no bindings).
//
// There is deliberately no built-in HTTP handler. A handler that does call
// out is where the allow-list of endpoints, the SSRF checks, credentials and
// timeouts belong; it can use ServiceCall.Query as the request body.

// ErrServiceUnsupported is returned when a query evaluates a SERVICE clause and
// the context carries no ServiceHandler. Federated queries make network calls
// and are off by default: install a handler with WithServiceHandler (and have
// it check the endpoint against an allow-list), or write SERVICE SILENT to
// have the clause ignored.
var ErrServiceUnsupported = errors.New("sparql: SERVICE is not available: no service handler is installed; install one with WithServiceHandler, or use SERVICE SILENT to ignore the clause")

// ErrServiceEndpoint is returned when the endpoint of a SERVICE clause is a
// variable that is unbound where the clause is evaluated, or is bound to
// something that is not an IRI. Bind the variable in a pattern that is joined
// before the SERVICE clause (or with VALUES), or write the IRI.
var ErrServiceEndpoint = errors.New("sparql: SERVICE endpoint is not a bound IRI")

// ErrServiceFailed wraps the error a ServiceHandler returned for a SERVICE
// clause that is not SILENT, so a caller can tell a failing remote service
// (a gateway problem) from a bad query or a store failure.
var ErrServiceFailed = errors.New("sparql: SERVICE call failed")

// ServiceCall is one request to a ServiceHandler.
type ServiceCall struct {
	// Endpoint is the IRI of the service.
	Endpoint rdflibgo.URIRef
	// Silent is SERVICE SILENT. The engine applies it to any error the
	// handler returns; the handler need not.
	Silent bool
	// Pattern is the parsed group graph pattern of the clause.
	Pattern Pattern
	// Prefixes and BaseURI are those of the query the clause is in.
	Prefixes map[string]string
	BaseURI  string
	// Query is a complete "SELECT * WHERE { ... }" query for the service,
	// with the query's PREFIX and BASE declarations in front: what a handler
	// that calls a remote endpoint sends.
	Query string
	// Input holds the bindings of the row the clause is evaluated for: the
	// row of the enclosing group to its left, when there is one (nil when the
	// clause starts the group). A handler may use it to restrict the request;
	// it must not rely on it, because the engine joins the returned rows with
	// the enclosing solutions anyway. It must not be modified.
	Input map[string]rdflibgo.Term
}

// Select returns the clause as a SELECT * query that can be evaluated with
// EvalQuery against a local graph, which is how a handler serves a service
// from data it holds.
func (c *ServiceCall) Select() *ParsedQuery {
	return &ParsedQuery{
		Type:     "SELECT",
		Where:    c.Pattern,
		Limit:    -1,
		Prefixes: c.Prefixes,
		BaseURI:  c.BaseURI,
	}
}

// ServiceHandler answers a SERVICE clause: it returns the solutions of the
// clause's pattern at the endpoint, one map per solution. The rows are not
// modified or retained by the engine beyond the query. An error ends the query
// unless the clause is SILENT; return a context error unchanged so that
// cancellation is reported as such. A handler is called from the goroutine
// that evaluates the query, once per row of the enclosing group when the
// endpoint is a variable or the clause cannot be evaluated once.
type ServiceHandler func(ctx context.Context, call *ServiceCall) ([]map[string]rdflibgo.Term, error)

type serviceHandlerKey struct{}

// WithServiceHandler returns a context whose queries and updates answer
// SERVICE clauses with h. A nil h removes a handler installed further out.
func WithServiceHandler(ctx context.Context, h ServiceHandler) context.Context {
	return context.WithValue(ctx, serviceHandlerKey{}, h)
}

func serviceHandlerFrom(ctx context.Context) ServiceHandler {
	h, _ := ctx.Value(serviceHandlerKey{}).(ServiceHandler)
	return h
}

// evalService evaluates a SERVICE clause for the row in (nil: none) and
// returns its solutions, which the caller joins with the enclosing ones.
func evalService(ec *evalCtx, sp *ServicePattern, in map[string]rdflibgo.Term, prefixes map[string]string) []map[string]rdflibgo.Term {
	if ec.stop() {
		return nil
	}
	rows, err := callService(ec, sp, in, prefixes)
	if err == nil {
		return rows
	}
	if ec.poll() || ec.context().Err() != nil {
		// Cancellation is never silenced: SILENT ignores a failing service,
		// not a query that was told to stop.
		ec.fail(err)
		return nil
	}
	if sp.Silent {
		// SPARQL 1.1 Federated Query §4: SERVICE SILENT ignores errors and
		// returns one solution with no bindings.
		return []map[string]rdflibgo.Term{{}}
	}
	ec.fail(err)
	return nil
}

func callService(ec *evalCtx, sp *ServicePattern, in map[string]rdflibgo.Term, prefixes map[string]string) ([]map[string]rdflibgo.Term, error) {
	ctx := ec.context()
	h := serviceHandlerFrom(ctx)
	if h == nil {
		return nil, ErrServiceUnsupported
	}
	var ep rdflibgo.Term
	endpointVar := ""
	if strings.HasPrefix(sp.Endpoint, "?") {
		endpointVar = sp.Endpoint[1:]
		ep = in[endpointVar]
		if ep == nil {
			ep = ec.pre[endpointVar]
		}
	} else {
		ep = resolveTermRef(sp.Endpoint, prefixes)
	}
	u, ok := ep.(rdflibgo.URIRef)
	if !ok {
		if endpointVar != "" {
			return nil, fmt.Errorf("%w: ?%s is %s at SERVICE", ErrServiceEndpoint, endpointVar, describeBinding(ep))
		}
		return nil, fmt.Errorf("%w: %s", ErrServiceEndpoint, sp.Endpoint)
	}
	call := &ServiceCall{
		Endpoint: u,
		Silent:   sp.Silent,
		Pattern:  sp.Pattern,
		Prefixes: userPrefixes(prefixes),
		BaseURI:  prefixes[baseURIKey],
		Input:    in,
	}
	call.Query = serviceQueryText(call, sp.Text)
	rows, err := h(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("%w: <%s>: %w", ErrServiceFailed, u.Value(), err)
	}
	out := make([]map[string]rdflibgo.Term, 0, len(rows))
	for _, r := range rows {
		if ec.stop() || ec.produced(1) {
			return nil, ec.failure()
		}
		nr := copyBindings(r)
		if endpointVar != "" {
			if old, bound := nr[endpointVar]; bound && old.N3() != u.N3() {
				continue
			}
			nr[endpointVar] = u
		}
		out = append(out, nr)
	}
	return out, nil
}

func describeBinding(t rdflibgo.Term) string {
	if t == nil {
		return "unbound"
	}
	return "bound to " + t.N3()
}

// userPrefixes copies the PREFIX declarations of a query, leaving out the
// engine's own bookkeeping entries.
func userPrefixes(prefixes map[string]string) map[string]string {
	out := make(map[string]string, len(prefixes))
	for k, v := range prefixes {
		if strings.HasPrefix(k, "__") && strings.HasSuffix(k, "__") {
			continue
		}
		out[k] = v
	}
	return out
}

// serviceQueryText writes the query a remote service is sent.
func serviceQueryText(c *ServiceCall, group string) string {
	var b strings.Builder
	if c.BaseURI != "" {
		fmt.Fprintf(&b, "BASE <%s>\n", c.BaseURI)
	}
	names := make([]string, 0, len(c.Prefixes))
	for k := range c.Prefixes {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(&b, "PREFIX %s: <%s>\n", k, c.Prefixes[k])
	}
	b.WriteString("SELECT * WHERE ")
	b.WriteString(group)
	return b.String()
}
