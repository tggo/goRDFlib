// Package turtle implements Turtle (Terse RDF Triple Language) serialization
// and parsing for RDF graphs. It supports the W3C Turtle specification including
// prefix declarations, blank node syntax, collections, and literal shorthands.
//
// Note: The parser reads the entire input into memory via io.ReadAll.
// For very large Turtle files this may be problematic.
package turtle
