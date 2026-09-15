package results_test

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/turtle"
)

// specExample is the result used by the examples in SPARQL 1.2 Query Results
// CSV and TSV Formats, §2.3 and §3.3 (W3C Working Draft 2026-07-23).
func specExample() *sparql.Result {
	x := iri("http://example/x")
	b0, b1 := bnode("blank0"), bnode("blank1")
	return selectResult([]string{"x", "literal"},
		row{"x": x, "literal": lit("String")},
		row{"x": x, "literal": lit(`String-with-dquote"`)},
		row{"x": b0, "literal": lit("Blank node")},
		row{"literal": lit("Missing 'x'")},
		row{},
		row{"x": x},
		row{"x": b1, "literal": lit("String-with-lang", rdflibgo.WithLang("en"))},
		row{"x": b1, "literal": lit("String-with-lang-dir", rdflibgo.WithLang("en"), rdflibgo.WithDir("ltr"))},
		row{"x": b1, "literal": typed("123", xsd+"integer")},
	)
}

func TestSpecExampleCSV(t *testing.T) {
	want := "x,literal\r\n" +
		"http://example/x,String\r\n" +
		"http://example/x,\"String-with-dquote\"\"\"\r\n" +
		"_:b0,Blank node\r\n" +
		",Missing 'x'\r\n" +
		",\r\n" +
		"http://example/x,\r\n" +
		"_:b1,String-with-lang\r\n" +
		"_:b1,String-with-lang-dir\r\n" +
		"_:b1,123\r\n"
	if got := write(t, results.FormatCSV, specExample()); got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestSpecExampleTSV(t *testing.T) {
	// The spec labels the blank nodes blank0 and blank1; ours are b0 and b1.
	want := "?x\t?literal\n" +
		"<http://example/x>\t\"String\"\n" +
		"<http://example/x>\t\"String-with-dquote\\\"\"\n" +
		"_:b0\t\"Blank node\"\n" +
		"\t\"Missing 'x'\"\n" +
		"\t\n" +
		"<http://example/x>\t\n" +
		"_:b1\t\"String-with-lang\"@en\n" +
		"_:b1\t\"String-with-lang-dir\"@en--ltr\n" +
		"_:b1\t123\n"
	if got := write(t, results.FormatTSV, specExample()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func specTripleExample() *sparql.Result {
	ex := func(s string) rdflibgo.URIRef { return iri("http://example/" + s) }
	return selectResult([]string{"x", "triple"},
		row{"x": lit("Alice"), "triple": triple(ex("alice"), "http://example/knows", ex("bob"))},
		row{"x": lit("Bob"), "triple": triple(ex("bob"), "http://example/knows", ex("alice"))},
		row{"x": lit("Carol"), "triple": triple(ex("carol"), "http://example/says", lit(`Hello world, my name is "Alice".`))},
	)
}

// The spec's CSV triple term example quotes the literal "Alice" although
// RFC 4180 does not require it, so the comparison is on parsed records.
func TestSpecTripleTermExampleCSV(t *testing.T) {
	want := "x,triple\n" +
		"\"Alice\",<<( http://example/alice http://example/knows http://example/bob )>>\n" +
		"\"Bob\",<<( http://example/bob http://example/knows http://example/alice )>>\n" +
		"\"Carol\",\"<<( http://example/carol http://example/says \"\"Hello world, my name is \"\"\"\"Alice\"\"\"\".\"\" )>>\"\n"
	got := write(t, results.FormatCSV, specTripleExample())
	wr, err := csv.NewReader(strings.NewReader(want)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	gr, err := csv.NewReader(strings.NewReader(got)).ReadAll()
	if err != nil {
		t.Fatalf("%v\n%s", err, got)
	}
	if len(wr) != len(gr) {
		t.Fatalf("got %q", got)
	}
	for i := range wr {
		if strings.Join(wr[i], "|") != strings.Join(gr[i], "|") {
			t.Errorf("record %d: got %q, want %q", i, gr[i], wr[i])
		}
	}
}

// The spec's TSV triple term example leaves the closing quote off the last
// literal; the expected text below has it.
func TestSpecTripleTermExampleTSV(t *testing.T) {
	want := "?x\t?triple\n" +
		"\"Alice\"\t<<( <http://example/alice> <http://example/knows> <http://example/bob> )>>\n" +
		"\"Bob\"\t<<( <http://example/bob> <http://example/knows> <http://example/alice> )>>\n" +
		"\"Carol\"\t<<( <http://example/carol> <http://example/says> \"Hello world, my name is \\\"Alice\\\".\" )>>\n"
	if got := write(t, results.FormatTSV, specTripleExample()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestGoldenJSONAndXML(t *testing.T) {
	r := selectResult([]string{"s", "o"},
		row{"s": iri("http://e/s"), "o": lit("a\"b", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl"))},
		row{"s": bnode("x"), "o": typed("1", xsd+"integer")},
		row{"o": triple(bnode("x"), "http://e/p", lit("v"))},
	)
	wantJSON := `{"head":{"vars":["s","o"]},
"results":{"bindings":[
{"s":{"type":"uri","value":"http://e/s"},"o":{"type":"literal","value":"a\"b","xml:lang":"ar","its:dir":"rtl"}},
{"s":{"type":"bnode","value":"b0"},"o":{"type":"literal","value":"1","datatype":"http://www.w3.org/2001/XMLSchema#integer"}},
{"o":{"type":"triple","value":{"subject":{"type":"bnode","value":"b0"},"predicate":{"type":"uri","value":"http://e/p"},"object":{"type":"literal","value":"v"}}}}
]}}
`
	if got := write(t, results.FormatJSON, r); got != wantJSON {
		t.Errorf("JSON got\n%s\nwant\n%s", got, wantJSON)
	}
	wantXML := `<?xml version="1.0" encoding="UTF-8"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#" xmlns:its="http://www.w3.org/2005/11/its">
  <head>
    <variable name="s"/>
    <variable name="o"/>
  </head>
  <results>
    <result>
      <binding name="s"><uri>http://e/s</uri></binding>
      <binding name="o"><literal xml:lang="ar" its:dir="rtl">a"b</literal></binding>
    </result>
    <result>
      <binding name="s"><bnode>b0</bnode></binding>
      <binding name="o"><literal datatype="http://www.w3.org/2001/XMLSchema#integer">1</literal></binding>
    </result>
    <result>
      <binding name="o"><triple><subject><bnode>b0</bnode></subject><predicate><uri>http://e/p</uri></predicate><object><literal>v</literal></object></triple></binding>
    </result>
  </results>
</sparql>
`
	if got := write(t, results.FormatXML, r); got != wantXML {
		t.Errorf("XML got\n%s\nwant\n%s", got, wantXML)
	}
	empty := selectResult(nil)
	if got, want := write(t, results.FormatJSON, empty), "{\"head\":{\"vars\":[]},\n\"results\":{\"bindings\":[]}}\n"; got != want {
		t.Errorf("empty JSON %q", got)
	}
	if got, want := write(t, results.FormatJSON, &sparql.Result{Type: "ASK", AskResult: true}), "{\"head\":{},\"boolean\":true}\n"; got != want {
		t.Errorf("ASK JSON %q", got)
	}
}

func TestEscaping(t *testing.T) {
	in := "q\"b\\t\tn\nr\rc\x01d\x7fe\u2028f\u2029<&>]]>,"
	r := selectResult([]string{"o"}, row{"o": lit(in)})
	tests := map[results.Format]string{
		results.FormatJSON: `"value":"q\"b\\t\tn\nr\rc\u0001d` + "\x7f" + `e\u2028f\u2029<&>]]>,"`,
		results.FormatTSV:  `"q\"b\\t\tn\nr\rc\u0001d\u007fe` + "\u2028f\u2029" + `<&>]]>,"` + "\n",
		results.FormatCSV:  "\"q\"\"b\\t\tn\nr\rc\x01d\x7fe\u2028f\u2029<&>]]>,\"\r\n",
	}
	for f, want := range tests {
		if got := write(t, f, r); !strings.Contains(got, want) {
			t.Errorf("%s: want %q in\n%q", f, want, got)
		}
	}
	xmlIn := selectResult([]string{"o"}, row{"o": lit("q\"b\\t\tn\nr\rc<&>]]>\u2028")})
	if got, want := write(t, results.FormatXML, xmlIn), "<literal>q\"b\\t\tn\nr&#xD;c&lt;&amp;&gt;]]&gt;\u2028</literal>"; !strings.Contains(got, want) {
		t.Errorf("xml: want %q in\n%q", want, got)
	}
	attr := selectResult([]string{"o"}, row{"o": typed("x", "http://e/a\"b&c<d>\te\nf\rg")})
	if got, want := write(t, results.FormatXML, attr), `datatype="http://e/a&quot;b&amp;c&lt;d&gt;&#x9;e&#xA;f&#xD;g"`; !strings.Contains(got, want) {
		t.Errorf("XML attribute: want %q in %s", want, got)
	}
}

// Every TSV cell must parse back, as Turtle, to the term that was written.
func TestTSVCellsParseAsTurtle(t *testing.T) {
	src := trickyResult()
	for _, s := range xmlUnsafeStrings {
		src.Bindings = append(src.Bindings, row{"o": lit(s)})
	}
	src.Bindings = append(src.Bindings,
		row{"o": typed("+007", xsd+"integer")},
		row{"o": typed("12345678901234567890123", xsd+"integer")},
		row{"o": typed(".5", xsd+"decimal")},
		row{"o": typed("5.", xsd+"decimal")}, // not a Turtle DECIMAL, must stay quoted
		row{"o": typed("1e0", xsd+"double")},
		row{"o": typed("INF", xsd+"double")},
		row{"o": typed("1.5e3", xsd+"decimal")},
		row{"o": typed("true", xsd+"boolean")},
	)
	doc := write(t, results.FormatTSV, src)
	lines := strings.Split(strings.TrimSuffix(doc, "\n"), "\n")
	if len(lines) != len(src.Bindings)+1 {
		t.Fatalf("%d lines for %d rows", len(lines), len(src.Bindings))
	}
	b := newBijection()
	for i, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		for j, v := range src.Vars {
			want := src.Bindings[i][v]
			if want == nil {
				if cells[j] != "" {
					t.Errorf("row %d ?%s: unbound written as %q", i+1, v, cells[j])
				}
				continue
			}
			got := parseTurtleObject(t, cells[j])
			if !b.equal(want, got) {
				t.Errorf("row %d ?%s: %q parsed as %s, want %s", i+1, v, cells[j], describe(got), describe(want))
			}
		}
	}
}

func parseTurtleObject(t *testing.T, cell string) rdflibgo.Term {
	t.Helper()
	g := rdflibgo.NewGraph()
	doc := "<http://s> <http://p> " + cell + " ."
	if err := turtle.Parse(g, strings.NewReader(doc), turtle.WithPreserveBlankNodeIDs()); err != nil {
		t.Fatalf("cell %q does not parse as Turtle: %v", cell, err)
	}
	for tr := range g.Triples(nil, nil, nil) {
		return tr.Object
	}
	t.Fatalf("cell %q produced no triple", cell)
	return nil
}

func TestTSVRejectsUnrepresentableIRI(t *testing.T) {
	for _, term := range []rdflibgo.Term{
		iri("http://e/a b"),
		iri("http://e/<x>"),
		typed("x", "http://e/dt with space"),
		triple(iri("http://e/s"), "http://e/p", iri("http://e/{x}")),
	} {
		var buf bytes.Buffer
		err := results.WriteTSV(&buf, selectResult([]string{"a"}, row{"a": term}))
		if !errors.Is(err, results.ErrUnrepresentableIRI) || buf.Len() != 0 {
			t.Errorf("%v: err %v, %d bytes", term, err, buf.Len())
		}
		// The other formats carry it.
		for _, f := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV} {
			if err := results.Write(&buf, f, selectResult([]string{"a"}, row{"a": term})); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
}

func TestResultForms(t *testing.T) {
	ask := &sparql.Result{Type: "ASK", AskResult: true}
	construct := &sparql.Result{Type: "CONSTRUCT", Graph: rdflibgo.NewGraph()}
	for _, f := range []results.Format{results.FormatCSV, results.FormatTSV} {
		var buf bytes.Buffer
		if err := results.Write(&buf, f, ask); !errors.Is(err, results.ErrUnsupportedResultForm) || buf.Len() != 0 {
			t.Errorf("ASK as %s: %v", f, err)
		}
	}
	for _, f := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV} {
		var buf bytes.Buffer
		if err := results.Write(&buf, f, construct); !errors.Is(err, results.ErrUnsupportedResultForm) {
			t.Errorf("CONSTRUCT as %s: %v", f, err)
		}
		if err := results.Write(&buf, f, nil); !errors.Is(err, results.ErrUnsupportedResultForm) {
			t.Errorf("nil as %s: %v", f, err)
		}
		if err := results.Write(&buf, f, selectResult([]string{"a"}, row{"a": rdflibgo.NewVariable("v")})); !errors.Is(err, results.ErrUnsupportedTerm) {
			t.Errorf("Variable as %s: %v", f, err)
		}
		for _, name := range []string{"", "a b", "a,b", "?a", "a\tb", ".internal", "bad\xff"} {
			if err := results.Write(&buf, f, selectResult([]string{name})); !errors.Is(err, results.ErrInvalidVariableName) {
				t.Errorf("variable %q as %s: %v", name, f, err)
			}
		}
		if buf.Len() != 0 {
			t.Errorf("%s: output written on error", f)
		}
	}
	for _, name := range []string{"a", "_", "0abc", "ünï", "a·b", "x‿y"} {
		if err := results.Write(&bytes.Buffer{}, results.FormatXML, selectResult([]string{name})); err != nil {
			t.Errorf("variable %q: %v", name, err)
		}
	}
	if err := results.Write(&bytes.Buffer{}, results.Format(0), ask); !errors.Is(err, results.ErrUnknownFormat) {
		t.Errorf("unknown format: %v", err)
	}
	if _, err := results.NewWriter(&bytes.Buffer{}, results.Format(42), nil); !errors.Is(err, results.ErrUnknownFormat) {
		t.Errorf("unknown format: %v", err)
	}
}

func TestStreamingWriter(t *testing.T) {
	for _, f := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV} {
		t.Run(f.String(), func(t *testing.T) {
			var buf bytes.Buffer
			w, err := results.NewWriter(&buf, f, []string{"a", "b"})
			if err != nil {
				t.Fatal(err)
			}
			if buf.Len() != 0 {
				t.Fatal("constructor wrote output")
			}
			if err := w.WriteRow(row{"a": iri("http://e/1")}); err != nil {
				t.Fatal(err)
			}
			if buf.Len() != 0 {
				t.Fatal("a small row must stay buffered")
			}
			// A bad row is rejected whole and the writer carries on.
			err = w.WriteRow(row{"a": iri("http://e/ok"), "b": lit("bad\xff")})
			if !errors.Is(err, results.ErrInvalidUTF8) || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "?b") {
				t.Fatalf("bad row: %v", err)
			}
			if err := w.WriteRow(row{"b": lit("2")}); err != nil {
				t.Fatal(err)
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
			if buf.Len() == 0 {
				t.Fatal("Flush wrote nothing")
			}
			if w.Rows() != 2 {
				t.Errorf("Rows() = %d", w.Rows())
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Errorf("second Close: %v", err)
			}
			if err := w.WriteRow(row{}); !errors.Is(err, results.ErrWriterClosed) {
				t.Errorf("WriteRow after Close: %v", err)
			}
			if strings.Contains(buf.String(), "http://e/ok") {
				t.Errorf("part of the rejected row was written:\n%s", buf.String())
			}
			want := selectResult([]string{"a", "b"}, row{"a": iri("http://e/1")}, row{"b": lit("2")})
			if got := write(t, f, want); got != buf.String() {
				t.Errorf("streamed output differs from Write:\n%s\n---\n%s", buf.String(), got)
			}
		})
	}
}

func TestStreamingWriterFlushesLargeResults(t *testing.T) {
	cw := &countingWriter{}
	w, err := results.NewJSONWriter(cw, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	r := row{"a": lit(strings.Repeat("x", 1000))}
	for i := 0; i < 1000; i++ {
		if err := w.WriteRow(r); err != nil {
			t.Fatal(err)
		}
	}
	if cw.writes < 10 {
		t.Errorf("1 MB of rows reached the writer in %d writes; the buffer is not bounded", cw.writes)
	}
	if cw.max > 64<<10 {
		t.Errorf("largest write %d bytes", cw.max)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyStreamIsAValidDocument(t *testing.T) {
	for _, f := range []results.Format{results.FormatJSON, results.FormatXML} {
		var buf bytes.Buffer
		w, _ := results.NewWriter(&buf, f, []string{"x"})
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		parse := sparql.ParseSRJ
		if f == results.FormatXML {
			parse = sparql.ParseSRX
		}
		got, err := parse(&buf)
		if err != nil || len(got.Bindings) != 0 || len(got.Vars) != 1 {
			t.Errorf("%s: %v %+v", f, err, got)
		}
	}
}

type countingWriter struct{ writes, max int }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	c.max = max(c.max, len(p))
	return len(p), nil
}

type failingWriter struct{ calls int }

var errDisk = errors.New("disk full")

func (f *failingWriter) Write(p []byte) (int, error) {
	f.calls++
	return 0, errDisk
}

func TestIOErrorIsSticky(t *testing.T) {
	fw := &failingWriter{}
	w, _ := results.NewTSVWriter(fw, []string{"a"})
	if err := w.WriteRow(row{"a": lit("x")}); err != nil {
		t.Fatal(err) // still buffered
	}
	if err := w.Flush(); !errors.Is(err, errDisk) {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.WriteRow(row{"a": lit("y")}); !errors.Is(err, errDisk) {
		t.Errorf("WriteRow after I/O error: %v", err)
	}
	if err := w.Close(); !errors.Is(err, errDisk) {
		t.Errorf("Close after I/O error: %v", err)
	}
	if fw.calls != 1 {
		t.Errorf("%d writes after the failure, want none", fw.calls-1)
	}
	if err := results.WriteJSON(&failingWriter{}, &sparql.Result{Type: "ASK"}); !errors.Is(err, errDisk) {
		t.Errorf("ASK write error: %v", err)
	}
}

// A single-column CSV row with nothing in it must not become a blank line,
// which CSV readers skip.
func TestCSVSingleEmptyColumn(t *testing.T) {
	r := selectResult([]string{"a"}, row{}, row{"a": lit("")}, row{"a": lit("x")})
	got := write(t, results.FormatCSV, r)
	if want := "a\r\n\"\"\r\n\"\"\r\nx\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	recs, err := csv.NewReader(strings.NewReader(got)).ReadAll()
	if err != nil || len(recs) != 4 {
		t.Errorf("%d records, %v", len(recs), err)
	}
}
