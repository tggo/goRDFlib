// Package results writes SPARQL query results in the four standard result
// formats: SPARQL Results JSON, SPARQL Results XML, CSV and TSV.
//
// Specifications:
//
//   - SPARQL 1.1 Query Results JSON Format, and the SPARQL 1.2 draft
//     (triple terms, "its:dir" base direction)
//   - SPARQL Query Results XML Format (Second Edition), and the SPARQL 1.2
//     draft (<triple>, its:dir)
//   - SPARQL 1.1 Query Results CSV and TSV Formats, and the SPARQL 1.2 draft
//     (<<( s p o )>> triple terms, @lang--dir in TSV)
//
// # One-shot and streaming
//
// WriteJSON, WriteXML, WriteCSV, WriteTSV and Write take a whole
// *sparql.Result. They check every value before writing the first byte, so a
// value that the format cannot carry is reported with nothing written. Only
// an error from the underlying io.Writer can leave a partial document.
//
// Writer streams a SELECT result row by row (NewJSONWriter, NewXMLWriter,
// NewCSVWriter, NewTSVWriter). Rows are encoded into an internal buffer of
// about 32 KiB that is handed to the io.Writer when it fills, so memory does
// not grow with the result. A row that cannot be encoded is rejected whole:
// WriteRow returns the error, writes nothing of that row, and the writer can
// go on with the next row. Nothing, not even the header, is written before
// the first WriteRow, Flush or Close, so an HTTP endpoint can still send an
// error status when evaluation fails before the first solution.
//
// # Blank nodes
//
// Blank nodes are relabelled per document as b0, b1, ... in order of first
// appearance, including blank nodes inside triple terms. A label is therefore
// stable within one document, valid in every format, and does not expose
// store-internal identifiers. The relabelling map grows with the number of
// distinct blank nodes written, which is the only state that grows with the
// result.
//
// # Escaping decisions
//
//   - JSON: '"', '\\' and U+0000-U+001F are escaped, and so are U+2028 and
//     U+2029, so the output is also safe to embed in JavaScript source. '<',
//     '>' and '&' are not escaped: the media type is not HTML.
//   - XML: '&', '<' and '>' are escaped in text, and '"', TAB, LF and CR in
//     attributes as well; CR in text is written as &#xD; so an XML parser does
//     not normalise it away. XML 1.0 cannot carry U+0000-U+0008, U+000B,
//     U+000C, U+000E-U+001F, U+FFFE or U+FFFF even as character references,
//     so those return ErrUnrepresentableXMLChar.
//   - CSV: RFC 4180. A field containing '"', ',', CR or LF is quoted and
//     inner quotes are doubled. Records end with CRLF. Literals are written as
//     their lexical form; language tags, directions and datatypes are lost,
//     as the format specifies. A triple term is written <<( s p o )>> with
//     an object literal in quotes (inner quotes doubled), as in the 1.2 draft.
//     In a one-column result an empty field is written as "" rather than as
//     a blank line, which CSV readers skip.
//   - TSV: IRIs as <iri>, blank nodes as _:bN, literals in N-Triples syntax
//     with @lang, @lang--dir or ^^<datatype>. xsd:integer, xsd:decimal and
//     xsd:double literals whose lexical form matches the Turtle INTEGER,
//     DECIMAL or DOUBLE production are written unquoted. Inside a string
//     '\\', '"', TAB, LF, CR, BS and FF use their ECHAR escapes, the other
//     C0 controls and DEL are written as \u00XX, and everything else is
//     written as is. An IRI holding a character an IRIREF cannot contain
//     even as a \u escape returns ErrUnrepresentableIRI.
//   - Every format: a string that is not valid UTF-8 returns ErrInvalidUTF8.
//   - A base direction is written only with a language tag, as Literal.N3
//     does; RDF 1.2 has no directional literal without one.
//
// The functions in this package are safe for concurrent use; a Writer is not.
//
// # Not written
//
// The optional JSON "version" member and the XML its:version attribute are
// not emitted: a streaming writer does not know, when it writes the head,
// whether a triple term or a directional literal will follow. The its
// namespace is declared on the XML root element in every document.
//
// ASK results in CSV or TSV, and CONSTRUCT or DESCRIBE results in any of
// these formats, return ErrUnsupportedResultForm.
package results
