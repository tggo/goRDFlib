package sparql

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

func pathCandidate(t *testing.T, query string, pre map[string]term.Term) (bgpCandidate, map[string]string) {
	t.Helper()
	pq, err := Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	tps := pq.Where.(*BGP).Triples
	if len(tps) != 1 || tps[0].PredicatePath == nil {
		t.Fatalf("want one path pattern, got %v", tps)
	}
	return newBGPCandidate(tps[0], pre, pq.Prefixes), pq.Prefixes
}

// A probe ends after bgpPathProbeSteps steps, not only after limit pairs: a
// walk from a subject with 20,000 ex:a values, none of which has an ex:b,
// yields no pair at all, and probing it must not read all of them. A probe
// that runs out ranks as capped, not as an empty path.
func TestOrderBGP_PathProbeStepLimit(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	for i := range 20_000 {
		g.Add(ctx, u("s"), u("a"), u(fmt.Sprintf("m%d", i)))
	}
	c, _ := pathCandidate(t, `SELECT * WHERE { <urn:s> <urn:a>/<urn:b> ?x }`, nil)
	if !c.pathProbe {
		t.Fatal("a path from a constant must be probed")
	}
	ec := newEvalCtx(ctx)
	cands := []bgpCandidate{c}
	probeCandidates(ec, g, cands)
	if ec.err != nil {
		t.Fatalf("the step limit must not fail the query: %v", ec.err)
	}
	if cands[0].pathExact {
		t.Errorf("a probe stopped by its step limit counted %d pairs as exact", cands[0].count)
	}
}

// The probes of one evaluation are bounded in work, not in number: once their
// steps pass probeAllowance, paths rank unprobed, and work the evaluation does
// itself raises the allowance again.
func TestOrderBGP_PathProbesCappedPerEvaluation(t *testing.T) {
	g := pathProbeGraph(t)
	c, _ := pathCandidate(t, `SELECT * WHERE { ?x <urn:type>/<urn:sub>* <urn:Class1> }`, nil)
	probe := func(ec *evalCtx) bool {
		cands := []bgpCandidate{c}
		probeCandidates(ec, g, cands)
		return cands[0].pathExact
	}

	ec := newEvalCtx(t.Context())
	if !probe(ec) {
		t.Fatal("a fresh evaluation must probe")
	}
	if ec.probeSteps == 0 {
		t.Fatal("a probe was not charged for its steps")
	}
	// Repeating the probe spends the floor, then stops: the total stays
	// within the floor plus one probe, which is bounded by bgpPathProbeSteps.
	for range 10 * bgpProbeFloor {
		if !probe(ec) {
			break
		}
	}
	if probe(ec) {
		t.Fatal("probing never stopped")
	}
	if ec.probeSteps > bgpProbeFloor+bgpPathProbeSteps {
		t.Errorf("probes took %d steps, floor %d", ec.probeSteps, bgpProbeFloor)
	}
	// Work done by the evaluation itself pays for more probes.
	ec.n += 2 * bgpProbeShare * ec.probeSteps
	if !probe(ec) {
		t.Error("probing did not resume after the evaluation did work")
	}
}

// Probes are not counted: an OPTIONAL planned once per row of 1,500 rows
// probes for (nearly) every row, because each probe is cheap, and the answer
// does not change. A count of 64 used to end probing at the 65th row.
func TestOrderBGP_PathProbesPerRowAreBounded(t *testing.T) {
	g := pathProbeGraph(t)
	q := `SELECT ?y ?c WHERE { <urn:root> <urn:part> ?y OPTIONAL { ?y <urn:type>/<urn:sub>* ?c . ?c <urn:sub>* <urn:Class1> } }`
	probes := 0
	pathProbeHook = func() { probes++ }
	defer func() { pathProbeHook = nil }()
	defer setOpts(false)
	var counts [2]int
	for i, off := range []bool{false, true} {
		setOpts(off)
		res, err := Query(g, q)
		if err != nil {
			t.Fatal(err)
		}
		counts[i] = len(res.Bindings)
		if i == 0 {
			if probes <= 64 {
				t.Errorf("%d path probes over 1500 rows, want more than 64", probes)
			}
			probes = 0
		} else if probes != 0 {
			t.Errorf("%d path probes with probing switched off", probes)
		}
	}
	if counts[0] != 1500 || counts[1] != 1500 {
		t.Errorf("rows %v, want 1500 both ways", counts)
	}
}

// A path whose subject is bound to a literal has no subject to walk from: it
// is not probed (a literal is not a Subject), and the query still answers.
func TestOrderBGP_PathLiteralSubjectNotProbed(t *testing.T) {
	pre := map[string]term.Term{"s": term.NewLiteral("x")}
	c, _ := pathCandidate(t, `SELECT * WHERE { ?s <urn:part>+ ?x }`, pre)
	if c.pathProbe {
		t.Error("a path with a literal subject was marked for probing")
	}
	if c.fixed != 1 {
		t.Errorf("fixed = %d, want 1: the literal still binds the subject", c.fixed)
	}
	c, _ = pathCandidate(t, `SELECT * WHERE { "x" ^<urn:label> ?x }`, nil)
	if c.pathProbe {
		t.Error("a path with a literal subject constant was marked for probing")
	}
}

// A path whose object alone is known is probed, backwards, and counted
// exactly.
func TestOrderBGP_PathBoundObjectProbed(t *testing.T) {
	g := pathProbeGraph(t)
	for _, tc := range []struct {
		name, query string
		pre         map[string]term.Term
	}{
		{"constant", `SELECT * WHERE { ?x <urn:type>/<urn:sub>* <urn:Class1> }`, nil},
		{"pre-bound", `SELECT * WHERE { ?x <urn:type>/<urn:sub>* ?c }`, map[string]term.Term{"c": term.NewURIRefUnsafe("urn:Class1")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := pathCandidate(t, tc.query, tc.pre)
			if !c.pathProbe || c.s != nil || c.o == nil {
				t.Fatalf("pathProbe=%v s=%v o=%v, want a probe from the object", c.pathProbe, c.s, c.o)
			}
			cands := []bgpCandidate{c}
			probeCandidates(newEvalCtx(t.Context()), g, cands)
			if !cands[0].pathExact || cands[0].count != 20 {
				t.Errorf("exact=%v count=%d, want exactly 20", cands[0].pathExact, cands[0].count)
			}
		})
	}
}
