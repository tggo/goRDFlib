package jsonld

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// ErrProcessorPanic reports that the bundled JSON-LD processor crashed on the
// input instead of rejecting it.
//
// The JSON-LD algorithms are implemented by [github.com/piprate/json-gold],
// which is not always defensive about malformed input: an `@id` of "%" with a
// base IRI, for instance, makes it resolve a reference against a URL that
// failed to parse and dereference the nil result.
//
// A parser is routinely pointed at untrusted bytes, and a library that takes
// the process down with it is not usable for that. So a panic from the
// processor is caught and returned as an error wrapping this sentinel, with the
// original panic value in the message and the stack retained for a bug report.
// Callers can tell it apart with errors.Is.
//
// A panic here is always an upstream bug, never a legitimate rejection. It is
// worth reporting to json-gold with the document that triggered it.
var ErrProcessorPanic = errors.New("jsonld: the JSON-LD processor panicked")

// guard runs fn and converts a panic into an error.
//
// The stack is captured at the point of the panic rather than reconstructed
// afterwards, because by the time the error reaches a caller the frames that
// matter are gone.
func guard(what string, fn func() error) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err = fmt.Errorf("%w during %s: %v\n\n%s", ErrProcessorPanic, what, r, debug.Stack())
	}()
	return fn()
}
