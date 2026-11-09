package ntsyntax

import (
	"errors"
	"strings"
)

// ErrBadVersion reports a malformed VERSION announcement line.
var ErrBadVersion = errors.New(`malformed VERSION announcement: write VERSION "1.2" (upper case keyword, one double-quoted string, no trailing '.')`)

// VersionLine reports whether line (already trimmed) is an RDF 1.2
// N-Triples/N-Quads version announcement, `VERSION "1.2"`. The keyword is
// case sensitive and the specifier a double-quoted string; a line that starts
// with the keyword but is otherwise malformed returns ErrBadVersion, so
// `VERSION 1.2` is an error rather than a statement with a strange subject.
// The announcement is accepted wherever a statement could stand and its value
// is not interpreted (the grammar does not restrict it).
func VersionLine(line string) (bool, error) {
	const kw = "VERSION"
	if !strings.HasPrefix(line, kw) {
		return false, nil
	}
	rest := line[len(kw):]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return false, nil
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" || rest[0] != '"' {
		return true, ErrBadVersion
	}
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 || strings.ContainsAny(rest[1:1+end], "\\\r\n") {
		return true, ErrBadVersion
	}
	tail := strings.TrimLeft(rest[end+2:], " \t")
	if tail != "" && tail[0] != '#' {
		return true, ErrBadVersion
	}
	return true, nil
}
