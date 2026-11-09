package shacl

import "fmt"

// CheckWellformed checks the SPARQL-RL well-formedness conditions of every
// rule of rs ("Well-formedness Conditions"). In a well-formed rule every
// variable of a FILTER or SET expression is bound by an element before it;
// SET and aggregation variables are new; a negation and a group pattern are
// themselves well-formed given the variables in scope; and every variable of
// the head is bound by the body. The error wraps ErrSRLNotWellFormed.
func CheckWellformed(rs *RuleSet) error {
	for i, rule := range rs.Rules {
		if err := checkRuleWellformed(rule); err != nil {
			return fmt.Errorf("%w: rule %d: %s", ErrSRLNotWellFormed, i+1, err)
		}
	}
	return nil
}

type srlVarSet map[string]bool

func (s srlVarSet) clone() srlVarSet {
	out := make(srlVarSet, len(s)+4)
	for k := range s {
		out[k] = true
	}
	return out
}

func checkRuleWellformed(rule SRLRule) error {
	bound, err := checkSequence(rule.Body, srlVarSet{})
	if err != nil {
		return err
	}
	for _, t := range rule.Head {
		for _, term := range []SRLTerm{t.Subject, t.Predicate, t.Object} {
			for _, v := range srlTermVars(term) {
				if !bound[v] {
					return fmt.Errorf("head variable ?%s is not bound by the rule body", v)
				}
			}
		}
	}
	return nil
}

// checkSequence checks that elems is a well-formed sequence given the initial
// variables v0 and returns vars(S, v0).
func checkSequence(elems []SRLBodyElement, v0 srlVarSet) (srlVarSet, error) {
	bound := v0.clone()
	for _, elem := range elems {
		switch elem.Kind {
		case SRLBodyTriple:
			for _, term := range []SRLTerm{elem.Triple.Subject, elem.Triple.Predicate, elem.Triple.Object} {
				for _, v := range srlTermVars(term) {
					bound[v] = true
				}
			}
		case SRLBodyFilter:
			for _, v := range exprVars(elem.FilterExpr) {
				if !bound[v] {
					return nil, fmt.Errorf("FILTER uses ?%s, which is not bound before it", v)
				}
			}
		case SRLBodyBind:
			for _, v := range exprVars(elem.BindExpr) {
				if !bound[v] {
					return nil, fmt.Errorf("SET ?%s uses ?%s, which is not bound before it", elem.BindVar, v)
				}
			}
			if bound[elem.BindVar] {
				return nil, fmt.Errorf("SET assigns ?%s, which is already in use", elem.BindVar)
			}
			bound[elem.BindVar] = true
		case SRLBodyNot:
			if _, err := checkSequence(elem.NotBody, bound); err != nil {
				return nil, fmt.Errorf("in NOT: %w", err)
			}
		case SRLBodyGroup:
			if err := checkGroup(elem, bound); err != nil {
				return nil, err
			}
			for _, k := range elem.GroupKeys {
				bound[k] = true
			}
			for _, a := range elem.GroupAggs {
				bound[a.Var] = true
			}
		}
	}
	return bound, nil
}

func checkGroup(g SRLBodyElement, vin srlVarSet) error {
	gk := srlVarSet{}
	for _, k := range g.GroupKeys {
		if vin[k] {
			gk[k] = true
		}
	}
	inner, err := checkSequence(g.GroupBody, gk)
	if err != nil {
		return fmt.Errorf("in GROUP: %w", err)
	}
	for _, k := range g.GroupKeys {
		if !inner[k] {
			return fmt.Errorf("GROUP BY ?%s: the group pattern does not bind it", k)
		}
	}
	seen := map[string]bool{}
	for _, a := range g.GroupAggs {
		if inner[a.Var] || vin[a.Var] {
			return fmt.Errorf("aggregation variable ?%s is already in use", a.Var)
		}
		if seen[a.Var] {
			return fmt.Errorf("aggregation variable ?%s is assigned twice", a.Var)
		}
		seen[a.Var] = true
		if a.Arg != "*" && !inner[a.Arg] {
			return fmt.Errorf("%s(?%s): the group pattern does not bind ?%s", a.Func, a.Arg, a.Arg)
		}
	}
	return nil
}

func srlTermVars(t SRLTerm) []string {
	switch t.Kind {
	case SRLTermVariable:
		return []string{t.Value}
	case SRLTermTripleTerm:
		var vars []string
		for _, c := range []*SRLTerm{t.TTSubject, t.TTPredicate, t.TTObject} {
			if c != nil {
				vars = append(vars, srlTermVars(*c)...)
			}
		}
		return vars
	}
	return nil
}
