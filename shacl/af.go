package shacl

import (
	"errors"
	"fmt"

	"github.com/tggo/goRDFlib/provenance"
)

// SHACL Advanced Features (SHACL-AF) support.
//
// SHACL-AF is a W3C Working Group Note, not a Recommendation:
// https://www.w3.org/TR/shacl-af/. It is nevertheless what the established
// tooling implements — pySHACL and TopQuadrant's Java API both do — so shapes
// written against it are common in the wild.
//
// SHACL 1.2 respecifies much of the same ground with different vocabulary and
// different semantics: node expressions move to the shnex: namespace and rules
// become SRL. Both are supported here, and they are kept apart deliberately.
// AF is off by default, so a graph that uses neither pays nothing and a graph
// written for 1.2 cannot have AF semantics applied to it by accident. Pass
// WithAdvancedFeatures to opt in:
//
//	report := shacl.Validate(data, shapes, shacl.WithAdvancedFeatures())
//
// The two vocabularies do not overlap: AF keys off sh:rule, sh:target,
// sh:union, sh:intersection, sh:filterShape and sh:path-bearing blank nodes,
// while SHACL 1.2 keys off shnex:*. Enabling AF therefore does not change how
// any 1.2 construct behaves.

// AF vocabulary. These IRIs are only interpreted when AF is enabled.
const (
	// Rules (SHACL-AF §7).
	SHRule       = SH + "rule"
	SHTripleRule = SH + "TripleRule"
	SHSPARQLRule = SH + "SPARQLRule"
	SHCondition  = SH + "condition"
	SHConstruct  = SH + "construct"
	SHSubject    = SH + "subject"
	SHPredicate  = SH + "predicate"
	SHObject     = SH + "object"
	SHOrder      = SH + "order"

	// Node expressions (SHACL-AF §6).
	SHThis         = SH + "this"
	SHUnion        = SH + "union"
	SHIntersection = SH + "intersection"
	SHFilterShape  = SH + "filterShape"
	SHNodes        = SH + "nodes"

	// Functions (SHACL-AF §5).
	SHSPARQLFunction = SH + "SPARQLFunction"
	SHReturnType     = SH + "returnType"
	SHOptional       = SH + "optional"

	// Targets (SHACL-AF §4).
	SHTarget           = SH + "target"
	SHSPARQLTarget     = SH + "SPARQLTarget"
	SHSPARQLTargetType = SH + "SPARQLTargetType"
)

// ErrAdvancedFeatures is the base for every SHACL-AF error, so a caller can
// separate an AF problem from anything else with errors.Is.
var ErrAdvancedFeatures = errors.New("shacl: advanced features")

// ErrRuleIterationLimit is returned when rule application does not reach a
// fixed point. Rules can be genuinely non-terminating — a rule whose head feeds
// its own body through a function that mints new values never settles — so the
// number of rounds is capped rather than trusted.
var ErrRuleIterationLimit = fmt.Errorf("%w: rule iteration limit reached", ErrAdvancedFeatures)

// ErrMalformedRule is returned for a rule that cannot be read: a sh:rule value
// that is neither a sh:TripleRule nor a sh:SPARQLRule, a sh:TripleRule missing
// one of sh:subject/sh:predicate/sh:object, or a sh:SPARQLRule without
// sh:construct.
var ErrMalformedRule = fmt.Errorf("%w: malformed rule", ErrAdvancedFeatures)

// ErrMalformedFunction is returned for a sh:SPARQLFunction that declares
// neither sh:select nor sh:ask, or declares both.
var ErrMalformedFunction = fmt.Errorf("%w: malformed function", ErrAdvancedFeatures)

// ErrMalformedExpression is returned for a node expression that cannot be
// read, such as a function call naming an IRI that no function declares.
var ErrMalformedExpression = fmt.Errorf("%w: malformed node expression", ErrAdvancedFeatures)

// DefaultRuleIterationLimit bounds how many times the rule set is re-applied
// while it keeps producing triples. It matches pySHACL's limit.
const DefaultRuleIterationLimit = 100

// Option configures validation and rule application.
//
// Options are additive and order-independent.
type Option func(*config)

type config struct {
	advanced           bool
	ruleIteration      bool
	ruleIterationLimit int
	errorHandler       func(error)
	provenance         *provenance.Index
}

func newConfig(opts []Option) *config {
	c := &config{ruleIterationLimit: DefaultRuleIterationLimit}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	return c
}

// report hands err to the caller's error handler, if any. Used on the paths
// that cannot return an error, so an AF problem is visible rather than silent.
func (c *config) report(err error) {
	if err != nil && c.errorHandler != nil {
		c.errorHandler(err)
	}
}

// WithAdvancedFeatures enables SHACL-AF: rules (sh:rule), SPARQL-based targets
// (sh:target), SHACL functions (sh:SPARQLFunction) and AF node expressions.
//
// Under Validate, rules run first and validation then sees the triples they
// infer. The caller's data graph is not modified; use ApplyRules to inspect or
// keep the inferred triples.
func WithAdvancedFeatures() Option {
	return func(c *config) { c.advanced = true }
}

// WithRuleIteration re-runs each shape's rules until they stop producing
// triples, instead of making one pass in sh:order.
//
// SHACL-AF specifies ordered execution, not a fixed point: a rule set that
// needs chaining says so by giving the later rule a larger sh:order, and that
// is what a single pass implements. Iterating is the right choice when the
// chain is not expressible as an order — rules on different shapes feeding each
// other, or a rule whose own output re-triggers it. It costs a re-evaluation of
// every rule per round and can fail with ErrRuleIterationLimit on a rule set
// that never settles, which is why it is not the default.
func WithRuleIteration() Option {
	return func(c *config) { c.ruleIteration = true }
}

// WithRuleIterationLimit overrides how many rounds of rule application are
// attempted before ErrRuleIterationLimit. It only has an effect together with
// WithRuleIteration. A limit below 1 is ignored, since it would make every rule
// set fail.
func WithRuleIterationLimit(n int) Option {
	return func(c *config) {
		if n >= 1 {
			c.ruleIterationLimit = n
		}
	}
}

// WithErrorHandler installs a callback for problems that Validate cannot
// return, because its signature has no error: a malformed rule, a function that
// fails to load, a rule set that does not terminate. Without a handler these
// are silent — validation continues on whatever was inferred before the
// problem — so a handler is the way to tell "the shapes conform" from "the
// rules never ran".
//
// The handler may be called from validation; it must be safe for concurrent use
// if the same handler is passed to concurrent Validate calls.
func WithErrorHandler(h func(error)) Option {
	return func(c *config) { c.errorHandler = h }
}

// ApplyRules applies the SHACL-AF rules in shapesGraph to dataGraph, adding the
// inferred triples to dataGraph in place, and returns how many were added.
//
// Shapes run in ascending sh:order and each shape's rules run in ascending
// sh:order, once per focus node — the execution model SHACL-AF specifies. Pass
// WithRuleIteration to repeat instead until nothing new is produced.
//
// A malformed rule does not stop the others: the triples the readable rules
// produce are still added, and the first problem is returned alongside the
// count. Check the error even when the count is non-zero.
//
// Unlike Validate, this does not require WithAdvancedFeatures — asking for
// rules by name is the opt-in. It is not safe to call concurrently with other
// use of dataGraph.
func ApplyRules(dataGraph, shapesGraph *Graph, opts ...Option) (int, error) {
	c := newConfig(opts)
	c.advanced = true
	ctx, err := newAFContext(dataGraph, shapesGraph, c)
	if err != nil {
		return 0, err
	}
	return ctx.applyRules()
}
