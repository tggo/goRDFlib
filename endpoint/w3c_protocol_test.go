package endpoint_test

import (
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// The W3C SPARQL 1.1 Protocol tests
// (testdata/w3c/rdf-tests/sparql/sparql11/protocol/manifest.ttl) describe HTTP
// exchanges in prose. Each one is transcribed below as a subtest with the
// manifest entry's name. The manifest assumes a service that holds the
// kasei.us data*.rdf graphs; kaseiDataset stands in for them.

const kasei = "http://kasei.us/2009/09/sparql/data/"

func kaseiDataset() *sparql.Dataset {
	foafDocument := iri("http://xmlns.com/foaf/0.1/Document")
	rdfType := iri(term.RDFNamespace + "type")
	ds := &sparql.Dataset{Default: graph.NewGraph(), NamedGraphs: map[string]*graph.Graph{}}
	for _, name := range []string{"data1.rdf", "data2.rdf", "data3.rdf"} {
		g := graph.NewGraph()
		g.Add(iri(kasei+name), rdfType, foafDocument)
		ds.NamedGraphs[kasei+name] = g
	}
	// So that query_multiple_dataset can tell the protocol dataset from the
	// FROM clause: only data2.rdf says something about data1.rdf besides
	// data1.rdf itself.
	ds.NamedGraphs[kasei+"data2.rdf"].Add(iri(kasei+"data1.rdf"), iri("http://www.w3.org/2000/01/rdf-schema#seeAlso"), iri(kasei+"data2.rdf"))
	return ds
}

func is2xxOr3xx(t *testing.T, r response) {
	t.Helper()
	if r.status < 200 || r.status >= 400 {
		t.Fatalf("status = %d, want 2xx or 3xx; body: %s", r.status, r.body)
	}
}

func is4xx(t *testing.T, r response) {
	t.Helper()
	if r.status < 400 || r.status >= 500 {
		t.Fatalf("status = %d, want 4xx; body: %s", r.status, r.body)
	}
}

func resultContentType(t *testing.T, r response, allowed ...string) {
	t.Helper()
	ct := r.header.Get("Content-Type")
	for _, a := range allowed {
		if strings.HasPrefix(ct, a) {
			return
		}
	}
	t.Fatalf("Content-Type = %q, want one of %v", ct, allowed)
}

var srxJSON = []string{"application/sparql-results+xml", "application/sparql-results+json"}

func TestW3CProtocol(t *testing.T) {
	newTS := func(t *testing.T) string {
		// The W3C server runs at /sparql; the base IRI makes relative IRIs in
		// the tests resolve the way the kasei.us data expects.
		mux := http.NewServeMux()
		mux.Handle("/sparql", endpoint.New(kaseiDataset(), endpoint.WithBaseIRI(kasei)))
		mux.Handle("/sparql/", endpoint.New(kaseiDataset(), endpoint.WithBaseIRI(kasei)))
		return httptestServer(t, mux) + "/sparql"
	}
	askTrue := func(t *testing.T, r response) {
		t.Helper()
		is2xxOr3xx(t, r)
		resultContentType(t, r, srxJSON...)
		if !parseResult(t, r).AskResult {
			t.Fatalf("ASK returned false: %s", r.body)
		}
	}
	// two runs an update and then the ASK that checks it, on one server.
	two := func(t *testing.T, u string, update string, ask string) {
		t.Helper()
		is2xxOr3xx(t, postUpdate(t, u, update))
		r := postQuery(t, strings.SplitN(u, "?", 2)[0], ask, "Accept", "application/sparql-results+xml")
		askTrue(t, r)
		resultContentType(t, r, "application/sparql-results+xml")
	}
	const prefixes = "PREFIX dc: <http://purl.org/dc/terms/>\nPREFIX foaf: <http://xmlns.com/foaf/0.1/>\n"
	const insertData3 = `CLEAR ALL ;
INSERT DATA {
    GRAPH <http://kasei.us/2009/09/sparql/data/data1.rdf> { <http://kasei.us/2009/09/sparql/data/data1.rdf> a foaf:Document }
    GRAPH <http://kasei.us/2009/09/sparql/data/data2.rdf> { <http://kasei.us/2009/09/sparql/data/data2.rdf> a foaf:Document }
    GRAPH <http://kasei.us/2009/09/sparql/data/data3.rdf> { <http://kasei.us/2009/09/sparql/data/data3.rdf> a foaf:Document }
} ;
`
	d1 := url.QueryEscape(kasei + "data1.rdf")
	d2 := url.QueryEscape(kasei + "data2.rdf")
	d3 := url.QueryEscape(kasei + "data3.rdf")

	tests := map[string]func(t *testing.T, u string){
		"query_post_form": func(t *testing.T, u string) {
			askTrue(t, do(t, newReq(t, "POST", u+"/", "application/x-www-form-urlencoded", "query=ASK%20%7B%7D")))
		},
		"query_dataset_default_graphs_get": func(t *testing.T, u string) {
			askTrue(t, get(t, u+"?query=ASK%20%7B%20%3Chttp%3A%2F%2Fkasei.us%2F2009%2F09%2Fsparql%2Fdata%2Fdata1.rdf%3E%20a%20%3Ftype%20.%20%3Chttp%3A%2F%2Fkasei.us%2F2009%2F09%2Fsparql%2Fdata%2Fdata2.rdf%3E%20a%20%3Ftype%20.%20%7D&default-graph-uri="+d1+"&default-graph-uri="+d2, nil))
		},
		"query_dataset_default_graphs_post": func(t *testing.T, u string) {
			askTrue(t, postQuery(t, u+"/?default-graph-uri="+d1+"&default-graph-uri="+d2,
				`ASK { <http://kasei.us/2009/09/sparql/data/data1.rdf> ?p ?o . <http://kasei.us/2009/09/sparql/data/data2.rdf> ?p ?o }`))
		},
		"query_dataset_named_graphs_post": func(t *testing.T, u string) {
			askTrue(t, postQuery(t, u+"/?named-graph-uri="+d1+"&named-graph-uri="+d2, `ASK { GRAPH ?g { ?s ?p ?o } }`))
		},
		"query_dataset_named_graphs_get": func(t *testing.T, u string) {
			askTrue(t, get(t, u+"/?named-graph-uri="+d1+"&named-graph-uri="+d2+"&query=ASK%20%7B%20GRAPH%20%3Fg%20%7B%20%3Fs%20%3Fp%20%3Fo%20%7D%20%7D", nil))
		},
		"query_dataset_full": func(t *testing.T, u string) {
			r := postQuery(t, u+"/?default-graph-uri="+d3+"&named-graph-uri="+d1+"&named-graph-uri="+d2,
				`SELECT ?g ?x ?s { ?x ?y ?o  GRAPH ?g { ?s ?p ?o } }`)
			is2xxOr3xx(t, r)
			resultContentType(t, r, srxJSON...)
			// The manifest only says "true"; check the dataset actually used:
			// ?x is data3.rdf (default), ?g ranges over data1 and data2.
			res := parseResult(t, r)
			gs := map[string]bool{}
			for _, b := range res.Bindings {
				if b["x"].String() != kasei+"data3.rdf" {
					t.Fatalf("?x = %v, want data3.rdf from the default graph", b["x"])
				}
				gs[b["g"].String()] = true
			}
			if len(gs) != 2 || !gs[kasei+"data1.rdf"] || !gs[kasei+"data2.rdf"] {
				t.Fatalf("?g = %v, want data1.rdf and data2.rdf", gs)
			}
		},
		"query_multiple_dataset": func(t *testing.T, u string) {
			askTrue(t, postQuery(t, u+"/?default-graph-uri="+d2, `ASK FROM <http://kasei.us/2009/09/sparql/data/data1.rdf> { <data1.rdf> ?p ?o }`))
			// Discriminating variant: true only with the protocol dataset.
			askTrue(t, postQuery(t, u+"/?default-graph-uri="+d2, `ASK FROM <http://kasei.us/2009/09/sparql/data/data1.rdf> { <data1.rdf> ?p <data2.rdf> }`))
		},
		"query_get": func(t *testing.T, u string) {
			askTrue(t, get(t, u+"?query=ASK%20%7B%7D", nil))
		},
		"query_content_type_select": func(t *testing.T, u string) {
			r := postQuery(t, u+"/", `SELECT (1 AS ?value) {}`)
			is2xxOr3xx(t, r)
			resultContentType(t, r, "application/sparql-results+xml", "application/sparql-results+json", "text/tab-separated-values", "text/csv")
		},
		"query_content_type_ask": func(t *testing.T, u string) {
			r := postQuery(t, u+"/", `ASK {}`)
			is2xxOr3xx(t, r)
			resultContentType(t, r, srxJSON...)
		},
		"query_content_type_describe": func(t *testing.T, u string) {
			// The engine has no DESCRIBE: 501 with a message, not a 2xx.
			r := postQuery(t, u+"/", `DESCRIBE <http://example.org/>`)
			if r.status != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501 (DESCRIBE unsupported)", r.status)
			}
		},
		"query_content_type_construct": func(t *testing.T, u string) {
			r := postQuery(t, u+"/", `CONSTRUCT { <s> <p> 1 } WHERE {}`)
			is2xxOr3xx(t, r)
			resultContentType(t, r, "application/rdf+xml", "application/rdf+json", "text/turtle")
		},
		"update_dataset_default_graph": func(t *testing.T, u string) {
			two(t, u+"?using-graph-uri="+d1, prefixes+`CLEAR ALL ;
INSERT DATA {
    GRAPH <http://kasei.us/2009/09/sparql/data/data1.rdf> {
        <http://kasei.us/2009/09/sparql/data/data1.rdf> a foaf:Document
    }
} ;
INSERT {
    GRAPH <http://example.org/protocol-update-dataset-test/> {
        ?s a dc:BibliographicResource
    }
}
WHERE {
    ?s a foaf:Document
}`, `ASK {
    GRAPH <http://example.org/protocol-update-dataset-test/> {
        <http://kasei.us/2009/09/sparql/data/data1.rdf> a <http://purl.org/dc/terms/BibliographicResource>
    }
}`)
		},
		"update_dataset_default_graphs": func(t *testing.T, u string) {
			two(t, u+"?using-graph-uri="+d1+"&using-graph-uri="+d2, prefixes+insertData3+`INSERT {
    GRAPH <http://example.org/protocol-update-dataset-graphs-test/> {
        ?s a dc:BibliographicResource
    }
}
WHERE {
    ?s a foaf:Document
}`, `ASK {
    GRAPH <http://example.org/protocol-update-dataset-graphs-test/> {
        <http://kasei.us/2009/09/sparql/data/data1.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
        <http://kasei.us/2009/09/sparql/data/data2.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
    }
    FILTER NOT EXISTS {
        GRAPH <http://example.org/protocol-update-dataset-graphs-test/> {
            <http://kasei.us/2009/09/sparql/data/data3.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
        }
    }
}`)
		},
		"update_dataset_named_graphs": func(t *testing.T, u string) {
			two(t, u+"?using-named-graph-uri="+d1+"&using-named-graph-uri="+d2, prefixes+insertData3+`INSERT {
    GRAPH <http://example.org/protocol-update-dataset-named-graphs-test/> {
        ?s a dc:BibliographicResource
    }
}
WHERE {
    GRAPH ?g {
        ?s a foaf:Document
    }
}`, `ASK {
    GRAPH <http://example.org/protocol-update-dataset-named-graphs-test/> {
        <http://kasei.us/2009/09/sparql/data/data1.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
        <http://kasei.us/2009/09/sparql/data/data2.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
    }
    FILTER NOT EXISTS {
        GRAPH <http://example.org/protocol-update-dataset-named-graphs-test/> {
            <http://kasei.us/2009/09/sparql/data/data3.rdf> a <http://purl.org/dc/terms/BibliographicResource> .
        }
    }
}`)
		},
		"update_dataset_full": func(t *testing.T, u string) {
			two(t, u+"?using-graph-uri="+d1+"&using-named-graph-uri="+d2, prefixes+insertData3+`INSERT {
    GRAPH <http://example.org/protocol-update-dataset-full-test/> {
        ?s <http://example.org/in> ?in
    }
}
WHERE {
    {
        GRAPH ?g { ?s a foaf:Document }
        BIND(?g AS ?in)
    }
    UNION
    {
        ?s a foaf:Document .
        BIND("default" AS ?in)
    }
}`, `ASK {
    GRAPH <http://example.org/protocol-update-dataset-full-test/> {
        <http://kasei.us/2009/09/sparql/data/data1.rdf> <http://example.org/in> "default" .
        <http://kasei.us/2009/09/sparql/data/data2.rdf> <http://example.org/in> <http://kasei.us/2009/09/sparql/data/data2.rdf> .
    }
    FILTER NOT EXISTS {
        GRAPH <http://example.org/protocol-update-dataset-full-test/> {
            <http://kasei.us/2009/09/sparql/data/data3.rdf> ?p ?o
        }
    }
}`)
		},
		"update_post_form": func(t *testing.T, u string) {
			is2xxOr3xx(t, do(t, newReq(t, "POST", u+"/", "application/x-www-form-urlencoded", "update=CLEAR%20ALL")))
		},
		"update_post_direct": func(t *testing.T, u string) {
			is2xxOr3xx(t, postUpdate(t, u+"/", "CLEAR ALL"))
		},
		"update_base_uri": func(t *testing.T, u string) {
			is2xxOr3xx(t, postUpdate(t, u+"/", `CLEAR GRAPH <http://example.org/protocol-base-test/> ;
INSERT DATA { GRAPH <http://example.org/protocol-base-test/> { <http://example.org/s> <http://example.org/p> <test> } }`))
			r := postQuery(t, u+"/", `SELECT ?o WHERE {
    GRAPH <http://example.org/protocol-base-test/> {
        <http://example.org/s> <http://example.org/p> ?o
    }
}`, "Accept", "application/sparql-results+xml")
			is2xxOr3xx(t, r)
			resultContentType(t, r, "application/sparql-results+xml")
			res := parseResult(t, r)
			if len(res.Bindings) != 1 {
				t.Fatalf("got %d results, want one", len(res.Bindings))
			}
			o, ok := res.Bindings[0]["o"].(term.URIRef)
			if !ok || o.Value() == "test" || !strings.HasSuffix(o.Value(), "/test") {
				t.Fatalf("?o = %v, want an absolute IRI resolved from <test>", res.Bindings[0]["o"])
			}
		},
		"query_post_direct": func(t *testing.T, u string) {
			askTrue(t, postQuery(t, u+"/", `ASK {}`))
		},
		"bad_query_method": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "PUT", u+"?query=ASK%20%7B%7D", "", "")))
		},
		"bad_multiple_queries": func(t *testing.T, u string) {
			is4xx(t, get(t, u+"?query=ASK%20%7B%7D&query=SELECT%20%2A%20%7B%7D", nil))
		},
		"bad_query_wrong_media_type": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "text/plain", "ASK {}")))
		},
		"bad_query_missing_form_type": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "", "query=ASK%20%7B%7D")))
		},
		"bad_query_missing_direct_type": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "", "ASK {}")))
		},
		"bad_query_non_utf8": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "application/sparql-query; charset=UTF-16", utf16("ASK {}"))))
		},
		"bad_query_syntax": func(t *testing.T, u string) {
			is4xx(t, get(t, u+"?query=ASK%20%7B", nil))
		},
		"bad_update_get": func(t *testing.T, u string) {
			is4xx(t, get(t, u+"?update=CLEAR%20ALL", nil))
		},
		"bad_multiple_updates": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "application/x-www-form-urlencoded", "update=CLEAR%20NAMED&update=CLEAR%20DEFAULT")))
		},
		"bad_update_wrong_media_type": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "text/plain", "CLEAR NAMED")))
		},
		"bad_update_missing_form_type": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "", "update=CLEAR%20NAMED")))
		},
		"bad_update_non_utf8": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "application/sparql-update; charset=UTF-16", utf16("CLEAR NAMED"))))
		},
		"bad_update_syntax": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "application/x-www-form-urlencoded", "update=CLEAR%20XYZ")))
		},
		"bad_update_dataset_conflict": func(t *testing.T, u string) {
			is4xx(t, do(t, newReq(t, "POST", u+"/", "application/x-www-form-urlencoded", "using-named-graph-uri=http%3A%2F%2Fexample%2Fpeople&update=%09%09PREFIX%20foaf%3A%20%20%3Chttp%3A%2F%2Fxmlns.com%2Ffoaf%2F0.1%2F%3E%0A%09%09WITH%20%3Chttp%3A%2F%2Fexample%2Faddresses%3E%0A%09%09DELETE%20%7B%20%3Fperson%20foaf%3AgivenName%20%27Bill%27%20%7D%0A%09%09INSERT%20%7B%20%3Fperson%20foaf%3AgivenName%20%27William%27%20%7D%0A%09%09WHERE%20%7B%0A%09%09%09%3Fperson%20foaf%3AgivenName%20%27Bill%27%0A%09%09%7D%0A")))
		},
	}

	for _, name := range manifestEntries(t, "protocol") {
		fn, ok := tests[name]
		if !ok {
			t.Errorf("manifest entry %s has no transcription", name)
			continue
		}
		t.Run(name, func(t *testing.T) { fn(t, newTS(t)) })
	}
	if len(tests) != len(manifestEntries(t, "protocol")) {
		t.Errorf("%d transcriptions for %d manifest entries", len(tests), len(manifestEntries(t, "protocol")))
	}
}

func utf16(s string) string {
	var b strings.Builder
	b.WriteString("\xff\xfe")
	for _, c := range s {
		b.WriteByte(byte(c))
		b.WriteByte(0)
	}
	return b.String()
}

var entryRe = regexp.MustCompile(`(?s)mf:entries\s*\((.*?)\)`)

// manifestEntries lists the local names of the entries of a W3C manifest in
// sparql11/<dir>, so that a test added upstream shows up as a failure.
func manifestEntries(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile("../testdata/w3c/rdf-tests/sparql/sparql11/" + dir + "/manifest.ttl")
	if err != nil {
		t.Skipf("W3C test suite not checked out (git submodule update --init): %v", err)
	}
	m := entryRe.FindSubmatch(data)
	if m == nil {
		t.Fatal("no mf:entries in manifest")
	}
	var out []string
	for _, f := range strings.Fields(string(m[1])) {
		_, local, _ := strings.Cut(f, ":")
		out = append(out, local)
	}
	return out
}
