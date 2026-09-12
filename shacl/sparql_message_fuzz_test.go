package shacl

import (
	"strings"
	"testing"
)

// FuzzExpandTemplate checks that message expansion never panics on arbitrary
// shapes-graph input, that a template with nothing bound comes back unchanged,
// and that a substituted value is never expanded a second time.
func FuzzExpandTemplate(f *testing.F) {
	for _, seed := range []string{"", "{?x}", "{$x}", "{?", "{?}", "{{?x}}", "a {?x} b {$y}", "{?naïve}", "{?x\xff}", "{?x}{?x}"} {
		f.Add(seed, "v")
	}
	f.Fuzz(func(t *testing.T, tmpl, value string) {
		if got := expandTemplate(tmpl, nil); got != tmpl {
			t.Fatalf("expandTemplate(%q, nil) = %q", tmpl, got)
		}
		// x's value holds a placeholder for y, whose value is a marker byte
		// that nothing else supplies. Every marker in the output must come from
		// the template itself or from a {?y}/{$y} written in it; one more means
		// x's value was expanded again.
		const marker = "\x01"
		value = strings.ReplaceAll(value, marker, "")
		vars := map[string]Term{"x": Literal("{?y}"+value, "", ""), "y": Literal(marker, "", "")}
		got := expandTemplate(tmpl, vars)
		limit := strings.Count(tmpl, marker) + strings.Count(tmpl, "{?y}") + strings.Count(tmpl, "{$y}")
		if n := strings.Count(got, marker); n > limit {
			t.Fatalf("expandTemplate(%q) = %q: %d markers, at most %d", tmpl, got, n, limit)
		}
	})
}
