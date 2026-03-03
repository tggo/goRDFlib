package sparql

import (
	"fmt"
	"strings"
)

func (p *sparqlParser) validate(q *ParsedQuery) error {
	// Validate nested aggregates in project expressions
	for _, pe := range q.ProjectExprs {
		if err := validateNoNestedAggregates(pe.Expr); err != nil {
			return err
		}
	}



	if q.Type == "SELECT" {
		// SELECT * with GROUP BY is invalid
		if q.Variables == nil && len(q.GroupBy) > 0 {
			return fmt.Errorf("sparql parse error: SELECT * not allowed with GROUP BY")
		}

		// With GROUP BY, all selected variables must be grouped or aggregated
		if len(q.GroupBy) > 0 && q.Variables != nil {
			grouped := make(map[string]bool)
			for _, g := range q.GroupBy {
				if ve, ok := g.(*VarExpr); ok {
					grouped[ve.Name] = true
				}
			}
			for _, alias := range q.GroupByAliases {
				if alias != "" {
					grouped[alias] = true
				}
			}
			// Variables from project expressions (aggregates) are ok
			for _, pe := range q.ProjectExprs {
				grouped[pe.Var] = true
			}
			for _, v := range q.Variables {
				if !grouped[v] {
					return fmt.Errorf("sparql parse error: variable ?%s not in GROUP BY or aggregate", v)
				}
			}
		}

		// Without GROUP BY but with aggregates, plain variables are invalid
		if len(q.GroupBy) == 0 && len(q.ProjectExprs) > 0 {
			hasAgg := false
			for _, pe := range q.ProjectExprs {
				if containsAggregate(pe.Expr) {
					hasAgg = true
					break
				}
			}
			if hasAgg {
				aggVars := make(map[string]bool)
				for _, pe := range q.ProjectExprs {
					aggVars[pe.Var] = true
				}
				for _, v := range q.Variables {
					if !aggVars[v] {
						return fmt.Errorf("sparql parse error: variable ?%s must be aggregated (no GROUP BY)", v)
					}
				}
			}
		}

		// Duplicate AS variables
		seen := make(map[string]bool)
		for _, pe := range q.ProjectExprs {
			if seen[pe.Var] {
				return fmt.Errorf("sparql parse error: duplicate variable ?%s in SELECT", pe.Var)
			}
			seen[pe.Var] = true
		}

		// Validate that project expressions only reference grouped variables or aggregates
		if len(q.GroupBy) > 0 {
			grouped := make(map[string]bool)
			for _, g := range q.GroupBy {
				if ve, ok := g.(*VarExpr); ok {
					grouped[ve.Name] = true
				}
			}
			for _, alias := range q.GroupByAliases {
				if alias != "" {
					grouped[alias] = true
				}
			}
			for _, pe := range q.ProjectExprs {
				if !containsAggregate(pe.Expr) {
					// Check that all variable references are grouped
					refs := collectExprVars(pe.Expr)
					for v := range refs {
						if !grouped[v] {
							return fmt.Errorf("sparql parse error: variable ?%s in expression not in GROUP BY", v)
						}
					}
				}
			}
		}

		// Check for scope conflict: project expression variable from inner subquery
		if q.Where != nil {
			innerVars := collectPatternVars(q.Where)
			for _, pe := range q.ProjectExprs {
				if innerVars[pe.Var] {
					return fmt.Errorf("sparql parse error: variable ?%s already defined in inner scope", pe.Var)
				}
			}
		}
	}
	return nil
}

// validateBindScope checks that BIND variables don't conflict with variables already in scope.
func validateBindScope(p Pattern) error {
	if p == nil {
		return nil
	}
	switch pat := p.(type) {
	case *BindPattern:
		// Check if BIND variable is already used in the inner pattern
		vars := collectPatternVars(pat.Pattern)
		if vars[pat.Var] {
			return fmt.Errorf("sparql parse error: BIND variable ?%s already in scope", pat.Var)
		}
		return validateBindScope(pat.Pattern)
	case *JoinPattern:
		if err := validateBindScope(pat.Left); err != nil {
			return err
		}
		return validateBindScope(pat.Right)
	case *OptionalPattern:
		if err := validateBindScope(pat.Main); err != nil {
			return err
		}
		return validateBindScope(pat.Optional)
	case *UnionPattern:
		if err := validateBindScope(pat.Left); err != nil {
			return err
		}
		return validateBindScope(pat.Right)
	case *FilterPattern:
		return validateBindScope(pat.Pattern)
	case *MinusPattern:
		if err := validateBindScope(pat.Left); err != nil {
			return err
		}
		return validateBindScope(pat.Right)
	case *SubqueryPattern:
		return validateBindScope(pat.Query.Where)
	}
	return nil
}

func collectPatternVars(p Pattern) map[string]bool {
	vars := make(map[string]bool)
	collectVarsInto(p, vars)
	return vars
}

func collectVarsInto(p Pattern, vars map[string]bool) {
	if p == nil {
		return
	}
	switch pat := p.(type) {
	case *BGP:
		for _, t := range pat.Triples {
			if strings.HasPrefix(t.Subject, "?") {
				vars[t.Subject[1:]] = true
			}
			if strings.HasPrefix(t.Predicate, "?") {
				vars[t.Predicate[1:]] = true
			}
			if strings.HasPrefix(t.Object, "?") {
				vars[t.Object[1:]] = true
			}
		}
	case *JoinPattern:
		collectVarsInto(pat.Left, vars)
		collectVarsInto(pat.Right, vars)
	case *OptionalPattern:
		collectVarsInto(pat.Main, vars)
		collectVarsInto(pat.Optional, vars)
	case *UnionPattern:
		collectVarsInto(pat.Left, vars)
		collectVarsInto(pat.Right, vars)
	case *FilterPattern:
		collectVarsInto(pat.Pattern, vars)
	case *BindPattern:
		collectVarsInto(pat.Pattern, vars)
		vars[pat.Var] = true
	case *SubqueryPattern:
		for _, v := range pat.Query.Variables {
			vars[v] = true
		}
	}
}

func collectExprVars(expr Expr) map[string]bool {
	vars := make(map[string]bool)
	collectExprVarsInto(expr, vars)
	return vars
}

func collectExprVarsInto(expr Expr, vars map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *VarExpr:
		vars[e.Name] = true
	case *BinaryExpr:
		collectExprVarsInto(e.Left, vars)
		collectExprVarsInto(e.Right, vars)
	case *UnaryExpr:
		collectExprVarsInto(e.Arg, vars)
	case *FuncExpr:
		if !isAggregateFuncName(e.Name) {
			for _, a := range e.Args {
				collectExprVarsInto(a, vars)
			}
		}
		// Don't descend into aggregate function args
	}
}

func validateConstructWhere(p Pattern) error {
	if p == nil {
		return nil
	}
	switch pat := p.(type) {
	case *BGP:
		return nil // simple BGP is fine
	case *JoinPattern:
		if err := validateConstructWhere(pat.Left); err != nil {
			return err
		}
		return validateConstructWhere(pat.Right)
	case *FilterPattern:
		return fmt.Errorf("sparql parse error: FILTER not allowed in CONSTRUCT WHERE")
	default:
		return fmt.Errorf("sparql parse error: complex pattern not allowed in CONSTRUCT WHERE")
	}
}

// validateNoNestedAggregates checks that aggregate expressions don't contain nested aggregates.
func validateNoNestedAggregates(expr Expr) error {
	switch e := expr.(type) {
	case *FuncExpr:
		if isAggregateFuncName(e.Name) {
			// Check that arguments don't contain aggregates
			for _, a := range e.Args {
				if containsAggregate(a) {
					return fmt.Errorf("sparql parse error: nested aggregate functions not allowed")
				}
			}
		}
		for _, a := range e.Args {
			if err := validateNoNestedAggregates(a); err != nil {
				return err
			}
		}
	case *BinaryExpr:
		if err := validateNoNestedAggregates(e.Left); err != nil {
			return err
		}
		return validateNoNestedAggregates(e.Right)
	case *UnaryExpr:
		return validateNoNestedAggregates(e.Arg)
	}
	return nil
}

// validateTripleTerms validates triple term constraints in patterns.
func validateTripleTerms(p Pattern) error {
	if p == nil {
		return nil
	}
	switch pat := p.(type) {
	case *BGP:
		for _, t := range pat.Triples {
			if err := validateTripleTermString(t.Subject, "subject"); err != nil {
				return err
			}
			if err := validateTripleTermString(t.Object, "object"); err != nil {
				return err
			}
		}
	case *JoinPattern:
		if err := validateTripleTerms(pat.Left); err != nil {
			return err
		}
		return validateTripleTerms(pat.Right)
	case *OptionalPattern:
		if err := validateTripleTerms(pat.Main); err != nil {
			return err
		}
		return validateTripleTerms(pat.Optional)
	case *UnionPattern:
		if err := validateTripleTerms(pat.Left); err != nil {
			return err
		}
		return validateTripleTerms(pat.Right)
	case *FilterPattern:
		return validateTripleTerms(pat.Pattern)
	case *BindPattern:
		return validateTripleTerms(pat.Pattern)
	case *ValuesPattern:
		return nil
	case *SubqueryPattern:
		return validateTripleTerms(pat.Query.Where)
	}
	return nil
}

// validateTripleTermString checks that a triple term string is valid:
// - No collection syntax inside triple terms
// - Subject of triple term must not be a literal or another triple term
func validateTripleTermString(s, position string) error {
	if !strings.HasPrefix(s, "<<( ") {
		return nil
	}
	inner := s[4 : len(s)-4]
	parts := splitTripleTermPartsParser(inner)
	if len(parts) != 3 {
		return nil
	}
	// Check subject: must not be a literal
	subj := parts[0]
	if strings.HasPrefix(subj, "\"") || strings.HasPrefix(subj, "'") {
		return fmt.Errorf("sparql parse error: literal in subject position of triple term")
	}
	if strings.HasPrefix(subj, "<<( ") {
		return fmt.Errorf("sparql parse error: triple term in subject position of triple term")
	}
	// Check for collection syntax inside
	for _, p := range parts {
		if strings.HasPrefix(p, "?_coll") {
			return fmt.Errorf("sparql parse error: collection syntax not allowed inside triple term")
		}
	}
	// Recursively validate nested triple terms
	for _, p := range parts {
		if strings.HasPrefix(p, "<<( ") {
			if err := validateTripleTermString(p, "nested"); err != nil {
				return err
			}
		}
	}
	return nil
}

// unescapePNLocal removes backslash escapes from PN_LOCAL_ESC sequences.
func unescapePNLocal(s string) string {
	if !strings.ContainsAny(s, `\%`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
		} else if s[i] == '%' && i+2 < len(s) {
			// Keep percent encoding as-is in the URI
			b.WriteString(s[i : i+3])
			i += 2
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// isPNLocalEscChar returns true if the char can be escaped with \ in PN_LOCAL.
func isPNLocalEscChar(ch byte) bool {
	return strings.ContainsRune(`_~.-!$&'()*+,;=/?#@%`, rune(ch))
}

// collectDataBnodeLabels extracts bnode labels only from DATA operations (INSERT DATA/DELETE DATA).
// Bnode reuse across DATA operations is a syntax error per spec.
func collectDataBnodeLabels(op UpdateOperation) map[string]bool {
	labels := make(map[string]bool)
	var collectFromTriples func(triples []Triple)
	collectFromTriples = func(triples []Triple) {
		for _, t := range triples {
			if strings.HasPrefix(t.Subject, "_:") {
				labels[t.Subject[2:]] = true
			}
			if strings.HasPrefix(t.Object, "_:") {
				labels[t.Object[2:]] = true
			}
		}
	}
	var collectFromQuads func(quads []QuadPattern)
	collectFromQuads = func(quads []QuadPattern) {
		for _, qp := range quads {
			collectFromTriples(qp.Triples)
		}
	}
	switch o := op.(type) {
	case *InsertDataOp:
		collectFromQuads(o.Quads)
	case *DeleteDataOp:
		collectFromQuads(o.Quads)
	}
	return labels
}
