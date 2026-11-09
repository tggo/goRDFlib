package nt_test

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/nt"
)

const versionDoc = "VERSION \"1.2\"\n<http://a> <http://b> \"c\" .\n"

func TestParseVersionAnnouncement(t *testing.T) {
	g := rdflibgo.NewGraph()
	if err := nt.Parse(t.Context(), g, strings.NewReader(versionDoc)); err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Len(t.Context()); n != 1 {
		t.Fatalf("want 1 triple, got %d", n)
	}
	var streamed int
	err := nt.ParseStream(strings.NewReader(versionDoc), func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error {
		streamed++
		return nil
	})
	if err != nil || streamed != 1 {
		t.Fatalf("stream: %d triples, err %v", streamed, err)
	}
}

func TestParseVersionAnnouncementMalformed(t *testing.T) {
	for _, doc := range []string{"VERSION 1.2\n", "VERSION \"1.2\" .\n", "VERSION <http://x/>\n"} {
		err := nt.Parse(t.Context(), rdflibgo.NewGraph(), strings.NewReader(doc))
		if !errors.Is(err, ntsyntax.ErrBadVersion) {
			t.Errorf("%q: want ErrBadVersion, got %v", doc, err)
		}
		if err != nil && !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%q: error lacks position: %v", doc, err)
		}
	}
	// Wrong case is not an announcement: it fails as a statement.
	if err := nt.Parse(t.Context(), rdflibgo.NewGraph(), strings.NewReader("version \"1.2\"\n")); err == nil {
		t.Error("lower-case version must be rejected")
	}
}
