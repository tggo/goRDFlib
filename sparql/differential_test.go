package sparql

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// TestDifferentialAgainstReferences runs the corpus in testdata/sparql-diff:
// generated queries over a deliberately awkward dataset, each with the
// result two independent engines (Oxigraph and rdflib) agree on. A query
// enters the corpus only when they agree, so a mismatch here is almost
// certainly ours. generate.py documents how to regenerate it.
//
// Comparison is exact on terms: lexical form, datatype and language, so a
// numeric result of the wrong type or lexical form fails (the W3C runner
// compares numbers by value and cannot see that). Blank nodes compare as
// "some blank node"; rows compare as a multiset.
func TestDifferentialAgainstReferences(t *testing.T) {
	dir := "../testdata/sparql-diff/"
	raw, err := os.ReadFile(dir + "queries.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Query    string `json:"query"`
		Expected struct {
			Ask  *bool                       `json:"ask"`
			Vars []string                    `json:"vars"`
			Rows []map[string]map[string]any `json:"rows"`
		} `json:"expected"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	data, err := os.Open(dir + "data.ttl")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, data); err != nil {
		t.Fatal(err)
	}

	failed := 0
	for i, c := range corpus {
		res, err := QueryContext(t.Context(), g, c.Query)
		if err != nil {
			failed++
			t.Errorf("query %d: %v\n%s", i, err, c.Query)
			continue
		}
		if c.Expected.Ask != nil {
			if res.AskResult != *c.Expected.Ask {
				failed++
				t.Errorf("query %d: ASK %v, want %v\n%s", i, res.AskResult, *c.Expected.Ask, c.Query)
			}
			continue
		}
		want := make([]string, len(c.Expected.Rows))
		for j, r := range c.Expected.Rows {
			want[j] = canonRow(r)
		}
		got := make([]string, len(res.Bindings))
		for j, r := range res.Bindings {
			got[j] = canonRow(rowJSON(r, c.Expected.Vars))
		}
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			failed++
			t.Errorf("query %d:\n%s\n got  %d rows: %v\n want %d rows: %v", i, c.Query, len(got), got, len(want), want)
		}
	}
	t.Logf("%d of %d corpus queries agree with the references", len(corpus)-failed, len(corpus))
}

// rowJSON converts a solution to the generator's term encoding, keeping only
// the projected variables.
func rowJSON(row map[string]term.Term, vars []string) map[string]map[string]any {
	out := make(map[string]map[string]any, len(vars))
	for _, v := range vars {
		t := row[v]
		if t == nil {
			continue
		}
		switch x := t.(type) {
		case term.URIRef:
			out[v] = map[string]any{"type": "uri", "value": x.Value()}
		case term.BNode:
			out[v] = map[string]any{"type": "bnode"}
		case term.Literal:
			m := map[string]any{"type": "literal", "value": x.Lexical()}
			if x.Language() != "" {
				m["lang"] = x.Language()
			} else if dt := x.Datatype().Value(); dt != "" && dt != term.XSDString.Value() {
				m["datatype"] = dt
			}
			out[v] = m
		default:
			out[v] = map[string]any{"type": "other", "value": t.N3()}
		}
	}
	return out
}

func canonRow(r map[string]map[string]any) string {
	b, _ := json.Marshal(r) // encoding/json sorts map keys
	return string(b)
}
