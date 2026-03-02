package w3c

import "testing"

func TestParseManifestTurtle(t *testing.T) {
	m, err := ParseManifest("rdf-tests/rdf/rdf11/rdf-turtle/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) == 0 {
		t.Fatal("no entries")
	}
	t.Logf("entries=%d base=%q first=%q", len(m.Entries), m.AssumedTestBase, m.Entries[0].Name)
}

func TestParseManifestNTriples(t *testing.T) {
	m, err := ParseManifest("rdf-tests/rdf/rdf11/rdf-n-triples/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("entries=%d", len(m.Entries))
}

func TestParseManifestNQuads(t *testing.T) {
	m, err := ParseManifest("rdf-tests/rdf/rdf11/rdf-n-quads/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("entries=%d", len(m.Entries))
}

func TestParseManifestRDFXML(t *testing.T) {
	m, err := ParseManifest("rdf-tests/rdf/rdf11/rdf-xml/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("entries=%d base=%q", len(m.Entries), m.AssumedTestBase)
}
