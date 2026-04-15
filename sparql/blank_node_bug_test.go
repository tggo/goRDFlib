package sparql

import (
	"testing"
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
		t.Error("expected graph")
	}

	// Should produce 5 triples for each person:
	// ?s ex:details _:bnode1  .
	// _:bnode1 ex:contact _:bnode2 ;
	//   a ex:Contact .
	// _:bnode2 ex:name ?name ;
	//   a ex:Name .
	// With 3 people, that's 15 triples total
	expectedTripleCount := 15
	if r.Graph.Len() != expectedTripleCount {
		t.Errorf("expected %d triples, got %d", expectedTripleCount, r.Graph.Len())
	}
}
