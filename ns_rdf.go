package rdflibgo

// Built-in RDF namespace constants.
// Ported from: rdflib.namespace._RDF, _RDFS, _OWL, _XSD

// --- RDF Namespace ---
// Ported from: rdflib.namespace._RDF.RDF

var RDF = struct {
	NS Namespace

	// Properties
	Direction, First, Language, Object, Predicate, Rest, Subject, Type, Value URIRef

	// Classes
	Alt, Bag, CompoundLiteral, List, Property, Seq, Statement URIRef

	// Datatypes
	HTML, JSON, PlainLiteral, XMLLiteral, LangString URIRef

	// Special
	Nil URIRef
}{
	NS: NewNamespace(RDFNamespace),

	Direction: NewURIRefUnsafe(RDFNamespace + "direction"),
	First:     NewURIRefUnsafe(RDFNamespace + "first"),
	Language:  NewURIRefUnsafe(RDFNamespace + "language"),
	Object:    NewURIRefUnsafe(RDFNamespace + "object"),
	Predicate: NewURIRefUnsafe(RDFNamespace + "predicate"),
	Rest:      NewURIRefUnsafe(RDFNamespace + "rest"),
	Subject:   NewURIRefUnsafe(RDFNamespace + "subject"),
	Type:      NewURIRefUnsafe(RDFNamespace + "type"),
	Value:     NewURIRefUnsafe(RDFNamespace + "value"),

	Alt:             NewURIRefUnsafe(RDFNamespace + "Alt"),
	Bag:             NewURIRefUnsafe(RDFNamespace + "Bag"),
	CompoundLiteral: NewURIRefUnsafe(RDFNamespace + "CompoundLiteral"),
	List:            NewURIRefUnsafe(RDFNamespace + "List"),
	Property:        NewURIRefUnsafe(RDFNamespace + "Property"),
	Seq:             NewURIRefUnsafe(RDFNamespace + "Seq"),
	Statement:       NewURIRefUnsafe(RDFNamespace + "Statement"),

	HTML:        NewURIRefUnsafe(RDFNamespace + "HTML"),
	JSON:        NewURIRefUnsafe(RDFNamespace + "JSON"),
	PlainLiteral: NewURIRefUnsafe(RDFNamespace + "PlainLiteral"),
	XMLLiteral:  NewURIRefUnsafe(RDFNamespace + "XMLLiteral"),
	LangString:  RDFLangString,

	Nil: NewURIRefUnsafe(RDFNamespace + "nil"),
}

// --- RDFS Namespace ---
// Ported from: rdflib.namespace._RDFS.RDFS

const RDFSNamespace = "http://www.w3.org/2000/01/rdf-schema#"

var RDFS = struct {
	NS Namespace

	// Properties
	Comment, Domain, IsDefinedBy, Label, Member, Range, SeeAlso, SubClassOf, SubPropertyOf URIRef

	// Classes
	Class, Container, ContainerMembershipProperty, Datatype, Literal, Resource URIRef
}{
	NS: NewNamespace(RDFSNamespace),

	Comment:       NewURIRefUnsafe(RDFSNamespace + "comment"),
	Domain:        NewURIRefUnsafe(RDFSNamespace + "domain"),
	IsDefinedBy:   NewURIRefUnsafe(RDFSNamespace + "isDefinedBy"),
	Label:         NewURIRefUnsafe(RDFSNamespace + "label"),
	Member:        NewURIRefUnsafe(RDFSNamespace + "member"),
	Range:         NewURIRefUnsafe(RDFSNamespace + "range"),
	SeeAlso:       NewURIRefUnsafe(RDFSNamespace + "seeAlso"),
	SubClassOf:    NewURIRefUnsafe(RDFSNamespace + "subClassOf"),
	SubPropertyOf: NewURIRefUnsafe(RDFSNamespace + "subPropertyOf"),

	Class:                       NewURIRefUnsafe(RDFSNamespace + "Class"),
	Container:                   NewURIRefUnsafe(RDFSNamespace + "Container"),
	ContainerMembershipProperty: NewURIRefUnsafe(RDFSNamespace + "ContainerMembershipProperty"),
	Datatype:                    NewURIRefUnsafe(RDFSNamespace + "Datatype"),
	Literal:                     NewURIRefUnsafe(RDFSNamespace + "Literal"),
	Resource:                    NewURIRefUnsafe(RDFSNamespace + "Resource"),
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
	Nothing, Thing URIRef

	// Properties
	AllValuesFrom, AnnotatedProperty, AnnotatedSource, AnnotatedTarget,
	AssertionProperty, Cardinality, ComplementOf, DatatypeComplementOf,
	DifferentFrom, DisjointUnionOf, DisjointWith, DistinctMembers,
	EquivalentClass, EquivalentProperty, HasKey, HasSelf, HasValue,
	IntersectionOf, InverseOf, MaxCardinality, MaxQualifiedCardinality,
	Members, MinCardinality, MinQualifiedCardinality, OnClass, OnDataRange,
	OnDatatype, OnProperties, OnProperty, OneOf, PropertyChainAxiom,
	PropertyDisjointWith, QualifiedCardinality, SameAs, SomeValuesFrom,
	SourceIndividual, TargetIndividual, TargetValue, UnionOf, WithRestrictions URIRef

	// Annotation properties
	BackwardCompatibleWith, Deprecated, IncompatibleWith, PriorVersion, VersionInfo URIRef

	// Data/Object properties
	BottomDataProperty, TopDataProperty, BottomObjectProperty, TopObjectProperty URIRef

	// Ontology properties
	Imports, VersionIRI URIRef
}{
	NS: NewNamespace(OWLNamespace),

	AllDifferent:              NewURIRefUnsafe(OWLNamespace + "AllDifferent"),
	AllDisjointClasses:        NewURIRefUnsafe(OWLNamespace + "AllDisjointClasses"),
	AllDisjointProperties:     NewURIRefUnsafe(OWLNamespace + "AllDisjointProperties"),
	Annotation:                NewURIRefUnsafe(OWLNamespace + "Annotation"),
	AnnotationProperty:        NewURIRefUnsafe(OWLNamespace + "AnnotationProperty"),
	AsymmetricProperty:        NewURIRefUnsafe(OWLNamespace + "AsymmetricProperty"),
	Axiom:                     NewURIRefUnsafe(OWLNamespace + "Axiom"),
	Class:                     NewURIRefUnsafe(OWLNamespace + "Class"),
	DataRange:                 NewURIRefUnsafe(OWLNamespace + "DataRange"),
	DatatypeProperty:          NewURIRefUnsafe(OWLNamespace + "DatatypeProperty"),
	DeprecatedClass:           NewURIRefUnsafe(OWLNamespace + "DeprecatedClass"),
	DeprecatedProperty:        NewURIRefUnsafe(OWLNamespace + "DeprecatedProperty"),
	FunctionalProperty:        NewURIRefUnsafe(OWLNamespace + "FunctionalProperty"),
	InverseFunctionalProperty: NewURIRefUnsafe(OWLNamespace + "InverseFunctionalProperty"),
	IrreflexiveProperty:       NewURIRefUnsafe(OWLNamespace + "IrreflexiveProperty"),
	NamedIndividual:           NewURIRefUnsafe(OWLNamespace + "NamedIndividual"),
	NegativePropertyAssertion: NewURIRefUnsafe(OWLNamespace + "NegativePropertyAssertion"),
	ObjectProperty:            NewURIRefUnsafe(OWLNamespace + "ObjectProperty"),
	Ontology:                  NewURIRefUnsafe(OWLNamespace + "Ontology"),
	OntologyProperty:          NewURIRefUnsafe(OWLNamespace + "OntologyProperty"),
	ReflexiveProperty:         NewURIRefUnsafe(OWLNamespace + "ReflexiveProperty"),
	Restriction:               NewURIRefUnsafe(OWLNamespace + "Restriction"),
	SymmetricProperty:         NewURIRefUnsafe(OWLNamespace + "SymmetricProperty"),
	TransitiveProperty:        NewURIRefUnsafe(OWLNamespace + "TransitiveProperty"),
	Nothing:                   NewURIRefUnsafe(OWLNamespace + "Nothing"),
	Thing:                     NewURIRefUnsafe(OWLNamespace + "Thing"),

	AllValuesFrom:            NewURIRefUnsafe(OWLNamespace + "allValuesFrom"),
	AnnotatedProperty:        NewURIRefUnsafe(OWLNamespace + "annotatedProperty"),
	AnnotatedSource:          NewURIRefUnsafe(OWLNamespace + "annotatedSource"),
	AnnotatedTarget:          NewURIRefUnsafe(OWLNamespace + "annotatedTarget"),
	AssertionProperty:        NewURIRefUnsafe(OWLNamespace + "assertionProperty"),
	Cardinality:              NewURIRefUnsafe(OWLNamespace + "cardinality"),
	ComplementOf:             NewURIRefUnsafe(OWLNamespace + "complementOf"),
	DatatypeComplementOf:     NewURIRefUnsafe(OWLNamespace + "datatypeComplementOf"),
	DifferentFrom:            NewURIRefUnsafe(OWLNamespace + "differentFrom"),
	DisjointUnionOf:          NewURIRefUnsafe(OWLNamespace + "disjointUnionOf"),
	DisjointWith:             NewURIRefUnsafe(OWLNamespace + "disjointWith"),
	DistinctMembers:          NewURIRefUnsafe(OWLNamespace + "distinctMembers"),
	EquivalentClass:          NewURIRefUnsafe(OWLNamespace + "equivalentClass"),
	EquivalentProperty:       NewURIRefUnsafe(OWLNamespace + "equivalentProperty"),
	HasKey:                   NewURIRefUnsafe(OWLNamespace + "hasKey"),
	HasSelf:                  NewURIRefUnsafe(OWLNamespace + "hasSelf"),
	HasValue:                 NewURIRefUnsafe(OWLNamespace + "hasValue"),
	IntersectionOf:           NewURIRefUnsafe(OWLNamespace + "intersectionOf"),
	InverseOf:                NewURIRefUnsafe(OWLNamespace + "inverseOf"),
	MaxCardinality:           NewURIRefUnsafe(OWLNamespace + "maxCardinality"),
	MaxQualifiedCardinality:  NewURIRefUnsafe(OWLNamespace + "maxQualifiedCardinality"),
	Members:                  NewURIRefUnsafe(OWLNamespace + "members"),
	MinCardinality:           NewURIRefUnsafe(OWLNamespace + "minCardinality"),
	MinQualifiedCardinality:  NewURIRefUnsafe(OWLNamespace + "minQualifiedCardinality"),
	OnClass:                  NewURIRefUnsafe(OWLNamespace + "onClass"),
	OnDataRange:              NewURIRefUnsafe(OWLNamespace + "onDataRange"),
	OnDatatype:               NewURIRefUnsafe(OWLNamespace + "onDatatype"),
	OnProperties:             NewURIRefUnsafe(OWLNamespace + "onProperties"),
	OnProperty:               NewURIRefUnsafe(OWLNamespace + "onProperty"),
	OneOf:                    NewURIRefUnsafe(OWLNamespace + "oneOf"),
	PropertyChainAxiom:       NewURIRefUnsafe(OWLNamespace + "propertyChainAxiom"),
	PropertyDisjointWith:     NewURIRefUnsafe(OWLNamespace + "propertyDisjointWith"),
	QualifiedCardinality:     NewURIRefUnsafe(OWLNamespace + "qualifiedCardinality"),
	SameAs:                   NewURIRefUnsafe(OWLNamespace + "sameAs"),
	SomeValuesFrom:           NewURIRefUnsafe(OWLNamespace + "someValuesFrom"),
	SourceIndividual:         NewURIRefUnsafe(OWLNamespace + "sourceIndividual"),
	TargetIndividual:         NewURIRefUnsafe(OWLNamespace + "targetIndividual"),
	TargetValue:              NewURIRefUnsafe(OWLNamespace + "targetValue"),
	UnionOf:                  NewURIRefUnsafe(OWLNamespace + "unionOf"),
	WithRestrictions:         NewURIRefUnsafe(OWLNamespace + "withRestrictions"),

	BackwardCompatibleWith: NewURIRefUnsafe(OWLNamespace + "backwardCompatibleWith"),
	Deprecated:             NewURIRefUnsafe(OWLNamespace + "deprecated"),
	IncompatibleWith:       NewURIRefUnsafe(OWLNamespace + "incompatibleWith"),
	PriorVersion:           NewURIRefUnsafe(OWLNamespace + "priorVersion"),
	VersionInfo:            NewURIRefUnsafe(OWLNamespace + "versionInfo"),

	BottomDataProperty:   NewURIRefUnsafe(OWLNamespace + "bottomDataProperty"),
	TopDataProperty:      NewURIRefUnsafe(OWLNamespace + "topDataProperty"),
	BottomObjectProperty: NewURIRefUnsafe(OWLNamespace + "bottomObjectProperty"),
	TopObjectProperty:    NewURIRefUnsafe(OWLNamespace + "topObjectProperty"),

	Imports:    NewURIRefUnsafe(OWLNamespace + "imports"),
	VersionIRI: NewURIRefUnsafe(OWLNamespace + "versionIRI"),
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
