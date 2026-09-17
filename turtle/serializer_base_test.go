package turtle

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// Issue #34: with WithBase, IRIs under the base are written relative to it,
// and parsing the output (which carries @base) gives back the same graph.
func TestSerializeWithBaseWritesRelativeIRIs(t *testing.T) {
	const base = "http://localhost/dir/doc.ttl"
	u := rdflibgo.NewURIRefUnsafe
	g := rdflibgo.NewGraph()
	g.Add(u(base+"#local/ref"), u("http://localhost/dir/p"), u(base))
	g.Add(u(base+"#local/ref"), u("http://localhost/other/q"), rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(u(base+"#dt"))))
	g.Add(u("http://elsewhere/s"), u("http://localhost/dir/p"), rdflibgo.NewTripleTerm(u(base+"#a"), u("http://localhost/dir/p"), u("http://localhost/dir/x:y")))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithBase(base)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"@base <" + base + ">", "<#local/ref>", "<p>", "<>", "</other/q>", `^^<#dt>`, "<<( <#a> <p> <./x:y> )>>", "<http://elsewhere/s>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<"+base+"#") {
		t.Errorf("an IRI under the base was written absolute:\n%s", out)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(out)); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	testutil.AssertGraphEqual(t, g, g2)
}

// A prefixed name still wins over a relative IRI, and without WithBase nothing
// is relativized.
func TestSerializeWithBasePrefersPrefixes(t *testing.T) {
	u := rdflibgo.NewURIRefUnsafe
	g := rdflibgo.NewGraph()
	g.Bind("ex", u("http://localhost/ns#"))
	g.Add(u("http://localhost/ns#s"), u("http://localhost/ns#p"), u("http://localhost/o"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithBase("http://localhost/")); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "ex:s ex:p <o>") {
		t.Errorf("want prefixed names and a relative object:\n%s", out)
	}
	buf.Reset()
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "<http://localhost/o>") {
		t.Errorf("relativized without a base:\n%s", out)
	}
}
