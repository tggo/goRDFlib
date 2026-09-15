package endpoint_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// slowGraph has n subjects with one ex:p and one ex:q literal each.
// slowQuery compares every ex:p row with every ex:q row (MINUS without a shared
// variable): 10^8 comparisons at n = 10 000, many seconds of work that
// produces no rows, so memory stays flat while it runs.
func slowGraph(n int) *graph.Graph {
	g := graph.NewGraph()
	for i := range n {
		s := iri(fmt.Sprintf("%ss%d", ex, i))
		g.Add(s, iri(ex+"p"), term.NewLiteral(fmt.Sprintf("v%d", i)))
		g.Add(s, iri(ex+"q"), term.NewLiteral(fmt.Sprintf("w%d", i)))
	}
	return g
}

const slowQuery = `PREFIX ex: <http://example.org/>
SELECT ?a WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } }`

// hookChan delivers RequestInfo values to a test.
func hookChan() (chan endpoint.RequestInfo, endpoint.Option) {
	ch := make(chan endpoint.RequestInfo, 64)
	return ch, endpoint.WithRequestHook(func(_ *http.Request, info endpoint.RequestInfo) { ch <- info })
}

func waitInfo(t *testing.T, ch chan endpoint.RequestInfo, within time.Duration) endpoint.RequestInfo {
	t.Helper()
	select {
	case info := <-ch:
		return info
	case <-time.After(within):
		t.Fatalf("the handler did not finish within %v", within)
		return endpoint.RequestInfo{}
	}
}

// When the client disconnects, evaluation stops: the handler returns long
// before the query could have finished, and the lock is free again.
func TestClientDisconnectCancelsEvaluation(t *testing.T) {
	ch, hook := hookChan()
	ts, _ := serve(t, &sparql.Dataset{Default: slowGraph(10_000)}, hook)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL, strings.NewReader(slowQuery))
	req.Header.Set("Content-Type", "application/sparql-query")
	errc := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		errc <- err
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	start := time.Now()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("client error = %v", err)
	}
	info := waitInfo(t, ch, 3*time.Second)
	if info.Status != endpoint.StatusClientClosedRequest {
		t.Errorf("status = %d, want %d", info.Status, endpoint.StatusClientClosedRequest)
	}
	if stopped := time.Since(start); stopped > 2*time.Second {
		t.Errorf("evaluation ran %v after the disconnect", stopped)
	}
	// An update needs the write lock the query held.
	wantStatus(t, postUpdate(t, ts.URL, `INSERT DATA { <a:x> <a:y> <a:z> }`), http.StatusNoContent)
}

func TestQueryTimeout(t *testing.T) {
	ch, hook := hookChan()
	ts, _ := serve(t, &sparql.Dataset{Default: slowGraph(10_000)}, hook, endpoint.WithQueryTimeout(50*time.Millisecond))
	start := time.Now()
	r := postQuery(t, ts.URL, slowQuery)
	wantStatus(t, r, http.StatusServiceUnavailable)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("a 50ms timeout answered after %v", elapsed)
	}
	if !strings.Contains(r.body, "time limit") || r.header.Get("Retry-After") != "" {
		t.Errorf("body %q, Retry-After %q", r.body, r.header.Get("Retry-After"))
	}
	info := waitInfo(t, ch, time.Second)
	if !errors.Is(info.Err, endpoint.ErrTimeout) {
		t.Errorf("hook error = %v, want ErrTimeout", info.Err)
	}
}

// A query waiting for the lock behind a long update still honours its
// timeout, and an update waiting behind a long query honours its own.
func TestTimeoutWhileWaitingForTheLock(t *testing.T) {
	ds := &sparql.Dataset{Default: slowGraph(10_000)}
	ts, h := serve(t, ds, endpoint.WithQueryTimeout(100*time.Millisecond), endpoint.WithUpdateTimeout(100*time.Millisecond))

	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		_ = h.Write(context.Background(), func() { close(held); <-release })
	}()
	<-held
	start := time.Now()
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}), http.StatusServiceUnavailable)
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL"), http.StatusServiceUnavailable)
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"x"), "text/turtle", "")), http.StatusServiceUnavailable)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waiting requests took %v", elapsed)
	}
	close(release)
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}), http.StatusOK)
	if ds.Default.Len() != 20_000 {
		t.Fatal("a timed-out update was applied")
	}
}

// Updates never interleave with a query's evaluation. The invariant: every
// update moves one ex:balance between two accounts by deleting and inserting
// in one request, so the total seen by any query is constant.
func TestConcurrentQueriesAndUpdatesAreIsolated(t *testing.T) {
	const accounts = 20
	def := graph.NewGraph()
	for i := range accounts {
		def.Add(iri(fmt.Sprintf("%sacct%d", ex, i)), iri(ex+"balance"), term.NewLiteral(100))
	}
	ds := &sparql.Dataset{Default: def}
	ts, _ := serve(t, ds)
	const total = accounts * 100

	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	stop := make(chan struct{})
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 64}}
	send := func(req *http.Request) (int, string, error) {
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b), err
	}

	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				from, to := (w+i)%accounts, (w+i+7)%accounts
				upd := fmt.Sprintf(`PREFIX ex: <%s>
DELETE { ex:acct%d ex:balance ?a . ex:acct%d ex:balance ?b }
INSERT { ex:acct%d ex:balance ?a2 . ex:acct%d ex:balance ?b2 }
WHERE { ex:acct%d ex:balance ?a . ex:acct%d ex:balance ?b BIND(?a - 1 AS ?a2) BIND(?b + 1 AS ?b2) }`,
					ex, from, to, from, to, from, to)
				req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(upd))
				req.Header.Set("Content-Type", "application/sparql-update")
				if code, body, err := send(req); err != nil || code != http.StatusNoContent {
					errs <- fmt.Errorf("update: %d %s %v", code, body, err)
					return
				}
			}
		}()
	}
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				q := url.Values{"query": {`SELECT (SUM(?b) AS ?t) (COUNT(?b) AS ?n) WHERE { ?a <` + ex + `balance> ?b }`}}
				req, _ := http.NewRequest("GET", ts.URL+"?"+q.Encode(), nil)
				code, body, err := send(req)
				if err != nil || code != 200 {
					errs <- fmt.Errorf("query: %d %s %v", code, body, err)
					return
				}
				want := fmt.Sprintf(`"value":"%d"`, total)
				wantN := fmt.Sprintf(`"value":"%d"`, accounts)
				if !strings.Contains(body, want) || !strings.Contains(body, wantN) {
					errs <- fmt.Errorf("a query saw a half-applied update: %s", body)
					return
				}
			}
		}()
	}
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				g := fmt.Sprintf("%sside%d", ex, i%3)
				req, _ := http.NewRequest("PUT", graphURL(ts.URL, g), strings.NewReader(`<a:a> <a:b> "x" .`))
				req.Header.Set("Content-Type", "text/turtle")
				if code, body, err := send(req); err != nil || code >= 300 {
					errs <- fmt.Errorf("gsp put: %d %s %v", code, body, err)
					return
				}
				req, _ = http.NewRequest("GET", graphURL(ts.URL, g), nil)
				if code, body, err := send(req); err != nil || code != 200 {
					errs <- fmt.Errorf("gsp get: %d %s %v", code, body, err)
					return
				}
			}
		}()
	}
	time.Sleep(700 * time.Millisecond)
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
