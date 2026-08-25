// Package storetest is a backend-independent conformance suite for
// store.Store implementations.
//
// It exists so that a storage backend maintained outside this repository —
// MongoDB, Postgres, a cloud KV, anything — can prove it behaves like the
// backends shipped here without copying their test files. The contract that
// store.Store documents is only half a specification; the rest of it lives in
// the assertions this package makes, such as "adding the same triple twice
// leaves Len at 1", "Contexts does not report the default graph", or "a BNode
// context is treated as the default graph".
//
// Usage from a backend's own test file:
//
//	func TestConformance(t *testing.T) {
//		storetest.Run(t, storetest.Config{
//			New: func(t *testing.T) store.Store {
//				s, err := mongostore.New(mongostore.WithURI(uri))
//				if err != nil {
//					t.Fatalf("New: %v", err)
//				}
//				t.Cleanup(func() { s.Close() })
//				return s
//			},
//		})
//	}
//
// Optional interfaces are detected, not declared: a store that implements
// store.QueryableStore or store.ReachabilityStore automatically gets the extra
// sections, and one that does not simply skips them. Persistence is the one
// capability that cannot be detected, so it is opt-in through Config.Reopen.
//
// The suite is deliberately behavioural. It never inspects a backend's internal
// representation, so it says nothing about encoding, error paths, or resource
// cleanup — a backend still needs its own tests for those. What it guarantees is
// that a graph, a SPARQL query, or a SHACL validation behaves the same way on
// top of it as on top of MemoryStore.
package storetest
