package endpoint

import (
	"context"
	"fmt"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/term"
)

// SD is the SPARQL 1.1 Service Description namespace.
const SD = "http://www.w3.org/ns/sparql-service-description#"

var resultFormatIRIs = map[results.Format]string{
	results.FormatJSON: "http://www.w3.org/ns/formats/SPARQL_Results_JSON",
	results.FormatXML:  "http://www.w3.org/ns/formats/SPARQL_Results_XML",
	results.FormatCSV:  "http://www.w3.org/ns/formats/SPARQL_Results_CSV",
	results.FormatTSV:  "http://www.w3.org/ns/formats/SPARQL_Results_TSV",
}

// ServiceDescription returns the service description of the handler as a
// graph, with endpointIRI as the sd:endpoint. It lists the supported
// languages (sd:SPARQL11Update only when updates are enabled), every result
// and RDF format the handler writes, and the default dataset. It claims no
// sd:feature: the default graph is not the union of the named graphs, URIs are
// not dereferenced, and a dataset is not required.
//
// The graph lives in memory, so building it cannot fail; the context is only
// the one the in-memory store is asked with.
func (h *Handler) ServiceDescription(endpointIRI string) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph()
	// An in-memory store never reports an error, so add and bind cannot fail.
	must := func(err error) {
		if err != nil {
			panic(fmt.Sprintf("endpoint: building the service description: %v", err))
		}
	}
	add := func(s term.Subject, p term.URIRef, o term.Term) { must(g.Add(ctx, s, p, o)) }
	must(g.Bind(ctx, "sd", term.NewURIRefUnsafe(SD)))
	must(g.Bind(ctx, "formats", term.NewURIRefUnsafe("http://www.w3.org/ns/formats/")))
	sd := func(local string) term.URIRef { return term.NewURIRefUnsafe(SD + local) }
	rdfType := term.NewURIRefUnsafe(term.RDFNamespace + "type")

	svc := term.NewBNode()
	add(svc, rdfType, sd("Service"))
	if u, err := term.NewURIRef(endpointIRI); err == nil {
		add(svc, sd("endpoint"), u)
	}
	add(svc, sd("supportedLanguage"), sd("SPARQL11Query"))
	if !h.cfg.readOnly {
		add(svc, sd("supportedLanguage"), sd("SPARQL11Update"))
	}
	for _, f := range h.cfg.resultFormats {
		if iri, ok := resultFormatIRIs[f]; ok {
			add(svc, sd("resultFormat"), term.NewURIRefUnsafe(iri))
		}
	}
	for _, f := range rdfFormats {
		add(svc, sd("resultFormat"), term.NewURIRefUnsafe(f.formatIRI))
	}
	ds := term.NewBNode()
	dg := term.NewBNode()
	add(svc, sd("defaultDataset"), ds)
	add(ds, rdfType, sd("Dataset"))
	add(ds, sd("defaultGraph"), dg)
	add(dg, rdfType, sd("Graph"))
	return g
}

// serviceDescription answers a GET without a query.
func (x *exchange) serviceDescription() error {
	x.w.Header().Add("Vary", "Accept")
	accept := x.r.Header.Get("Accept")
	f := negotiateRDF(accept)
	if f == nil {
		return notAcceptable(accept, rdfMediaTypes())
	}
	g := x.h.ServiceDescription(x.h.requestIRI(x.r))
	return x.writeGraph(x.r.Context(), g, f)
}
