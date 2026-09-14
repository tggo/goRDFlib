package badgerstore

import (
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

func TestEncodeDecodeTriple(t *testing.T) {
	tests := []struct {
		name   string
		triple term.Triple
	}{
		{
			"uri terms",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewURIRefUnsafe("http://example.org/o"),
			},
		},
		{
			"bnode subject",
			term.Triple{
				Subject:   term.NewBNode("b1"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral("hello"),
			},
		},
		{
			"lang literal",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral("bonjour", term.WithLang("fr")),
			},
		},
		{
			"typed literal",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral(42),
			},
		},
		{
			"dir lang literal",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral("hello", term.WithLang("en"), term.WithDir("ltr")),
			},
		},
		{
			"literal with newline",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral("line1\nline2"),
			},
		},
		{
			"literal with quotes",
			term.Triple{
				Subject:   term.NewURIRefUnsafe("http://example.org/s"),
				Predicate: term.NewURIRefUnsafe("http://example.org/p"),
				Object:    term.NewLiteral(`she said "hello"`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := encodeTriple(tt.triple)
			decoded, err := decodeTriple(data)
			if err != nil {
				t.Fatalf("decodeTriple: %v", err)
			}
			if !decoded.Subject.Equal(tt.triple.Subject) {
				t.Errorf("subject: got %s, want %s", decoded.Subject.N3(), tt.triple.Subject.N3())
			}
			if decoded.Predicate != tt.triple.Predicate {
				t.Errorf("predicate: got %s, want %s", decoded.Predicate.N3(), tt.triple.Predicate.N3())
			}
			if !decoded.Object.Equal(tt.triple.Object) {
				t.Errorf("object: got %s, want %s", decoded.Object.N3(), tt.triple.Object.N3())
			}
		})
	}
}

func TestDecodeTripleErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{1, 2, 3}},
		{"truncated subject", []byte{100, 0, 0, 0}},
		{"truncated predicate length", append([]byte{3, 0, 0, 0}, []byte("U:x")...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeTriple(tt.data)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestGraphKey(t *testing.T) {
	if gk := graphKey(nil); gk != "" {
		t.Errorf("graphKey(nil) = %q, want empty", gk)
	}
	if gk := graphKey(store.DefaultGraph); gk != "" {
		t.Errorf("graphKey(DefaultGraph) = %q, want empty", gk)
	}
	// A blank node names a graph (rdflib #2445).
	bn := term.NewBNode("b1")
	if gk := graphKey(bn); gk != term.TermKey(bn) {
		t.Errorf("graphKey(BNode) = %q, want %q", gk, term.TermKey(bn))
	}
	uri := term.NewURIRefUnsafe("http://example.org/g")
	if gk := graphKey(uri); gk == "" {
		t.Error("graphKey(URIRef) should not be empty")
	}
}

func TestKeyFormats(t *testing.T) {
	// Verify key separator is null byte.
	key := spoKey("g", "s", "p", "o")
	if key[0] != pfxSPO || key[1] != sep {
		t.Errorf("unexpected SPO key prefix: %v", key[:2])
	}

	// Verify prefix key construction.
	prefix := makePrefixKey(pfxPOS, "g", "p")
	if prefix[0] != pfxPOS {
		t.Errorf("unexpected prefix byte: %v", prefix[0])
	}
}
