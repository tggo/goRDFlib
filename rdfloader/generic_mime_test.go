package rdfloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/rdfloader"
)

// Issue #43: documents served as text/plain were always parsed as N-Triples.
func TestLoadHTTPTextPlain(t *testing.T) {
	tests := []struct {
		name, body string
		want       int
	}{
		{"turtle", "@prefix ex: <http://example.org/> .\nex:s ex:p ex:o ; ex:q \"x\" .\n", 2},
		{"turtle without directive", "<http://example.org/s> a <http://example.org/C> .\n", 1},
		{"json-ld", `{"@id": "http://example.org/s", "http://example.org/p": "o"}`, 1},
		{"n-triples", "<http://example.org/s> <http://example.org/p> <http://example.org/o> .\n", 1},
		{"comments only falls back to n-triples", "# only\n# comments\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			g := graph.NewGraph()
			if err := rdfloader.DefaultLoader().Load(context.Background(), g, srv.URL+"/data"); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if g.Len() != tt.want {
				t.Errorf("got %d triples, want %d", g.Len(), tt.want)
			}
		})
	}
}
