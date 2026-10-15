package jsonld

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/piprate/json-gold/ld"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/iricache"
	"github.com/tggo/goRDFlib/term"
)

// nodeBatchSize is how many nodes a NodeParser converts to RDF together.
// json-gold remembers which IRIs it has validated only for one conversion,
// so converting each node alone re-ran its IRI regular expression on every
// predicate of every node: five times slower than a whole document. 256
// nodes keep that cost negligible and the batch small.
const nodeBatchSize = 256

// NodeParser converts a stream of JSON-LD node objects that share one
// context (issue #53). It is for input such as newline-delimited JSON where
// every line is a node object and the `@context` is known up front: the
// context is processed once, each node is expanded on its own against it,
// and memory is bounded by a batch of nodes, not by the stream.
//
// ParseNode expands a node and holds it; every 256 nodes, and on Flush, the
// held nodes are converted and their triples delivered. Call Flush after the
// last node.
//
// The triples are those Parse would produce for one document whose `@graph`
// holds all the nodes, except that:
//   - a node that fails to expand is left out and reported, and the stream
//     goes on; Parse would fail the whole document;
//   - a subject described in several batches produces its triples from each,
//     so a triple may be delivered more than once (a graph holds it once);
//   - provenance reports a triple at the position of the node, within its
//     batch, that describes the triple's subject (see ParseNode).
//
// Blank node identifiers are scoped to the parser, as they are to a
// document: `_:x` in two nodes is one node, and anonymous nodes are always
// fresh. Labels are minted from a per-parser prefix and json-gold's counter,
// so no table of them is kept; only explicit `_:` identifiers are remembered
// (by json-gold) for the life of the parser.
//
// Options are those of Parse. WithExpandContext is applied before the shared
// context, which a node's own `@context` may in turn extend.
//
// A NodeParser is not safe for concurrent use.
type NodeParser struct {
	cfg     config
	h       TripleHandler
	opts    *ld.JsonLdOptions
	active  *ld.Context
	issuer  *ld.IdentifierIssuer
	conv    *converter
	docBase string
	nodes   int // nodes given to ParseNode so far

	// The batch: node maps merged by graph name and node id, how many
	// nodes they hold, and (with provenance) the position of the first node
	// that described each subject, keyed by the subject's TermKey.
	batch      map[string]any
	batchNodes int
	lineOf     map[string]int
}

// NewNodeParser processes context once and returns a parser that sends
// triples to h. context is a JSON-LD context value — an object, an array, or
// an IRI string — or a whole document holding "@context", in decoded form or
// as JSON bytes (json.RawMessage or []byte). nil means the nodes carry their
// own contexts.
func NewNodeParser(context any, h TripleHandler, opts ...Option) (*NodeParser, error) {
	if h == nil {
		return nil, ErrNilHandler
	}
	p := &NodeParser{h: h}
	for _, o := range opts {
		o(&p.cfg)
	}
	shared, err := decodeContextArg(context)
	if err != nil {
		return nil, err
	}

	p.opts = ld.NewJsonLdOptions(p.cfg.base)
	if p.cfg.documentLoader != nil {
		p.opts.DocumentLoader = p.cfg.documentLoader
	}
	// The expand context and then the shared context, in the order a
	// processor would apply them, each corrected as Parse corrects its
	// expand context (see resolveEmptyFragmentVocab).
	exp, base := fixExpandContextVocab(p.cfg.expandContext, p.cfg.base)
	sh, base := fixExpandContextVocab(shared, base)
	p.docBase = base
	p.active = ld.NewContext(nil, p.opts)
	for _, c := range []any{exp, sh} {
		if c = unwrapContext(c); c == nil {
			continue
		}
		var parsed *ld.Context
		if err := guard("context processing", func() error {
			var err error
			parsed, err = p.active.Parse(c)
			return err
		}); err != nil {
			return nil, fmt.Errorf("jsonld: processing the shared context: %w", err)
		}
		p.active = parsed
	}

	p.issuer = ld.NewIdentifierIssuer("_:b")
	p.conv = &converter{
		bnode:     streamLabeler(p.cfg.preserveBlankNodeIDs),
		unbounded: p.cfg.unbounded,
		iris:      make(map[string]rdflibgo.URIRef, 64),
		vocab:     iricache.New(),
	}
	p.resetBatch()
	return p, nil
}

func (p *NodeParser) resetBatch() {
	p.batch = map[string]any{"@default": map[string]any{}}
	p.batchNodes = 0
	if p.cfg.provenance != nil {
		p.lineOf = make(map[string]int, nodeBatchSize)
	}
}

// streamLabeler maps json-gold's labels ("b0", "b1", ...) to blank nodes
// without a table. json-gold's issuer is shared by the whole stream, so a
// label belongs to one node of the stream; one random prefix per parser
// keeps it apart from every other parse. Under WithPreserveBlankNodeIDs the
// labels are json-gold's, as in Parse.
//
// The label's width is bounded (the prefix plus a counter): json-gold
// discards source labels, so a parse → serialize → parse hop cannot grow it.
func streamLabeler(preserve bool) func(string) rdflibgo.BNode {
	if preserve {
		return func(label string) rdflibgo.BNode { return rdflibgo.NewBNode(label) }
	}
	u := uuid.New()
	prefix := "N" + hex.EncodeToString(u[:])
	return func(label string) rdflibgo.BNode { return rdflibgo.NewBNode(prefix + label) }
}

// decodeContextArg turns NewNodeParser's context argument into a decoded
// JSON value.
func decodeContextArg(c any) (any, error) {
	var raw []byte
	switch v := c.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	default:
		return c, nil
	}
	doc, err := decodeDocument(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("jsonld: shared context: %w", err)
	}
	return doc, nil
}

// ParseNode expands one node — a JSON object, or an array of them — and
// adds it to the current batch, converting and delivering the batch once it
// is full. An error that names the node leaves the batch as it was. Its
// provenance position is its 1-based ordinal among the nodes this parser has
// been given. ctx is checked before the node and before a delivery.
func (p *NodeParser) ParseNode(ctx context.Context, node []byte) error {
	pos := p.nodes + 1
	if err := p.parseNode(ctx, node, pos); err != nil {
		return fmt.Errorf("jsonld: node %d: %w", pos, err)
	}
	return nil
}

// Flush converts the nodes held since the last delivery and delivers their
// triples. Call it after the last node.
func (p *NodeParser) Flush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.batchNodes == 0 {
		return nil
	}
	batch, lineOf := p.batch, p.lineOf
	p.resetBatch()

	ds := ld.NewRDFDataset()
	for name, g := range batch {
		if ld.IsRelativeIri(name) {
			continue
		}
		ds.GraphToRDF(name, g.(map[string]any), p.issuer, p.opts.ProduceGeneralizedRdf)
	}
	cfg := p.cfg
	cfg.provenance = nil // reported below, from lineOf
	prov := p.cfg.provenance
	return addDatasetWith(func(st statement) error {
		if err := p.h(st.s, st.p, st.o); err != nil {
			return err
		}
		if prov != nil {
			if line := lineOf[term.TermKey(st.s)]; line > 0 {
				prov(st.s, st.p, st.o, line)
			}
		}
		return nil
	}, ds, &cfg, p.conv, nil)
}

// parseNode adds the node at pos to the batch; an error leaves the batch as
// it was.
func (p *NodeParser) parseNode(ctx context.Context, node []byte, pos int) error {
	p.nodes++
	if err := ctx.Err(); err != nil {
		return err
	}
	doc, err := decodeDocument(bytes.NewReader(node))
	if err != nil {
		return err
	}
	resolveEmptyFragmentVocab(doc, p.docBase)

	var nodeMap map[string]any
	if err := guard("expansion to RDF", func() error {
		var err error
		nodeMap, err = p.nodeMap(doc)
		return err
	}); err != nil {
		return err
	}
	p.merge(nodeMap, pos)
	if p.batchNodes++; p.batchNodes >= nodeBatchSize {
		return p.Flush(ctx)
	}
	return nil
}

// nodeMap expands doc against the shared context and flattens it into its
// own node map, as json-gold's JsonLdProcessor.ToRDF does for a document,
// but with the active context processed once and the identifier issuer
// shared by the stream.
func (p *NodeParser) nodeMap(doc any) (map[string]any, error) {
	api := ld.NewJsonLdApi()
	expanded, err := api.Expand(p.active, "", doc, p.opts, false, nil)
	if err != nil {
		return nil, err
	}
	// The final step of the expansion algorithm, as json-gold's processor
	// does it: an empty result is nothing, a lone @graph is its contents.
	if m, ok := expanded.(map[string]any); ok {
		if g, has := m["@graph"]; has && len(m) == 1 {
			expanded = g
		} else if len(m) == 0 {
			expanded = nil
		}
	}
	if expanded == nil {
		expanded = []any{}
	}
	if _, ok := expanded.([]any); !ok {
		expanded = []any{expanded}
	}
	// A node map of its own: GenerateNodeMap can fail (conflicting @index)
	// after it has written part of the node, and the batch must not see it.
	nodeMap := map[string]any{"@default": map[string]any{}}
	if _, err := api.GenerateNodeMap(expanded, nodeMap, "@default", p.issuer, "", "", nil); err != nil {
		return nil, err
	}
	return nodeMap, nil
}

// merge adds a node's node map to the batch. A node object that is already
// in the batch gets the new values appended: GraphToRDF turns every value of
// a property into a triple, so a value both nodes carry becomes a repeated
// triple, which the docs allow. With provenance, a subject the node
// describes (more than a bare reference) is attributed to pos unless an
// earlier node of the batch described it.
func (p *NodeParser) merge(nodeMap map[string]any, pos int) {
	for name, g := range nodeMap {
		graph := g.(map[string]any)
		into, ok := p.batch[name].(map[string]any)
		if !ok {
			into = make(map[string]any, len(graph))
			p.batch[name] = into
		}
		for id, n := range graph {
			node := n.(map[string]any)
			if p.lineOf != nil && len(node) > 1 {
				key := p.subjectKey(id)
				if _, seen := p.lineOf[key]; !seen {
					p.lineOf[key] = pos
				}
			}
			prev, ok := into[id].(map[string]any)
			if !ok {
				into[id] = node
				continue
			}
			for k, v := range node {
				vs, isList := v.([]any)
				old, had := prev[k].([]any)
				switch {
				case isList && had:
					prev[k] = append(old, vs...)
				case !had:
					if _, present := prev[k]; !present {
						prev[k] = v
					}
				}
			}
		}
	}
}

// subjectKey is the TermKey the converter will give the node id.
func (p *NodeParser) subjectKey(id string) string {
	if len(id) > 2 && id[0] == '_' && id[1] == ':' {
		return term.TermKey(p.conv.bnode(id[2:]))
	}
	return term.TermKey(rdflibgo.NewURIRefUnsafe(id))
}
