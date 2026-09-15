package results

import "errors"

var (
	// ErrUnsupportedResultForm is returned when the format cannot express the
	// result form: ASK in CSV or TSV (neither spec defines it), CONSTRUCT or
	// DESCRIBE in any result format (serialize the graph with an RDF
	// serializer instead), or a nil result. An endpoint should negotiate
	// among JSON and XML for ASK, see NegotiateAmong.
	ErrUnsupportedResultForm = errors.New("results: result form not supported by this format")

	// ErrInvalidUTF8 is returned when a variable name, IRI, literal or
	// language tag is not valid UTF-8. None of the formats can carry it;
	// fix the data at its source.
	ErrInvalidUTF8 = errors.New("results: string is not valid UTF-8")

	// ErrUnrepresentableXMLChar is returned by the XML writer for a character
	// XML 1.0 cannot carry, not even as a character reference (U+0000-U+0008,
	// U+000B, U+000C, U+000E-U+001F, U+FFFE, U+FFFF). Serve the result as
	// JSON or TSV, which can carry it.
	ErrUnrepresentableXMLChar = errors.New("results: character cannot be represented in XML 1.0")

	// ErrUnrepresentableIRI is returned by the TSV writer for an IRI that
	// contains a character an N-Triples IRIREF cannot hold, even escaped
	// (U+0000-U+0020 and < > " { } | \ ^ `). Such an IRI was created without
	// validation (NewURIRefUnsafe). Serve the result as JSON, XML or CSV,
	// which carry IRIs as plain strings.
	ErrUnrepresentableIRI = errors.New("results: IRI cannot be represented in TSV")

	// ErrUnsupportedTerm is returned for a binding that is not an IRI, blank
	// node, literal or triple term (for example a Variable).
	ErrUnsupportedTerm = errors.New("results: unsupported term type in binding")

	// ErrInvalidVariableName is returned when a variable name does not match
	// the SPARQL VARNAME production. Names produced by the query parser always
	// match; check how a hand-built Result was constructed.
	ErrInvalidVariableName = errors.New("results: invalid variable name")

	// ErrUnknownFormat is returned for a Format value that is not one of the
	// declared constants.
	ErrUnknownFormat = errors.New("results: unknown format")

	// ErrWriterClosed is returned by WriteRow and Flush after Close.
	ErrWriterClosed = errors.New("results: writer is closed")
)
