package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/paths"
)

// ParsedQuery is the parsed representation of a SPARQL query.
// Ported from: rdflib.plugins.sparql.parserutils.CompValue
type ParsedQuery struct {
	Type           string // "SELECT", "ASK", "CONSTRUCT"
	Distinct       bool
	Variables      []string // projection vars (nil = *)
	ProjectExprs   []ProjectExpr
	Where          Pattern
	OrderBy        []OrderExpr
	Limit          int // -1 = no limit
	Offset         int
	Prefixes       map[string]string // prefix → namespace
	Construct      []TripleTemplate  // CONSTRUCT template
	GroupBy        []Expr
	GroupByAliases []string // parallel to GroupBy: variable name if (expr AS ?var), else ""
	Having         Expr
	BaseURI        string
	NamedGraphs    map[string]*rdflibgo.Graph // graph IRI → graph data (for GRAPH clause)
}

// ProjectExpr is a (expr AS ?var) in SELECT.
type ProjectExpr struct {
	Expr Expr
	Var  string
}

// TripleTemplate is a triple pattern used in CONSTRUCT.
type TripleTemplate struct {
	Subject, Predicate, Object string // variable names or N3 terms
}

// OrderExpr is an ORDER BY expression.
type OrderExpr struct {
	Expr Expr
	Desc bool
}

// Pattern represents a WHERE clause pattern.
type Pattern interface {
	isPattern()
}

// BGP is a Basic Graph Pattern.
type BGP struct {
	Triples []Triple
}

func (*BGP) isPattern() {}

// Triple is a triple pattern with possible variables.
type Triple struct {
	Subject, Predicate, Object string // "?var" or N3 term
	PredicatePath              paths.Path
}

// JoinPattern joins two patterns.
type JoinPattern struct {
	Left, Right Pattern
}

func (*JoinPattern) isPattern() {}

// OptionalPattern is a LEFT JOIN.
type OptionalPattern struct {
	Main, Optional Pattern
}

func (*OptionalPattern) isPattern() {}

// UnionPattern is a UNION of two patterns.
type UnionPattern struct {
	Left, Right Pattern
}

func (*UnionPattern) isPattern() {}

// FilterPattern wraps a pattern with a FILTER expression.
type FilterPattern struct {
	Pattern Pattern
	Expr    Expr
}

func (*FilterPattern) isPattern() {}

// BindPattern introduces a new variable binding.
type BindPattern struct {
	Pattern Pattern
	Expr    Expr
	Var     string
}

func (*BindPattern) isPattern() {}

// ValuesPattern provides inline data.
type ValuesPattern struct {
	Vars   []string
	Values [][]rdflibgo.Term
}

func (*ValuesPattern) isPattern() {}

// GraphPattern wraps a pattern inside a GRAPH clause.
type GraphPattern struct {
	Name    string // graph name (variable or IRI)
	Pattern Pattern
}

func (*GraphPattern) isPattern() {}

// MinusPattern removes solutions from left that are compatible with right.
type MinusPattern struct {
	Left, Right Pattern
}

func (*MinusPattern) isPattern() {}

// SubqueryPattern wraps a sub-SELECT query as a pattern.
type SubqueryPattern struct {
	Query *ParsedQuery
}

func (*SubqueryPattern) isPattern() {}

// Expr is a filter/bind expression.
type Expr interface {
	isExpr()
}

type VarExpr struct{ Name string }

func (*VarExpr) isExpr() {}

type LiteralExpr struct{ Value rdflibgo.Term }

func (*LiteralExpr) isExpr() {}

type IRIExpr struct{ Value string }

func (*IRIExpr) isExpr() {}

type BinaryExpr struct {
	Op          string // "=", "!=", "<", ">", "<=", ">=", "&&", "||", "+", "-", "*", "/"
	Left, Right Expr
}

func (*BinaryExpr) isExpr() {}

type UnaryExpr struct {
	Op  string // "!", "-"
	Arg Expr
}

func (*UnaryExpr) isExpr() {}

type FuncExpr struct {
	Name      string
	Args      []Expr
	Distinct  bool   // COUNT(DISTINCT ?x)
	Separator string // GROUP_CONCAT(... ; SEPARATOR=",")
	Star      bool   // COUNT(*)
	IRI       string // full IRI, verbatim, when the call was written as <iri>(...) or prefix:local(...)

	// Fn is a query-scoped extension function bound by ParsedQuery.BindFunctions.
	// When set it is called instead of consulting the global registry. It is
	// written before evaluation starts and only read thereafter.
	Fn Function

	// FnCtx is Fn for a function bound by ParsedQuery.BindContextFunctions,
	// which receives the evaluation's context. At most one of Fn and FnCtx is
	// set: the binding made last wins.
	FnCtx ContextFunction
}

func (*FuncExpr) isExpr() {}

// ExistsExpr evaluates EXISTS { pattern } or NOT EXISTS { pattern }.
type ExistsExpr struct {
	Pattern Pattern
	Not     bool
}

func (*ExistsExpr) isExpr() {}

// --- SPARQL 1.1 Update AST types ---

// ParsedUpdate is the parsed representation of a SPARQL Update request.
type ParsedUpdate struct {
	Operations []UpdateOperation
	Prefixes   map[string]string
	BaseURI    string
}

// UpdateOperation is a single SPARQL Update operation.
type UpdateOperation interface{ isUpdateOp() }

// InsertDataOp represents INSERT DATA { quads }.
type InsertDataOp struct{ Quads []QuadPattern }

func (*InsertDataOp) isUpdateOp() {}

// DeleteDataOp represents DELETE DATA { quads }.
type DeleteDataOp struct{ Quads []QuadPattern }

func (*DeleteDataOp) isUpdateOp() {}

// DeleteWhereOp represents DELETE WHERE { quads }.
type DeleteWhereOp struct{ Quads []QuadPattern }

func (*DeleteWhereOp) isUpdateOp() {}

// ModifyOp represents DELETE { } INSERT { } WHERE { } with optional WITH/USING.
type ModifyOp struct {
	With   string        // WITH <graph>
	Delete []QuadPattern // DELETE template
	Insert []QuadPattern // INSERT template
	Using  []UsingClause // USING clauses
	Where  Pattern       // WHERE clause
}

func (*ModifyOp) isUpdateOp() {}

// GraphMgmtOp represents CLEAR, DROP, CREATE, LOAD, ADD, MOVE, COPY.
type GraphMgmtOp struct {
	Op     string // CLEAR, DROP, CREATE, LOAD, ADD, MOVE, COPY
	Silent bool
	Target string // graph IRI, DEFAULT, NAMED, ALL
	Source string // for ADD/MOVE/COPY source
	Into   string // for LOAD INTO GRAPH <g>
}

func (*GraphMgmtOp) isUpdateOp() {}

// QuadPattern groups triples under a graph name ("" = default graph).
type QuadPattern struct {
	Graph   string // "" = default graph
	Triples []Triple
}

// UsingClause is a USING [NAMED] <iri> clause.
type UsingClause struct {
	IRI   string
	Named bool
}
