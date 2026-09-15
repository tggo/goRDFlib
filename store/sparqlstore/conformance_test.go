package sparqlstore_test

import (
	"net/http/httptest"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/sparqlstore"
	"github.com/tggo/goRDFlib/store/storetest"
)

// SPARQLStore is the closest thing in this repository to a backend maintained
// outside it: everything crosses a network boundary and nothing is indexed
// locally. Running the shared suite against it is what keeps the suite honest
// about which assumptions are really about store.Store and which are about
// having a local index.
const (
	blankNodes = "a blank node label cannot survive the SPARQL protocol: a " +
		"blank node in INSERT DATA denotes a fresh node on the server, so the " +
		"label written is never the label read back"
	blankNodeGraphs = "SPARQL cannot name a graph with a blank node (GRAPH takes " +
		"VarOrIri), so a blank-node context has no representation on the wire " +
		"and falls back to the default graph"
)

func TestConformance(t *testing.T) {
	storetest.Run(t, storetest.Config{
		// Everything listed here is a limitation of the protocol, not a bug
		// to be fixed in the store. Anything not listed has to pass. Named
		// graphs and triple terms used to be listed too, as limitations of the
		// bundled test server; it serves them since it delegates to endpoint.
		Known: map[string]string{
			"ContextConventions/a BNode context is a named graph": blankNodeGraphs,

			"TermRoundTrip/BNode subject": blankNodes,
			"TermRoundTrip/BNode object":  blankNodes,
		},
		New: func(t *testing.T) store.Store {
			g := graph.NewGraph()
			ds := &sparql.Dataset{
				Default:     g,
				NamedGraphs: make(map[string]*rdflibgo.Graph),
			}
			ts := httptest.NewServer(sparqlstore.NewServer(ds).Handler())
			t.Cleanup(ts.Close)
			return sparqlstore.New(ts.URL+"/query", sparqlstore.WithUpdate(ts.URL+"/update"))
		},
	})
}
