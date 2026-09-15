package results

import "strings"

// Format identifies a SPARQL query results format. The zero value is not a
// valid format. Format values are safe for concurrent use.
type Format int

// The supported result formats.
const (
	FormatJSON Format = iota + 1 // application/sparql-results+json
	FormatXML                    // application/sparql-results+xml
	FormatCSV                    // text/csv
	FormatTSV                    // text/tab-separated-values
)

// Media types of the result formats.
const (
	MediaTypeJSON = "application/sparql-results+json"
	MediaTypeXML  = "application/sparql-results+xml"
	MediaTypeCSV  = "text/csv"
	MediaTypeTSV  = "text/tab-separated-values"
)

// allFormats is the server preference order used by Negotiate.
var allFormats = []Format{FormatJSON, FormatXML, FormatCSV, FormatTSV}

// MediaType returns the media type without parameters, or "" for an unknown
// format.
func (f Format) MediaType() string {
	switch f {
	case FormatJSON:
		return MediaTypeJSON
	case FormatXML:
		return MediaTypeXML
	case FormatCSV:
		return MediaTypeCSV
	case FormatTSV:
		return MediaTypeTSV
	}
	return ""
}

// ContentType returns the value for a Content-Type header, or "" for an
// unknown format. CSV and TSV carry "; charset=utf-8": RFC 4180 makes
// US-ASCII the default charset of text/csv, and the output is UTF-8.
func (f Format) ContentType() string {
	switch f {
	case FormatCSV, FormatTSV:
		return f.MediaType() + "; charset=utf-8"
	}
	return f.MediaType()
}

// String returns the short name of the format: "json", "xml", "csv", "tsv".
func (f Format) String() string {
	switch f {
	case FormatJSON:
		return "json"
	case FormatXML:
		return "xml"
	case FormatCSV:
		return "csv"
	case FormatTSV:
		return "tsv"
	}
	return "unknown"
}

// SupportsBoolean reports whether the format can carry an ASK result. Only
// JSON and XML can; the CSV/TSV specification does not define booleans.
func (f Format) SupportsBoolean() bool {
	return f == FormatJSON || f == FormatXML
}

// aliases maps generic media types onto a result format. Clients commonly
// send them (curl -H 'Accept: application/json'). They rank below the
// canonical media type in negotiation.
var aliases = map[string]Format{
	"application/json": FormatJSON,
	"application/xml":  FormatXML,
	"text/xml":         FormatXML,
}

// ParseFormat maps a short name ("json", "xml", "csv", "tsv", as used in a
// format= query parameter) or a media type, canonical or alias, to a Format.
// Matching is case-insensitive and media type parameters are ignored.
func ParseFormat(s string) (Format, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	for _, f := range allFormats {
		if s == f.String() || s == f.MediaType() {
			return f, true
		}
	}
	if f, ok := aliases[s]; ok {
		return f, true
	}
	return 0, false
}
