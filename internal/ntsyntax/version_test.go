package ntsyntax

import (
	"errors"
	"testing"
)

func TestVersionLine(t *testing.T) {
	cases := []struct {
		line  string
		isVer bool
		bad   bool
	}{
		{`VERSION "1.2"`, true, false},
		{`VERSION   "1.2"  # note`, true, false},
		{`VERSION "anything"`, true, false},
		{`version "1.2"`, false, false}, // wrong case: falls through, fails as a statement
		{`VERSION 1.2`, true, true},
		{`VERSION <http://example.org/v>`, true, true},
		{`VERSION "1.2" .`, true, true},
		{`VERSION "1.2`, true, true},
		{`VERSION "a\"b"`, true, true},
		{`VERSION`, false, false},
		{`VERSIONED "x"`, false, false},
		{`<http://a> <http://b> "c" .`, false, false},
	}
	for _, c := range cases {
		isVer, err := VersionLine(c.line)
		if isVer != c.isVer || (err != nil) != c.bad {
			t.Errorf("VersionLine(%q) = %v, %v; want %v, bad=%v", c.line, isVer, err, c.isVer, c.bad)
		}
		if err != nil && !errors.Is(err, ErrBadVersion) {
			t.Errorf("VersionLine(%q): error %v is not ErrBadVersion", c.line, err)
		}
	}
}
