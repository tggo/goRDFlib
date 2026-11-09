# Where Oxigraph and rdflib disagree

`disagreements.json` holds the generated queries on which the two reference
engines give different answers (or one fails). They are not in the corpus,
because neither reference can be the judge. This is the analysis of all 209
against the specifications (2026-11-09, pyoxigraph 0.5.13, rdflib 7.6.0). It
compares goRDFlib's own answers with both references.

| Our answer equals | Count | Verdict |
|---|---|---|
| Oxigraph | 58 | Oxigraph is right. rdflib breaks group scoping (`{ BIND }` sees the outer row; a subquery's unprojected `?x` joins the outer one, §18.2.1). It drops the solution of `{ }` and of `VALUES (?x ?y) { (UNDEF UNDEF) }`. It treats `_:b ex:p _:b` as having no match. |
| rdflib | 3 | rdflib is right. Oxigraph canonicalises numeric literals when loading, so `"01"^^xsd:integer` and `1` become one term and `str()` sees `"1"` for both. RDF 1.1 Concepts §3.3: literals are equal only when lexical form, datatype and language are equal. |
| both (only the encoding differs) | 56 | Not a disagreement about the answer. |
| neither, `duplicates` / `repeat-path` | 42 | We are right, for the same §3.3 reason. `ex:d1 ex:v` has six distinct literals; Oxigraph merges two of them, rdflib one. So `?x ex:v ?v` gives 8 / 6 / 7 rows, and `?x ex:p* ?x` (one zero-length row per graph node) gives 57 / 55 / 56. |
| neither, `decorated` | 46 | **Our bug.** These are malformed triples such as `?x ex:q ?x ?z ex:p ?x`, which both references reject and we accepted. This is the class of W3C sparql10 syn-bad-02/03. |
| neither, corpus | 4 | Allowed by the spec. In decimal division, Oxigraph truncates to 18 fractional digits, rdflib keeps 28, and we round at 18. XPath F&O `op:numeric-divide` leaves xsd:decimal precision implementation-defined. |

Rerun the classification after regenerating, with a throwaway test that
evaluates each query and compares it to both references using
`canonRow`/`rowJSON` from `sparql/differential_test.go`.
