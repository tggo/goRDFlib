// Package reasoning provides RDF entailment (inference) support.
//
// Expand materialises inferences into a graph: the RDFS closure rules
// (rdfs2, rdfs3, rdfs5, rdfs7, rdfs9, rdfs11) and the OWL 2 RL rules
// (prp-symp, prp-trp, prp-inv, prp-eqp, prp-fp, prp-ifp, eq-sym, eq-trans,
// eq-rep, cax-eqc).
//
// Entails and Satisfiable decide entailment and consistency under the
// semantics of the RDF 1.1 and RDF 1.2 specifications (simple, RDF and RDFS
// entailment, with datatype recognition). They check small graphs, such as
// the W3C test suites, and do not modify their arguments.
package reasoning
