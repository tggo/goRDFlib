package sparql

import (
	"math/big"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// FuzzScaledMatchesRat holds the int64 fast path for decimal + - * to the
// exact big.Rat result: same canonical lexical form, or a decline.
func FuzzScaledMatchesRat(f *testing.F) {
	for _, s := range [][2]string{
		{"1.5", "0.25"}, {"-0.5", "2"}, {"99999999999999999", "1"}, {"0.1", "0.2"},
		{"12345678.901", "-98765.4321"}, {"0", "-0.000"}, {"+7.", "3"}, {"-0.00000000000000001", "100"},
	} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, x, y string) {
		if !isDecimalLexical(x) || !isDecimalLexical(y) {
			return
		}
		a, ok1 := numericOf(rdflibgo.NewLiteral(x, rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
		b, ok2 := numericOf(rdflibgo.NewLiteral(y, rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
		if !ok1 || !ok2 {
			return
		}
		for _, op := range []string{"+", "-", "*"} {
			got, ok := scaledArithmetic(op, a, b)
			if !ok {
				continue
			}
			r := new(big.Rat)
			switch op {
			case "+":
				r.Add(a.exact(), b.exact())
			case "-":
				r.Sub(a.exact(), b.exact())
			case "*":
				r.Mul(a.exact(), b.exact())
			}
			if want := formatRat(r); got != want {
				t.Fatalf("%s %s %s: fast path %q, exact %q", x, op, y, got, want)
			}
		}
	})
}

// FuzzFiniteDigits holds finiteDigits (bit length + one exponentiation) to
// the plain loop that divides out 2s and 5s.
func FuzzFiniteDigits(f *testing.F) {
	for _, s := range [][3]uint16{{0, 0, 1}, {3, 0, 1}, {0, 7, 1}, {5, 9, 1}, {2, 2, 3}, {0, 0, 7}, {60, 200, 1}} {
		f.Add(s[0], s[1], s[2])
	}
	f.Fuzz(func(t *testing.T, a, b, k uint16) {
		a, b = a%300, b%300
		if k == 0 {
			k = 1
		}
		d := new(big.Int).Lsh(big.NewInt(1), uint(a))
		d.Mul(d, new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(b)), nil))
		d.Mul(d, big.NewInt(int64(k)))
		want := -1
		n, twos, fives := new(big.Int).Set(d), 0, 0
		for n.Bit(0) == 0 {
			n.Rsh(n, 1)
			twos++
		}
		five, m := big.NewInt(5), new(big.Int)
		for {
			q, r := new(big.Int).QuoRem(n, five, m)
			if r.Sign() != 0 {
				break
			}
			n = q
			fives++
		}
		if n.Cmp(big.NewInt(1)) == 0 {
			want = max(twos, fives)
		}
		if got := finiteDigits(d); got != want {
			t.Fatalf("2^%d·5^%d·%d: %d, want %d", a, b, k, got, want)
		}
	})
}
