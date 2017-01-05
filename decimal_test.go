// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package apd

import (
	"fmt"
	"math"
	"math/big"
	"testing"
)

var (
	testCtx = &BaseContext
)

func (d *Decimal) GoString() string {
	return fmt.Sprintf(`{Coeff: %s, Exponent: %d}`, d.Coeff.String(), d.Exponent)
}

func testExponentError(t *testing.T, err error) {
	if err == nil {
		return
	}
	if err.Error() == errExponentOutOfRange {
		t.Skip(err)
	}
}

func newDecimal(t *testing.T, c *Context, s string) *Decimal {
	d, _, err := c.NewFromString(s)
	testExponentError(t, err)
	if err != nil {
		t.Fatalf("%s: %+v", s, err)
	}
	return d
}

func TestNewFromStringContext(t *testing.T) {
	c5 := Context{
		Precision:   3,
		MaxExponent: 10,
		MinExponent: -5,
		Rounding:    RoundHalfUp,
	}
	c5r := c5
	c5r.Rounding = RoundCeiling
	tests := []struct {
		s string
		c Context
		r string
	}{
		{s: "1e-10", c: c5, r: "0"},
		{s: "12e-10", c: c5, r: "0"},
		{s: "123e-10", c: c5, r: "0"},
		{s: "1234e-10", c: c5, r: "1E-7"},
		{s: "1e-10", c: c5r, r: "1E-7"},
		{s: "12e-10", c: c5r, r: "1E-7"},
		{s: "123e-10", c: c5r, r: "1E-7"},
		{s: "1234e-10", c: c5r, r: "2E-7"},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d:%s", i, tc.s), func(t *testing.T) {
			d, _, _ := tc.c.NewFromString(tc.s)
			r := d.String()
			if r != tc.r {
				t.Fatalf("expected %s, got %s", tc.r, r)
			}
		})
	}
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
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
			d := new(Decimal)
			_, err := testCtx.Add(d, x, y)
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
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
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
		{x: "1.0", i: "1", f: "0"},
		{x: "1.0e1", i: "10", f: "0"},
		{x: "1.0e2", i: "1.0E+2", f: "0"},
		{x: "1.0e-1", i: "0", f: "0.10"},
		{x: "1.0e-2", i: "0", f: "0.010"},
		{x: "1234.56", i: "1234", f: "0.56"},
		{x: "1234.56e2", i: "123456", f: "0"},
		{x: "1234.56e4", i: "1.23456E+7", f: "0"},
		{x: "1234.56e-2", i: "12", f: "0.3456"},
		{x: "1234.56e-4", i: "0", f: "0.123456"},
		{x: "1234.56e-6", i: "0", f: "0.00123456"},
		{x: "123456e-8", i: "0", f: "0.00123456"},
		{x: ".123456e8", i: "1.23456E+7", f: "0"},

		{x: "-1", i: "-1", f: "0"},
		{x: "-1.0", i: "-1", f: "0"},
		{x: "-1.0e1", i: "-10", f: "0"},
		{x: "-1.0e2", i: "-1.0E+2", f: "0"},
		{x: "-1.0e-1", i: "0", f: "-0.10"},
		{x: "-1.0e-2", i: "0", f: "-0.010"},
		{x: "-1234.56", i: "-1234", f: "-0.56"},
		{x: "-1234.56e2", i: "-123456", f: "0"},
		{x: "-1234.56e4", i: "-1.23456E+7", f: "0"},
		{x: "-1234.56e-2", i: "-12", f: "-0.3456"},
		{x: "-1234.56e-4", i: "0", f: "-0.123456"},
		{x: "-1234.56e-6", i: "0", f: "-0.00123456"},
		{x: "-123456e-8", i: "0", f: "-0.00123456"},
		{x: "-.123456e8", i: "-1.23456E+7", f: "0"},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			integ, frac := new(Decimal), new(Decimal)
			x.Modf(integ, frac)
			if tc.i != integ.String() {
				t.Fatalf("integ: expected: %s, got: %s", tc.i, integ)
			}
			if tc.f != frac.String() {
				t.Fatalf("frac: expected: %s, got: %s", tc.f, frac)
			}
			a := new(Decimal)
			if _, err := testCtx.Add(a, integ, frac); err != nil {
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

func TestInt64(t *testing.T) {
	tests := []struct {
		x   string
		i   int64
		err bool
	}{
		{x: "0.12e1", err: true},
		{x: "0.1e1", i: 1},
		{x: "10", i: 10},
		{x: "12.3e3", i: 12300},
		{x: "1e-1", err: true},
		{x: "1e2", i: 100},
		{x: "1", i: 1},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			i, err := x.Int64()
			hasErr := err != nil
			if tc.err != hasErr {
				t.Fatalf("expected error: %v, got error: %v", tc.err, err)
			}
			if hasErr {
				return
			}
			if i != tc.i {
				t.Fatalf("expected: %v, got %v", tc.i, i)
			}
		})
	}
}

func TestQuoErr(t *testing.T) {
	tests := []struct {
		x, y string
		p    uint32
		err  string
	}{
		{x: "1", y: "1", p: 0, err: "Quo requires a Context with > 0 Precision"},
		{x: "1", y: "0", p: 1, err: "division by zero"},
	}
	for _, tc := range tests {
		c := testCtx.WithPrecision(tc.p)
		x := newDecimal(t, testCtx, tc.x)
		y := newDecimal(t, testCtx, tc.y)
		d := new(Decimal)
		_, err := c.Quo(d, x, y)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != tc.err {
			t.Fatalf("expected %s, got %s", tc.err, err)
		}
	}
}

func TestConditionString(t *testing.T) {
	tests := map[Condition]string{
		Overflow:             "overflow",
		Overflow | Underflow: "overflow, underflow",
		Subnormal | Inexact:  "inexact, subnormal",
	}
	for c, s := range tests {
		t.Run(s, func(t *testing.T) {
			cs := c.String()
			if cs != s {
				t.Errorf("expected %s; got %s", s, cs)
			}
		})
	}
}

func TestFloat64(t *testing.T) {
	tests := []float64{
		0,
		1,
		-1,
		math.MaxFloat32,
		math.SmallestNonzeroFloat32,
		math.MaxFloat64,
		math.SmallestNonzeroFloat64,
	}

	for _, tc := range tests {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			d := new(Decimal)
			d.SetFloat64(tc)
			f, err := d.Float64()
			if err != nil {
				t.Fatal(err)
			}
			if tc != f {
				t.Fatalf("expected %v, got %v", tc, f)
			}
		})
	}
}

func TestCeil(t *testing.T) {
	tests := map[float64]int64{
		0:    0,
		-0.1: 0,
		0.1:  1,
		-0.9: 0,
		0.9:  1,
		-1:   -1,
		1:    1,
		-1.1: -1,
		1.1:  2,
	}

	for f, r := range tests {
		t.Run(fmt.Sprint(f), func(t *testing.T) {
			d, err := new(Decimal).SetFloat64(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = testCtx.Ceil(d, d)
			if err != nil {
				t.Fatal(err)
			}
			i, err := d.Int64()
			if err != nil {
				t.Fatal(err)
			}
			if i != r {
				t.Fatalf("got %v, expected %v", i, r)
			}
		})
	}
}

func TestFloor(t *testing.T) {
	tests := map[float64]int64{
		0:    0,
		-0.1: -1,
		0.1:  0,
		-0.9: -1,
		0.9:  0,
		-1:   -1,
		1:    1,
		-1.1: -2,
		1.1:  1,
	}

	for f, r := range tests {
		t.Run(fmt.Sprint(f), func(t *testing.T) {
			d, err := new(Decimal).SetFloat64(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = testCtx.Floor(d, d)
			if err != nil {
				t.Fatal(err)
			}
			i, err := d.Int64()
			if err != nil {
				t.Fatal(err)
			}
			if i != r {
				t.Fatalf("got %v, expected %v", i, r)
			}
		})
	}
}
