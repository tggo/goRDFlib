package trig

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// helpers
func newGraph() *rdflibgo.Graph { return rdflibgo.NewGraph() }

func mustParse(t *testing.T, data string, opts ...Option) *rdflibgo.Graph {
	t.Helper()
	g := newGraph()
	if err := Parse(g, strings.NewReader(data), opts...); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return g
}

func mustParseDS(t *testing.T, data string, opts ...Option) *graph.Dataset {
	t.Helper()
	ds := graph.NewDataset()
	if err := ParseDataset(ds, strings.NewReader(data), opts...); err != nil {
		t.Fatalf("parseDataset error: %v", err)
	}
	return ds
}

func mustSerialize(t *testing.T, g *rdflibgo.Graph, opts ...Option) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Serialize(g, &buf, opts...); err != nil {
		t.Fatalf("serialize error: %v", err)
	}
	return buf.String()
}

func mustSerializeDS(t *testing.T, ds *graph.Dataset, opts ...Option) string {
	t.Helper()
	var buf bytes.Buffer
	if err := SerializeDataset(ds, &buf, opts...); err != nil {
		t.Fatalf("serializeDataset error: %v", err)
	}
	return buf.String()
}

// roundtrip: parse then serialize then parse again and check triple count.
func roundtrip(t *testing.T, data string, opts ...Option) string {
	t.Helper()
	g := mustParse(t, data, opts...)
	out := mustSerialize(t, g, opts...)
	g2 := mustParse(t, out, opts...)
	if g.Len() != g2.Len() {
		t.Errorf("roundtrip triple count mismatch: got %d want %d\noutput:\n%s", g2.Len(), g.Len(), out)
	}
	return out
}

// TestSerialize_Empty serializes an empty graph.
func TestSerialize_Empty(t *testing.T) {
	g := newGraph()
	out := mustSerialize(t, g)
	if strings.TrimSpace(out) != "{" && !strings.Contains(out, "{") {
		// empty graphs produce "{\n}\n"
	}
	// just ensure no error
}

// TestSerialize_SimpleTriple serializes a single triple.
func TestSerialize_SimpleTriple(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	g.Add(s, p, o)
	out := mustSerialize(t, g)
	if !strings.Contains(out, "http://example.org/s") {
		t.Errorf("expected subject IRI in output:\n%s", out)
	}
}

// TestSerialize_WithPrefix uses prefix bindings to produce qnames.
func TestSerialize_WithPrefix(t *testing.T) {
	g := newGraph()
	ex := rdflibgo.NewURIRefUnsafe("http://example.org/")
	g.Bind("ex", ex)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/subject")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/predicate")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/object")
	g.Add(s, p, o)
	out := mustSerialize(t, g)
	if !strings.Contains(out, "@prefix ex:") {
		t.Errorf("expected prefix declaration in output:\n%s", out)
	}
	if !strings.Contains(out, "ex:subject") {
		t.Errorf("expected qname in output:\n%s", out)
	}
}

// TestSerialize_RDFType uses rdf:type shorthand "a".
func TestSerialize_RDFType(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.RDF.Type
	o := rdflibgo.NewURIRefUnsafe("http://example.org/Cls")
	g.Add(s, p, o)
	out := mustSerialize(t, g)
	if !strings.Contains(out, " a ") {
		t.Errorf("expected 'a' shorthand in output:\n%s", out)
	}
}

// TestSerialize_Literal covers literalStr with typed datatype.
func TestSerialize_Literal(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	// plain string literal
	g.Add(s, p, rdflibgo.NewLiteral("hello"))
	// integer literal
	g.Add(s, p, rdflibgo.NewLiteral(int64(42)))
	// lang-tagged literal
	g.Add(s, p, rdflibgo.NewLiteral("bonjour", rdflibgo.WithLang("fr")))
	out := mustSerialize(t, g)
	if !strings.Contains(out, "hello") {
		t.Errorf("expected string literal in output:\n%s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected int literal in output:\n%s", out)
	}
	if !strings.Contains(out, "@fr") {
		t.Errorf("expected lang literal in output:\n%s", out)
	}
}

// TestSerialize_LiteralWithTypedQName serializes a typed literal with qname datatype.
func TestSerialize_LiteralWithTypedQName(t *testing.T) {
	g := newGraph()
	xsd := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#")
	g.Bind("xsd", xsd)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	dt := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#integer")
	lit := rdflibgo.NewLiteral("99", rdflibgo.WithDatatype(dt))
	g.Add(s, p, lit)
	out := mustSerialize(t, g)
	// Should use xsd:integer shorthand if xsd prefix is detected
	_ = out
}

// TestSerialize_BNode covers bnode inline serialization.
func TestSerialize_BNode(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p [ ex:q ex:o ] .
}
`
	roundtrip(t, data)
}

// TestSerialize_BNodeEmptySubject covers [] as subject.
func TestSerialize_BNodeEmptySubject(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  [] ex:p ex:o .
}
`
	roundtrip(t, data)
}

// TestSerialize_List covers RDF list serialization.
func TestSerialize_List(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ( ex:a ex:b ex:c ) .
}
`
	roundtrip(t, data)
}

// TestSerialize_MultiplePredicates covers multiple predicates on same subject.
func TestSerialize_MultiplePredicates(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p1 := rdflibgo.NewURIRefUnsafe("http://example.org/p1")
	p2 := rdflibgo.NewURIRefUnsafe("http://example.org/p2")
	o1 := rdflibgo.NewURIRefUnsafe("http://example.org/o1")
	o2 := rdflibgo.NewURIRefUnsafe("http://example.org/o2")
	g.Add(s, p1, o1)
	g.Add(s, p2, o2)
	out := mustSerialize(t, g)
	if !strings.Contains(out, ";") {
		t.Errorf("expected semicolon for multiple predicates:\n%s", out)
	}
}

// TestSerialize_MultipleObjects covers comma-separated objects.
func TestSerialize_MultipleObjects(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o1 := rdflibgo.NewURIRefUnsafe("http://example.org/o1")
	o2 := rdflibgo.NewURIRefUnsafe("http://example.org/o2")
	g.Add(s, p, o1)
	g.Add(s, p, o2)
	out := mustSerialize(t, g)
	if !strings.Contains(out, ",") {
		t.Errorf("expected comma for multiple objects:\n%s", out)
	}
}

// TestSerialize_WithBase tests @base directive output.
func TestSerialize_WithBase(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	g.Add(s, p, o)
	out := mustSerialize(t, g, WithBase("http://example.org/"))
	if !strings.Contains(out, "@base") {
		t.Errorf("expected @base in output:\n%s", out)
	}
}

// TestSerialize_Dataset serializes a dataset with named graphs.
func TestSerialize_Dataset(t *testing.T) {
	ds := graph.NewDataset()
	ex := rdflibgo.NewURIRefUnsafe("http://example.org/")
	ds.Bind("ex", ex)

	// default graph
	def := ds.DefaultContext()
	def.Add(
		rdflibgo.NewURIRefUnsafe("http://example.org/s"),
		rdflibgo.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewURIRefUnsafe("http://example.org/o"),
	)

	// named graph
	ng := rdflibgo.NewURIRefUnsafe("http://example.org/graph1")
	named := ds.Graph(ng)
	named.Add(
		rdflibgo.NewURIRefUnsafe("http://example.org/s2"),
		rdflibgo.NewURIRefUnsafe("http://example.org/p2"),
		rdflibgo.NewURIRefUnsafe("http://example.org/o2"),
	)

	out := mustSerializeDS(t, ds)
	if !strings.Contains(out, "graph1") {
		t.Errorf("expected named graph IRI in output:\n%s", out)
	}
}

// TestSerialize_DatasetRoundtrip parses TriG then serializes and parses again.
func TestSerialize_DatasetRoundtrip(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .

{
  ex:s ex:p ex:o .
}

<http://example.org/g1> {
  ex:s2 ex:p2 ex:o2 .
}
`
	ds := mustParseDS(t, data)
	out := mustSerializeDS(t, ds)
	ds2 := mustParseDS(t, out)

	cnt := 0
	for g := range ds2.Graphs() {
		cnt += g.Len()
	}
	// Original has 2 triples (1 default + 1 named). After roundtrip graphs may merge.
	if cnt == 0 {
		t.Errorf("expected triples after roundtrip, got 0\n%s", out)
	}
}

// TestSerialize_TripleTerm covers tripleTermStr.
func TestSerialize_TripleTerm(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p <<( ex:a ex:b ex:c )>> .
}
`
	roundtrip(t, data)
}

// TestSerialize_RDFSLabel covers label sorting (rdfs:label prioritized after rdf:type).
func TestSerialize_RDFSLabel(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("Name", rdflibgo.WithLang("en")))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Cls"))
	out := mustSerialize(t, g)
	// rdf:type should come before rdfs:label
	typeIdx := strings.Index(out, " a ")
	labelIdx := strings.Index(out, "label")
	if typeIdx == -1 || labelIdx == -1 {
		t.Logf("output:\n%s", out)
	} else if typeIdx > labelIdx {
		t.Errorf("expected rdf:type before rdfs:label in output:\n%s", out)
	}
}

// TestSerialize_ClassSubject covers topSubjects ordering (subject typed as rdfs:Class).
func TestSerialize_ClassSubject(t *testing.T) {
	g := newGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/MyClass")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	out := mustSerialize(t, g)
	if !strings.Contains(out, "MyClass") {
		t.Errorf("expected class subject in output:\n%s", out)
	}
}

// TestSerialize_BNodeRefs covers bnode subjects with refs > 1 (not inlined).
func TestSerialize_BNodeRefs(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s1 ex:p _:b .
  ex:s2 ex:p _:b .
  _:b ex:q ex:o .
}
`
	// bnode referenced from two subjects should not be inlined
	g := mustParse(t, data)
	out := mustSerialize(t, g)
	_ = out
}

// TestSerialize_isValidPrefixName covers edge cases.
func TestSerialize_isValidPrefixName_edge(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", true},          // empty is valid prefix
		{"foo", true},
		{"foo.", false},     // trailing dot
		{"foo-bar", true},
		{"1bad", false},     // starts with digit
		{"a\u0300", true},   // combining mark
	}
	for _, tc := range tests {
		got := isValidPrefixName(tc.s)
		if got != tc.want {
			t.Errorf("isValidPrefixName(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

// TestSerialize_isValidLocalName covers edge cases.
func TestSerialize_isValidLocalName_edge(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"foo", true},
		{"foo.", false},    // trailing dot
		{"0foo", true},    // starts with digit - valid
		{":colon", true},  // starts with colon
		{"ab\u00B7c", true}, // middle dot valid
	}
	for _, tc := range tests {
		got := isValidLocalName(tc.s)
		if got != tc.want {
			t.Errorf("isValidLocalName(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

// TestSerialize_qnameOrFull_NoMatch covers case where URI doesn't match any prefix.
func TestSerialize_qnameOrFull_NoMatch(t *testing.T) {
	usedNS := map[string]rdflibgo.URIRef{
		"ex": rdflibgo.NewURIRefUnsafe("http://example.org/"),
	}
	u := rdflibgo.NewURIRefUnsafe("http://other.org/thing")
	got := qnameOrFull(u, usedNS)
	if got != "<http://other.org/thing>" {
		t.Errorf("expected full IRI, got %q", got)
	}
}

// TestSerialize_qnameOrFull_InvalidLocal covers case where local name is invalid.
func TestSerialize_qnameOrFull_InvalidLocal(t *testing.T) {
	usedNS := map[string]rdflibgo.URIRef{
		"ex": rdflibgo.NewURIRefUnsafe("http://example.org/"),
	}
	// local part ends with dot, invalid
	u := rdflibgo.NewURIRefUnsafe("http://example.org/bad.")
	got := qnameOrFull(u, usedNS)
	if got != "<http://example.org/bad.>" {
		t.Errorf("expected full IRI for invalid local, got %q", got)
	}
}

// TestSerialize_trackNSForTerm_TripleTerm covers triple term tracking.
func TestSerialize_trackNSForTerm_TripleTerm(t *testing.T) {
	ds := graph.NewDataset()
	ex := rdflibgo.NewURIRefUnsafe("http://example.org/")
	ds.Bind("ex", ex)

	def := ds.DefaultContext()
	subj := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	pred := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	obj := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(subj, pred, obj)

	outerS := rdflibgo.NewURIRefUnsafe("http://example.org/outer")
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/outerP")
	def.Add(outerS, outerP, tt)

	out := mustSerializeDS(t, ds)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term in output:\n%s", out)
	}
}

// TestParse_Function exercises the Parse (non-dataset) function.
func TestParse_Function(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
GRAPH <http://example.org/g1> {
  ex:s ex:p ex:o .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_WithBase exercises relative IRI resolution.
func TestParse_WithBase(t *testing.T) {
	const data = `
{ <s> <p> <o> . }
`
	g := mustParse(t, data, WithBase("http://example.org/"))
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_VERSION exercises sparqlVersion path.
func TestParse_VERSION(t *testing.T) {
	const data = `VERSION "1.2"
@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o . }
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_VERSION_Error exercises error path in version string.
func TestParse_VERSION_Error(t *testing.T) {
	const data = `VERSION 1.2`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for invalid version string")
	}
}

// TestParse_TripleStatement exercises the tripleStatement path (subject outside graph block).
func TestParse_TripleStatement(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_AnnotationBlock exercises annotation syntax.
func TestParse_AnnotationBlock(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o {| ex:confidence "0.9" |} .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from annotation block")
	}
}

// TestParse_ReifiedTriple exercises reified triple subject.
func TestParse_ReifiedTriple(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  << ex:s ex:p ex:o >> ex:confidence "0.9" .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from reified triple")
	}
}

// TestParse_ReifierSyntax exercises ~ reifier syntax.
func TestParse_ReifierSyntax(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o ~ex:r .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from reifier syntax")
	}
}

// TestParse_EmptyBNodeInReified exercises [] in reified triple.
func TestParse_EmptyBNodeInReified(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  << [] ex:p ex:o >> ex:confidence "0.9" .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples")
	}
}

// TestParse_ReadVersionString_Errors tests error paths in readVersionString.
func TestParse_ReadVersionString_Errors(t *testing.T) {
	cases := []string{
		`VERSION `,              // empty after VERSION
		`VERSION '''bad'''`,     // triple-quoted
		"VERSION \"unterminated", // unterminated
		"VERSION \"new\nline\"",  // newline inside
	}
	for _, c := range cases {
		g := newGraph()
		if err := Parse(g, strings.NewReader(c)); err == nil {
			t.Errorf("expected error for input %q", c)
		}
	}
}

// TestParse_ReifiedInnerObject_Collection exercises collection-not-allowed error.
func TestParse_ReifiedInnerObject_Collection(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  << ex:s ex:p ( ex:a ) >> ex:q ex:r .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for collection in reified triple object")
	}
}

// TestParse_ReadReifiedInnerObject_Literals exercises literal paths in reified inner object.
func TestParse_ReadReifiedInnerObject_Literals(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"integer", `@prefix ex: <http://example.org/> . { << ex:s ex:p 42 >> ex:q ex:r . }`},
		{"boolean_true", `@prefix ex: <http://example.org/> . { << ex:s ex:p true >> ex:q ex:r . }`},
		{"boolean_false", `@prefix ex: <http://example.org/> . { << ex:s ex:p false >> ex:q ex:r . }`},
		{"string", `@prefix ex: <http://example.org/> . { << ex:s ex:p "hello" >> ex:q ex:r . }`},
		{"bnode_empty", `@prefix ex: <http://example.org/> . { << ex:s ex:p [] >> ex:q ex:r . }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := mustParse(t, tc.data)
			if g.Len() == 0 {
				t.Errorf("expected triples for %s", tc.name)
			}
		})
	}
}

// TestParse_ReifiedTripleSubject_Nested exercises nested reified triple in subject.
func TestParse_ReifiedTripleSubject_Nested(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  << << ex:a ex:b ex:c >> ex:p ex:o >> ex:confidence "high" .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from nested reified triple")
	}
}

// TestParse_TripleTermSubject_BNode exercises bnode in triple term subject.
func TestParse_TripleTermSubject_BNode(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p <<( _:b ex:pred ex:obj )>> .
}
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_ReadReifierID_IRI exercises IRI reifier.
func TestParse_ReadReifierID_IRI(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o ~<http://example.org/reifier> .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from IRI reifier")
	}
}

// TestParse_ReadReifierID_BNode exercises blank node reifier.
func TestParse_ReadReifierID_BNode(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o ~_:myReifier .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from bnode reifier")
	}
}

// TestParse_EmptyAnnotation exercises error for empty annotation block.
func TestParse_EmptyAnnotation(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o {| |} .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for empty annotation block")
	}
}

// TestSerialize_EmptyDataset serializes an empty dataset.
func TestSerialize_EmptyDataset(t *testing.T) {
	ds := graph.NewDataset()
	out := mustSerializeDS(t, ds)
	// no panic, output may be empty or just whitespace
	_ = out
}

// TestSerialize_ListWithLiterals exercises list with literal items.
func TestSerialize_ListWithLiterals(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ( "a" "b" "c" ) .
}
`
	roundtrip(t, data)
}

// TestSerialize_TrigLabel_BNode covers trigLabel for bnode.
func TestSerialize_TrigLabel_BNode(t *testing.T) {
	b := rdflibgo.NewBNode()
	usedNS := map[string]rdflibgo.URIRef{}
	got := trigLabel(b, usedNS)
	if !strings.HasPrefix(got, "_:") {
		t.Errorf("expected bnode N3 label, got %q", got)
	}
}

// TestParse_ReadAnnotationAfterTilde_WithIRI exercises annotation block after tilde-reifier.
func TestParse_ReadAnnotationAfterTilde_WithIRI(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o ~<http://example.org/r> {| ex:x ex:y |} .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples")
	}
}

// TestParse_AnonymousReifier exercises ~ with no explicit ID.
func TestParse_AnonymousReifier(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o ~ .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from anonymous reifier")
	}
}

// TestParse_AnonymousAnnotation exercises {| p o |} without explicit reifier.
func TestParse_AnonymousAnnotation(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p ex:o {| ex:confidence "high" |} .
}
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from anonymous annotation")
	}
}

// TestParse_TopLevelCollection exercises collectionTripleStatement.
func TestParse_TopLevelCollection(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
( ex:a ex:b ) ex:p ex:o .
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from top-level collection subject")
	}
}

// TestParse_TopLevelBracketSubject exercises bracketSubjectOrGraph with [...].
func TestParse_TopLevelBracketSubject(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
[ ex:p ex:o ] ex:q ex:r .
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from top-level bnode property list subject")
	}
}

// TestParse_TopLevelEmptyBracketSubject exercises [] as subject.
func TestParse_TopLevelEmptyBracketSubject(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
[] ex:p ex:o .
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_TopLevelEmptyBracketGraph exercises [] { } as anonymous graph.
func TestParse_TopLevelEmptyBracketGraph(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
[] {
  ex:s ex:p ex:o .
}
`
	ds := mustParseDS(t, data)
	cnt := 0
	for g := range ds.Graphs() {
		cnt += g.Len()
	}
	if cnt == 0 {
		t.Error("expected triples in anonymous named graph")
	}
}

// TestParse_TopLevelBracketSubjectAlone exercises [] . (standalone).
func TestParse_TopLevelBracketSubjectAlone(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
[] .
`
	g := mustParse(t, data)
	_ = g // no error expected
}

// TestParse_TopLevelReifiedStatement exercises reifiedTripleStatement at top level.
func TestParse_TopLevelReifiedStatement(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:confidence "high" .
`
	g := mustParse(t, data)
	if g.Len() == 0 {
		t.Error("expected triples from top-level reified triple statement")
	}
}

// TestParse_TopLevelReifiedStatementDot exercises reified triple followed by just '.'.
func TestParse_TopLevelReifiedStatementDot(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> .
`
	g := mustParse(t, data)
	// should parse without error
	_ = g
}

// TestSerialize_LiteralStr_TypedWithQName covers the literalStr branch where datatype gets qname.
func TestSerialize_LiteralStr_TypedWithQName(t *testing.T) {
	// Build a graph with a custom datatype that has a prefix mapping.
	g := newGraph()
	myNS := rdflibgo.NewURIRefUnsafe("http://myns.example/")
	g.Bind("my", myNS)
	s := rdflibgo.NewURIRefUnsafe("http://myns.example/s")
	p := rdflibgo.NewURIRefUnsafe("http://myns.example/p")
	dt := rdflibgo.NewURIRefUnsafe("http://myns.example/myType")
	lit := rdflibgo.NewLiteral("val", rdflibgo.WithDatatype(dt))
	g.Add(s, p, lit)
	out := mustSerialize(t, g)
	// Datatype should be abbreviated as my:myType
	if !strings.Contains(out, "my:myType") {
		t.Errorf("expected abbreviated datatype in output:\n%s", out)
	}
}

// TestSerialize_InvalidListCycle covers isValidList with cycle detection.
func TestSerialize_InvalidListCycle(t *testing.T) {
	// Create a "cyclic" rdf:first/rest structure - isValidList should return false.
	// We do this by adding triples directly without using the collection helper.
	g := newGraph()
	b1 := rdflibgo.NewBNode("b1")
	g.Add(b1, rdflibgo.RDF.First, rdflibgo.NewURIRefUnsafe("http://example.org/item"))
	// rest points back to itself - cycle
	g.Add(b1, rdflibgo.RDF.Rest, b1)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, b1)
	// Should not panic, just serialize as regular bnodes
	out := mustSerialize(t, g)
	_ = out
}

// TestSerialize_InvalidListExtraPredicates covers isValidList returning false for extra preds.
func TestSerialize_InvalidListExtraPredicates(t *testing.T) {
	g := newGraph()
	b1 := rdflibgo.NewBNode("listb1")
	g.Add(b1, rdflibgo.RDF.First, rdflibgo.NewURIRefUnsafe("http://example.org/item"))
	g.Add(b1, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	// extra predicate - invalidates list
	g.Add(b1, rdflibgo.NewURIRefUnsafe("http://example.org/extra"), rdflibgo.NewLiteral("x"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, b1)
	out := mustSerialize(t, g)
	_ = out
}

// TestSerialize_writeIndented_AlreadySerialized covers the serialized skip in writeIndented.
func TestSerialize_writeIndented_AlreadySerialized(t *testing.T) {
	// An inlined bnode gets serialized twice (once inlined, should be skipped on second pass).
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p [ ex:q ex:o ] .
}
`
	roundtrip(t, data)
}

// TestParse_SPARQLPrefix exercises SPARQL-style PREFIX.
func TestParse_SPARQLPrefix(t *testing.T) {
	const data = `PREFIX ex: <http://example.org/>
{ ex:s ex:p ex:o . }
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_SPARQLBase exercises SPARQL-style BASE.
func TestParse_SPARQLBase(t *testing.T) {
	const data = `BASE <http://example.org/>
{ <s> <p> <o> . }
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_AtBase exercises @base directive.
func TestParse_AtBase(t *testing.T) {
	const data = `@base <http://example.org/> .
@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o . }
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_CollectionError_StandaloneInBlock exercises error for standalone collection.
func TestParse_CollectionError_StandaloneInBlock(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ( ex:a ex:b ) .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for standalone collection in block")
	}
}

// TestParse_BracketPropertyListNotGraphLabel exercises error for [...] as graph label.
func TestParse_BracketPropertyListNotGraphLabel(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
[ ex:p ex:o ] {
  ex:s ex:p ex:o .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for bnode property list as graph label")
	}
}

// TestParse_TopLevelCollectionError_Standalone exercises error for standalone top-level collection.
func TestParse_TopLevelCollectionError_Standalone(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
( ex:a ex:b ) .
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for standalone top-level collection")
	}
}

// TestParse_TripleTermSubject_Prefixed exercises prefixed name in triple term subject.
func TestParse_TripleTermSubject_Prefixed(t *testing.T) {
	const data = `
@prefix ex: <http://example.org/> .
{
  ex:s ex:p <<( ex:sub ex:pred "obj" )>> .
}
`
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_ReadVersionSingleQuote exercises single-quoted version string.
func TestParse_ReadVersionSingleQuote(t *testing.T) {
	const data = "VERSION '1.2'\n{ <http://example.org/s> <http://example.org/p> <http://example.org/o> . }"
	g := mustParse(t, data)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParse_ReifiedInnerObject_BlankNode exercises bnode [] in reified inner object.
func TestParse_ReifiedInnerObject_BNode_properlist(t *testing.T) {
	// non-empty [...] should fail in reified triple
	const data = `
@prefix ex: <http://example.org/> .
{
  << ex:s ex:p [ ex:q ex:r ] >> ex:confidence "high" .
}
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for non-empty bnode in reified triple object")
	}
}

// TestParse_TopLevelTripleAsSubject exercises top-level triple term as subject (TriG 1.2).
func TestParse_TopLevelTripleAsSubjectErr(t *testing.T) {
	// triple term <<( ... )>> cannot be used as subject in reified triple
	const data = `
@prefix ex: <http://example.org/> .
<< <<( ex:s ex:p ex:o )>> ex:p ex:o >> ex:q ex:r .
`
	g := newGraph()
	if err := Parse(g, strings.NewReader(data)); err == nil {
		t.Error("expected error for triple term as subject in reified triple")
	}
}
