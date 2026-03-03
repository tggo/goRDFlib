// Package trig implements TriG (Turtle with named graphs) serialization
// and parsing for RDF datasets. TriG extends Turtle with GRAPH blocks to
// support named graphs (quads). It supports both W3C TriG 1.1 and TriG 1.2
// including RDF-star features (triple terms, reified triples, annotations).
//
// Note: The parser reads the entire input into memory via io.ReadAll.
package trig
