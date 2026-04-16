package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestConstructNestedBlankNodePropertyList(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:details [ ex:contact [ ex:name ?name ; a ex:Name ] ;
			                a ex:Contact
			              ]
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}

	// 5 triples per person (Alice, Bob, Charlie) = 15 total:
	//   ?s ex:details _:b1  .
	//   _:b1 ex:contact _:b2 .
	//   _:b1 a ex:Contact .
	//   _:b2 ex:name ?name .
	//   _:b2 a ex:Name .
	if r.Graph.Len() != 15 {
		t.Errorf("expected 15 triples, got %d", r.Graph.Len())
	}

	// Verify Alice has an ex:details link to something
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	details, _ := rdflibgo.NewURIRef("http://example.org/details")
	found := false
	r.Graph.Triples(alice, &details, nil)(func(_ rdflibgo.Triple) bool {
		found = true
		return false
	})
	if !found {
		t.Error("expected Alice to have ex:details triple")
	}
}

func TestConstructEmptyBnodePropertyList(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:ref [] }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// 1 triple per person: ?s ex:ref _:b
	if r.Graph.Len() != 3 {
		t.Errorf("expected 3 triples, got %d", r.Graph.Len())
	}
}

func TestConstructDeeplyNestedBnodes(t *testing.T) {
	g := makeSPARQLGraph(t)
	// 4 levels deep: [ a [ b [ c [ d "x" ] ] ] ]
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:a [ ex:b [ ex:c [ ex:d ?name ] ] ]
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// 4 triples per person:
	//   ?s ex:a _:b1 .
	//   _:b1 ex:b _:b2 .
	//   _:b2 ex:c _:b3 .
	//   _:b3 ex:d ?name .
	if r.Graph.Len() != 12 {
		t.Errorf("expected 12 triples (4 * 3 people), got %d", r.Graph.Len())
	}
}

func TestConstructBnodeAsSubject(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			[ ex:who ?s ; ex:label ?name ] ex:type "record"
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// 3 triples per person:
	//   _:b ex:who ?s .
	//   _:b ex:label ?name .
	//   _:b ex:type "record" .
	if r.Graph.Len() != 9 {
		t.Errorf("expected 9 triples, got %d", r.Graph.Len())
	}
}

func TestConstructBnodeWithCommaObjects(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:info [ ex:val "a" , "b" ]
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// Per person: ?s ex:info _:b . _:b ex:val "a" . _:b ex:val "b" = 3 triples
	if r.Graph.Len() != 9 {
		t.Errorf("expected 9 triples, got %d", r.Graph.Len())
	}
}

func TestConstructCollectionInsideBnode(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:data [ ex:items (1 2) ]
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// Per person:
	//   ?s ex:data _:b .
	//   _:b ex:items _:list1 .
	//   _:list1 rdf:first 1 . _:list1 rdf:rest _:list2 .
	//   _:list2 rdf:first 2 . _:list2 rdf:rest rdf:nil .
	// = 6 triples per person = 18
	if r.Graph.Len() != 18 {
		t.Errorf("expected 18 triples, got %d", r.Graph.Len())
	}
}

func TestConstructCollectionAsObject(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:nums (1 2 3)
		}
		WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Fatal("expected graph")
	}
	// Per person: ?s ex:nums _:h . plus 3 rdf:first + 3 rdf:rest = 7
	if r.Graph.Len() != 21 {
		t.Errorf("expected 21 triples, got %d", r.Graph.Len())
	}
}
