package results_test

import (
	"testing"

	"github.com/tggo/goRDFlib/sparql/results"
)

func TestNegotiate(t *testing.T) {
	const (
		json = results.FormatJSON
		xml  = results.FormatXML
		csv  = results.FormatCSV
		tsv  = results.FormatTSV
		none = results.Format(0)
	)
	tests := []struct {
		accept string
		want   results.Format
	}{
		// Empty or absent: anything, server preference.
		{"", json},
		{"   ", json},

		// Exact types, case-insensitive, parameters ignored.
		{"application/sparql-results+json", json},
		{"application/sparql-results+xml", xml},
		{"text/csv", csv},
		{"text/tab-separated-values", tsv},
		{"TEXT/CSV", csv},
		{"text/csv; charset=utf-8", csv},
		{" text/csv ;charset=\"utf-8\" ", csv},

		// Wildcards.
		{"*/*", json},
		{"text/*", csv},
		{"application/*", json},
		{"text/*;q=0.9, text/tab-separated-values", tsv},

		// q-values pick the best, not the first listed.
		{"application/sparql-results+xml;q=0.5, text/csv", csv},
		{"text/csv;q=0.1, application/sparql-results+xml;q=0.2", xml},
		{"text/csv;q=1.0, application/sparql-results+json;q=0.999", csv},
		{"text/tab-separated-values;q=0.3, */*;q=0.1", tsv},

		// Equal q: more specific match wins, then server order.
		{"*/*, text/csv", csv},
		{"application/sparql-results+xml, text/csv", xml},
		{"text/csv, application/sparql-results+xml", xml},

		// The most specific range decides; q=0 excludes even under */*.
		{"application/sparql-results+json;q=0, */*", xml},
		{"*/*;q=0, text/tab-separated-values", tsv},
		{"text/*;q=0, */*", json},
		{"text/*;q=0, application/*;q=0, */*", none},

		// Aliases rank below the canonical type.
		{"application/json", json},
		{"application/xml", xml},
		{"text/xml", xml},
		{"application/json, application/sparql-results+xml", xml},
		{"application/json;q=0.9, application/sparql-results+xml;q=0.5", json},

		// Unknown or unacceptable types.
		{"text/html", none},
		{"image/png, text/turtle", none},
		{"text/html, */*;q=0.1", json},
		{"application/sparql-results+json;q=0", none},

		// Malformed ranges and q-values are ignored, not fatal.
		{"garbage, text/csv", csv},
		{"*/csv, text/tab-separated-values", tsv},
		{"text/csv;q=2, text/tab-separated-values", tsv},
		{"text/csv;q=abc", none},
		{"text/csv;q=0.1234, application/sparql-results+xml;q=0.1", xml},
		{",,, ;;", none},
		{"text/csv;foo=\"a,b;q=0\";q=0.5, application/sparql-results+xml;q=0.4", csv},

		// Parameters after q are accept-ext and do not change q.
		{"text/csv;q=0.4;level=1, application/sparql-results+xml;q=0.3", csv},

		// Typical client headers.
		{"application/sparql-results+json, application/sparql-results+xml;q=0.9, */*;q=0.1", json},
		{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", xml},
	}
	for _, tc := range tests {
		got, ok := results.Negotiate(tc.accept)
		if ok != (tc.want != none) || got != tc.want {
			t.Errorf("Negotiate(%q) = %v, %v; want %v", tc.accept, got, ok, tc.want)
		}
	}
}

func TestNegotiateAmong(t *testing.T) {
	ask := []results.Format{results.FormatJSON, results.FormatXML}
	if f, ok := results.NegotiateAmong("text/csv", ask...); ok {
		t.Errorf("text/csv for ASK: got %v, want not acceptable", f)
	}
	if f, ok := results.NegotiateAmong("text/csv, */*;q=0.1", ask...); !ok || f != results.FormatJSON {
		t.Errorf("got %v %v, want json", f, ok)
	}
	if f, ok := results.NegotiateAmong("", results.FormatTSV, results.FormatCSV); !ok || f != results.FormatTSV {
		t.Errorf("empty Accept: got %v, want the first offer", f)
	}
	if _, ok := results.NegotiateAmong("*/*"); ok {
		t.Error("no offers must not be acceptable")
	}
	if _, ok := results.NegotiateAmong("*/*", results.Format(99)); ok {
		t.Error("an unknown format must never be picked")
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		f                        results.Format
		name, mediaType, content string
		boolean                  bool
	}{
		{results.FormatJSON, "json", "application/sparql-results+json", "application/sparql-results+json", true},
		{results.FormatXML, "xml", "application/sparql-results+xml", "application/sparql-results+xml", true},
		{results.FormatCSV, "csv", "text/csv", "text/csv; charset=utf-8", false},
		{results.FormatTSV, "tsv", "text/tab-separated-values", "text/tab-separated-values; charset=utf-8", false},
	}
	for _, tc := range tests {
		if tc.f.String() != tc.name || tc.f.MediaType() != tc.mediaType || tc.f.ContentType() != tc.content || tc.f.SupportsBoolean() != tc.boolean {
			t.Errorf("%d: %q %q %q %v", tc.f, tc.f.String(), tc.f.MediaType(), tc.f.ContentType(), tc.f.SupportsBoolean())
		}
		for _, s := range []string{tc.name, tc.mediaType, tc.content, "  " + tc.mediaType + ";q=1"} {
			if f, ok := results.ParseFormat(s); !ok || f != tc.f {
				t.Errorf("ParseFormat(%q) = %v %v", s, f, ok)
			}
		}
	}
	var zero results.Format
	if zero.ContentType() != "" || zero.String() != "unknown" {
		t.Error("zero Format must have no content type")
	}
	for s, want := range map[string]results.Format{"application/json": results.FormatJSON, "text/xml": results.FormatXML} {
		if f, ok := results.ParseFormat(s); !ok || f != want {
			t.Errorf("ParseFormat(%q) = %v", s, f)
		}
	}
	if _, ok := results.ParseFormat("text/turtle"); ok {
		t.Error("text/turtle is not a results format")
	}
}
