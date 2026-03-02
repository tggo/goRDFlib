"""Benchmarks for Python rdflib — comparable to Go rdflibgo benchmarks."""

import time
import sys

from rdflib import Graph, URIRef, Literal, BNode, Namespace
from rdflib.namespace import XSD, RDF


def bench(name, fn, n=None):
    """Run fn repeatedly, auto-calibrate iterations to get ~1s total."""
    if n is None:
        # warm up & calibrate
        n = 1
        while True:
            t0 = time.perf_counter_ns()
            for _ in range(n):
                fn()
            elapsed = time.perf_counter_ns() - t0
            if elapsed > 500_000_000:  # 0.5s
                break
            n *= 2

    t0 = time.perf_counter_ns()
    for _ in range(n):
        fn()
    elapsed = time.perf_counter_ns() - t0
    ns_per_op = elapsed // n
    print(f"Benchmark{name}\t{n}\t{ns_per_op} ns/op")


# --- Term creation ---

def bench_new_uriref():
    URIRef("http://example.org/resource")

def bench_new_bnode():
    BNode()

def bench_new_literal_string():
    Literal("hello world")

def bench_new_literal_int():
    Literal(42)

# --- N3 serialization ---

_uri = URIRef("http://example.org/resource")
def bench_uriref_n3():
    _uri.n3()

_lit = Literal("hello world")
def bench_literal_n3():
    _lit.n3()

# --- Literal equality ---

_l1 = Literal("1", datatype=XSD.integer)
_l2 = Literal("01", datatype=XSD.integer)
def bench_literal_eq():
    _l1 == _l2

# --- MemoryStore add ---

def bench_store_add():
    g = Graph()
    pred = URIRef("http://example.org/p")
    for i in range(10000):
        g.add((URIRef(f"http://example.org/s{i}"), pred, Literal(i)))

# --- MemoryStore triples lookup ---

def make_lookup_graph():
    g = Graph()
    sub = URIRef("http://example.org/s")
    pred = URIRef("http://example.org/p")
    for i in range(1000):
        g.add((sub, pred, Literal(i)))
    return g, sub, pred

_lg, _ls, _lp = make_lookup_graph()
def bench_store_triples():
    for _ in _lg.triples((_ls, _lp, None)):
        pass

# --- Graph parse (Turtle, small) ---

_turtle_data = """
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Alice a ex:Person ;
    rdfs:label "Alice" ;
    ex:knows ex:Bob .

ex:Bob a ex:Person ;
    rdfs:label "Bob" .
"""

def bench_parse_turtle():
    g = Graph()
    g.parse(data=_turtle_data, format="turtle")

# --- Graph serialize (Turtle, small) ---

_sg = Graph()
_sg.parse(data=_turtle_data, format="turtle")
def bench_serialize_turtle():
    _sg.serialize(format="turtle")

# --- SPARQL query ---

_qg = Graph()
for i in range(100):
    _qg.add((URIRef(f"http://example.org/s{i}"), RDF.type, URIRef("http://example.org/Thing")))
    _qg.add((URIRef(f"http://example.org/s{i}"), URIRef("http://example.org/value"), Literal(i)))

def bench_sparql_select():
    list(_qg.query("SELECT ?s ?v WHERE { ?s a <http://example.org/Thing> ; <http://example.org/value> ?v } LIMIT 50"))


if __name__ == "__main__":
    print(f"Python {sys.version}")
    print(f"rdflib {__import__('rdflib').__version__}")
    print()

    bench("NewURIRef", bench_new_uriref)
    bench("NewBNode", bench_new_bnode)
    bench("NewLiteralString", bench_new_literal_string)
    bench("NewLiteralInt", bench_new_literal_int)
    bench("URIRefN3", bench_uriref_n3)
    bench("LiteralN3", bench_literal_n3)
    bench("LiteralEq", bench_literal_eq)
    bench("StoreAdd_10k", bench_store_add, n=10)
    bench("StoreTriples_1k", bench_store_triples)
    bench("ParseTurtle", bench_parse_turtle)
    bench("SerializeTurtle", bench_serialize_turtle)
    bench("SPARQLSelect", bench_sparql_select)
