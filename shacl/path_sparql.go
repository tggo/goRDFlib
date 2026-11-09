package shacl

import "strings"

// sparqlSyntax returns the property path in SPARQL surface syntax, every
// compound sub-path parenthesised, for substituting $PATH into the SPARQL
// query of a constraint on a property shape (SHACL-SPARQL 1.2, "Validation
// with SPARQL-based Constraints"). It returns "" for a path it cannot write.
func (p *PropertyPath) sparqlSyntax() string {
	if p == nil {
		return ""
	}
	sub := func(q *PropertyPath) string {
		if q == nil {
			return ""
		}
		s := q.sparqlSyntax()
		if s == "" || q.Kind == PathPredicate {
			return s
		}
		return "(" + s + ")"
	}
	join := func(sep string) string {
		parts := make([]string, len(p.Elements))
		for i, e := range p.Elements {
			if parts[i] = sub(e); parts[i] == "" {
				return ""
			}
		}
		return strings.Join(parts, sep)
	}
	switch p.Kind {
	case PathPredicate:
		if p.Pred.IsIRI() {
			return termToSPARQL(p.Pred)
		}
	case PathInverse:
		if s := sub(p.Sub); s != "" {
			return "^" + s
		}
	case PathSequence:
		return join("/")
	case PathAlternative:
		return join("|")
	case PathZeroOrMore:
		if s := sub(p.Sub); s != "" {
			return s + "*"
		}
	case PathOneOrMore:
		if s := sub(p.Sub); s != "" {
			return s + "+"
		}
	case PathZeroOrOne:
		if s := sub(p.Sub); s != "" {
			return s + "?"
		}
	}
	return ""
}
