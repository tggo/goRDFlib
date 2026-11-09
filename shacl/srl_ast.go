package shacl

// SRL namespace.
const SRL = "http://www.w3.org/ns/shacl-rules#"

// RuleSet is the top-level AST node for a SPARQL-RL (formerly SHACL Rules,
// "SRL") document.
//
// A RuleSet is read-only once parsed and is safe for concurrent use.
type RuleSet struct {
	Base     string
	Prefixes map[string]string
	// Version is the VERSION declaration, or "".
	Version string
	// Imports lists the IMPORTS declarations. Importing is optional for a
	// processor (SPARQL-RL "Processing Imports"); this one does not fetch
	// them, and EvalRuleSet refuses a rule set that has any (ErrSRLImports).
	Imports    []string
	DataBlocks [][]SRLTriple
	Rules      []SRLRule
}

// SRLRule represents a single RULE ... WHERE ... block.
type SRLRule struct {
	// ID is the optional rule identifier (an IRI), or "".
	ID   string
	Head []SRLTriple
	Body []SRLBodyElement
	// BaseGraphOnly is set by WHERE DATA: the body is matched against the
	// base graph (the input), not against what rules have inferred
	// (SPARQL-RL "rule.evaldata" == false). Such a rule depends on no rule.
	BaseGraphOnly bool

	// prologue is the SPARQL BASE/PREFIX text in force where the rule was
	// written; expressions are parsed and evaluated under it.
	prologue string
}

// SRLTriple is a triple pattern or template (subject, predicate, object).
//
// In a rule body the predicate position can be a property path: Path is then
// non-nil and Predicate is unused.
type SRLTriple struct {
	Subject   SRLTerm
	Predicate SRLTerm
	Object    SRLTerm
	Path      *SRLPath
}

// SRLPathKind distinguishes the nodes of an SRLPath.
type SRLPathKind int

const (
	SRLPathIRI     SRLPathKind = iota // a single predicate
	SRLPathInverse                    // ^path
	SRLPathSeq                        // path / path / ...
)

// SRLPath is a property path of a rule body: the SPARQL-RL grammar has
// sequence, inverse and grouping only.
type SRLPath struct {
	Kind  SRLPathKind
	IRI   string     // SRLPathIRI
	Elems []*SRLPath // SRLPathSeq (two or more), SRLPathInverse (one)
}

// SRLTermKind distinguishes different kinds of SRL terms.
type SRLTermKind int

const (
	SRLTermIRI SRLTermKind = iota
	SRLTermVariable
	SRLTermLiteral
	SRLTermBlankNode
	SRLTermTripleTerm // <<( s p o )>>
)

// SRLTerm represents a term in an SRL document.
type SRLTerm struct {
	Kind     SRLTermKind
	Value    string // IRI value, variable name, literal lexical value, blank node ID
	Datatype string // for literals
	Language string // for literals (may include --dir)

	// For triple terms
	TTSubject   *SRLTerm
	TTPredicate *SRLTerm
	TTObject    *SRLTerm
}

// IsVariable returns true if the term is a variable.
func (t SRLTerm) IsVariable() bool { return t.Kind == SRLTermVariable }

// SRLBodyElementKind distinguishes body element types.
type SRLBodyElementKind int

const (
	SRLBodyTriple SRLBodyElementKind = iota
	SRLBodyFilter
	SRLBodyNot
	// SRLBodyBind is the assignment element SET ( ?var := expr ).
	SRLBodyBind
	// SRLBodyGroup is the group element GROUP [BY keys] { pattern } [aggregations].
	SRLBodyGroup
)

// SRLAggregation is one aggregation assignment ?var := FUNC(arg) of a group.
type SRLAggregation struct {
	Var      string // without the sigil
	Func     string // COUNT, MIN, MAX or SUM
	Distinct bool   // COUNT(DISTINCT ...) only
	Arg      string // a variable name (without the sigil) or "*"
}

// SRLBodyElement is a single element in a rule body.
type SRLBodyElement struct {
	Kind SRLBodyElementKind

	// For SRLBodyTriple
	Triple SRLTriple

	// For SRLBodyFilter: the constraint, as SPARQL expression text.
	FilterExpr string

	// For SRLBodyNot
	NotBody []SRLBodyElement
	// NotData is NOT DATA: the negation is matched against the base graph.
	NotData bool

	// For SRLBodyBind (SET): the assigned expression as SPARQL text, and the
	// variable (without the sigil).
	BindExpr string
	BindVar  string

	// For SRLBodyGroup
	GroupKeys []string // without the sigil
	GroupBody []SRLBodyElement
	GroupAggs []SRLAggregation
}
