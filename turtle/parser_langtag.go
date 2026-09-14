package turtle

// isValidLangTag reports whether tag is a well-formed language tag in the form
// the N-Triples parser accepts (internal/ntsyntax) and term.WithLang stores: a
// primary subtag of 1-8 letters followed by any number of "-"-separated 1-8
// character alphanumeric subtags (BCP 47's basic shape, RFC 5646 §2.1).
//
// RDF 1.2 Turtle §7.2 requires a well-formed tag. The parser has to check it
// itself: term.WithLang silently ignores a tag it considers invalid, which
// turned "x"@abcdefghi into the plain literal "x".
func isValidLangTag(tag string) bool {
	if tag == "" {
		return false
	}
	start := 0
	for i := 0; i <= len(tag); i++ {
		if i < len(tag) && tag[i] != '-' {
			continue
		}
		part := tag[start:i]
		if len(part) == 0 || len(part) > 8 {
			return false
		}
		for j := 0; j < len(part); j++ {
			ch := part[j]
			letter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
			if !letter && (start == 0 || ch < '0' || ch > '9') {
				return false
			}
		}
		start = i + 1
	}
	return true
}
