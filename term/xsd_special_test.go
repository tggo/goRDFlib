package term

import (
	"math"
	"testing"
)

// XSD 1.1 §3.3.4 / §3.3.5: the special values are spelled INF, -INF and NaN.
// strconv's "+Inf" is not in the lexical space, so a Go infinity used to
// become an ill-typed literal.
func TestNewLiteralFloatSpecialValues(t *testing.T) {
	tests := []struct {
		in   any
		want string
		dt   URIRef
	}{
		{math.Inf(1), "INF", XSDDouble},
		{math.Inf(-1), "-INF", XSDDouble},
		{math.NaN(), "NaN", XSDDouble},
		{float32(math.Inf(1)), "INF", XSDFloat},
		{float32(math.Inf(-1)), "-INF", XSDFloat},
		{float32(math.NaN()), "NaN", XSDFloat},
		{1e21, "1e+21", XSDDouble},
	}
	for _, tt := range tests {
		l := NewLiteral(tt.in)
		if l.Lexical() != tt.want || l.Datatype() != tt.dt {
			t.Errorf("NewLiteral(%v) = %q^^%s, want %q^^%s", tt.in, l.Lexical(), l.Datatype().Value(), tt.want, tt.dt.Value())
		}
	}
}
