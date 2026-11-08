package sparql

import (
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// A path whose endpoint is a triple term with variables must match the term
// and bind the variables, like a plain predicate does; it was treated as an
// unconstrained endpoint, so every pair matched and the variables stayed
// unbound (found by FuzzOptimisationsAgree: the pre-bound join, which had the
// variable bound, and the nested loops, which had not, disagreed).
func TestPathEndpointIsATripleTermPattern(t *testing.T) {
	g := optGraph(t) // ex:rN ex:rt <<( ex:pN ex:type ex:KindM )>>
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	defer setOpts(false)
	for _, c := range []struct {
		query string
		rows  int
	}{
		{`SELECT * WHERE { ?r ex:rt/ex:x* <<( ?p ex:type ?k )>> }`, 7},
		{`SELECT * WHERE { ?r (ex:rt|ex:nope) <<( ?p ex:type ?k )>> }`, 7},
		{`SELECT * WHERE { ?r (ex:rt|ex:nope) <<( ?p ex:type ex:Kind0 )>> }`, 4},
		{`SELECT * WHERE { ?p ex:type ?k . ?r (ex:rt|ex:nope) <<( ?p ex:type ?k )>> }`, 7},
		{`SELECT * WHERE { ?p ex:part ?q . ?r (ex:rt|ex:nope) <<( ?p ex:type ?q )>> }`, 0},
		{`SELECT * WHERE { MINUS { } ?p ex:type ?k { ?r (ex:rt|ex:nope) <<( ?p ex:type ?k )>> } }`, 7},
	} {
		var first []string
		for i, v := range []struct {
			off bool
			pre map[string]term.Term
		}{{false, nil}, {true, nil}, {false, root}, {true, root}} {
			setOpts(v.off)
			got := queryRows(t, g, c.query, v.pre)
			if i == 0 {
				first = got
				if len(got) != c.rows {
					t.Errorf("%s: %d rows %v, want %d", c.query, len(got), got, c.rows)
				}
				continue
			}
			if !slices.Equal(got, first) {
				t.Errorf("%s: variant %d differs: %d rows, want %d", c.query, i, len(got), len(first))
			}
		}
	}
}
