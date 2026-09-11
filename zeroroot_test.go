package apd

import "testing"

// TestRootSpecialsZeroExponent checks the exponent and conditions of a
// square root or cube root of zero. TestGDA compares finite results with
// Cmp, which considers every zero equal regardless of exponent, so it
// cannot check these cases even though testdata carries them.
func TestRootSpecialsZeroExponent(t *testing.T) {
	tests := []struct {
		id   string
		prec uint32
		emax int32
		emin int32
		cbrt bool
		x    string
		exp  int32
		neg  bool
		cond Condition
	}{
		// testdata/squareroot.decTest
		{id: "sqtx006", prec: 9, emax: 384, emin: -383, x: "00.0", exp: -1},
		{id: "sqtx009", prec: 9, emax: 384, emin: -383, x: "00.000", exp: -2},
		{id: "sqtx017", prec: 9, emax: 384, emin: -383, x: "-0.0", exp: -1, neg: true},
		{id: "sqtx019", prec: 9, emax: 384, emin: -383, x: "-00.000", exp: -2, neg: true},
		{id: "sqtx9010", prec: 4, emax: 9, emin: -9, x: "0E-9", exp: -5},
		{id: "sqtx9012", prec: 4, emax: 9, emin: -9, x: "0E-11", exp: -6},
		{id: "sqtx9014", prec: 4, emax: 9, emin: -9, x: "0E-13", exp: -7},
		{id: "sqtx9023", prec: 4, emax: 9, emin: -9, x: "0E-24", exp: -12},
		{id: "sqtx9024", prec: 4, emax: 9, emin: -9, x: "0E-25", exp: -12, cond: Clamped},
		{id: "sqtx9027", prec: 4, emax: 9, emin: -9, x: "0E-28", exp: -12, cond: Clamped},
		{id: "sqtx9037", prec: 4, emax: 9, emin: -9, x: "0E+19", exp: 9},
		{id: "sqtx9038", prec: 4, emax: 9, emin: -9, x: "0E+20", exp: 9, cond: Clamped},
		{id: "sqtx9040", prec: 4, emax: 9, emin: -9, x: "0E+22", exp: 9, cond: Clamped},
		// testdata/cuberoot-apd.decTest
		{id: "cbtx015", prec: 9, emax: 384, emin: -383, cbrt: true, x: "-0.00", exp: -1, neg: true},
		{id: "cbtx016", prec: 9, emax: 384, emin: -383, cbrt: true, x: "-00.0", exp: -1, neg: true},
		{id: "cbtx017", prec: 9, emax: 384, emin: -383, cbrt: true, x: "-0E+9", exp: 3, neg: true},
		{id: "cbtx020", prec: 9, emax: 384, emin: -383, cbrt: true, x: "-0E+12", exp: 4, neg: true},
	}
	for _, tc := range tests {
		// Parse the operand exactly, as TestGDA does; the context under test
		// applies to the operation, not to the literal.
		x, _, err := NewFromString(tc.x)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		c := &Context{Precision: tc.prec, MaxExponent: tc.emax, MinExponent: tc.emin, Rounding: RoundHalfUp}
		d := new(Decimal)
		var res Condition
		if tc.cbrt {
			res, err = c.Cbrt(d, x)
		} else {
			res, err = c.Sqrt(d, x)
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if !d.IsZero() || d.Exponent != tc.exp || d.Negative != tc.neg {
			t.Errorf("%s: got %#v, expected zero with exponent %d, negative %t", tc.id, d, tc.exp, tc.neg)
		}
		if res != tc.cond {
			t.Errorf("%s: got conditions %q, expected %q", tc.id, res, tc.cond)
		}
	}
}
