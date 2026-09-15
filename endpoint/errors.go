package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel errors. Every error the handler answers with is wrapped in an
// *HTTPError that carries the status; errors.Is against these tells the cause.
var (
	// ErrUnauthenticated makes an Authorizer's refusal a 401 instead of a 403.
	ErrUnauthenticated = errors.New("endpoint: authentication required")

	// ErrReadOnly is returned for an update or Graph Store write on a handler
	// created with WithReadOnly.
	ErrReadOnly = errors.New("endpoint: this endpoint is read-only")

	// ErrLoadDisabled is returned for a LOAD operation when no Loader was
	// configured. Enable LOAD with WithLoader.
	ErrLoadDisabled = errors.New("endpoint: LOAD is disabled on this endpoint (enable it with WithLoader)")

	// ErrTimeout is returned when a query or update ran out of the time given
	// by WithQueryTimeout or WithUpdateTimeout.
	ErrTimeout = errors.New("endpoint: time limit exceeded")

	// ErrTooManyRows is returned when a result exceeds WithMaxResultRows.
	ErrTooManyRows = errors.New("endpoint: result exceeds the row limit")

	// ErrBodyTooLarge is returned when a request body exceeds
	// WithMaxRequestBytes or WithMaxGraphBytes.
	ErrBodyTooLarge = errors.New("endpoint: request body too large")

	// ErrDescribeUnsupported is returned for DESCRIBE queries, which the
	// query engine does not implement.
	ErrDescribeUnsupported = errors.New("endpoint: DESCRIBE queries are not supported by this engine")
)

// HTTPError is an error with the HTTP status (and optionally headers) it is
// answered with. An Authorizer may return one to control the response.
type HTTPError struct {
	Status int
	Header http.Header
	Err    error
}

func (e *HTTPError) Error() string {
	if e.Err == nil {
		return http.StatusText(e.Status)
	}
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error { return e.Err }

func httpErr(status int, err error) *HTTPError { return &HTTPError{Status: status, Err: err} }

func httpErrf(status int, format string, args ...any) *HTTPError {
	return &HTTPError{Status: status, Err: fmt.Errorf(format, args...)}
}

// posRe finds the byte offset the sparql parser reports ("at pos 12").
var posRe = regexp.MustCompile(`at pos (\d+)`)

// withLineColumn appends "(line L, column C)" to a parser error that carries a
// byte offset, so a client does not have to count bytes in a multi-line query.
// The parser counts after expanding \u escapes outside strings, so in a query
// that uses them the position can be a few characters off.
func withLineColumn(msg, text string) string {
	m := posRe.FindStringSubmatchIndex(msg)
	if m == nil {
		return msg
	}
	pos, err := strconv.Atoi(msg[m[2]:m[3]])
	if err != nil || pos < 0 {
		return msg
	}
	pos = min(pos, len(text))
	line := 1 + strings.Count(text[:pos], "\n")
	col := pos - strings.LastIndexByte(text[:pos], '\n')
	return fmt.Sprintf("%s (line %d, column %d)", msg, line, col)
}
