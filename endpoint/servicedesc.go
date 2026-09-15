package endpoint

import (
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
func (h *Handler) ServiceDescription(endpointIRI string) *graph.Graph {
	g := graph.NewGraph()
	g.Bind("sd", term.NewURIRefUnsafe(SD))
	g.Bind("formats", term.NewURIRefUnsafe("http://www.w3.org/ns/formats/"))
	sd := func(local string) term.URIRef { return term.NewURIRefUnsafe(SD + local) }
	rdfType := term.NewURIRefUnsafe(term.RDFNamespace + "type")

	svc := term.NewBNode()
	g.Add(svc, rdfType, sd("Service"))
	if u, err := term.NewURIRef(endpointIRI); err == nil {
		g.Add(svc, sd("endpoint"), u)
	}
	g.Add(svc, sd("supportedLanguage"), sd("SPARQL11Query"))
	if !h.cfg.readOnly {
		g.Add(svc, sd("supportedLanguage"), sd("SPARQL11Update"))
	}
	for _, f := range h.cfg.resultFormats {
		if iri, ok := resultFormatIRIs[f]; ok {
			g.Add(svc, sd("resultFormat"), term.NewURIRefUnsafe(iri))
		}
	}
	for _, f := range rdfFormats {
		g.Add(svc, sd("resultFormat"), term.NewURIRefUnsafe(f.formatIRI))
	}
	ds := term.NewBNode()
	dg := term.NewBNode()
	g.Add(svc, sd("defaultDataset"), ds)
	g.Add(ds, rdfType, sd("Dataset"))
	g.Add(ds, sd("defaultGraph"), dg)
	g.Add(dg, rdfType, sd("Graph"))
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
	return x.writeGraph(g, f)
}
