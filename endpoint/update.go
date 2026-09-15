package endpoint

import (
	"errors"
	"net/http"

	"github.com/tggo/goRDFlib/internal/iri"
	"github.com/tggo/goRDFlib/sparql"
)

// update evaluates an update request.
func (x *exchange) update(req *protocolRequest) error {
	h := x.h
	u, err := sparql.ParseUpdate(req.text)
	if err != nil {
		return httpErrf(http.StatusBadRequest, "%s", withLineColumn(err.Error(), req.text))
	}
	if u.BaseURI == "" {
		u.BaseURI = h.baseIRI(x.r)
	}
	if err := checkGraphRefs(u); err != nil {
		return err
	}

	if len(req.defaultGraphs) > 0 || len(req.namedGraphs) > 0 {
		// SPARQL 1.1 Protocol §2.2.3: the parameters define the dataset of every
		// operation with a WHERE clause, and must not meet USING or WITH.
		using := make([]sparql.UsingClause, 0, len(req.defaultGraphs)+len(req.namedGraphs))
		for _, g := range req.defaultGraphs {
			using = append(using, sparql.UsingClause{IRI: g})
		}
		for _, g := range req.namedGraphs {
			using = append(using, sparql.UsingClause{IRI: g, Named: true})
		}
		for _, op := range u.Operations {
			if m, ok := op.(*sparql.ModifyOp); ok && (m.With != "" || len(m.Using) > 0) {
				return httpErrf(http.StatusBadRequest,
					"using-graph-uri and using-named-graph-uri must not be combined with USING, USING NAMED or WITH in the update (SPARQL 1.1 Protocol §2.2.3)")
			}
		}
		for _, op := range u.Operations {
			if m, ok := op.(*sparql.ModifyOp); ok {
				m.Using = using
			}
		}
	}

	if h.cfg.loader == nil {
		for _, op := range u.Operations {
			if g, ok := op.(*sparql.GraphMgmtOp); ok && g.Op == "LOAD" && !g.Silent {
				// Refused before anything runs, so the request is not left
				// half applied.
				return httpErr(http.StatusForbidden, ErrLoadDisabled)
			}
		}
	}

	ctx, cancel := withTimeout(x.r.Context(), h.cfg.updateTimeout)
	defer cancel()
	if err := h.lock.lock(ctx); err != nil {
		return evalError(ctx, err)
	}
	err = func() error {
		// Deferred, so that a panic in the engine does not leave the lock held.
		defer h.lock.unlock()
		ds := *h.backend.updateDataset()
		ds.Loader = h.cfg.loader
		defer h.backend.commit(&ds)
		return sparql.EvalUpdateContext(ctx, &ds, u)
	}()
	if err != nil {
		if errors.Is(err, sparql.ErrQueryCancelled) {
			return evalError(ctx, err)
		}
		return httpErrf(http.StatusBadRequest, "update failed: %v", err)
	}
	x.w.Header().Set("Cache-Control", "no-store")
	x.w.WriteHeader(http.StatusNoContent)
	return nil
}

// checkGraphRefs rejects graph management operations whose graph is neither
// DEFAULT, NAMED, ALL nor an absolute IRI. The update parser accepts any term
// there (CLEAR XYZ parses and clears nothing), which would turn a typo into a
// silent success.
func checkGraphRefs(u *sparql.ParsedUpdate) error {
	for _, op := range u.Operations {
		g, ok := op.(*sparql.GraphMgmtOp)
		if !ok {
			continue
		}
		refs := []string{g.Target, g.Into}
		if g.Op != "LOAD" {
			refs = append(refs, g.Source)
		}
		for _, ref := range refs {
			switch ref {
			case "", "DEFAULT", "NAMED", "ALL":
				continue
			}
			if !iri.IsAbsolute(ref) || !validIRIChars(ref) {
				return httpErrf(http.StatusBadRequest,
					"%s: %q is not a graph (expected DEFAULT, NAMED, ALL or GRAPH <absolute IRI>)", g.Op, ref)
			}
		}
	}
	return nil
}
