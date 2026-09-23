package endpoint

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestScanForm(t *testing.T) {
	cases := []struct {
		query string
		form  string
	}{
		{`SELECT * WHERE { ?s ?p ?o }`, "SELECT"},
		{`select * from <g> where {}`, "SELECT"},
		{`PREFIX ex: <http://ex.org/> BASE <http://other.example/>
		  ASK FROM ex:a FROM NAMED ex:b {}`, "ASK"},
		{`PREFIX : <http://empty/> SELECT * FROM :g {}`, "SELECT"},
		{`CONSTRUCT { ?s <from> ?o } WHERE { ?s ?p ?o }`, "CONSTRUCT"},
		{`VERSION "1.2" DESCRIBE <x>`, "DESCRIBE"},
		{`# DESCRIBE <x>
		  SELECT * WHERE { ?s ?p "DESCRIBE <x>" }`, "SELECT"},
		{`INSERT DATA { <a> <b> <c> }`, "INSERT"},
		{``, ""},
		{`<a> "b"`, ""},
	}
	for _, c := range cases {
		if form := scanForm(c.query); form != c.form {
			t.Errorf("scanForm(%q) = %q; want %q", c.query, form, c.form)
		}
	}
}

// FuzzScanForm checks that the scanner terminates and never panics on
// arbitrary input.
func FuzzScanForm(f *testing.F) {
	for _, s := range []string{`SELECT * FROM <g> {}`, `"""`, `'`, `<`, `PREFIX`, `FROM NAMED`, "\\", `?`, `ex:a.`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		scanForm(s)
	})
}

func TestWithLineColumn(t *testing.T) {
	text := "SELECT ?x\nWHERE {\n  bad"
	pos := strings.Index(text, "bad")
	got := withLineColumn(fmt.Sprintf("sparql parse error at pos %d: boom", pos), text)
	if !strings.HasSuffix(got, "(line 3, column 3)") {
		t.Fatalf("got %q", got)
	}
	if got := withLineColumn("no position", text); got != "no position" {
		t.Fatalf("got %q", got)
	}
	if got := withLineColumn("at pos 999", "ab"); !strings.HasSuffix(got, "(line 1, column 3)") {
		t.Fatalf("a position past the end: %q", got)
	}
}

func TestNegotiate(t *testing.T) {
	cases := []struct {
		accept string
		want   string
	}{
		{"", "text/turtle"},
		{"*/*", "text/turtle"},
		{"application/n-triples", "application/n-triples"},
		{"text/*", "text/turtle"},
		{"application/json", "application/ld+json"},
		{"application/rdf+xml;q=0.9, text/turtle;q=0.1", "application/rdf+xml"},
		{"text/turtle;q=0, */*", "application/n-triples"},
		{"text/turtle;q=abc, application/trig", "application/trig"},
		{"image/png", ""},
		{"TEXT/TURTLE", "text/turtle"},
	}
	for _, c := range cases {
		f := negotiateRDF(c.accept)
		got := ""
		if f != nil {
			got = f.mediaType
		}
		if got != c.want {
			t.Errorf("negotiateRDF(%q) = %q, want %q", c.accept, got, c.want)
		}
	}
}

func TestRWLock(t *testing.T) {
	l := newRWLock()
	ctx := context.Background()

	// Readers share.
	if err := l.rlock(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.rlock(ctx); err != nil {
		t.Fatal(err)
	}

	// A writer waits for readers and gives up with its context.
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := l.lock(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock with readers inside = %v", err)
	}
	// A writer that gave up does not keep new readers out.
	if err := l.rlock(ctx); err != nil {
		t.Fatal(err)
	}
	l.runlock()

	// A waiting writer holds back new readers (no writer starvation).
	var writerIn atomic.Bool
	done := make(chan struct{})
	go func() {
		if err := l.lock(ctx); err != nil {
			t.Error(err)
		}
		writerIn.Store(true)
		l.unlock()
		close(done)
	}()
	waitFor(t, func() bool { l.mu.Lock(); defer l.mu.Unlock(); return l.writersWaiting == 1 })
	blocked, cancel2 := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel2()
	if err := l.rlock(blocked); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a new reader got in ahead of a waiting writer: %v", err)
	}
	l.runlock()
	l.runlock()
	<-done
	if !writerIn.Load() {
		t.Fatal("writer never ran")
	}

	// Exclusion under load.
	var inside, maxInside atomic.Int32
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%5 == 0 {
				_ = l.lock(ctx)
				if n := inside.Add(100); n != 100 {
					t.Errorf("writer shares the lock: %d", n)
				}
				inside.Add(-100)
				l.unlock()
				return
			}
			_ = l.rlock(ctx)
			n := inside.Add(1)
			if n >= 100 {
				t.Errorf("reader inside with a writer: %d", n)
			}
			for m := maxInside.Load(); n > m && !maxInside.CompareAndSwap(m, n); m = maxInside.Load() {
			}
			time.Sleep(time.Millisecond)
			inside.Add(-1)
			l.runlock()
		}()
	}
	wg.Wait()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(time.Millisecond)
	}
}
