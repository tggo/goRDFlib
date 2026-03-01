package namespace

import "github.com/tggo/goRDFlib/term"

// Built-in RDF namespace constants.
// Ported from: rdflib.namespace._RDF, _RDFS, _OWL, _XSD

// --- RDF Namespace ---
// Ported from: rdflib.namespace._RDF.RDF

var RDF = struct {
	NS Namespace

	// Properties
	Direction, First, Language, Object, Predicate, Rest, Subject, Type, Value term.URIRef

	// Classes
	Alt, Bag, CompoundLiteral, List, Property, Seq, Statement term.URIRef

	// Datatypes
	HTML, JSON, PlainLiteral, XMLLiteral, LangString term.URIRef

	// Special
	Nil term.URIRef
}{
	NS: NewNamespace(term.RDFNamespace),

	Direction: term.NewURIRefUnsafe(term.RDFNamespace + "direction"),
	First:     term.NewURIRefUnsafe(term.RDFNamespace + "first"),
	Language:  term.NewURIRefUnsafe(term.RDFNamespace + "language"),
	Object:    term.NewURIRefUnsafe(term.RDFNamespace + "object"),
	Predicate: term.NewURIRefUnsafe(term.RDFNamespace + "predicate"),
	Rest:      term.NewURIRefUnsafe(term.RDFNamespace + "rest"),
	Subject:   term.NewURIRefUnsafe(term.RDFNamespace + "subject"),
	Type:      term.NewURIRefUnsafe(term.RDFNamespace + "type"),
	Value:     term.NewURIRefUnsafe(term.RDFNamespace + "value"),

	Alt:             term.NewURIRefUnsafe(term.RDFNamespace + "Alt"),
	Bag:             term.NewURIRefUnsafe(term.RDFNamespace + "Bag"),
	CompoundLiteral: term.NewURIRefUnsafe(term.RDFNamespace + "CompoundLiteral"),
	List:            term.NewURIRefUnsafe(term.RDFNamespace + "List"),
	Property:        term.NewURIRefUnsafe(term.RDFNamespace + "Property"),
	Seq:             term.NewURIRefUnsafe(term.RDFNamespace + "Seq"),
	Statement:       term.NewURIRefUnsafe(term.RDFNamespace + "Statement"),

	HTML:        term.NewURIRefUnsafe(term.RDFNamespace + "HTML"),
	JSON:        term.NewURIRefUnsafe(term.RDFNamespace + "JSON"),
	PlainLiteral: term.NewURIRefUnsafe(term.RDFNamespace + "PlainLiteral"),
	XMLLiteral:  term.NewURIRefUnsafe(term.RDFNamespace + "XMLLiteral"),
	LangString:  term.RDFLangString,

	Nil: term.NewURIRefUnsafe(term.RDFNamespace + "nil"),
}

// --- RDFS Namespace ---
// Ported from: rdflib.namespace._RDFS.RDFS

const RDFSNamespace = "http://www.w3.org/2000/01/rdf-schema#"

var RDFS = struct {
	NS Namespace

	// Properties
	Comment, Domain, IsDefinedBy, Label, Member, Range, SeeAlso, SubClassOf, SubPropertyOf term.URIRef

	// Classes
	Class, Container, ContainerMembershipProperty, Datatype, Literal, Resource term.URIRef
}{
	NS: NewNamespace(RDFSNamespace),

	Comment:       term.NewURIRefUnsafe(RDFSNamespace + "comment"),
	Domain:        term.NewURIRefUnsafe(RDFSNamespace + "domain"),
	IsDefinedBy:   term.NewURIRefUnsafe(RDFSNamespace + "isDefinedBy"),
	Label:         term.NewURIRefUnsafe(RDFSNamespace + "label"),
	Member:        term.NewURIRefUnsafe(RDFSNamespace + "member"),
	Range:         term.NewURIRefUnsafe(RDFSNamespace + "range"),
	SeeAlso:       term.NewURIRefUnsafe(RDFSNamespace + "seeAlso"),
	SubClassOf:    term.NewURIRefUnsafe(RDFSNamespace + "subClassOf"),
	SubPropertyOf: term.NewURIRefUnsafe(RDFSNamespace + "subPropertyOf"),

	Class:                       term.NewURIRefUnsafe(RDFSNamespace + "Class"),
	Container:                   term.NewURIRefUnsafe(RDFSNamespace + "Container"),
	ContainerMembershipProperty: term.NewURIRefUnsafe(RDFSNamespace + "ContainerMembershipProperty"),
	Datatype:                    term.NewURIRefUnsafe(RDFSNamespace + "Datatype"),
	Literal:                     term.NewURIRefUnsafe(RDFSNamespace + "Literal"),
	Resource:                    term.NewURIRefUnsafe(RDFSNamespace + "Resource"),
}

// --- OWL Namespace ---
// Ported from: rdflib.namespace._OWL.OWL

const OWLNamespace = "http://www.w3.org/2002/07/owl#"

var OWL = struct {
	NS Namespace

	// Classes
	AllDifferent, AllDisjointClasses, AllDisjointProperties, Annotation,
	AnnotationProperty, AsymmetricProperty, Axiom, Class, DataRange,
	DatatypeProperty, DeprecatedClass, DeprecatedProperty, FunctionalProperty,
	InverseFunctionalProperty, IrreflexiveProperty, NamedIndividual,
	NegativePropertyAssertion, ObjectProperty, Ontology, OntologyProperty,
	ReflexiveProperty, Restriction, SymmetricProperty, TransitiveProperty,
	Nothing, Thing term.URIRef

	// Properties
	AllValuesFrom, AnnotatedProperty, AnnotatedSource, AnnotatedTarget,
	AssertionProperty, Cardinality, ComplementOf, DatatypeComplementOf,
	DifferentFrom, DisjointUnionOf, DisjointWith, DistinctMembers,
	EquivalentClass, EquivalentProperty, HasKey, HasSelf, HasValue,
	IntersectionOf, InverseOf, MaxCardinality, MaxQualifiedCardinality,
	Members, MinCardinality, MinQualifiedCardinality, OnClass, OnDataRange,
	OnDatatype, OnProperties, OnProperty, OneOf, PropertyChainAxiom,
	PropertyDisjointWith, QualifiedCardinality, SameAs, SomeValuesFrom,
	SourceIndividual, TargetIndividual, TargetValue, UnionOf, WithRestrictions term.URIRef

	// Annotation properties
	BackwardCompatibleWith, Deprecated, IncompatibleWith, PriorVersion, VersionInfo term.URIRef

	// Data/Object properties
	BottomDataProperty, TopDataProperty, BottomObjectProperty, TopObjectProperty term.URIRef

	// Ontology properties
	Imports, VersionIRI term.URIRef
}{
	NS: NewNamespace(OWLNamespace),

	AllDifferent:              term.NewURIRefUnsafe(OWLNamespace + "AllDifferent"),
	AllDisjointClasses:        term.NewURIRefUnsafe(OWLNamespace + "AllDisjointClasses"),
	AllDisjointProperties:     term.NewURIRefUnsafe(OWLNamespace + "AllDisjointProperties"),
	Annotation:                term.NewURIRefUnsafe(OWLNamespace + "Annotation"),
	AnnotationProperty:        term.NewURIRefUnsafe(OWLNamespace + "AnnotationProperty"),
	AsymmetricProperty:        term.NewURIRefUnsafe(OWLNamespace + "AsymmetricProperty"),
	Axiom:                     term.NewURIRefUnsafe(OWLNamespace + "Axiom"),
	Class:                     term.NewURIRefUnsafe(OWLNamespace + "Class"),
	DataRange:                 term.NewURIRefUnsafe(OWLNamespace + "DataRange"),
	DatatypeProperty:          term.NewURIRefUnsafe(OWLNamespace + "DatatypeProperty"),
	DeprecatedClass:           term.NewURIRefUnsafe(OWLNamespace + "DeprecatedClass"),
	DeprecatedProperty:        term.NewURIRefUnsafe(OWLNamespace + "DeprecatedProperty"),
	FunctionalProperty:        term.NewURIRefUnsafe(OWLNamespace + "FunctionalProperty"),
	InverseFunctionalProperty: term.NewURIRefUnsafe(OWLNamespace + "InverseFunctionalProperty"),
	IrreflexiveProperty:       term.NewURIRefUnsafe(OWLNamespace + "IrreflexiveProperty"),
	NamedIndividual:           term.NewURIRefUnsafe(OWLNamespace + "NamedIndividual"),
	NegativePropertyAssertion: term.NewURIRefUnsafe(OWLNamespace + "NegativePropertyAssertion"),
	ObjectProperty:            term.NewURIRefUnsafe(OWLNamespace + "ObjectProperty"),
	Ontology:                  term.NewURIRefUnsafe(OWLNamespace + "Ontology"),
	OntologyProperty:          term.NewURIRefUnsafe(OWLNamespace + "OntologyProperty"),
	ReflexiveProperty:         term.NewURIRefUnsafe(OWLNamespace + "ReflexiveProperty"),
	Restriction:               term.NewURIRefUnsafe(OWLNamespace + "Restriction"),
	SymmetricProperty:         term.NewURIRefUnsafe(OWLNamespace + "SymmetricProperty"),
	TransitiveProperty:        term.NewURIRefUnsafe(OWLNamespace + "TransitiveProperty"),
	Nothing:                   term.NewURIRefUnsafe(OWLNamespace + "Nothing"),
	Thing:                     term.NewURIRefUnsafe(OWLNamespace + "Thing"),

	AllValuesFrom:            term.NewURIRefUnsafe(OWLNamespace + "allValuesFrom"),
	AnnotatedProperty:        term.NewURIRefUnsafe(OWLNamespace + "annotatedProperty"),
	AnnotatedSource:          term.NewURIRefUnsafe(OWLNamespace + "annotatedSource"),
	AnnotatedTarget:          term.NewURIRefUnsafe(OWLNamespace + "annotatedTarget"),
	AssertionProperty:        term.NewURIRefUnsafe(OWLNamespace + "assertionProperty"),
	Cardinality:              term.NewURIRefUnsafe(OWLNamespace + "cardinality"),
	ComplementOf:             term.NewURIRefUnsafe(OWLNamespace + "complementOf"),
	DatatypeComplementOf:     term.NewURIRefUnsafe(OWLNamespace + "datatypeComplementOf"),
	DifferentFrom:            term.NewURIRefUnsafe(OWLNamespace + "differentFrom"),
	DisjointUnionOf:          term.NewURIRefUnsafe(OWLNamespace + "disjointUnionOf"),
	DisjointWith:             term.NewURIRefUnsafe(OWLNamespace + "disjointWith"),
	DistinctMembers:          term.NewURIRefUnsafe(OWLNamespace + "distinctMembers"),
	EquivalentClass:          term.NewURIRefUnsafe(OWLNamespace + "equivalentClass"),
	EquivalentProperty:       term.NewURIRefUnsafe(OWLNamespace + "equivalentProperty"),
	HasKey:                   term.NewURIRefUnsafe(OWLNamespace + "hasKey"),
	HasSelf:                  term.NewURIRefUnsafe(OWLNamespace + "hasSelf"),
	HasValue:                 term.NewURIRefUnsafe(OWLNamespace + "hasValue"),
	IntersectionOf:           term.NewURIRefUnsafe(OWLNamespace + "intersectionOf"),
	InverseOf:                term.NewURIRefUnsafe(OWLNamespace + "inverseOf"),
	MaxCardinality:           term.NewURIRefUnsafe(OWLNamespace + "maxCardinality"),
	MaxQualifiedCardinality:  term.NewURIRefUnsafe(OWLNamespace + "maxQualifiedCardinality"),
	Members:                  term.NewURIRefUnsafe(OWLNamespace + "members"),
	MinCardinality:           term.NewURIRefUnsafe(OWLNamespace + "minCardinality"),
	MinQualifiedCardinality:  term.NewURIRefUnsafe(OWLNamespace + "minQualifiedCardinality"),
	OnClass:                  term.NewURIRefUnsafe(OWLNamespace + "onClass"),
	OnDataRange:              term.NewURIRefUnsafe(OWLNamespace + "onDataRange"),
	OnDatatype:               term.NewURIRefUnsafe(OWLNamespace + "onDatatype"),
	OnProperties:             term.NewURIRefUnsafe(OWLNamespace + "onProperties"),
	OnProperty:               term.NewURIRefUnsafe(OWLNamespace + "onProperty"),
	OneOf:                    term.NewURIRefUnsafe(OWLNamespace + "oneOf"),
	PropertyChainAxiom:       term.NewURIRefUnsafe(OWLNamespace + "propertyChainAxiom"),
	PropertyDisjointWith:     term.NewURIRefUnsafe(OWLNamespace + "propertyDisjointWith"),
	QualifiedCardinality:     term.NewURIRefUnsafe(OWLNamespace + "qualifiedCardinality"),
	SameAs:                   term.NewURIRefUnsafe(OWLNamespace + "sameAs"),
	SomeValuesFrom:           term.NewURIRefUnsafe(OWLNamespace + "someValuesFrom"),
	SourceIndividual:         term.NewURIRefUnsafe(OWLNamespace + "sourceIndividual"),
	TargetIndividual:         term.NewURIRefUnsafe(OWLNamespace + "targetIndividual"),
	TargetValue:              term.NewURIRefUnsafe(OWLNamespace + "targetValue"),
	UnionOf:                  term.NewURIRefUnsafe(OWLNamespace + "unionOf"),
	WithRestrictions:         term.NewURIRefUnsafe(OWLNamespace + "withRestrictions"),

	BackwardCompatibleWith: term.NewURIRefUnsafe(OWLNamespace + "backwardCompatibleWith"),
	Deprecated:             term.NewURIRefUnsafe(OWLNamespace + "deprecated"),
	IncompatibleWith:       term.NewURIRefUnsafe(OWLNamespace + "incompatibleWith"),
	PriorVersion:           term.NewURIRefUnsafe(OWLNamespace + "priorVersion"),
	VersionInfo:            term.NewURIRefUnsafe(OWLNamespace + "versionInfo"),

	BottomDataProperty:   term.NewURIRefUnsafe(OWLNamespace + "bottomDataProperty"),
	TopDataProperty:      term.NewURIRefUnsafe(OWLNamespace + "topDataProperty"),
	BottomObjectProperty: term.NewURIRefUnsafe(OWLNamespace + "bottomObjectProperty"),
	TopObjectProperty:    term.NewURIRefUnsafe(OWLNamespace + "topObjectProperty"),

	Imports:    term.NewURIRefUnsafe(OWLNamespace + "imports"),
	VersionIRI: term.NewURIRefUnsafe(OWLNamespace + "versionIRI"),
}

// --- Common Extended Namespaces ---
// Ported from: rdflib.namespace (FOAF, DC, DCTERMS, SKOS, PROV, etc.)

var (
	FOAF    = NewNamespace("http://xmlns.com/foaf/0.1/")
	DC      = NewNamespace("http://purl.org/dc/elements/1.1/")
	DCTERMS = NewNamespace("http://purl.org/dc/terms/")
	SKOS    = NewNamespace("http://www.w3.org/2004/02/skos/core#")
	PROV    = NewNamespace("http://www.w3.org/ns/prov#")
	SH      = NewNamespace("http://www.w3.org/ns/shacl#")
	SOSA    = NewNamespace("http://www.w3.org/ns/sosa/")
	SSN     = NewNamespace("http://www.w3.org/ns/ssn/")
	DCAT    = NewNamespace("http://www.w3.org/ns/dcat#")
	VOID    = NewNamespace("http://rdfs.org/ns/void#")
)
