package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// An empty <uri/> is the empty relative IRI, not an unbound variable.
func TestParseSRX_EmptyIRIIsBound(t *testing.T) {
	doc := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/><variable name="y"/></head>
  <results><result><binding name="x"><uri></uri></binding></result></results>
</sparql>`
	res, err := ParseSRX(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := res.Bindings[0]["x"]
	if !ok {
		t.Fatal("x is unbound; an empty <uri></uri> must bind the empty IRI")
	}
	if u, isIRI := got.(rdflibgo.URIRef); !isIRI || u.Value() != "" {
		t.Errorf("x = %v, want the empty IRI", got)
	}
	if _, ok := res.Bindings[0]["y"]; ok {
		t.Error("y has no binding element and must stay unbound")
	}
}
