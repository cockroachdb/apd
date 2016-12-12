package apd

import (
	"fmt"
	"math/big"
	"testing"
)

func (d *Decimal) GoString() string {
	return fmt.Sprintf(`{Coeff: %s, Exponent: %d, MaxExponent: %d, MinExponent: %d, Precision: %d}`, d.Coeff.String(), d.Exponent, d.MaxExponent, d.MinExponent, d.Precision)
}

func TestNewFromString(t *testing.T) {
	tests := []struct {
		s   string
		out string
	}{
		{s: "0"},
		{s: "0.0"},
		{s: "00.0", out: "0.0"},
		{s: "0.00"},
		{s: "00.00", out: "0.00"},
		{s: "1"},
		{s: "1.0"},
		{s: "0.1"},
		{s: ".1", out: "0.1"},
		{s: "01.10", out: "1.10"},
		{s: "123456.789"},
		{s: "-123"},
		{s: "1e1", out: "10"},
		{s: "1e-1", out: "0.1"},
		{s: "0.1e1", out: "1"},
		{s: "0.10e1", out: "1.0"},
		{s: "0.1e-1", out: "0.01"},
		{s: "1e10", out: "10000000000"},
		{s: "1e-10", out: "0.0000000001"},
		{s: "0.1e10", out: "1000000000"},
		{s: "0.1e-10", out: "0.00000000001"},
	}
	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			d := newDecimal(t, tc.s)
			expect := tc.out
			if expect == "" {
				expect = tc.s
			}
			s := d.String()
			if s != expect {
				t.Errorf("expected: %s, got %s", expect, s)
			}
		})
	}
}

func newDecimal(t *testing.T, s string) *Decimal {
	d, err := NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestUpscale(t *testing.T) {
	tests := []struct {
		x, y *Decimal
		a, b *big.Int
		s    int32
	}{
		{x: New(1, 0), y: New(100, -1), a: big.NewInt(10), b: big.NewInt(100), s: -1},
		{x: New(1, 0), y: New(10, -1), a: big.NewInt(10), b: big.NewInt(10), s: -1},
		{x: New(1, 0), y: New(10, 0), a: big.NewInt(1), b: big.NewInt(10), s: 0},
		{x: New(1, 1), y: New(1, 0), a: big.NewInt(10), b: big.NewInt(1), s: 0},
		{x: New(10, -2), y: New(1, -1), a: big.NewInt(10), b: big.NewInt(10), s: -2},
		{x: New(1, -2), y: New(100, 1), a: big.NewInt(1), b: big.NewInt(100000), s: -2},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			a, b, s, err := upscale(tc.x, tc.y)
			if err != nil {
				t.Fatal(err)
			}
			if a.Cmp(tc.a) != 0 {
				t.Errorf("a: expected %s, got %s", tc.a, a)
			}
			if b.Cmp(tc.b) != 0 {
				t.Errorf("b: expected %s, got %s", tc.b, b)
			}
			if s != tc.s {
				t.Errorf("s: expected %d, got %d", tc.s, s)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		x, y string
		r    string
	}{
		{x: "1", y: "10", r: "11"},
		{x: "1", y: "1e1", r: "11"},
		{x: "1e1", y: "1", r: "11"},
		{x: ".1e1", y: "100e-1", r: "11.0"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, tc.x)
			y := newDecimal(t, tc.y)
			d := new(Decimal)
			err := d.Add(x, y)
			if err != nil {
				t.Fatal(err)
			}
			s := d.String()
			if s != tc.r {
				t.Fatalf("expected: %s, got: %s", tc.r, s)
			}
		})
	}
}

func TestCmp(t *testing.T) {
	tests := []struct {
		x, y string
		c    int
	}{
		{x: "1", y: "10", c: -1},
		{x: "1", y: "1e1", c: -1},
		{x: "1e1", y: "1", c: 1},
		{x: ".1e1", y: "100e-1", c: -1},

		{x: ".1e1", y: "100e-2", c: 0},
		{x: "1", y: ".1e1", c: 0},
		{x: "1", y: "1", c: 0},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, tc.x)
			y := newDecimal(t, tc.y)
			c, err := x.Cmp(y)
			if err != nil {
				t.Fatal(err)
			}
			if c != tc.c {
				t.Fatalf("expected: %d, got: %d", tc.c, c)
			}
		})
	}
}

func TestModf(t *testing.T) {
	tests := []struct {
		x string
		i string
		f string
	}{
		{x: "1", i: "1", f: "0"},
		{x: "1.0", i: "1", f: "0.0"},
		{x: "1.0e1", i: "10", f: "0"},
		{x: "1.0e2", i: "100", f: "0"},
		{x: "1.0e-1", i: "0", f: "0.10"},
		{x: "1.0e-2", i: "0", f: "0.010"},
		{x: "1234.56", i: "1234", f: "0.56"},
		{x: "1234.56e2", i: "123456", f: "0"},
		{x: "1234.56e4", i: "12345600", f: "0"},
		{x: "1234.56e-2", i: "12", f: "0.3456"},
		{x: "1234.56e-4", i: "0", f: "0.123456"},
		{x: "1234.56e-6", i: "0", f: "0.00123456"},
		{x: "123456e-8", i: "0", f: "0.00123456"},
		{x: ".123456e8", i: "12345600", f: "0"},

		{x: "-1", i: "-1", f: "0"},
		{x: "-1.0", i: "-1", f: "0.0"},
		{x: "-1.0e1", i: "-10", f: "0"},
		{x: "-1.0e2", i: "-100", f: "0"},
		{x: "-1.0e-1", i: "0", f: "-0.10"},
		{x: "-1.0e-2", i: "0", f: "-0.010"},
		{x: "-1234.56", i: "-1234", f: "-0.56"},
		{x: "-1234.56e2", i: "-123456", f: "0"},
		{x: "-1234.56e4", i: "-12345600", f: "0"},
		{x: "-1234.56e-2", i: "-12", f: "-0.3456"},
		{x: "-1234.56e-4", i: "0", f: "-0.123456"},
		{x: "-1234.56e-6", i: "0", f: "-0.00123456"},
		{x: "-123456e-8", i: "0", f: "-0.00123456"},
		{x: "-.123456e8", i: "-12345600", f: "0"},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			x := newDecimal(t, tc.x)
			integ, frac := new(Decimal), new(Decimal)
			x.Modf(integ, frac)
			if tc.i != integ.String() {
				t.Fatalf("integ: expected: %s, got: %s", tc.i, integ)
			}
			if tc.f != frac.String() {
				t.Fatalf("frac: expected: %s, got: %s", tc.f, frac)
			}
			a := new(Decimal)
			if err := a.Add(integ, frac); err != nil {
				t.Fatal(err)
			}
			if c, err := a.Cmp(x); err != nil {
				t.Fatal(err)
			} else if c != 0 {
				t.Fatalf("%s != %s", a, x)
			}
			if integ.Exponent < 0 {
				t.Fatal(integ.Exponent)
			}
			if frac.Exponent > 0 {
				t.Fatal(frac.Exponent)
			}
		})
	}
}
