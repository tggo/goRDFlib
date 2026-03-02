package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/paths"
)

// ParsedQuery is the parsed representation of a SPARQL query.
// Ported from: rdflib.plugins.sparql.parserutils.CompValue
type ParsedQuery struct {
	Type         string // "SELECT", "ASK", "CONSTRUCT"
	Distinct     bool
	Variables    []string // projection vars (nil = *)
	ProjectExprs []ProjectExpr
	Where        Pattern
	OrderBy      []OrderExpr
	Limit        int // -1 = no limit
	Offset       int
	Prefixes     map[string]string // prefix → namespace
	Construct    []TripleTemplate  // CONSTRUCT template
	GroupBy        []Expr
	GroupByAliases []string // parallel to GroupBy: variable name if (expr AS ?var), else ""
	Having         Expr
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
	patternType() string
}

// BGP is a Basic Graph Pattern.
type BGP struct {
	Triples []Triple
}

func (b *BGP) patternType() string { return "BGP" }

// Triple is a triple pattern with possible variables.
type Triple struct {
	Subject, Predicate, Object string // "?var" or N3 term
	PredicatePath              paths.Path
}

// JoinPattern joins two patterns.
type JoinPattern struct {
	Left, Right Pattern
}

func (j *JoinPattern) patternType() string { return "Join" }

// OptionalPattern is a LEFT JOIN.
type OptionalPattern struct {
	Main, Optional Pattern
}

func (o *OptionalPattern) patternType() string { return "Optional" }

// UnionPattern is a UNION of two patterns.
type UnionPattern struct {
	Left, Right Pattern
}

func (u *UnionPattern) patternType() string { return "Union" }

// FilterPattern wraps a pattern with a FILTER expression.
type FilterPattern struct {
	Pattern Pattern
	Expr    Expr
}

func (f *FilterPattern) patternType() string { return "Filter" }

// BindPattern introduces a new variable binding.
type BindPattern struct {
	Pattern Pattern
	Expr    Expr
	Var     string
}

func (b *BindPattern) patternType() string { return "Bind" }

// ValuesPattern provides inline data.
type ValuesPattern struct {
	Vars   []string
	Values [][]rdflibgo.Term
}

func (v *ValuesPattern) patternType() string { return "Values" }

// MinusPattern removes solutions from left that are compatible with right.
type MinusPattern struct {
	Left, Right Pattern
}

func (m *MinusPattern) patternType() string { return "Minus" }

// SubqueryPattern wraps a sub-SELECT query as a pattern.
type SubqueryPattern struct {
	Query *ParsedQuery
}

func (s *SubqueryPattern) patternType() string { return "Subquery" }

// Expr is a filter/bind expression.
type Expr interface {
	exprType() string
}

type VarExpr struct{ Name string }

func (e *VarExpr) exprType() string { return "Var" }

type LiteralExpr struct{ Value rdflibgo.Term }

func (e *LiteralExpr) exprType() string { return "Literal" }

type IRIExpr struct{ Value string }

func (e *IRIExpr) exprType() string { return "IRI" }

type BinaryExpr struct {
	Op          string // "=", "!=", "<", ">", "<=", ">=", "&&", "||", "+", "-", "*", "/"
	Left, Right Expr
}

func (e *BinaryExpr) exprType() string { return "Binary" }

type UnaryExpr struct {
	Op  string // "!", "-"
	Arg Expr
}

func (e *UnaryExpr) exprType() string { return "Unary" }

type FuncExpr struct {
	Name      string
	Args      []Expr
	Distinct  bool   // COUNT(DISTINCT ?x)
	Separator string // GROUP_CONCAT(... ; SEPARATOR=",")
	Star      bool   // COUNT(*)
}

func (e *FuncExpr) exprType() string { return "Func" }

// ExistsExpr evaluates EXISTS { pattern } or NOT EXISTS { pattern }.
type ExistsExpr struct {
	Pattern Pattern
	Not     bool
}

func (e *ExistsExpr) exprType() string { return "Exists" }
