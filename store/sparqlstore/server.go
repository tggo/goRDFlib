package sparqlstore

import (
	"io"
	"net/http"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

// Server serves a dataset over the W3C SPARQL 1.1 Protocol at /query and
// /update, for testing SPARQLStore against a live endpoint.
//
// It is a thin wrapper around endpoint.Handler and inherits its behaviour:
// queries see the named graphs, requests are safe for concurrent use, and
// updates are answered with 204 No Content. SPARQL results are sent as XML
// unless the client asks for another format.
//
// Deprecated: use endpoint.New, which also serves the Graph Store Protocol
// and has timeouts, limits and authorization hooks.
type Server struct {
	handler *endpoint.Handler
}

// NewServer creates a Server backed by the given dataset.
//
// Deprecated: use endpoint.New.
func NewServer(ds *sparql.Dataset) *Server {
	return &Server{handler: endpoint.New(ds, serverOptions()...)}
}

// NewServerWithGraph creates a Server backed by a single default graph.
//
// Deprecated: use endpoint.NewForGraph.
func NewServerWithGraph(g *rdflibgo.Graph) *Server {
	return &Server{handler: endpoint.NewForGraph(g, serverOptions()...)}
}

// serverOptions keeps XML as the format for clients that accept anything,
// which is what the server sent before it delegated to endpoint.
func serverOptions() []endpoint.Option {
	return []endpoint.Option{
		endpoint.WithResultFormats(results.FormatXML, results.FormatJSON, results.FormatCSV, results.FormatTSV),
	}
}

// QueryHandler returns an http.Handler for the SPARQL query endpoint: GET
// with ?query=, POST application/x-www-form-urlencoded, POST
// application/sparql-query, and the service description for a GET without a
// query.
func (s *Server) QueryHandler() http.Handler {
	return s.handler.QueryHandler()
}

// UpdateHandler returns an http.Handler for the SPARQL update endpoint: POST
// application/sparql-update and POST application/x-www-form-urlencoded.
func (s *Server) UpdateHandler() http.Handler {
	return s.handler.UpdateHandler()
}

// Handler returns a combined handler that routes /query and /update paths.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/query", s.QueryHandler())
	mux.Handle("/update", s.UpdateHandler())
	return mux
}

// WriteNTriples serializes a graph to an io.Writer in N-Triples format.
// Exposed for test utilities that need to compare graph contents.
func WriteNTriples(g *rdflibgo.Graph, w io.Writer) error {
	return nt.Serialize(g, w)
}
