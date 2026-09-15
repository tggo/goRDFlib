package results

import (
	"fmt"
	"io"
	"strconv"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
)

// flushThreshold is the buffered size after which a row is handed to the
// underlying io.Writer.
const flushThreshold = 32 << 10

// Writer streams a SELECT result in one format. Create it with NewWriter or
// a format-specific constructor, call WriteRow per solution, then Close.
//
// Nothing is written to the io.Writer before the first WriteRow, Flush or
// Close. A row that fails to encode is not written at all and the writer
// stays usable. An error from the io.Writer is sticky: every later call
// returns it.
//
// A Writer is not safe for concurrent use.
type Writer struct {
	out    io.Writer
	format Format
	vars   []string
	// prefix[i] is the pre-encoded text that introduces vars[i] in a row
	// (JSON: `"name":`, XML: `<binding name="name">`).
	prefix [][]byte

	buf     []byte
	scratch []byte // CSV field quoting
	bnodes  map[string]int
	rows    int

	started bool
	closed  bool
	err     error
}

// NewWriter returns a streaming writer for a SELECT result with the given
// projection, in the given format. vars is the column order.
func NewWriter(w io.Writer, f Format, vars []string) (*Writer, error) {
	if f.MediaType() == "" {
		return nil, fmt.Errorf("%w: %d", ErrUnknownFormat, int(f))
	}
	for _, v := range vars {
		if !validVarName(v) {
			return nil, fmt.Errorf("%w: %q (it must match the SPARQL VARNAME production)", ErrInvalidVariableName, v)
		}
	}
	wr := &Writer{
		out:    w,
		format: f,
		vars:   vars,
		buf:    make([]byte, 0, flushThreshold+flushThreshold/4),
	}
	wr.prefix = make([][]byte, len(vars))
	for i, v := range vars {
		switch f {
		case FormatJSON:
			p, _ := appendJSONString([]byte{}, v) // validated above
			wr.prefix[i] = append(p, ':')
		case FormatXML:
			p := append([]byte(`<binding name="`), v...) // VARNAME needs no escaping
			wr.prefix[i] = append(p, `">`...)
		}
	}
	return wr, nil
}

// NewJSONWriter returns a streaming SPARQL Results JSON writer.
func NewJSONWriter(w io.Writer, vars []string) (*Writer, error) {
	return NewWriter(w, FormatJSON, vars)
}

// NewXMLWriter returns a streaming SPARQL Results XML writer.
func NewXMLWriter(w io.Writer, vars []string) (*Writer, error) {
	return NewWriter(w, FormatXML, vars)
}

// NewCSVWriter returns a streaming CSV results writer.
func NewCSVWriter(w io.Writer, vars []string) (*Writer, error) {
	return NewWriter(w, FormatCSV, vars)
}

// NewTSVWriter returns a streaming TSV results writer.
func NewTSVWriter(w io.Writer, vars []string) (*Writer, error) {
	return NewWriter(w, FormatTSV, vars)
}

// Format returns the writer's format.
func (w *Writer) Format() Format { return w.format }

// Rows returns the number of rows written so far.
func (w *Writer) Rows() int { return w.rows }

// WriteRow writes one solution. Variables missing from row, or bound to nil,
// are unbound. Keys not in the projection are ignored.
func (w *Writer) WriteRow(row map[string]rdflibgo.Term) error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return ErrWriterClosed
	}
	w.start()
	mark := len(w.buf)
	var err error
	switch w.format {
	case FormatJSON:
		w.buf, err = w.jsonRow(w.buf, row)
	case FormatXML:
		w.buf, err = w.xmlRow(w.buf, row)
	case FormatCSV:
		w.buf, err = w.csvRow(w.buf, row)
	case FormatTSV:
		w.buf, err = w.tsvRow(w.buf, row)
	}
	if err != nil {
		w.buf = w.buf[:mark]
		return fmt.Errorf("row %d: %w", w.rows+1, err)
	}
	w.rows++
	if len(w.buf) >= flushThreshold {
		return w.flushBuf()
	}
	return nil
}

// Flush writes the header, if not yet written, and all buffered rows to the
// underlying io.Writer. It does not flush that writer itself (call
// http.Flusher.Flush on a ResponseWriter after it).
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return ErrWriterClosed
	}
	w.start()
	return w.flushBuf()
}

// Close writes the header if no row was written, the end of the document,
// and flushes. It does not close the underlying io.Writer. Calling Close
// again returns nil, or the sticky I/O error.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	w.start()
	switch w.format {
	case FormatJSON:
		if w.rows > 0 {
			w.buf = append(w.buf, '\n')
		}
		w.buf = append(w.buf, "]}}\n"...)
	case FormatXML:
		w.buf = append(w.buf, "  </results>\n</sparql>\n"...)
	}
	w.closed = true
	return w.flushBuf()
}

func (w *Writer) start() {
	if w.started {
		return
	}
	w.started = true
	switch w.format {
	case FormatJSON:
		w.buf = append(w.buf, `{"head":{"vars":[`...)
		for i, v := range w.vars {
			if i > 0 {
				w.buf = append(w.buf, ',')
			}
			w.buf, _ = appendJSONString(w.buf, v)
		}
		w.buf = append(w.buf, "]},\n\"results\":{\"bindings\":["...)
	case FormatXML:
		w.buf = append(w.buf, xmlDocStart...)
		w.buf = append(w.buf, "  <head>\n"...)
		for _, v := range w.vars {
			w.buf = append(w.buf, `    <variable name="`...)
			w.buf = append(w.buf, v...)
			w.buf = append(w.buf, "\"/>\n"...)
		}
		w.buf = append(w.buf, "  </head>\n  <results>\n"...)
	case FormatCSV:
		for i, v := range w.vars {
			if i > 0 {
				w.buf = append(w.buf, ',')
			}
			w.buf = append(w.buf, v...) // VARNAME never needs quoting
		}
		w.buf = append(w.buf, '\r', '\n')
	case FormatTSV:
		for i, v := range w.vars {
			if i > 0 {
				w.buf = append(w.buf, '\t')
			}
			w.buf = append(w.buf, '?')
			w.buf = append(w.buf, v...)
		}
		w.buf = append(w.buf, '\n')
	}
}

func (w *Writer) flushBuf() error {
	if len(w.buf) == 0 {
		return nil
	}
	_, err := w.out.Write(w.buf)
	w.buf = w.buf[:0]
	if err != nil {
		w.err = fmt.Errorf("results: write: %w", err)
		return w.err
	}
	return nil
}

// appendBNodeLabel appends the per-document label of b ("b0", "b1", ...).
func (w *Writer) appendBNodeLabel(buf []byte, b rdflibgo.BNode) []byte {
	if w.bnodes == nil {
		w.bnodes = make(map[string]int, 16)
	}
	n, ok := w.bnodes[b.Value()]
	if !ok {
		n = len(w.bnodes)
		w.bnodes[b.Value()] = n
	}
	buf = append(buf, 'b')
	return strconv.AppendInt(buf, int64(n), 10)
}

// Write writes a SELECT or ASK result in format f. Every value is checked
// before anything is written, so a value the format cannot carry returns an
// error with nothing written; see the package documentation.
func Write(out io.Writer, f Format, r *sparql.Result) error {
	if f.MediaType() == "" {
		return fmt.Errorf("%w: %d", ErrUnknownFormat, int(f))
	}
	if r == nil {
		return fmt.Errorf("%w: nil result", ErrUnsupportedResultForm)
	}
	switch r.Type {
	case "ASK":
		if !f.SupportsBoolean() {
			return fmt.Errorf("%w: ASK result as %s (use JSON or XML)", ErrUnsupportedResultForm, f)
		}
		var buf []byte
		if f == FormatJSON {
			buf = append(buf, `{"head":{},"boolean":`...)
			buf = strconv.AppendBool(buf, r.AskResult)
			buf = append(buf, "}\n"...)
		} else {
			buf = append(buf, xmlDocStart...)
			buf = append(buf, "  <head/>\n  <boolean>"...)
			buf = strconv.AppendBool(buf, r.AskResult)
			buf = append(buf, "</boolean>\n</sparql>\n"...)
		}
		if _, err := out.Write(buf); err != nil {
			return fmt.Errorf("results: write: %w", err)
		}
		return nil
	case "SELECT":
	default:
		return fmt.Errorf("%w: %q result (serialize the graph with an RDF serializer)", ErrUnsupportedResultForm, r.Type)
	}

	wr, err := NewWriter(out, f, r.Vars)
	if err != nil {
		return err
	}
	for i, row := range r.Bindings {
		for _, v := range r.Vars {
			if t := row[v]; t != nil {
				if err := checkTerm(f, t); err != nil {
					return fmt.Errorf("row %d: variable ?%s: %w", i+1, v, err)
				}
			}
		}
	}
	for _, row := range r.Bindings {
		if err := wr.WriteRow(row); err != nil {
			return err
		}
	}
	return wr.Close()
}

// WriteJSON writes a SELECT or ASK result as SPARQL Results JSON.
func WriteJSON(w io.Writer, r *sparql.Result) error { return Write(w, FormatJSON, r) }

// WriteXML writes a SELECT or ASK result as SPARQL Results XML.
func WriteXML(w io.Writer, r *sparql.Result) error { return Write(w, FormatXML, r) }

// WriteCSV writes a SELECT result as CSV. An ASK result returns
// ErrUnsupportedResultForm.
func WriteCSV(w io.Writer, r *sparql.Result) error { return Write(w, FormatCSV, r) }

// WriteTSV writes a SELECT result as TSV. An ASK result returns
// ErrUnsupportedResultForm.
func WriteTSV(w io.Writer, r *sparql.Result) error { return Write(w, FormatTSV, r) }
