package store

// mustV returns v, failing the test binary on err. It stands in for the error
// results the store and graph APIs gained in v0.9 (issue #52), in tests whose
// stores cannot fail.
func mustV[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
