package sparqlstore_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store/sparqlstore"
)

// Issue #35: a query's context reaches the HTTP requests the store makes, so a
// deadline stops a request to an endpoint that does not answer, and a tracing
// transport sees the caller's span.
func TestQueryContextReachesHTTPRequests(t *testing.T) {
	type key struct{}
	seen := make(chan any, 16)
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-unblock:
		}
	}))
	defer srv.Close()
	defer close(unblock)

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen <- r.Context().Value(key{})
		return http.DefaultTransport.RoundTrip(r)
	})}
	st := sparqlstore.New(srv.URL, sparqlstore.WithHTTPClient(client))
	g := graph.NewGraph(graph.WithStore(st))

	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), key{}, "span-1"), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, err := sparql.QueryContext(ctx, g, `SELECT * WHERE { ?s ?p ?o }`)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("query took %v: the request did not carry the deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, sparql.ErrQueryCancelled) {
		t.Fatalf("err = %v, result = %v; want a cancellation, not an empty result", err, res)
	}
	select {
	case v := <-seen:
		if v != "span-1" {
			t.Fatalf("request context value = %v, want span-1", v)
		}
	default:
		t.Fatal("no request was made")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
