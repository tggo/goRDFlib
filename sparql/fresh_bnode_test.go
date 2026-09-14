package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func subjectsOf(g *rdflibgo.Graph, pred string) map[string]bool {
	p := rdflibgo.NewURIRefUnsafe(pred)
	subs := map[string]bool{}
	for tr := range g.Triples(nil, &p, nil) {
		subs[tr.Subject.N3()] = true
	}
	return subs
}

// rdflib #892 / Update §3.1.1: blank node labels are scoped to the request, so
// _:a in two INSERT DATA requests is two nodes.
func TestFreshBNodes_InsertDataRequests(t *testing.T) {
	ds := updateTestDataset(t, ``)
	for _, u := range []string{
		`INSERT DATA { _:a <urn:label> "A" }`,
		`INSERT DATA { _:a <urn:label> "B" }`,
	} {
		if err := Update(ds, u); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(subjectsOf(ds.Default, "urn:label")); n != 2 {
		t.Errorf("two INSERT DATA requests with _:a: %d distinct nodes, want 2", n)
	}
}

// Within one INSERT DATA operation a label still names one node.
func TestFreshBNodes_InsertDataLabelSharedInOperation(t *testing.T) {
	ds := updateTestDataset(t, ``)
	if err := Update(ds, `INSERT DATA { _:a <urn:p> 1 . _:a <urn:q> 2 }`); err != nil {
		t.Fatal(err)
	}
	p, q := subjectsOf(ds.Default, "urn:p"), subjectsOf(ds.Default, "urn:q")
	for s := range p {
		if !q[s] {
			t.Errorf("_:a became two nodes inside one operation")
		}
	}
}

// Update §3.1.3: template blank nodes are fresh for each solution, and shared by
// the template's triples within a solution.
func TestFreshBNodes_InsertTemplatePerSolution(t *testing.T) {
	ds := updateTestDataset(t, `<urn:x> <urn:p> 1 . <urn:y> <urn:p> 2 .`)
	if err := Update(ds, `INSERT { _:b <urn:of> ?s . _:b <urn:val> ?o . [] <urn:anon> ?s } WHERE { ?s <urn:p> ?o }`); err != nil {
		t.Fatal(err)
	}
	of, val := subjectsOf(ds.Default, "urn:of"), subjectsOf(ds.Default, "urn:val")
	if len(of) != 2 {
		t.Errorf("_:b over 2 solutions: %d distinct nodes, want 2", len(of))
	}
	for s := range of {
		if !val[s] {
			t.Errorf("_:b is not shared by the template triples of one solution")
		}
	}
	if n := len(subjectsOf(ds.Default, "urn:anon")); n != 2 {
		t.Errorf("[] over 2 solutions: %d distinct nodes, want 2", n)
	}
	// A second run of the same update mints new nodes again.
	if err := Update(ds, `INSERT { _:b <urn:of> ?s } WHERE { ?s <urn:p> ?o }`); err != nil {
		t.Fatal(err)
	}
	if n := len(subjectsOf(ds.Default, "urn:of")); n != 4 {
		t.Errorf("after a second request: %d distinct nodes, want 4", n)
	}
}

// Query §16.2.1: CONSTRUCT template blank nodes are fresh per solution.
func TestFreshBNodes_ConstructPerSolution(t *testing.T) {
	g := updateTestDataset(t, `<urn:x> <urn:p> 1 . <urn:y> <urn:p> 2 .`).Default
	res, err := Query(g, `CONSTRUCT { _:b <urn:of> ?s . _:b <urn:val> ?o } WHERE { ?s <urn:p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	of, val := subjectsOf(res.Graph, "urn:of"), subjectsOf(res.Graph, "urn:val")
	if len(of) != 2 {
		t.Errorf("CONSTRUCT _:b over 2 solutions: %d distinct nodes, want 2", len(of))
	}
	for s := range of {
		if !val[s] {
			t.Errorf("_:b is not shared by the template triples of one solution")
		}
	}
}
