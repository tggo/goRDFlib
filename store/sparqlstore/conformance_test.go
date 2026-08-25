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
	namedGraphs = "the bundled test server queries ds.Default only, so named " +
		"graphs are not readable back over the protocol"
	blankNodes = "a blank node label cannot survive the SPARQL protocol: a " +
		"blank node in INSERT DATA denotes a fresh node on the server, so the " +
		"label written is never the label read back"
	tripleTerms = "the bundled test server does not round-trip RDF 1.2 triple " +
		"terms through the SPARQL results formats"
)

func TestConformance(t *testing.T) {
	storetest.Run(t, storetest.Config{
		// Everything listed here is a limitation of the protocol or of the
		// bundled test server, not a bug to be fixed in the store. Anything not
		// listed has to pass.
		Known: map[string]string{
			"Add/the same triple in two graphs is two triples": namedGraphs,
			"Remove/stays inside its graph":                    namedGraphs,
			"Triples/is scoped to its graph":                   namedGraphs,
			"Contexts":                                         namedGraphs,

			"TermRoundTrip/BNode subject": blankNodes,
			"TermRoundTrip/BNode object":  blankNodes,

			"TermRoundTrip/triple term object":  tripleTerms,
			"TermRoundTrip/triple term subject": tripleTerms,
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
