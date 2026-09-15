package endpoint_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/term"
)

func Example() {
	g := graph.NewGraph()
	g.Add(term.NewURIRefUnsafe("http://example.org/alice"),
		term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/name"), term.NewLiteral("Alice"))

	h := endpoint.New(&sparql.Dataset{Default: g},
		endpoint.WithQueryTimeout(30*time.Second),
		endpoint.WithMaxResultRows(100_000),
		endpoint.WithLogger(slog.Default()),
		endpoint.WithAuthorizer(func(r *http.Request, op endpoint.Operation) error {
			if op == endpoint.OpUpdate || op == endpoint.OpGraphWrite {
				if r.Header.Get("Authorization") != "Bearer secret" {
					return errors.New("writes need a token")
				}
			}
			return nil
		}),
		endpoint.WithRequestHook(func(r *http.Request, info endpoint.RequestInfo) {
			// e.g. a Prometheus histogram by info.Op and info.Status
		}),
	)

	mux := http.NewServeMux()
	mux.Handle("/sparql", h)
	mux.Handle("/rdf-graphs/", http.StripPrefix("/rdf-graphs", h.GraphStoreHandler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	q := url.Values{"query": {`SELECT ?name WHERE { ?s <http://xmlns.com/foaf/0.1/name> ?name }`}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/sparql?"+q.Encode(), nil)
	req.Header.Set("Accept", results.MediaTypeCSV)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, resp.Header.Get("Content-Type"))
	fmt.Print(strings.ReplaceAll(string(body), "\r\n", "\n"))

	resp, _ = http.Post(srv.URL+"/sparql", "application/sparql-update", strings.NewReader(`CLEAR ALL`))
	resp.Body.Close()
	fmt.Println(resp.StatusCode)
	// Output:
	// 200 text/csv; charset=utf-8
	// name
	// Alice
	// 403
}
