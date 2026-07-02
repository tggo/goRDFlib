package shacl

import (
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// wellKnownPrefixes are automatically available in SHACL SPARQL queries.
var wellKnownPrefixes = []struct{ prefix, ns string }{
	{"rdf", RDF},
	{"rdfs", RDFS},
	{"xsd", XSD},
	{"owl", OWL},
	{"sh", SH},
}

// addWellKnownPrefixes prepends missing well-known PREFIX declarations to a query.
func addWellKnownPrefixes(query string) string {
	upper := strings.ToUpper(query)
	var sb strings.Builder
	for _, wk := range wellKnownPrefixes {
		// Check if PREFIX <name>: is already declared (case-insensitive)
		needle := "PREFIX " + strings.ToUpper(wk.prefix) + ":"
		if !strings.Contains(upper, needle) {
			// Only add if the prefix is actually used in the query
			if strings.Contains(query, wk.prefix+":") {
				sb.WriteString("PREFIX ")
				sb.WriteString(wk.prefix)
				sb.WriteString(": <")
				sb.WriteString(wk.ns)
				sb.WriteString(">\n")
			}
		}
	}
	if sb.Len() == 0 {
		return query
	}
	sb.WriteString(query)
	return sb.String()
}

// executeSPARQL runs a SPARQL SELECT query against the underlying graph.Graph,
// returning result bindings converted to shacl Terms.
func executeSPARQL(g *Graph, query string, initBindings map[string]term.Term, namedGraphs map[string]*graph.Graph) ([]map[string]Term, error) {
	query = addWellKnownPrefixes(query)
	query = fixSPARQLSyntax(query)
	pq, err := sparql.Parse(query)
	if err != nil {
		return nil, err
	}
	if namedGraphs != nil {
		pq.NamedGraphs = namedGraphs
	}
	result, err := sparql.EvalQuery(g.g, pq, initBindings)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]Term, 0, len(result.Bindings))
	for _, row := range result.Bindings {
		converted := make(map[string]Term, len(row))
		for k, v := range row {
			converted[k] = fromRDFLib(v)
		}
		rows = append(rows, converted)
	}
	return rows, nil
}

// fixSPARQLSyntax applies minor syntax fixes to SPARQL queries from SHACL constraints.
// Some SHACL test queries use syntax (like trailing . after BIND) that our parser
// doesn't accept but is technically valid per the SPARQL grammar.
func fixSPARQLSyntax(query string) string {
	// Remove trailing . after ) before } or another non-triple pattern
	// Pattern: ")" whitespace "." whitespace "}"
	var sb strings.Builder
	sb.Grow(len(query))
	for i := 0; i < len(query); i++ {
		if query[i] == '.' {
			// Check if this dot follows a ) (with optional whitespace) and precedes } (with optional whitespace)
			// Look back for )
			j := i - 1
			for j >= 0 && (query[j] == ' ' || query[j] == '\t' || query[j] == '\n' || query[j] == '\r') {
				j--
			}
			if j >= 0 && query[j] == ')' {
				// Look forward for }
				k := i + 1
				for k < len(query) && (query[k] == ' ' || query[k] == '\t' || query[k] == '\n' || query[k] == '\r') {
					k++
				}
				if k < len(query) && query[k] == '}' {
					continue // skip this dot
				}
			}
		}
		sb.WriteByte(query[i])
	}
	return sb.String()
}

// executeSPARQLAsk runs a SPARQL ASK query against the underlying graph.Graph.
func executeSPARQLAsk(g *Graph, query string, initBindings map[string]term.Term, namedGraphs map[string]*graph.Graph) (bool, error) {
	query = addWellKnownPrefixes(query)
	query = fixSPARQLSyntax(query)
	pq, err := sparql.Parse(query)
	if err != nil {
		return false, err
	}
	if namedGraphs != nil {
		pq.NamedGraphs = namedGraphs
	}
	result, err := sparql.EvalQuery(g.g, pq, initBindings)
	if err != nil {
		return false, err
	}
	return result.AskResult, nil
}

// resolvePrefixes walks sh:prefixes → sh:declare → sh:prefix/sh:namespace
// to build a SPARQL PREFIX preamble string.
func resolvePrefixes(g *Graph, prefixesNode Term) string {
	var sb strings.Builder
	seen := make(map[string]bool)
	collectPrefixes(g, prefixesNode, &sb, seen, make(map[string]bool))
	return sb.String()
}

func collectPrefixes(g *Graph, node Term, sb *strings.Builder, seen, visited map[string]bool) {
	key := node.TermKey()
	if visited[key] {
		return
	}
	visited[key] = true

	declarePred := IRI(SH + "declare")
	prefixPred := IRI(SH + "prefix")
	namespacePred := IRI(SH + "namespace")

	for _, decl := range g.Objects(node, declarePred) {
		prefixes := g.Objects(decl, prefixPred)
		namespaces := g.Objects(decl, namespacePred)
		if len(prefixes) > 0 && len(namespaces) > 0 {
			prefix := prefixes[0].Value()
			ns := namespaces[0].Value()
			if !seen[prefix] {
				seen[prefix] = true
				sb.WriteString("PREFIX ")
				sb.WriteString(prefix)
				sb.WriteString(": <")
				sb.WriteString(ns)
				sb.WriteString(">\n")
			}
		}
	}

	// Follow owl:imports
	owlImports := IRI("http://www.w3.org/2002/07/owl#imports")
	for _, imp := range g.Objects(node, owlImports) {
		collectPrefixes(g, imp, sb, seen, visited)
	}
}

// normalizeDollarVars replaces $varName with ?varName in a SPARQL query,
// since $ and ? are interchangeable in SPARQL but our parser only supports ?.
func normalizeDollarVars(query string) string {
	var sb strings.Builder
	sb.Grow(len(query))
	inString := false
	var stringDelim byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		// Track string literals to avoid replacing $ inside strings
		if !inString && (c == '"' || c == '\'') {
			// Check for triple-quoted strings
			if i+2 < len(query) && query[i+1] == c && query[i+2] == c {
				inString = true
				stringDelim = c
				sb.WriteByte(c)
				sb.WriteByte(c)
				sb.WriteByte(c)
				i += 2
				continue
			}
			inString = true
			stringDelim = c
			sb.WriteByte(c)
			continue
		}
		if inString {
			if c == stringDelim {
				// Check for end of triple-quoted string
				if i+2 < len(query) && query[i+1] == stringDelim && query[i+2] == stringDelim {
					inString = false
					sb.WriteByte(c)
					sb.WriteByte(c)
					sb.WriteByte(c)
					i += 2
					continue
				}
				// Single quote end
				if stringDelim != query[i] || (i > 0 && query[i-1] == '\\') {
					sb.WriteByte(c)
					continue
				}
				inString = false
			}
			sb.WriteByte(c)
			continue
		}
		if c == '$' && i+1 < len(query) && isVarStartChar(query[i+1]) {
			sb.WriteByte('?')
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func isVarStartChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isVarChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// preBindTerm registers a pre-bound SHACL variable, choosing the correct
// mechanism for its term kind. IRI and literal values go into textual
// bindings (spliced into the query text). Blank-node values go into initial
// bindings (real solution pre-binding) because a blank node label in SPARQL
// query text is a fresh, query-scoped node rather than a reference to a
// specific data-graph node (SPARQL 1.1 §4.1.4); textual substitution of a
// blank node therefore cannot scope a triple pattern to that focus node.
func preBindTerm(textual map[string]string, initial map[string]term.Term, name string, t Term) {
	if t.Kind() == TermBlankNode {
		initial[name] = toTerm(t)
		return
	}
	textual[name] = termToSPARQL(t)
}

// termToSPARQL converts a shacl Term to a SPARQL term string suitable for
// textual substitution into a query (e.g. <http://example.org/> or "hello"@en).
func termToSPARQL(t Term) string {
	switch t.Kind() {
	case TermIRI:
		return "<" + t.Value() + ">"
	case TermLiteral:
		escaped := strings.ReplaceAll(t.Value(), `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		s := `"` + escaped + `"`
		if t.Language() != "" {
			s += "@" + t.Language()
		} else if t.Datatype() != "" && t.Datatype() != XSD+"string" {
			s += "^^<" + t.Datatype() + ">"
		}
		return s
	case TermBlankNode:
		return "_:" + t.Value()
	}
	return `""`
}

// preBindQuery substitutes ?varName occurrences in a SPARQL query with the
// given SPARQL term representation. In the SELECT clause, bound variables are
// simply removed (since their values are known). In the rest of the query,
// they are replaced with the actual term value.
func preBindQuery(query string, bindings map[string]string) string {
	// Replace bound(?var) with true for all pre-bound variables, since they
	// are always bound. After textual substitution, bound(<iri>) would
	// incorrectly evaluate to false.
	for varName := range bindings {
		query = replaceBoundCheck(query, "?"+varName)
		query = replaceBoundCheck(query, "$"+varName)
	}

	// SHACL uses $var syntax as well as ?var; normalize $var to ?var first
	for varName := range bindings {
		query = replaceVar(query, "$"+varName, "?"+varName)
	}

	// Find the WHERE keyword to split SELECT clause from body
	upper := strings.ToUpper(query)
	whereIdx := -1
	for i := 0; i < len(upper)-5; i++ {
		if upper[i:i+5] == "WHERE" && (i == 0 || !isVarChar(upper[i-1])) && (i+5 >= len(upper) || !isVarChar(upper[i+5])) {
			whereIdx = i
			break
		}
	}

	if whereIdx == -1 {
		// No WHERE found, substitute everywhere
		for varName, value := range bindings {
			query = replaceVar(query, "?"+varName, value)
		}
		return query
	}

	selectPart := query[:whereIdx]
	bodyPart := query[whereIdx:]

	// In SELECT clause: remove bound variables (they'd become IRIs which break parsing)
	for varName := range bindings {
		selectPart = removeVarFromSelect(selectPart, "?"+varName)
	}

	// If SELECT clause has no remaining variables, use SELECT *
	selectUpper := strings.ToUpper(strings.TrimSpace(selectPart))
	if strings.HasSuffix(selectUpper, "SELECT") || strings.HasSuffix(selectUpper, "SELECT DISTINCT") {
		if strings.HasSuffix(selectUpper, "DISTINCT") {
			selectPart = strings.TrimSpace(selectPart) + " * "
		} else {
			selectPart = strings.TrimSpace(selectPart) + " * "
		}
	}

	// In body: substitute with actual values
	for varName, value := range bindings {
		bodyPart = replaceVar(bodyPart, "?"+varName, value)
	}

	return selectPart + bodyPart
}

// removeVarFromSelect removes a variable token from a SELECT clause.
func removeVarFromSelect(selectClause, varToken string) string {
	var sb strings.Builder
	sb.Grow(len(selectClause))
	for i := 0; i < len(selectClause); {
		if i+len(varToken) <= len(selectClause) && selectClause[i:i+len(varToken)] == varToken {
			end := i + len(varToken)
			if end >= len(selectClause) || !isVarChar(selectClause[end]) {
				// Skip the variable and any trailing whitespace
				i = end
				for i < len(selectClause) && (selectClause[i] == ' ' || selectClause[i] == '\t') {
					i++
				}
				continue
			}
		}
		sb.WriteByte(selectClause[i])
		i++
	}
	return sb.String()
}

// replaceVar replaces occurrences of a variable (like ?this) in a SPARQL query,
// being careful to only replace when followed by a non-alphanumeric character.
func replaceVar(query, varToken, replacement string) string {
	var sb strings.Builder
	sb.Grow(len(query))
	for i := 0; i < len(query); {
		if i+len(varToken) <= len(query) && query[i:i+len(varToken)] == varToken {
			end := i + len(varToken)
			if end >= len(query) || !isVarChar(query[end]) {
				sb.WriteString(replacement)
				i = end
				continue
			}
		}
		sb.WriteByte(query[i])
		i++
	}
	return sb.String()
}

// replaceBoundCheck replaces "bound(?var)" with "true" in a SPARQL query,
// handling optional whitespace inside the parentheses. This is needed because
// after textual substitution of pre-bound variables, bound(<iri>) would
// incorrectly evaluate to false.
func replaceBoundCheck(query, varToken string) string {
	var sb strings.Builder
	sb.Grow(len(query))
	lower := strings.ToLower(query)
	for i := 0; i < len(query); {
		// Look for "bound" keyword (case-insensitive)
		if i+5 <= len(query) && lower[i:i+5] == "bound" {
			// Check it's not part of a longer identifier
			if i > 0 && isVarChar(query[i-1]) {
				sb.WriteByte(query[i])
				i++
				continue
			}
			// Skip whitespace after "bound"
			j := i + 5
			for j < len(query) && (query[j] == ' ' || query[j] == '\t') {
				j++
			}
			if j < len(query) && query[j] == '(' {
				// Skip whitespace after '('
				k := j + 1
				for k < len(query) && (query[k] == ' ' || query[k] == '\t') {
					k++
				}
				// Check if varToken follows
				if k+len(varToken) <= len(query) && query[k:k+len(varToken)] == varToken {
					end := k + len(varToken)
					// Must not be followed by a var char (to avoid partial match)
					if end >= len(query) || !isVarChar(query[end]) {
						// Skip whitespace before ')'
						for end < len(query) && (query[end] == ' ' || query[end] == '\t') {
							end++
						}
						if end < len(query) && query[end] == ')' {
							// Replace entire bound(?var) with true
							sb.WriteString("true")
							i = end + 1
							continue
						}
					}
				}
			}
		}
		sb.WriteByte(query[i])
		i++
	}
	return sb.String()
}

// localName extracts the local name from an IRI (after last # or /).
func localName(iri string) string {
	for i := len(iri) - 1; i >= 0; i-- {
		if iri[i] == '#' || iri[i] == '/' {
			return iri[i+1:]
		}
	}
	return iri
}
