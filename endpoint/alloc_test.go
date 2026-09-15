package endpoint_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/sparql"
)

// The handler must not allocate per result row beyond what the engine does:
// serving 10x the rows may cost at most 10x the engine's allocations plus a
// constant. This guards the streaming path against a per-row buffer or map.
func TestSelectAllocationsDoNotScalePerRow(t *testing.T) {
	if testing.Short() {
		t.Skip("allocation measurement")
	}
	measure := func(rows int) (handler, engine float64) {
		g := benchGraph(rows)
		h := endpoint.New(&sparql.Dataset{Default: g})
		req := httptest.NewRequest(http.MethodGet, "/?query="+url.QueryEscape(benchQuery), nil)
		w := &discard{h: http.Header{}}
		handler = testing.AllocsPerRun(3, func() { clear(w.h); h.ServeHTTP(w, req) })
		q, _ := sparql.Parse(benchQuery)
		engine = testing.AllocsPerRun(3, func() { _, _ = sparql.EvalQuery(g, q, nil) })
		return handler, engine
	}
	h1, e1 := measure(1_000)
	h2, e2 := measure(20_000)
	over1, over2 := h1-e1, h2-e2
	t.Logf("handler overhead: %.0f allocs at 1k rows, %.0f at 20k rows", over1, over2)
	if over2 > over1+50 {
		t.Fatalf("the handler's own allocations grow with the result: %.0f at 1k rows, %.0f at 20k rows", over1, over2)
	}
}
