package term

// TermKey returns a stable string key for a term.
// It uses a type-prefixed representation that avoids the allocation overhead of
// N3() (which wraps URIRefs in angle brackets on every call).
func TermKey(t Term) string {
	switch v := t.(type) {
	case URIRef:
		return "U:" + v.Value()
	case BNode:
		return "B:" + v.Value()
	case Literal:
		return "L:" + v.N3()
	case Variable:
		return "V:" + v.Name
	default:
		return t.N3()
	}
}

// OptTermKey returns TermKey(t) or "" if t is nil.
func OptTermKey(t Term) string {
	if t == nil {
		return ""
	}
	return TermKey(t)
}

// OptPredKey returns TermKey(*p) or "" if p is nil.
func OptPredKey(p *URIRef) string {
	if p == nil {
		return ""
	}
	return TermKey(*p)
}
