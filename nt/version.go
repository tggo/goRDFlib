package nt

import "github.com/tggo/goRDFlib/internal/ntsyntax"

// ErrBadVersion: a line starts with VERSION but is not `VERSION "1.2"` (upper
// case keyword, one double-quoted string, no trailing '.'). The same value
// nq returns, so errors.Is works with either package.
var ErrBadVersion = ntsyntax.ErrBadVersion
