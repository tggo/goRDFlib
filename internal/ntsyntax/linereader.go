package ntsyntax

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrLineTooLong is returned (wrapped) when a single line exceeds the scanner's
// byte cap. It is a sentinel so callers can detect the condition with
// errors.Is and react — e.g. retry with WithUnboundedLines. The wrapped message
// names the limit and the two options that lift it, so the failure is
// self-explanatory instead of surfacing the opaque bufio.ErrTooLong.
var ErrLineTooLong = errors.New("line exceeds maximum length")

// NextLine returns one logical line at a time from a line-based RDF stream
// (N-Triples / N-Quads). It returns io.EOF (with an empty line) once the input
// is exhausted. The trailing newline is stripped; callers still trim surrounding
// whitespace themselves.
type NextLine func() (string, error)

// NewLineReader builds a NextLine over r.
//
// The mode is chosen as follows, in order of precedence:
//   - unbounded == true: lines may be arbitrarily long. A bufio.Reader grows its
//     buffer to hold the longest line. This trades a fixed memory ceiling for
//     the ability to read inputs whose longest line cannot be bounded ahead of
//     time (e.g. multi-megabyte WKT geometry literals).
//   - maxLineLen > 0: a bufio.Scanner with its buffer cap raised to maxLineLen.
//     Lines longer than that fail with bufio.ErrTooLong.
//   - otherwise: a default bufio.Scanner (64KB line cap).
//
// Both Scanner branches share identical fast-path behavior, so adding the
// unbounded option does not affect the default scanning performance.
func NewLineReader(r io.Reader, maxLineLen int, unbounded bool) NextLine {
	if unbounded {
		br := bufio.NewReader(r)
		return func() (string, error) {
			line, err := br.ReadString('\n')
			if err == io.EOF && len(line) > 0 {
				// Final line without a trailing newline: hand it back now,
				// report EOF on the next call.
				return strings.TrimRight(line, "\r\n"), nil
			}
			if err != nil {
				return "", err
			}
			return strings.TrimRight(line, "\r\n"), nil
		}
	}

	sc := bufio.NewScanner(r)
	limit := bufio.MaxScanTokenSize
	if maxLineLen > 0 {
		sc.Buffer(make([]byte, 0, 64*1024), maxLineLen)
		limit = maxLineLen
	}
	return func() (string, error) {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				if errors.Is(err, bufio.ErrTooLong) {
					// Translate the opaque stdlib error into an actionable one:
					// name the limit and the two options that lift it.
					return "", fmt.Errorf("%w (%d bytes); raise it with WithMaxLineLength or remove it with WithUnboundedLines", ErrLineTooLong, limit)
				}
				return "", err
			}
			return "", io.EOF
		}
		return sc.Text(), nil
	}
}
