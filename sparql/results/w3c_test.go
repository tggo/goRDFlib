package results_test

import (
	"bytes"
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/turtle"
)

const w3cSPARQL = "../../testdata/w3c/rdf-tests/sparql"

// TestW3CCSVTSV runs the SPARQL 1.1 CSV/TSV result format tests
// (sparql11/csv-tsv-res/manifest.ttl): evaluate the query on the data,
// write CSV or TSV, compare with the expected file.
//
// The comparison is on the table, not the bytes, where the format allows
// more than one spelling of the same result:
//   - columns are matched by header name, because SELECT * leaves the
//     variable order to the implementation and ours sorts it (select_star.go);
//   - blank node labels are compared up to one consistent renaming;
//   - CSV is read with encoding/csv, so CRLF (which the spec requires and we
//     write) and the LF of the expected files compare equal;
//   - an unquoted TSV number compares case-insensitively, since 1.0E6 and
//     1.0e6 are the same Turtle DOUBLE token and we keep the lexical form.
//
// Everything else must match exactly, including the quoting of every
// literal, datatype IRIs and unbound cells.
func TestW3CCSVTSV(t *testing.T) {
	dir := filepath.Join(w3cSPARQL, "sparql11/csv-tsv-res")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("W3C rdf-tests submodule not checked out")
	}
	tests := []struct {
		name, query, data, result string
		format                    results.Format
	}{
		{"csv01", "csvtsv01.rq", "data.ttl", "csvtsv01.csv", results.FormatCSV},
		{"tsv01", "csvtsv01.rq", "data.ttl", "csvtsv01.tsv", results.FormatTSV},
		{"csv02", "csvtsv02.rq", "data.ttl", "csvtsv02.csv", results.FormatCSV},
		{"tsv02", "csvtsv02.rq", "data.ttl", "csvtsv02.tsv", results.FormatTSV},
		{"csv03", "csvtsv01.rq", "data2.ttl", "csvtsv03.csv", results.FormatCSV},
		{"tsv03", "csvtsv01.rq", "data2.ttl", "csvtsv03.tsv", results.FormatTSV},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := rdflibgo.NewGraph()
			dataPath := filepath.Join(dir, tc.data)
			f, err := os.Open(dataPath)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := turtle.Parse(g, f, turtle.WithBase("file://"+dataPath)); err != nil {
				t.Fatal(err)
			}
			q, err := os.ReadFile(filepath.Join(dir, tc.query))
			if err != nil {
				t.Fatal(err)
			}
			r, err := sparql.Query(g, string(q))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(dir, tc.result))
			if err != nil {
				t.Fatal(err)
			}
			got := write(t, tc.format, r)
			wantTable, gotTable := readTable(t, tc.format, string(want)), readTable(t, tc.format, got)
			compareTables(t, tc.format, wantTable, gotTable)
			if t.Failed() {
				t.Logf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func readTable(t *testing.T, f results.Format, doc string) [][]string {
	t.Helper()
	if f == results.FormatCSV {
		rd := csv.NewReader(strings.NewReader(doc))
		rd.FieldsPerRecord = -1
		recs, err := rd.ReadAll()
		if err != nil {
			t.Fatalf("CSV does not parse: %v\n%s", err, doc)
		}
		return recs
	}
	var out [][]string
	for _, line := range strings.Split(strings.TrimSuffix(doc, "\n"), "\n") {
		out = append(out, strings.Split(line, "\t"))
	}
	return out
}

func compareTables(t *testing.T, f results.Format, want, got [][]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	col := map[string]int{}
	for i, name := range got[0] {
		col[name] = i
	}
	if len(got[0]) != len(want[0]) {
		t.Fatalf("header %v, want %v", got[0], want[0])
	}
	b := map[string]string{}
	for i := 1; i < len(want); i++ {
		if len(got[i]) != len(want[0]) {
			t.Fatalf("line %d has %d fields, want %d", i+1, len(got[i]), len(want[0]))
		}
		for j, name := range want[0] {
			k, ok := col[name]
			if !ok {
				t.Fatalf("column %s missing from header %v", name, got[0])
			}
			w, g := want[i][j], got[i][k]
			switch {
			case w == g && !strings.HasPrefix(w, "_:"):
			case strings.HasPrefix(w, "_:") && strings.HasPrefix(g, "_:"):
				if m, seen := b[w]; seen && m != g {
					t.Errorf("line %d %s: blank node %s, earlier matched %s", i+1, name, g, m)
				}
				b[w] = g
			case f == results.FormatTSV && isBareToken(w) && strings.EqualFold(w, g):
			default:
				t.Errorf("line %d %s: got %q, want %q", i+1, name, g, w)
			}
		}
	}
}

func isBareToken(s string) bool {
	return s != "" && !strings.ContainsAny(s[:1], `"'<_`)
}

// TestW3CResultFilesRoundTrip reads every .srj and .srx expected result in
// the SPARQL 1.1 and 1.2 suites (the 1.2 ones carry triple terms and
// its:dir literals), writes it back as JSON and XML, parses that again, and
// requires the identical result. CSV and TSV output of the same results must
// write without error and have one line per row.
func TestW3CResultFilesRoundTrip(t *testing.T) {
	if _, err := os.Stat(w3cSPARQL); err != nil {
		t.Skip("W3C rdf-tests submodule not checked out")
	}
	var files, withTriples, withDir int
	for _, suite := range []string{"sparql11", "sparql12"} {
		err := filepath.WalkDir(filepath.Join(w3cSPARQL, suite), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			ext := filepath.Ext(path)
			if d.IsDir() || (ext != ".srj" && ext != ".srx") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			parse := sparql.ParseSRX
			if ext == ".srj" {
				parse = sparql.ParseSRJ
			}
			src, err := parse(bytes.NewReader(raw))
			if err != nil {
				return nil // a few files are deliberately malformed or non-result documents
			}
			files++
			if bytes.Contains(raw, []byte("triple")) {
				withTriples++
			}
			if bytes.Contains(raw, []byte("its:dir")) {
				withDir++
			}
			rel, _ := filepath.Rel(w3cSPARQL, path)
			for _, rt := range []struct {
				f     results.Format
				parse func(r *bytes.Reader) (*sparql.Result, error)
			}{
				{results.FormatJSON, func(r *bytes.Reader) (*sparql.Result, error) { return sparql.ParseSRJ(r) }},
				{results.FormatXML, func(r *bytes.Reader) (*sparql.Result, error) { return sparql.ParseSRX(r) }},
			} {
				var buf bytes.Buffer
				if err := results.Write(&buf, rt.f, src); err != nil {
					t.Errorf("%s: write %s: %v", rel, rt.f, err)
					continue
				}
				back, err := rt.parse(bytes.NewReader(buf.Bytes()))
				if err != nil {
					t.Errorf("%s: reparse %s: %v", rel, rt.f, err)
					continue
				}
				if err := sameResult(src, back); err != nil {
					t.Errorf("%s via %s: %v", rel, rt.f, err)
				}
			}
			if src.Type != "SELECT" {
				return nil
			}
			for _, f := range []results.Format{results.FormatCSV, results.FormatTSV} {
				if len(src.Vars) == 0 {
					continue // every line is empty; nothing to count
				}
				var buf bytes.Buffer
				if err := results.Write(&buf, f, src); err != nil {
					t.Errorf("%s: write %s: %v", rel, f, err)
					continue
				}
				if f == results.FormatTSV {
					if n := strings.Count(buf.String(), "\n"); n != len(src.Bindings)+1 {
						t.Errorf("%s: TSV has %d lines, want %d", rel, n, len(src.Bindings)+1)
					}
				} else {
					rd := csv.NewReader(&buf)
					rd.FieldsPerRecord = len(src.Vars)
					recs, err := rd.ReadAll()
					if err != nil {
						t.Errorf("%s: CSV does not parse: %v", rel, err)
					} else if len(recs) != len(src.Bindings)+1 {
						t.Errorf("%s: CSV has %d records, want %d", rel, len(recs), len(src.Bindings)+1)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d result files round-tripped (%d with triple terms, %d with its:dir)", files, withTriples, withDir)
	if files < 100 || withTriples == 0 || withDir == 0 {
		t.Errorf("suspiciously few files: %d total, %d with triple terms, %d with its:dir", files, withTriples, withDir)
	}
}
