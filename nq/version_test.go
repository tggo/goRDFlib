package nq_test

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"
)

func TestParseStreamVersionAnnouncement(t *testing.T) {
	doc := "VERSION \"1.2\"\n<http://a> <http://b> \"c\" <http://g> .\n"
	var n int
	err := nq.ParseStream(strings.NewReader(doc), func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term, rdflibgo.Term) error {
		n++
		return nil
	})
	if err != nil || n != 1 {
		t.Fatalf("%d quads, err %v", n, err)
	}
}

func TestParseStreamVersionAnnouncementMalformed(t *testing.T) {
	err := nq.ParseStream(strings.NewReader("VERSION 1.2\n"), func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term, rdflibgo.Term) error { return nil })
	if !errors.Is(err, nq.ErrBadVersion) {
		t.Fatalf("want ErrBadVersion, got %v", err)
	}
}
