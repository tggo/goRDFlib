package results_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

const xsd = "http://www.w3.org/2001/XMLSchema#"

// trickyStrings are lexical forms every format must carry. U+0000 and the
// other C0 controls are legal in an RDF literal but not in XML 1.0, so they
// are in xmlUnsafeStrings instead.
var trickyStrings = []string{
	"",
	"plain",
	`quote " inside`,
	`"leading and trailing"`,
	`back\slash \n not a newline`,
	"new\nline",
	"carriage\rreturn",
	"crlf\r\n",
	"tab\tseparated",
	"comma, separated",
	"semi;colon",
	"  spaces  ",
	"<tag attr=\"v\">&amp;</tag>",
	"]]> cdata end",
	"'single'",
	"unicode: Привіт, 日本語, emoji \U0001F9AB, combining é",
	"U+2028[\u2028] U+2029[\u2029]",
	"DEL[\x7f] U+0085[\u0085]",
	"replacement char � is valid",
	`"""triple quotes"""`,
	"<<( not a triple term )>>",
	"_:b0",
}

var xmlUnsafeStrings = []string{
	"nul[\x00]",
	"bell[\x07] backspace[\b] formfeed[\f] vt[\x0b] esc[\x1b]",
	"nonchar[\uFFFE]",
}

func trickyResult() *sparql.Result {
	vars := []string{"s", "o", "unbound", "t"}
	var rows []row
	b1, b2 := bnode("Nabc"), bnode("x")
	for _, s := range trickyStrings {
		rows = append(rows,
			row{"s": iri("http://example.org/s"), "o": lit(s)},
			row{"s": b1, "o": lit(s, rdflibgo.WithLang("en-GB"))},
			row{"s": b2, "o": lit(s, rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl"))},
			row{"o": typed(s, "http://example.org/dt#custom")},
			row{"o": typed(s, xsd+"string"), "t": triple(b1, "http://example.org/p", lit(s, rdflibgo.WithLang("en"), rdflibgo.WithDir("ltr")))},
		)
	}
	rows = append(rows,
		row{},
		row{"s": nil, "o": nil},
		row{"s": iri("http://example.org/ünïcödé?q=a&b=c#frag"), "o": typed("42", xsd+"integer")},
		row{"o": typed("-0.5", xsd+"decimal"), "t": triple(iri("http://e/s"), "http://e/p", triple(b2, "http://e/q", b1))},
		row{"o": typed("1.0E6", xsd+"double"), "t": triple(b1, "http://e/p", b1)},
		row{"s": b1, "o": b2, "unbound": nil, "t": triple(bnode("inner"), "http://e/p", typed("x", "http://e/dt"))},
		row{"s": bnode("a label with spaces and \"quotes\"")},
	)
	return selectResult(vars, rows...)
}

func TestRoundTripJSON(t *testing.T) {
	want := trickyResult()
	doc := write(t, results.FormatJSON, want)
	if !json.Valid([]byte(doc)) {
		t.Fatalf("invalid JSON:\n%s", doc)
	}
	got, err := sparql.ParseSRJ(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := sameResult(want, got); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTripXML(t *testing.T) {
	want := trickyResult()
	doc := write(t, results.FormatXML, want)
	got, err := sparql.ParseSRX(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("%v\n%s", err, doc)
	}
	if err := sameResult(want, got); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTripControlCharactersJSON(t *testing.T) {
	var rows []row
	for _, s := range xmlUnsafeStrings {
		rows = append(rows, row{"o": lit(s)})
	}
	var all strings.Builder
	for c := 0; c < 0x20; c++ {
		all.WriteByte(byte(c))
	}
	rows = append(rows, row{"o": lit(all.String())})
	want := selectResult([]string{"o"}, rows...)

	doc := write(t, results.FormatJSON, want)
	got, err := sparql.ParseSRJ(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := sameResult(want, got); err != nil {
		t.Fatal(err)
	}
	for c := 0; c < 0x20; c++ {
		if bytes.IndexByte([]byte(doc), byte(c)) >= 0 && c != '\n' {
			t.Errorf("raw control character %#x in JSON output", c)
		}
	}
}

func TestXMLRejectsUnrepresentableCharacters(t *testing.T) {
	for _, s := range xmlUnsafeStrings {
		for _, term := range []rdflibgo.Term{lit(s), iri("http://e/" + s), typed("x", "http://e/"+s), triple(iri("http://e/s"), "http://e/p", lit(s))} {
			var buf bytes.Buffer
			err := results.WriteXML(&buf, selectResult([]string{"a", "b"}, row{"a": iri("http://ok")}, row{"b": term}))
			if !errors.Is(err, results.ErrUnrepresentableXMLChar) {
				t.Errorf("%q: error %v, want ErrUnrepresentableXMLChar", s, err)
			}
			if buf.Len() != 0 {
				t.Errorf("%q: %d bytes written before the error", s, buf.Len())
			}
		}
	}
}

func TestInvalidUTF8IsAnErrorInEveryFormat(t *testing.T) {
	bad := "bad\xff"
	for _, f := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV} {
		for _, term := range []rdflibgo.Term{lit(bad), iri("http://e/" + bad), triple(iri("http://e/s"), "http://e/p", lit(bad))} {
			var buf bytes.Buffer
			err := results.Write(&buf, f, selectResult([]string{"a"}, row{"a": term}))
			if !errors.Is(err, results.ErrInvalidUTF8) {
				t.Errorf("%s %v: error %v, want ErrInvalidUTF8", f, term, err)
			}
			if buf.Len() != 0 {
				t.Errorf("%s: output written before the error", f)
			}
		}
	}
}

func TestRoundTripASK(t *testing.T) {
	for _, v := range []bool{true, false} {
		want := &sparql.Result{Type: "ASK", AskResult: v}
		j := write(t, results.FormatJSON, want)
		if !json.Valid([]byte(j)) {
			t.Fatalf("invalid JSON %s", j)
		}
		got, err := sparql.ParseSRJ(strings.NewReader(j))
		if err != nil || sameResult(want, got) != nil {
			t.Fatalf("JSON %v: %v %v", v, err, j)
		}
		x := write(t, results.FormatXML, want)
		got, err = sparql.ParseSRX(strings.NewReader(x))
		if err != nil || sameResult(want, got) != nil {
			t.Fatalf("XML %v: %v %v", v, err, x)
		}
	}
}

// Variables come out in projection order, not sorted or map order.
func TestVariableOrderIsProjectionOrder(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(iri("http://e/s"), iri("http://e/p"), lit("o"))
	r, err := sparql.Query(g, `SELECT ?z ?a ?m WHERE { ?a ?m ?z }`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[results.Format]string{
		results.FormatJSON: `{"head":{"vars":["z","a","m"]}`,
		results.FormatCSV:  "z,a,m\r\n",
		results.FormatTSV:  "?z\t?a\t?m\n",
		results.FormatXML:  `<variable name="z"/>` + "\n" + `    <variable name="a"/>` + "\n" + `    <variable name="m"/>`,
	}
	for f, prefix := range want {
		if doc := write(t, f, r); !strings.Contains(doc, prefix) {
			t.Errorf("%s: want %q in\n%s", f, prefix, doc)
		}
	}
}

func TestBlankNodeLabelsStableWithinDocument(t *testing.T) {
	a, b := bnode("N0123456789abcdef"), bnode("other")
	r := selectResult([]string{"x", "y"},
		row{"x": a, "y": b},
		row{"x": b, "y": triple(a, "http://e/p", b)},
		row{"x": a},
	)
	if got, want := write(t, results.FormatTSV, r), "?x\t?y\n_:b0\t_:b1\n_:b1\t<<( _:b0 <http://e/p> _:b1 )>>\n_:b0\t\n"; got != want {
		t.Errorf("TSV\ngot  %q\nwant %q", got, want)
	}
	if got, want := write(t, results.FormatCSV, r), "x,y\r\n_:b0,_:b1\r\n_:b1,<<( _:b0 http://e/p _:b1 )>>\r\n_:b0,\r\n"; got != want {
		t.Errorf("CSV\ngot  %q\nwant %q", got, want)
	}
	// Labels restart per document.
	if a, b := write(t, results.FormatTSV, r), write(t, results.FormatTSV, r); a != b {
		t.Error("two writes of one result differ")
	}
}
