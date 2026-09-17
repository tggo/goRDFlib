package iri

import "testing"

func TestRelativize(t *testing.T) {
	tests := []struct {
		base, target, want string
	}{
		{"http://localhost/doc.ttl", "http://localhost/doc.ttl#local/ref", "#local/ref"}, // issue #34
		{"http://localhost/doc.ttl", "http://localhost/doc.ttl", ""},
		{"http://a/b/c/d;p?q", "http://a/b/c/g", "g"},
		{"http://a/b/c/d;p?q", "http://a/b/c/g/h?x#y", "g/h?x#y"},
		{"http://a/b/c/d;p?q", "http://a/b/g", "../g"},
		{"http://a/b/c/d;p?q", "http://a/g", "/g"},
		{"http://a/b/c/d;p?q", "http://a/b/c/", "./"},
		{"http://a/b/c/d;p?q", "http://a/b/c/d;p?y", "?y"},
		{"http://a/b/c/d;p?q", "http://a/b/c/d;p?q#s", "#s"},
		{"http://a/b/c/d;p?q", "http://a/b/c/d;p", "d;p"},
		{"http://a/b/c/d;p?q", "http://a/b/c/x:y", "./x:y"},
		{"http://a/doc#frag", "http://a/doc", ""},
		{"http://a/doc#frag", "http://a/doc#other", "#other"},
		{"http://a", "http://a/x", "/x"},
		{"urn:isbn:123", "urn:isbn:123#x", "#x"},
	}
	for _, tt := range tests {
		got, ok := Relativize(tt.base, tt.target)
		if got != tt.want || !ok {
			t.Errorf("Relativize(%q, %q) = %q, %v; want %q", tt.base, tt.target, got, ok, tt.want)
		}
		if r := Resolve(tt.base, got); r != tt.target {
			t.Errorf("Resolve(%q, %q) = %q; want %q", tt.base, got, r, tt.target)
		}
	}
}

func TestRelativizeKeepsAbsolute(t *testing.T) {
	for _, tt := range []struct{ base, target string }{
		{"http://a/b", "https://a/b"},     // scheme
		{"http://a/b", "http://other/b"},  // authority
		{"http://a/b", "http://a/x/../b"}, // resolution would normalise it
		{"", "http://a/b"},                // no base
		{"relative/base", "http://a/b"},   // base without a scheme
		{"http://a/b", "http://a//x"},     // "//x" would be an authority
	} {
		if got, ok := Relativize(tt.base, tt.target); ok || got != tt.target {
			t.Errorf("Relativize(%q, %q) = %q, %v; want the target unchanged", tt.base, tt.target, got, ok)
		}
	}
}

func FuzzRelativize(f *testing.F) {
	f.Add("http://a/b/c/d;p?q", "http://a/b/g#x")
	f.Add("http://localhost/doc.ttl", "http://localhost/doc.ttl#local/ref")
	f.Add("urn:a:b", "urn:a:c")
	f.Add("http://a", "http://a?q")
	f.Fuzz(func(t *testing.T, base, target string) {
		got, ok := Relativize(base, target)
		if !ok {
			if got != target {
				t.Fatalf("not relativized but changed: %q", got)
			}
			return
		}
		if len(got) >= len(target) {
			t.Fatalf("Relativize(%q, %q) = %q is not shorter", base, target, got)
		}
		if r := Resolve(base, got); r != target {
			t.Fatalf("Relativize(%q, %q) = %q resolves to %q", base, target, got, r)
		}
	})
}
