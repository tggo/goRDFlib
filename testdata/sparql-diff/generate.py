"""Generate the SPARQL differential test corpus.

Each query is built from real SPARQL constructs (OPTIONAL with a filter on
the left side, EXISTS/NOT EXISTS with an inner FILTER, MINUS, UNION, BIND,
VALUES, subqueries, aggregates, property paths, built-in functions) over
data.ttl. Every candidate is run in two independent engines, Oxigraph and
rdflib; a query enters queries.json with its expected result only when the
two agree. Candidates they disagree on are written to disagreements.json for
inspection and are not part of the test.

Regenerate (needs pyoxigraph and rdflib):

    uv venv /tmp/v && VIRTUAL_ENV=/tmp/v uv pip install pyoxigraph rdflib
    /tmp/v/bin/python -I testdata/sparql-diff/generate.py testdata/sparql-diff

The seed is fixed, so the corpus is reproducible.
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

    (out / "queries.json").write_text(json.dumps(agreed, indent=1, ensure_ascii=False) + "\n")
    (out / "disagreements.json").write_text(json.dumps(disagreed, indent=1, ensure_ascii=False) + "\n")
    print(f"{len(agreed)} agreed, {len(disagreed)} disagreed (of {len(seen)})")


if __name__ == "__main__":
    main(sys.argv[1])
