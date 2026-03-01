package term

import "errors"

// Sentinel errors for the term package.
var (
	// ErrInvalidIRI indicates an IRI contains forbidden characters.
	ErrInvalidIRI = errors.New("rdflibgo: invalid IRI")

	// ErrUnknownFormat indicates a parser/serializer format is not registered.
	ErrUnknownFormat = errors.New("rdflibgo: unknown format")

	// ErrTermNotInNamespace indicates a term is not defined in a closed namespace.
	ErrTermNotInNamespace = errors.New("rdflibgo: term not in closed namespace")

	// ErrInvalidCURIE indicates a malformed CURIE string.
	ErrInvalidCURIE = errors.New("rdflibgo: invalid CURIE")

	// ErrPrefixNotBound indicates a namespace prefix has no binding.
	ErrPrefixNotBound = errors.New("rdflibgo: prefix not bound")
)
