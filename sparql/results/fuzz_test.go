package results_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

var sentinels = []error{
	results.ErrInvalidUTF8,
	results.ErrUnrepresentableXMLChar,
	results.ErrUnrepresentableIRI,
}

// fuzzResult builds a result from fuzzed strings: a plain, a language-tagged
// (with and without direction) and a typed literal, an IRI, blank nodes and
// triple terms nesting all of them.
func fuzzResult(lexical, lang, datatype, iriSuffix string, dir bool) *sparql.Result {
	opts := []rdflibgo.LiteralOption{rdflibgo.WithLang(lang)}
	// A direction without a language tag is not an RDF 1.2 literal, but
	// term.WithDir does not refuse it; Literal.N3 and these writers ignore it.
	if dir && lang != "" && rdflibgo.ValidLanguageTag(lang) {
		opts = append(opts, rdflibgo.WithDir("rtl"))
	}
	u := iri("http://e/" + iriSuffix)
	dt := lit(lexical)
	if datatype != "" {
		dt = typed(lexical, "http://e/"+datatype)
	}
	b := bnode("b")
	return selectResult([]string{"s", "p", "o"},
		row{"s": u, "p": lit(lexical), "o": lit(lexical, opts...)},
		row{"s": b, "o": dt},
		row{"p": triple(u, "http://e/p", lit(lexical, opts...)), "o": triple(b, "http://e/q", triple(u, u.Value(), dt))},
		row{},
	)
}

func FuzzWriters(f *testing.F) {
	for _, s := range append(append([]string{}, trickyStrings...), xmlUnsafeStrings...) {
		f.Add(s, "en", "", "x", false)
	}
	f.Add("1.0E6", "", "dt", "a b", true)
	f.Add("x", "ar", "", "ok", true)
	f.Add("\xff", "", "", "\xfe", false)
	f.Add("\r\n\"", "EN-gb", "a\"b", "<>", false)
	f.Fuzz(func(t *testing.T, lexical, lang, datatype, iriSuffix string, dir bool) {
		r := fuzzResult(lexical, lang, datatype, iriSuffix, dir)
		for _, format := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV} {
			var buf bytes.Buffer
			err := results.Write(&buf, format, r)

			// The streaming writer must reject exactly the rows the
			// pre-check rejects, with the same error.
			sw, _ := results.NewWriter(io.Discard, format, r.Vars)
			var streamErr error
			for _, rw := range r.Bindings {
				if e := sw.WriteRow(rw); e != nil && streamErr == nil {
					streamErr = e
				}
			}
			if (err == nil) != (streamErr == nil) {
				t.Fatalf("%s: Write error %v, streaming error %v", format, err, streamErr)
			}

			if err != nil {
				if buf.Len() != 0 {
					t.Fatalf("%s: %d bytes written before error %v", format, buf.Len(), err)
				}
				known := false
				for _, s := range sentinels {
					known = known || (errors.Is(err, s) && errors.Is(streamErr, s))
				}
				if !known {
					t.Fatalf("%s: unexpected error %v / %v", format, err, streamErr)
				}
				continue
			}
			doc := buf.Bytes()
			switch format {
			case results.FormatJSON:
				if !json.Valid(doc) {
					t.Fatalf("invalid JSON:\n%s", doc)
				}
				back, err := sparql.ParseSRJ(bytes.NewReader(doc))
				if err != nil {
					t.Fatal(err)
				}
				if err := sameResult(r, back); err != nil {
					t.Fatalf("JSON round trip: %v\n%s", err, doc)
				}
			case results.FormatXML:
				dec := xml.NewDecoder(bytes.NewReader(doc))
				for {
					_, err := dec.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatalf("XML not well-formed: %v\n%s", err, doc)
					}
				}
				back, err := sparql.ParseSRX(bytes.NewReader(doc))
				if err != nil {
					t.Fatal(err)
				}
				if err := sameResult(r, back); err != nil {
					t.Fatalf("XML round trip: %v\n%s", err, doc)
				}
			case results.FormatCSV:
				rd := csv.NewReader(bytes.NewReader(doc))
				rd.FieldsPerRecord = len(r.Vars)
				recs, err := rd.ReadAll()
				if err != nil {
					t.Fatalf("CSV does not parse: %v\n%q", err, doc)
				}
				if len(recs) != len(r.Bindings)+1 {
					t.Fatalf("CSV has %d records, want %d", len(recs), len(r.Bindings)+1)
				}
				// encoding/csv drops the CR of a CRLF inside a quoted field.
				want := strings.ReplaceAll(lexical, "\r\n", "\n")
				if recs[1][1] != want || recs[1][0] != "http://e/"+strings.ReplaceAll(iriSuffix, "\r\n", "\n") {
					t.Fatalf("CSV cells %q, want %q", recs[1], want)
				}
			case results.FormatTSV:
				lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")
				if len(lines) != len(r.Bindings)+1 {
					t.Fatalf("TSV has %d lines, want %d:\n%s", len(lines), len(r.Bindings)+1, doc)
				}
				for _, line := range lines {
					if n := strings.Count(line, "\t"); n != len(r.Vars)-1 {
						t.Fatalf("TSV line with %d tabs: %q", n, line)
					}
				}
				cells := strings.Split(lines[1], "\t")
				for i, v := range []string{"p", "o"} {
					if got := parseTurtleObject(t, cells[i+1]); !got.Equal(r.Bindings[0][v]) {
						t.Fatalf("TSV cell %q parsed as %s, want %s", cells[i+1], describe(got), describe(r.Bindings[0][v]))
					}
				}
			}
		}
	})
}
