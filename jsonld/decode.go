package jsonld

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidJSON is returned by Parse when the input is not a JSON document:
// empty, truncated, or with a syntax error. The message gives the line and
// column. JSON-LD is plain JSON, so comments and trailing commas (accepted by
// JSON5 and many editors' settings files) are syntax errors here.
//
// A failure to read the input is not this error; it wraps the reader's error.
var ErrInvalidJSON = errors.New("jsonld: input is not valid JSON")

// decodeDocument decodes the one JSON value r holds, turning encoding/json's
// byte offsets into a line and column a person can go to.
func decodeDocument(r io.Reader) (any, error) {
	lr := &newlineReader{r: r}
	var doc any
	err := json.NewDecoder(lr).Decode(&doc)
	if err == nil {
		return doc, nil
	}
	var syn *json.SyntaxError
	switch {
	case errors.Is(err, io.EOF):
		return nil, fmt.Errorf("%w: the input is empty", ErrInvalidJSON)
	case errors.Is(err, io.ErrUnexpectedEOF):
		line, col := lr.position(lr.read)
		return nil, fmt.Errorf("%w: the input ends inside a value at line %d, column %d; is it truncated?",
			ErrInvalidJSON, line, col)
	case errors.As(err, &syn):
		// Offset counts the bytes read before the error, so the offending
		// byte is the one before it.
		line, col := lr.position(syn.Offset - 1)
		return nil, fmt.Errorf("%w: line %d, column %d: %w (comments and trailing commas are not JSON)",
			ErrInvalidJSON, line, col, err)
	case lr.err != nil && errors.Is(err, lr.err):
		return nil, fmt.Errorf("jsonld: reading input: %w", err)
	default:
		// Decoding into any cannot produce a type error, so this is a decoder
		// error we do not classify; keep it wrapped rather than raw.
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
}

// newlineReader passes reads through and records where each line starts, so
// a byte offset can be turned into a line and column without keeping the
// document.
type newlineReader struct {
	r        io.Reader
	read     int64   // bytes read so far
	newlines []int64 // offset of every '\n' read
	err      error   // the last error from r, if any
}

func (n *newlineReader) Read(p []byte) (int, error) {
	k, err := n.r.Read(p)
	for i, b := range p[:k] {
		if b == '\n' {
			n.newlines = append(n.newlines, n.read+int64(i))
		}
	}
	n.read += int64(k)
	if err != nil && err != io.EOF {
		n.err = err
	}
	return k, err
}

// position returns the 1-based line and byte column of offset.
func (n *newlineReader) position(offset int64) (line, col int) {
	if offset < 0 {
		offset = 0
	}
	lineStart := int64(0)
	line = 1
	for _, nl := range n.newlines {
		if nl >= offset {
			break
		}
		line++
		lineStart = nl + 1
	}
	return line, int(offset-lineStart) + 1
}
