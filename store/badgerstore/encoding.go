package badgerstore

import (
	"encoding/binary"
	"fmt"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Key separator byte. Using 0x00 (null) since it cannot appear in valid
// TermKey strings (URIs, BNode IDs, or N3 literals).
const sep = '\x00'

// Index prefix bytes for the three triple indexes.
const (
	pfxSPO = 's' // subject → predicate → object
	pfxPOS = 'p' // predicate → object → subject
	pfxOSP = 'o' // object → subject → predicate
	pfxNS  = 'n' // namespace prefix → URI
	pfxNU  = 'u' // namespace URI → prefix
	pfxCTX = 'c' // context (named graph) tracker
)

// makeKey builds a KV key from an index prefix, graph key, and three term keys.
func makeKey(prefix byte, gk, k1, k2, k3 string) []byte {
	// Format: prefix 0x00 gk 0x00 k1 0x00 k2 0x00 k3
	size := 1 + 1 + len(gk) + 1 + len(k1) + 1 + len(k2) + 1 + len(k3)
	buf := make([]byte, 0, size)
	buf = append(buf, prefix, sep)
	buf = append(buf, gk...)
	buf = append(buf, sep)
	buf = append(buf, k1...)
	buf = append(buf, sep)
	buf = append(buf, k2...)
	buf = append(buf, sep)
	buf = append(buf, k3...)
	return buf
}

// spoKey builds an SPO index key.
func spoKey(gk, sk, pk, ok string) []byte {
	return makeKey(pfxSPO, gk, sk, pk, ok)
}

// posKey builds a POS index key.
func posKey(gk, pk, ok, sk string) []byte {
	return makeKey(pfxPOS, gk, pk, ok, sk)
}

// ospKey builds an OSP index key.
func ospKey(gk, ok, sk, pk string) []byte {
	return makeKey(pfxOSP, gk, ok, sk, pk)
}

// makePrefixKey builds a prefix for scanning: prefix 0x00 gk 0x00 [k1 0x00 [k2 0x00]]
func makePrefixKey(prefix byte, gk string, parts ...string) []byte {
	size := 1 + 1 + len(gk) + 1
	for _, p := range parts {
		size += len(p) + 1
	}
	buf := make([]byte, 0, size)
	buf = append(buf, prefix, sep)
	buf = append(buf, gk...)
	buf = append(buf, sep)
	for _, p := range parts {
		buf = append(buf, p...)
		buf = append(buf, sep)
	}
	return buf
}

// nsKey builds a namespace prefix → URI key.
func nsKey(prefix string) []byte {
	buf := make([]byte, 0, 2+len(prefix))
	buf = append(buf, pfxNS, sep)
	buf = append(buf, prefix...)
	return buf
}

// nuKey builds a namespace URI → prefix key.
func nuKey(namespace string) []byte {
	buf := make([]byte, 0, 2+len(namespace))
	buf = append(buf, pfxNU, sep)
	buf = append(buf, namespace...)
	return buf
}

// ctxKey builds a context tracker key.
func ctxKey(gk string) []byte {
	buf := make([]byte, 0, 2+len(gk))
	buf = append(buf, pfxCTX, sep)
	buf = append(buf, gk...)
	return buf
}

// graphKey returns the TermKey for a context term, or "" for the default graph
// (a nil context or store.DefaultGraph). A blank node names a graph like an IRI
// does.
//
// Data written before blank nodes named graphs is unaffected: a blank-node
// context was stored under "", which is still the default graph's key.
func graphKey(ctx term.Term) string {
	if store.IsDefaultGraph(ctx) {
		return ""
	}
	return term.TermKey(ctx)
}

// encodeTriple serializes a Triple as three length-prefixed TermKey strings.
func encodeTriple(t term.Triple) []byte {
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)
	size := 4 + len(sk) + 4 + len(pk) + 4 + len(ok)
	buf := make([]byte, size)
	n := 0
	binary.LittleEndian.PutUint32(buf[n:], uint32(len(sk)))
	n += 4
	copy(buf[n:], sk)
	n += len(sk)
	binary.LittleEndian.PutUint32(buf[n:], uint32(len(pk)))
	n += 4
	copy(buf[n:], pk)
	n += len(pk)
	binary.LittleEndian.PutUint32(buf[n:], uint32(len(ok)))
	n += 4
	copy(buf[n:], ok)
	return buf
}

// decodeTriple deserializes a Triple from the format produced by encodeTriple.
func decodeTriple(data []byte) (term.Triple, error) {
	if len(data) < 12 {
		return term.Triple{}, fmt.Errorf("badgerstore: encoded triple too short")
	}
	n := 0

	sLen := int(binary.LittleEndian.Uint32(data[n:]))
	n += 4
	if n+sLen > len(data) {
		return term.Triple{}, fmt.Errorf("badgerstore: truncated subject")
	}
	sk := string(data[n : n+sLen])
	n += sLen

	if n+4 > len(data) {
		return term.Triple{}, fmt.Errorf("badgerstore: truncated predicate length")
	}
	pLen := int(binary.LittleEndian.Uint32(data[n:]))
	n += 4
	if n+pLen > len(data) {
		return term.Triple{}, fmt.Errorf("badgerstore: truncated predicate")
	}
	pk := string(data[n : n+pLen])
	n += pLen

	if n+4 > len(data) {
		return term.Triple{}, fmt.Errorf("badgerstore: truncated object length")
	}
	oLen := int(binary.LittleEndian.Uint32(data[n:]))
	n += 4
	if n+oLen > len(data) {
		return term.Triple{}, fmt.Errorf("badgerstore: truncated object")
	}
	okStr := string(data[n : n+oLen])

	s, err := term.TermFromKey(sk)
	if err != nil {
		return term.Triple{}, fmt.Errorf("badgerstore: decode subject: %w", err)
	}
	p, err := term.TermFromKey(pk)
	if err != nil {
		return term.Triple{}, fmt.Errorf("badgerstore: decode predicate: %w", err)
	}
	o, err := term.TermFromKey(okStr)
	if err != nil {
		return term.Triple{}, fmt.Errorf("badgerstore: decode object: %w", err)
	}

	subj, ok := s.(term.Subject)
	if !ok {
		return term.Triple{}, fmt.Errorf("badgerstore: subject is not Subject type")
	}
	pred, ok := p.(term.URIRef)
	if !ok {
		return term.Triple{}, fmt.Errorf("badgerstore: predicate is not URIRef")
	}

	return term.Triple{Subject: subj, Predicate: pred, Object: o}, nil
}
