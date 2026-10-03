// Package turtle implements Turtle (Terse RDF Triple Language) serialization
// and parsing for RDF graphs. It supports the W3C Turtle specification including
// prefix declarations, blank node syntax, collections, and literal shorthands.
//
// # Output layout
//
// Serialize writes a compact document by default: one statement per subject,
// blank-node property lists inline. WithPretty (or WithIndent, to pick the
// indentation step) switches to an indented layout that puts every subject,
// predicate and nested blank node on a line of its own:
//
//	ex:document
//	    ex:entry
//	        [
//	            a ex:Entry ;
//	            ex:label "Alpha"
//	        ] .
//
// Whitespace is not significant in Turtle, so both layouts denote the same graph
// and round-trip through Parse unchanged. WithMaxNestDepth bounds how deeply
// blank nodes and collections nest inline; past the limit a node is written as a
// bare label and its statement is emitted at the top level instead.
//
// Note: The parser reads the entire input into memory via io.ReadAll.
// ParseStream avoids building a graph, which is most of the memory a large
// file costs, but not that buffer; N-Triples (package nt) streams line by line.
package turtle
