"""Generate the SPARQL differential test corpus.

Each query is built from real SPARQL constructs (OPTIONAL with a filter on
the left side, EXISTS/NOT EXISTS with an inner FILTER, MINUS, UNION, BIND,
VALUES, subqueries, aggregates, property paths, built-in functions) over
data.ttl. Every candidate is run in two independent engines, Oxigraph and
rdflib; a query enters queries.json with its expected result only when the
two agree. Candidates they disagree on are written to disagreements.json for
inspection and are not part of the test.

Regenerate (needs pyoxigraph and rdflib):

    uv venv /tmp/v && VIRTUAL_ENV=/tmp/v uv pip install pyoxigraph==0.5.13 rdflib==7.6.0
    /tmp/v/bin/python -I testdata/sparql-diff/generate.py testdata/sparql-diff

The seed is fixed, so the corpus is reproducible. Versions used for the
committed corpus: pyoxigraph 0.5.13, rdflib 7.6.0.

Besides the random candidates there is a fixed set of "tricky shape"
families (SHAPES below): a variable repeated within one pattern or path,
blank-node labels shared across patterns, variables bound on one side only,
empty groups, duplicates, BIND shadowing, VALUES with UNDEF. Each template
runs under several result forms, so duplicates and cardinality are checked
too. Triple terms / SPARQL 1.2 are not generated: rdflib cannot evaluate them.
"""

import json
import random
import sys
from pathlib import Path

import pyoxigraph as ox
import rdflib

SEED = 20261103
TARGET = 400  # candidates; the agreed subset is the corpus

PREFIX = "PREFIX ex: <http://example.org/>\nPREFIX xsd: <http://www.w3.org/2001/XMLSchema#>\n"

# A clause: SPARQL text, the variables it needs bound before it (for
# filters), and the variables it binds.
BASE = [
    ("?s a ex:Person .", [], ["s"]),
    ("?s ex:name ?n .", [], ["s", "n"]),
    ("?s ex:age ?a .", [], ["s", "a"]),
    ("?s ex:knows ?o .", [], ["s", "o"]),
    ("?s ex:score ?sc .", [], ["s", "sc"]),
]

EXTRA = [
    ("OPTIONAL { ?s ex:email ?e }", ["s"], ["e"]),
    ("OPTIONAL { ?s ex:age ?a2 FILTER (?a2 > 20) }", ["s"], ["a2"]),
    ("OPTIONAL { ?s ex:knows ?k FILTER (?k != ?s) }", ["s"], ["k"]),
    ("OPTIONAL { ?s ex:knows ?k2 . ?k2 ex:age ?ka FILTER (?ka > ?a) }", ["s", "a"], ["k2", "ka"]),
    ("OPTIONAL { ?s ex:name ?ln FILTER (lang(?ln) = \"uk\") }", ["s"], ["ln"]),
    ("OPTIONAL { { ?s ex:score ?os FILTER (?os > 5) } }", ["s"], ["os"]),
    ("FILTER EXISTS { ?s ex:knows ?x }", ["s"], []),
    ("FILTER NOT EXISTS { ?s ex:email ?x }", ["s"], []),
    ("FILTER NOT EXISTS { ?s ex:knows ?x . ?x ex:age ?xa FILTER (?xa < ?a) }", ["s", "a"], []),
    ("FILTER EXISTS { ?o ex:knows ?s }", ["s", "o"], []),
    ("MINUS { ?s a ex:Admin }", ["s"], []),
    ("MINUS { ?s ex:age ?m FILTER (?m < 20) }", ["s"], []),
    ("BIND (str(?n) AS ?ns)", ["n"], ["ns"]),
    ("BIND (?a * 2 AS ?a2x)", ["a"], ["a2x"]),
    ("BIND (IF(?a >= 30, \"old\", \"young\") AS ?grp)", ["a"], ["grp"]),
    ("BIND (?a / 3 AS ?third)", ["a"], ["third"]),
    ("BIND (?sc + 0.25 AS ?scp)", ["sc"], ["scp"]),
    ("BIND (?a - ?sc AS ?diff)", ["a", "sc"], ["diff"]),
    ("BIND (ROUND(?sc / 2) AS ?half)", ["sc"], ["half"]),
    ("BIND (ABS(?a - 40) AS ?dist)", ["a"], ["dist"]),
    ("FILTER (?a / 2 > 15)", ["a"], []),
    ("BIND (COALESCE(?e, \"none\") AS ?ce)", ["s"], ["ce"]),
    ("VALUES ?s { ex:alice ex:bob ex:frank }", [], ["s"]),
    ("FILTER (?a > 20)", ["a"], []),
    ("FILTER (?a = 30 || ?a = 41)", ["a"], []),
    ("FILTER (isLiteral(?n) && lang(?n) = \"\")", ["n"], []),
    ("FILTER (regex(str(?n), \"^[A-C]\"))", ["n"], []),
    ("FILTER (langMatches(lang(?n), \"en\"))", ["n"], []),
    ("FILTER (!bound(?e))", ["s"], []),
    ("FILTER (?sc >= 7)", ["sc"], []),
    ("FILTER (datatype(?a) = xsd:integer)", ["a"], []),
    ("FILTER (?o != ex:carol)", ["o"], []),
    ("?s ex:knows+ ?r .", ["s"], ["r"]),
    ("?s ex:knows/ex:name ?fn .", ["s"], ["fn"]),
    ("?s ^ex:knows ?in .", ["s"], ["in"]),
    ("?s ex:address/ex:city ?city .", ["s"], ["city"]),
    ("{ ?s ex:email ?u } UNION { ?s ex:score ?u }", ["s"], ["u"]),
    ("{ SELECT ?s (COUNT(?f) AS ?nf) WHERE { ?s ex:knows ?f } GROUP BY ?s }", ["s"], ["nf"]),
    ("{ SELECT (MAX(?ma) AS ?oldest) WHERE { ?p ex:age ?ma } }", [], ["oldest"]),
]

AGG = [
    "SELECT (COUNT(*) AS ?c) WHERE {{ {body} }}",
    "SELECT (COUNT(DISTINCT ?s) AS ?c) WHERE {{ {body} }}",
    "SELECT ?s (COUNT(*) AS ?c) WHERE {{ {body} }} GROUP BY ?s",
    "SELECT ?s (SAMPLE(?s) AS ?x) WHERE {{ {body} }} GROUP BY ?s HAVING (COUNT(*) > 1)",
]
SUM_AGG = [
    "SELECT (SUM(?a) AS ?t) (MIN(?a) AS ?lo) (MAX(?a) AS ?hi) WHERE {{ {body} }}",
    "SELECT ?grp (COUNT(*) AS ?c) WHERE {{ {body} }} GROUP BY ?grp",
    "SELECT (AVG(?a) AS ?avg) (SUM(?a) / COUNT(?a) AS ?avg2) WHERE {{ {body} }}",
]


def candidate(rng):
    base = rng.choice(BASE)
    clauses = [base[0]]
    bound = set(base[2])
    if "s" in bound and rng.random() < 0.5:
        extra_base = rng.choice([b for b in BASE if b is not base])
        clauses.append(extra_base[0])
        bound |= set(extra_base[2])
    for _ in range(rng.randint(1, 3)):
        usable = [c for c in EXTRA if set(c[1]) <= bound and c[0] not in clauses]
        if not usable:
            break
        c = rng.choice(usable)
        clauses.append(c[0])
        bound |= set(c[2])
    body = " ".join(clauses)
    form = rng.random()
    if form < 0.15:
        return PREFIX + rng.choice(AGG).format(body=body)
    if form < 0.22 and "a" in bound:
        if "grp" in bound or "IF(" not in body:
            tmpl = rng.choice([SUM_AGG[0], SUM_AGG[2]]) if "grp" not in bound else rng.choice(SUM_AGG)
            return PREFIX + tmpl.format(body=body)
    if form < 0.30:
        return PREFIX + "ASK { " + body + " }"
    vars_ = sorted(bound)
    proj = " ".join("?" + v for v in vars_)
    q = PREFIX + "SELECT " + ("DISTINCT " if rng.random() < 0.2 else "") + proj + " WHERE { " + body + " }"
    return q


# Tricky-shape families: (family, group pattern). Run under SHAPE_FORMS.
SHAPES = [
    # a variable repeated within one pattern
    ("repeat-pattern", "?x ex:p ?x"),
    ("repeat-pattern", "?x ex:q ?x"),
    ("repeat-pattern", "?s ?s ?o"),
    ("repeat-pattern", "?s ?p ?s"),
    ("repeat-pattern", "?o ?o ?o"),
    ("repeat-pattern", "?x ?p ?x . ?x ex:p ?y"),
    ("repeat-pattern", "?a ex:p ?b . ?b ex:p ?b"),
    ("repeat-pattern", "?b ex:p ?b . ?a ex:p ?b"),
    ("repeat-pattern", "?x ex:knows ?x"),
    ("repeat-pattern", "?x ex:t ?x . ?x ex:u ?x"),
    ("repeat-pattern", "?x ex:p ?x . ?x ex:q ?x"),
    ("repeat-pattern", "?x ex:v ?x"),
    # repeated across a path
    ("repeat-path", "?x ex:p+ ?x"),
    ("repeat-path", "?x ex:p* ?x"),
    ("repeat-path", "?x ex:p? ?x"),
    ("repeat-path", "?x ex:p/ex:q ?x"),
    ("repeat-path", "?x ^ex:p ?x"),
    ("repeat-path", "?x (ex:p|ex:q) ?x"),
    ("repeat-path", "?x (ex:p|ex:q)+ ?x"),
    ("repeat-path", "?x (ex:p/ex:q)+ ?x"),
    ("repeat-path", "?x ex:p/ex:p ?x"),
    ("repeat-path", "?x (^ex:p)+ ?x"),
    ("repeat-path", "?x ^ex:p+ ?x"),
    ("repeat-path", "?x !ex:q ?x"),
    ("repeat-path", "?x !(ex:q|ex:p) ?x"),
    ("repeat-path", "?x ex:p/^ex:p ?x"),
    ("repeat-path", "?x ex:knows+ ?x"),
    ("repeat-path", "?x ex:knows* ?x"),
    # blank-node labels in the query
    ("bnode-label", "_:b ex:p ?x . _:b ex:q ?y"),
    ("bnode-label", "?x ex:p _:b . _:b ex:p ?y"),
    ("bnode-label", "_:b ex:p _:b"),
    ("bnode-label", "?x ex:p _:b . ?y ex:q _:b"),
    ("bnode-label", "_:b ex:p ?x OPTIONAL { ?x ex:q ?y }"),
    ("bnode-label", "?x ex:p _:b OPTIONAL { ?x ex:q _:c }"),
    ("bnode-label", "?x ex:p _:b OPTIONAL { _:b ex:q ?y }"),
    ("bnode-label", "?x ex:p _:b FILTER NOT EXISTS { _:b ex:q ?x }"),
    ("bnode-label", "{ ?x ex:p _:b } UNION { ?x ex:q _:b }"),
    ("bnode-label", "?x ex:p _:b MINUS { ?x ex:q _:b }"),
    # variables bound on one side only
    ("one-side", "?x ex:p ?y OPTIONAL { ?y ex:q ?z }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?z ex:q ?x }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?y ex:q ?z } OPTIONAL { ?z ex:p ?w }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?x ex:q ?y }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?x ex:q ?z FILTER (?z = ?y) }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?x ex:q ?z FILTER (?w = ?y) }"),
    ("one-side", "?x ex:p ?y OPTIONAL { ?q ex:q ?z FILTER (?q = ?x) }"),
    ("one-side", "OPTIONAL { ?x ex:p ?y } OPTIONAL { ?x ex:q ?y }"),
    ("one-side", "OPTIONAL { ?x ex:p ?y } ?x ex:q ?z"),
    ("one-side", "?x ex:p ?y MINUS { ?y ex:q ?z }"),
    ("one-side", "?x ex:p ?y MINUS { ?z ex:q ?w }"),
    ("one-side", "?x ex:p ?y MINUS { ?w ex:q ?x }"),
    ("one-side", "?x ex:p ?y MINUS { ?x ex:q ?y }"),
    ("one-side", "?x ex:p ?y MINUS { }"),
    ("one-side", "?x ex:p ?y MINUS { FILTER (?x = ?y) }"),
    ("one-side", "?x ex:p ?y FILTER EXISTS { ?y ex:q ?z }"),
    ("one-side", "?x ex:p ?y FILTER EXISTS { ?z ex:q ?w }"),
    ("one-side", "?x ex:p ?y FILTER NOT EXISTS { ?y ex:q ?x }"),
    ("one-side", "?x ex:p ?y FILTER NOT EXISTS { ?z ex:q ?w }"),
    ("one-side", "?x ex:p ?y FILTER EXISTS { ?x ex:q ?z FILTER (?z = ?y) }"),
    ("one-side", "?x ex:p ?y FILTER NOT EXISTS { ?x ex:q ?z FILTER (?z != ?y) }"),
    ("one-side", "?x ex:p ?y FILTER NOT EXISTS { ?x ex:q ?z OPTIONAL { ?z ex:p ?y } }"),
    ("one-side", "?x ex:p ?y FILTER EXISTS { { ?x ex:q ?z } UNION { ?y ex:q ?z } }"),
    ("one-side", "?x ex:p ?y FILTER EXISTS { ?x ex:p ?x }"),
    ("one-side", "{ ?x ex:p ?y } UNION { ?x ex:q ?z }"),
    ("one-side", "{ ?x ex:p ?y } UNION { ?z ex:q ?x }"),
    ("one-side", "{ ?x ex:p ?y } UNION { ?x ex:p ?y }"),
    ("one-side", "{ ?x ex:p ?y } UNION { ?y ex:q ?z } ?x ex:r ?w"),
    ("one-side", "{ ?x ex:p ?y } UNION { ?y ex:q ?z } OPTIONAL { ?y ex:p ?w }"),
    ("one-side", "{ ?x ex:p ?y OPTIONAL { ?y ex:q ?z } } ?z ex:p ?w"),
    ("one-side", "{ ?x ex:p ?y OPTIONAL { ?y ex:q ?z } } UNION { ?z ex:p ?x }"),
    ("one-side", "{ ?x ex:p ?y } ?y ex:q ?z"),
    ("one-side", "{ ?x ex:p ?y . FILTER (?y = ?z) } ?z ex:q ?w"),
    # subqueries
    ("subquery", "{ SELECT ?x WHERE { ?x ex:p ?y } } ?x ex:q ?z"),
    ("subquery", "{ SELECT ?y WHERE { ?x ex:p ?y } } ?x ex:q ?y"),
    ("subquery", "{ SELECT ?y WHERE { ?x ex:p ?y } } ?y ex:q ?z"),
    ("subquery", "{ SELECT * WHERE { ?x ex:p ?y } } ?y ex:q ?z"),
    ("subquery", "{ SELECT * WHERE { ?x ex:p ?y } } ?x ex:q ?x"),
    ("subquery", "{ SELECT DISTINCT ?x WHERE { ?x ex:p ?y } } ?x ex:q ?z"),
    ("subquery", "{ SELECT ?x WHERE { ?x ex:p ?y } } { SELECT ?x WHERE { ?x ex:q ?y } }"),
    ("subquery", "{ SELECT ?x WHERE { ?x ex:p ?y } } OPTIONAL { { SELECT ?x ?z WHERE { ?x ex:q ?z } } }"),
    ("subquery", "?x ex:p ?y { SELECT ?y WHERE { ?x ex:q ?y } }"),
    ("subquery", "?x ex:p ?y FILTER EXISTS { SELECT ?y WHERE { ?x ex:q ?y } }"),
    ("subquery", "?x ex:p ?y { SELECT (COUNT(*) AS ?n) WHERE { ?x ex:q ?y } }"),
    ("subquery", "{ SELECT ?x (COUNT(?y) AS ?n) WHERE { ?x ex:p ?y } GROUP BY ?x } OPTIONAL { ?x ex:q ?z }"),
    ("subquery", "{ SELECT ?x WHERE { ?x ex:p ?y } ORDER BY ?x LIMIT 2 } ?x ex:q ?z"),
    ("subquery", "{ SELECT ?x ?w WHERE { ?x ex:p ?y BIND (?y AS ?w) } } ?w ex:q ?z"),
    ("subquery", "{ SELECT ?x WHERE { { SELECT ?x ?y WHERE { ?x ex:p ?y } } ?y ex:q ?z } }"),
    # empty groups
    ("empty-group", "{ }"),
    ("empty-group", "{ } ?x ex:p ?y"),
    ("empty-group", "?x ex:p ?y { }"),
    ("empty-group", "?x ex:p ?y OPTIONAL { }"),
    ("empty-group", "OPTIONAL { ?x ex:p ?y }"),
    ("empty-group", "{ } UNION { ?x ex:p ?y }"),
    ("empty-group", "{ } UNION { }"),
    ("empty-group", "?x ex:p ?y FILTER EXISTS { }"),
    ("empty-group", "?x ex:p ?y FILTER NOT EXISTS { }"),
    ("empty-group", "{ { } } ?x ex:p ?y"),
    ("empty-group", "OPTIONAL { } ?x ex:p ?y"),
    ("empty-group", "{ ?x ex:p ?y FILTER (false) } UNION { ?x ex:q ?y }"),
    # duplicate solutions / lexical forms
    ("duplicates", "?x ex:v ?v"),
    ("duplicates", "?x ex:v 1"),
    ("duplicates", "?x ex:v ?v FILTER (?v = 1)"),
    ("duplicates", "?x ex:v ?v FILTER (sameTerm(?v, 1))"),
    ("duplicates", "?x ex:v ?a . ?x ex:v ?b FILTER (?a = ?b)"),
    ("duplicates", "?x ex:v ?a . ?x ex:v ?b FILTER (str(?a) != str(?b))"),
    ("duplicates", "?x ex:v ?v BIND (str(?v) AS ?s)"),
    ("duplicates", "?x ex:v ?v BIND (datatype(?v) AS ?dt)"),
    ("duplicates", "?x ex:w ?w BIND (lcase(str(?w)) AS ?l)"),
    ("duplicates", "{ ?x ex:v ?v } UNION { ?x ex:v ?v }"),
    ("duplicates", "{ ?x ex:v ?v } UNION { ?x ex:v 1 }"),
    ("duplicates", "?x ex:knows ?y . ?y ex:knows ?x"),
    ("duplicates", "?x (ex:p|ex:q) ?y"),
    ("duplicates", "?x (ex:p|ex:p) ?y"),
    ("duplicates", "?x (ex:p|ex:q)/(ex:p|ex:q) ?y"),
    ("duplicates", "?x ex:p/ex:p ?y"),
    ("duplicates", "?x ex:p+ ?y"),
    ("duplicates", "?x ex:p* ?y"),
    ("duplicates", "?x (ex:p|ex:q)* ?y"),
    ("duplicates", "VALUES ?x { ex:a1 ex:a1 ex:a2 } ?x ex:p ?y"),
    ("duplicates", 'VALUES ?v { 1 "01"^^xsd:integer 1.0 } ?x ex:v ?v'),
    # BIND shadowing / scope
    ("bind", "?x ex:p ?y BIND (?y AS ?z)"),
    ("bind", "?x ex:p ?y BIND (?x AS ?z) ?z ex:q ?w"),
    ("bind", "BIND (1 AS ?x) ?x ex:p ?y"),
    ("bind", "?x ex:p ?y BIND (?y AS ?w) FILTER (?w = ?x)"),
    ("bind", "{ BIND (ex:a1 AS ?x) } ?x ex:p ?y"),
    ("bind", "?x ex:p ?y { BIND (?x AS ?z) }"),
    ("bind", "?x ex:p ?y { BIND (ex:a1 AS ?y) }"),
    ("bind", "?x ex:p ?y OPTIONAL { BIND (ex:a1 AS ?y) }"),
    ("bind", "?x ex:p ?y OPTIONAL { BIND (ex:a1 AS ?z) ?z ex:q ?w }"),
    ("bind", "?x ex:p ?y OPTIONAL { ?x ex:q ?z BIND (?y AS ?w) }"),
    ("bind", "?x ex:p ?y BIND (ex:a1 AS ?q) { ?q ex:p ?w }"),
    ("bind", "?x ex:p ?y FILTER EXISTS { BIND (?y AS ?z) ?z ex:q ?w }"),
    ("bind", "?x ex:p ?y BIND (COALESCE(?u, 0) AS ?z)"),
    ("bind", "?x ex:p ?y OPTIONAL { ?y ex:q ?u } BIND (COALESCE(?u, 0) AS ?z)"),
    ("bind", "?x ex:p ?y BIND (bound(?u) AS ?z)"),
    ("bind", "{ SELECT ?x (?y AS ?w) WHERE { ?x ex:p ?y } } ?w ex:q ?z"),
    # VALUES with UNDEF
    ("values-undef", "VALUES (?x ?y) { (ex:a1 UNDEF) (UNDEF ex:a2) (UNDEF UNDEF) } ?x ex:p ?y"),
    ("values-undef", "VALUES (?x ?y) { (ex:a1 UNDEF) (UNDEF ex:a2) }"),
    ("values-undef", "?x ex:p ?y VALUES (?x ?z) { (ex:a1 ex:a2) (UNDEF ex:a3) (ex:a2 UNDEF) }"),
    ("values-undef", "VALUES (?x ?y) { (UNDEF UNDEF) }"),
    ("values-undef", "VALUES ?x { UNDEF } ?x ex:p ?y"),
    ("values-undef", "VALUES ?x { UNDEF ex:a1 } OPTIONAL { ?x ex:p ?y }"),
    ("values-undef", "?x ex:p ?y OPTIONAL { VALUES (?y ?z) { (ex:a1 1) (UNDEF 2) } }"),
    ("values-undef", "?x ex:p ?y MINUS { VALUES (?x ?y) { (ex:a1 UNDEF) } }"),
    ("values-undef", "?x ex:p ?y FILTER EXISTS { VALUES (?y ?z) { (UNDEF 1) } }"),
    ("values-undef", "{ VALUES (?x ?y) { (ex:a1 UNDEF) (ex:a2 ex:a3) } } UNION { ?x ex:q ?y }"),
    ("values-undef", "VALUES (?x ?y) { (ex:a1 ex:a1) (ex:a1 UNDEF) } ?x ex:p ?y"),
    ("values-undef", "VALUES (?x ?x2) { (ex:a1 UNDEF) (UNDEF ex:a1) } ?x ex:p ?y"),
]

SHAPE_FORMS = [
    "SELECT * WHERE {{ {body} }}",
    "SELECT DISTINCT * WHERE {{ {body} }}",
    "SELECT (COUNT(*) AS ?c) WHERE {{ {body} }}",
    "ASK {{ {body} }}",
]


def _q(form, body):
    return PREFIX + SHAPE_FORMS[form].format(body=body)


# Queries the references agree on and the engine still gets wrong. They are
# kept out of queries.json so the test stays green, and written to
# pending.json; move them back with the fix. Empty: the COUNT/ASK shortcuts
# and blank-node labels as variables were fixed together.
PENDING = {}


def shape_candidates(rng):
    """(family, query) for every SHAPES template under every form, plus
    seeded decorations: a repeated-variable pattern combined with a random
    extra clause, so the planner has another pattern to put first."""
    out = []
    for fam, body in SHAPES:
        for f in SHAPE_FORMS:
            out.append((fam, PREFIX + f.format(body=body)))
    decorations = [
        "?y ex:q ?z", "?x ex:q ?z", "OPTIONAL { ?x ex:q ?z }", "FILTER EXISTS { ?x ex:q ?z }",
        "MINUS { ?x ex:r ?z }", "?z ex:p ?x", "FILTER (?x != ex:a2)", "BIND (str(?x) AS ?s)",
    ]
    repeated = [b for fam, b in SHAPES if fam in ("repeat-pattern", "repeat-path")]
    for b in repeated:
        for d in rng.sample(decorations, 2):
            for body in (f"{b} {d}", f"{d} {b}"):
                out.append(("decorated", PREFIX + "SELECT * WHERE { " + body + " }"))
    return out


def term_json(t):
    """A term as a dict the Go test can compare exactly."""
    if t is None:
        return None
    if isinstance(t, ox.NamedNode):
        return {"type": "uri", "value": t.value}
    if isinstance(t, ox.BlankNode):
        return {"type": "bnode"}
    if isinstance(t, ox.Literal):
        d = {"type": "literal", "value": t.value}
        if t.language:
            d["lang"] = t.language
        elif t.datatype.value != "http://www.w3.org/2001/XMLSchema#string":
            d["datatype"] = t.datatype.value
        return d
    raise TypeError(t)


def rdflib_term_json(t):
    if t is None:
        return None
    if isinstance(t, rdflib.URIRef):
        return {"type": "uri", "value": str(t)}
    if isinstance(t, rdflib.BNode):
        return {"type": "bnode"}
    if isinstance(t, rdflib.Literal):
        d = {"type": "literal", "value": str(t)}
        if t.language:
            d["lang"] = t.language
        elif t.datatype and str(t.datatype) != "http://www.w3.org/2001/XMLSchema#string":
            d["datatype"] = str(t.datatype)
        return d
    raise TypeError(t)


def canon(rows):
    return sorted(json.dumps(r, sort_keys=True, ensure_ascii=False) for r in rows)


def run_ox(store, q):
    r = store.query(q)
    if isinstance(r, (bool, ox.QueryBoolean)):
        return {"ask": bool(r)}
    vs = [v.value for v in r.variables]
    rows = []
    for sol in r:
        row = {}
        for v in vs:
            t = sol[v]
            if t is not None:
                row[v] = term_json(t)
        rows.append(row)
    return {"vars": vs, "rows": rows}


def run_rdflib(g, q):
    r = g.query(q)
    if r.type == "ASK":
        return {"ask": bool(r.askAnswer)}
    vs = [str(v) for v in r.vars]
    rows = []
    for sol in r:
        row = {}
        for v in vs:
            t = sol[v]
            if t is not None:
                row[v] = rdflib_term_json(t)
        rows.append(row)
    return {"vars": vs, "rows": rows}


def same(a, b):
    if "ask" in a or "ask" in b:
        return a == b
    return sorted(a["vars"]) == sorted(b["vars"]) and canon(a["rows"]) == canon(b["rows"])


def main(out):
    out = Path(out)
    data = (out / "data.ttl").read_text()
    store = ox.Store()
    store.load(data.encode(), ox.RdfFormat.TURTLE)
    g = rdflib.Graph()
    g.parse(data=data, format="turtle")

    rng = random.Random(SEED)
    seen, agreed, disagreed = set(), [], []
    while len(seen) < TARGET:
        q = candidate(rng)
        if q in seen:
            continue
        seen.add(q)
        try:
            a = run_ox(store, q)
        except Exception as e:  # noqa: BLE001 — a reference rejecting it is a disagreement
            a = {"error": str(e)}
        try:
            b = run_rdflib(g, q)
        except Exception as e:  # noqa: BLE001
            b = {"error": str(e)}
        if "error" not in a and "error" not in b and same(a, b):
            agreed.append({"query": q, "expected": a})
        else:
            disagreed.append({"query": q, "oxigraph": a, "rdflib": b})

    families = {}
    pending = []
    for fam, q in shape_candidates(random.Random(SEED + 1)):
        if q in seen:
            continue
        seen.add(q)
        try:
            a = run_ox(store, q)
        except Exception as e:  # noqa: BLE001
            a = {"error": str(e)}
        try:
            b = run_rdflib(g, q)
        except Exception as e:  # noqa: BLE001
            b = {"error": str(e)}
        st = families.setdefault(fam, {"agreed": 0, "differ": 0, "both-error": 0})
        if "error" not in a and "error" not in b and same(a, b):
            if q in PENDING:
                pending.append({"query": q, "expected": a, "bug": PENDING[q]})
                st["pending"] = st.get("pending", 0) + 1
            else:
                agreed.append({"query": q, "expected": a})
            st["agreed"] += 1
        else:
            st["both-error" if "error" in a and "error" in b else "differ"] += 1
            disagreed.append({"query": q, "family": fam, "oxigraph": a, "rdflib": b})
    for fam, st in sorted(families.items()):
        print(fam, st)

    (out / "pending.json").write_text(json.dumps(pending, indent=1, ensure_ascii=False) + "\n")
    (out / "queries.json").write_text(json.dumps(agreed, indent=1, ensure_ascii=False) + "\n")
    (out / "disagreements.json").write_text(json.dumps(disagreed, indent=1, ensure_ascii=False) + "\n")
    print(f"{len(agreed)} agreed, {len(disagreed)} disagreed (of {len(seen)})")


if __name__ == "__main__":
    main(sys.argv[1])
