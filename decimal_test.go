package apd

import (
	"fmt"
	"math/big"
	"testing"
)

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

func (d *Decimal) GoString() string {
	return fmt.Sprintf(`{Coeff: %s, Exponent: %d}`, d.Coeff.String(), d.Exponent)
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
			d, err := new(Decimal).Add(x, y)
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
