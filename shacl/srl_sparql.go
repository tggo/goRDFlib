package shacl

import (
	"fmt"
	"strconv"
	"strings"
)

// srlBaseGraphIRI names the base graph inside a rule's query: NOT DATA is
// matched with GRAPH <srlBaseGraphIRI> while the default graph is the
// evaluation graph.
const srlBaseGraphIRI = "urn:x-sparql-rl:base-graph"

// A rule body is evaluated by the sparql engine: ruleQuery writes it as a
// SPARQL SELECT whose solutions are the rule's solution sequence
// ("evalRuleElements" in the specification).
//
// The mapping keeps the specification's sequential reading of the body:
//
//   - triple patterns of consecutive elements form one basic graph pattern;
//   - every other element wraps what precedes it in a group of its own
//     ({ preceding FILTER ... }), so a FILTER, a negation or a SET sees
//     exactly the variables bound before it, whatever follows;
//   - SET ( ?v := e ) is BIND(e AS ?v) FILTER(BOUND(?v)): an error rejects
//     the solution instead of leaving ?v unbound (assignment element);
//   - NOT is FILTER NOT EXISTS, with the row substituted into the pattern;
//   - GROUP is a subquery projecting the keys and the aggregates, joined to
//     what precedes on the keys; an aggregation that errs leaves its variable
//     unbound and FILTER(BOUND()) drops the group, as the specification says;
//   - a blank node of the body is a variable nothing else uses.
type ruleQuery struct {
	b      strings.Builder
	bnodes map[string]string // body blank node label -> variable
	n      int               // counter for generated names
}

// buildRuleQuery returns the SPARQL text that evaluates the body of rule.
func buildRuleQuery(rule SRLRule) string {
	q := &ruleQuery{bnodes: map[string]string{}}
	body := q.sequence(rule.Body, false)
	return rule.prologue + "SELECT * WHERE " + body
}

// sequence writes a body element sequence as a group. useBase makes its
// triple patterns match the base graph (NOT DATA).
func (q *ruleQuery) sequence(elems []SRLBodyElement, useBase bool) string {
	acc := ""
	wrap := func(extra string) { acc = "{ " + acc + " " + extra + " }" }
	for _, e := range elems {
		switch e.Kind {
		case SRLBodyTriple:
			tp := q.triple(e.Triple)
			if useBase {
				tp = "GRAPH <" + srlBaseGraphIRI + "> { " + tp + " }"
			}
			acc += " " + tp
		case SRLBodyFilter:
			wrap("FILTER " + e.FilterExpr)
		case SRLBodyNot:
			wrap("FILTER NOT EXISTS " + q.sequence(e.NotBody, useBase || e.NotData))
		case SRLBodyBind:
			wrap(fmt.Sprintf("BIND ( %s AS ?%s ) FILTER ( BOUND ( ?%s ) )", e.BindExpr, e.BindVar, e.BindVar))
		case SRLBodyGroup:
			wrap(q.group(e, useBase) + q.boundAggs(e))
		}
	}
	return "{ " + acc + " }"
}

func (q *ruleQuery) boundAggs(g SRLBodyElement) string {
	var b strings.Builder
	for _, a := range g.GroupAggs {
		fmt.Fprintf(&b, " FILTER ( BOUND ( ?%s ) )", a.Var)
	}
	return b.String()
}

// group writes the subquery of a group element.
func (q *ruleQuery) group(g SRLBodyElement, useBase bool) string {
	var proj, by strings.Builder
	for _, k := range g.GroupKeys {
		fmt.Fprintf(&proj, "?%s ", k)
		fmt.Fprintf(&by, " ?%s", k)
	}
	for _, a := range g.GroupAggs {
		arg := "*"
		if a.Arg != "*" {
			arg = "?" + a.Arg
		}
		dist := ""
		if a.Distinct {
			dist = "DISTINCT "
		}
		fmt.Fprintf(&proj, "(%s(%s%s) AS ?%s) ", a.Func, dist, arg, a.Var)
	}
	if len(g.GroupAggs) == 0 && len(g.GroupKeys) == 0 {
		// Without keys there is exactly one group, even over no solutions;
		// an aggregate is what makes the engine form it.
		q.n++
		fmt.Fprintf(&proj, "(COUNT(*) AS ?srl_group_%d) ", q.n)
	}
	out := "{ SELECT " + proj.String() + "WHERE " + q.sequence(g.GroupBody, useBase)
	if len(g.GroupKeys) > 0 {
		out += " GROUP BY" + by.String()
	}
	return out + " }"
}

func (q *ruleQuery) triple(t SRLTriple) string {
	var pred string
	if t.Path != nil {
		pred = pathText(t.Path)
	} else {
		pred = q.term(t.Predicate)
	}
	return q.term(t.Subject) + " " + pred + " " + q.term(t.Object) + " ."
}

func pathText(p *SRLPath) string {
	switch p.Kind {
	case SRLPathInverse:
		return "^" + pathText(p.Elems[0])
	case SRLPathSeq:
		parts := make([]string, len(p.Elems))
		for i, e := range p.Elems {
			parts[i] = pathText(e)
		}
		return "(" + strings.Join(parts, "/") + ")"
	}
	return "<" + p.IRI + ">"
}

// term writes a term of a body pattern.
func (q *ruleQuery) term(t SRLTerm) string {
	switch t.Kind {
	case SRLTermIRI:
		return "<" + t.Value + ">"
	case SRLTermVariable:
		return "?" + t.Value
	case SRLTermBlankNode:
		v, ok := q.bnodes[t.Value]
		if !ok {
			v = "?srl_bnode_" + strconv.Itoa(len(q.bnodes))
			q.bnodes[t.Value] = v
		}
		return v
	case SRLTermLiteral:
		return literalText(t)
	case SRLTermTripleTerm:
		return "<<( " + q.term(*t.TTSubject) + " " + q.term(*t.TTPredicate) + " " + q.term(*t.TTObject) + " )>>"
	}
	return ""
}

func literalText(t SRLTerm) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range t.Value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	switch {
	case t.Language != "":
		b.WriteString("@" + t.Language)
	case t.Datatype != "" && t.Datatype != XSD+"string":
		b.WriteString("^^<" + t.Datatype + ">")
	}
	return b.String()
}
