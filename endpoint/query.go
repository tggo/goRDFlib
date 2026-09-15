package endpoint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/store"
)

// updateKeywords start an update operation. A query request that starts with
// one is answered with a hint instead of a bare syntax error.
var updateKeywords = map[string]bool{
	"INSERT": true, "DELETE": true, "LOAD": true, "CLEAR": true, "DROP": true,
	"CREATE": true, "ADD": true, "MOVE": true, "COPY": true, "WITH": true,
}

// query evaluates a query request and writes its result.
func (x *exchange) query(req *protocolRequest) error {
	h := x.h
	base := h.baseIRI(x.r)
	shape := scanQuery(req.text, base)
	switch {
	case shape.form == "DESCRIBE":
		return httpErr(http.StatusNotImplemented, ErrDescribeUnsupported)
	case updateKeywords[shape.form]:
		return httpErrf(http.StatusBadRequest,
			"%s is an update operation: send it with POST as update= or as %s", shape.form, mtUpdate)
	}

	parsed, err := sparql.Parse(req.text)
	if err != nil {
		return httpErrf(http.StatusBadRequest, "%s", withLineColumn(err.Error(), req.text))
	}

	// Negotiate before evaluating, so that a request nobody can read is not
	// evaluated at all.
	accept := x.r.Header.Get("Accept")
	x.w.Header().Add("Vary", "Accept")
	var resultFormat results.Format
	var rdfFmt *rdfFormat
	switch parsed.Type {
	case "SELECT":
		f, ok := results.NegotiateAmong(accept, h.cfg.resultFormats...)
		if !ok {
			return notAcceptable(accept, mediaTypesOf(h.cfg.resultFormats))
		}
		resultFormat = f
	case "ASK":
		var offers []results.Format
		for _, f := range h.cfg.resultFormats {
			if f.SupportsBoolean() {
				offers = append(offers, f)
			}
		}
		f, ok := results.NegotiateAmong(accept, offers...)
		if !ok {
			return notAcceptable(accept, mediaTypesOf(offers))
		}
		resultFormat = f
	case "CONSTRUCT":
		if rdfFmt = negotiateRDF(accept); rdfFmt == nil {
			return notAcceptable(accept, rdfMediaTypes())
		}
	default:
		return httpErrf(http.StatusNotImplemented, "%s queries are not supported", parsed.Type)
	}

	q := *parsed
	if q.BaseURI == "" {
		q.BaseURI = base
	}
	maxRows := h.cfg.maxResultRows
	if maxRows > 0 && q.Type != "ASK" && (q.Limit < 0 || q.Limit > maxRows) {
		// One more than allowed tells an over-limit result from one that is
		// exactly at the limit.
		q.Limit = maxRows + 1
	}

	ctx, cancel := withTimeout(x.r.Context(), h.cfg.queryTimeout)
	defer cancel()
	res, err := x.evaluate(ctx, &q, req, shape)
	if err != nil {
		return err
	}

	switch res.Type {
	case "CONSTRUCT":
		n := 0
		if res.Graph != nil {
			n = res.Graph.Len()
		}
		x.rows = n
		if maxRows > 0 && n > maxRows {
			return tooManyRows(maxRows, "triples")
		}
		g := res.Graph
		if g == nil {
			g = graph.NewGraph()
		}
		return x.writeGraph(g, rdfFmt)
	case "ASK":
		x.rows = 1
	default:
		x.rows = len(res.Bindings)
		if maxRows > 0 && x.rows > maxRows {
			return tooManyRows(maxRows, "solutions")
		}
	}
	return x.writeResult(res, resultFormat)
}

// evaluate runs q against the dataset the request selects, under the read
// lock.
func (x *exchange) evaluate(ctx context.Context, q *sparql.ParsedQuery, req *protocolRequest, shape queryShape) (*sparql.Result, error) {
	h := x.h
	if err := h.lock.rlock(ctx); err != nil {
		return nil, evalError(ctx, err)
	}
	defer h.lock.runlock()

	def, named := h.backend.view()
	defaults, nameds := req.defaultGraphs, req.namedGraphs
	if len(defaults) == 0 && len(nameds) == 0 {
		// SPARQL 1.1 Protocol §2.1.4: without protocol parameters the dataset
		// of the query applies. The engine skips FROM, so it is built here.
		defaults, nameds = shape.from, shape.fromNamed
	}
	if len(defaults) > 0 || len(nameds) > 0 {
		var err error
		def, named, err = datasetFor(ctx, def, named, defaults, nameds)
		if err != nil {
			return nil, evalError(ctx, err)
		}
	}
	q.NamedGraphs = named

	res, err := sparql.EvalQueryContext(ctx, def, q, nil)
	if err != nil {
		if errors.Is(err, sparql.ErrQueryCancelled) {
			return nil, evalError(ctx, err)
		}
		return nil, httpErrf(http.StatusInternalServerError, "query evaluation failed: %v", err)
	}
	if res == nil {
		return nil, httpErrf(http.StatusInternalServerError, "query evaluation returned no result")
	}
	return res, nil
}

// datasetFor builds the RDF dataset named by protocol parameters or FROM
// clauses out of the service's graphs: the default graph is the union of the
// listed graphs, the named graphs are the listed ones. Graphs are never
// fetched from the Web; an IRI the service has no graph for contributes an
// empty graph. store.DefaultGraphIRI names the service's default graph.
func datasetFor(ctx context.Context, def *graph.Graph, named map[string]*graph.Graph, defaults, nameds []string) (*graph.Graph, map[string]*graph.Graph, error) {
	lookup := func(iri string) *graph.Graph {
		if iri == store.DefaultGraphIRI {
			return def
		}
		return named[iri]
	}

	var outDef *graph.Graph
	switch len(defaults) {
	case 0:
		outDef = graph.NewGraph()
	case 1:
		if outDef = lookup(defaults[0]); outDef == nil {
			outDef = graph.NewGraph()
		}
	default:
		outDef = graph.NewGraph()
		seen := make(map[string]bool, len(defaults))
		n := 0
		for _, iri := range defaults {
			g := lookup(iri)
			if g == nil || seen[iri] {
				continue
			}
			seen[iri] = true
			for t := range g.Triples(nil, nil, nil) {
				if n++; n%1024 == 0 && ctx.Err() != nil {
					return nil, nil, ctx.Err()
				}
				outDef.Add(t.Subject, t.Predicate, t.Object)
			}
		}
	}

	outNamed := make(map[string]*graph.Graph, len(nameds))
	for _, iri := range nameds {
		if g := lookup(iri); g != nil {
			outNamed[iri] = g
		}
	}
	return outDef, outNamed, nil
}

// writeResult writes a SELECT or ASK result. results.Write checks every value
// before writing the first byte, so an encoding failure is still reported
// with a proper status; after that only the connection can fail.
func (x *exchange) writeResult(res *sparql.Result, f results.Format) error {
	hdr := x.w.Header()
	hdr.Set("Content-Type", f.ContentType())
	if x.r.Method == http.MethodHead {
		x.w.WriteHeader(http.StatusOK)
		return nil
	}
	err := results.Write(x.w, f, res)
	if err == nil {
		return nil
	}
	if !x.w.wroteHeader {
		hdr.Del("Content-Type")
		return httpErrf(http.StatusInternalServerError,
			"the result cannot be written as %s: %v (request another format with the Accept header)", f.MediaType(), err)
	}
	// Written partly: the client is gone or the connection broke.
	return err
}

// writeGraph serializes g in full before sending anything, so a serializer
// error is reported with a proper status and the response has a
// Content-Length.
func (x *exchange) writeGraph(g *graph.Graph, f *rdfFormat) error {
	var buf bytes.Buffer
	if err := f.serialize(g, &buf); err != nil {
		return httpErrf(http.StatusInternalServerError,
			"the graph cannot be written as %s: %v (request another format with the Accept header)", f.mediaType, err)
	}
	hdr := x.w.Header()
	hdr.Set("Content-Type", f.contentType)
	hdr.Set("Content-Length", strconv.Itoa(buf.Len()))
	x.w.WriteHeader(http.StatusOK)
	if x.r.Method == http.MethodHead {
		return nil
	}
	_, err := x.w.Write(buf.Bytes())
	return err
}

func notAcceptable(accept string, offered []string) error {
	return httpErrf(http.StatusNotAcceptable, "none of the media types in Accept: %q can carry this result; available: %s",
		accept, strings.Join(offered, ", "))
}

func tooManyRows(limit int, what string) error {
	return httpErr(http.StatusUnprocessableEntity,
		fmt.Errorf("%w: the result has more than %d %s; add a LIMIT or narrow the query (the limit is set with WithMaxResultRows)",
			ErrTooManyRows, limit, what))
}

func mediaTypesOf(fs []results.Format) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.MediaType()
	}
	return out
}

func rdfMediaTypes() []string {
	out := make([]string, len(rdfFormats))
	for i, f := range rdfFormats {
		out[i] = f.mediaType
	}
	return out
}
