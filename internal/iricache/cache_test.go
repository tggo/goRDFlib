package iricache

import (
	"strconv"
	"testing"
	"unsafe"
)

func TestInternShares(t *testing.T) {
	c := New()
	a := c.Intern("http://example.org/p")
	b := c.Intern("http://example.org/" + "p")
	if a != b || unsafe.StringData(a.Value()) != unsafe.StringData(b.Value()) {
		t.Fatal("second Intern built a new URIRef")
	}
}

func TestNilCacheCachesNothing(t *testing.T) {
	var c *Cache
	if u := c.Intern("http://example.org/p"); u.Value() != "http://example.org/p" {
		t.Fatalf("Intern on nil cache = %q", u.Value())
	}
	if _, ok := c.Get("http://example.org/p"); ok {
		t.Fatal("nil cache reported a hit")
	}
}

func TestCacheIsBounded(t *testing.T) {
	c := New()
	for i := range 2 * maxEntries {
		c.Intern("http://example.org/p" + strconv.Itoa(i))
	}
	if len(c.m) != maxEntries {
		t.Fatalf("cache holds %d entries, bound is %d", len(c.m), maxEntries)
	}
	// Past the bound, IRIs are still built correctly, just not shared.
	if u := c.Intern("http://example.org/late"); u.Value() != "http://example.org/late" {
		t.Fatalf("Intern past the bound = %q", u.Value())
	}
}
