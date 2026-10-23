// Copyright 2026 The Cockroach Authors.
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
	"testing"
)

// orderingCtx returns a context configured to match the General Decimal
// Arithmetic specification's default arithmetic testing parameters, so that the
// expected values below line up with the reference implementation.
func orderingCtx() *Context {
	c := BaseContext.WithPrecision(9)
	c.MaxExponent = 384
	c.MinExponent = -383
	c.Rounding = RoundHalfEven
	c.Traps = 0
	return c
}

// canon renders d as a comparable string that keeps operands like 1.0 and 1.00,
// which are numerically equal but held differently, distinct.
func canon(d *Decimal) string {
	switch d.Form {
	case Infinite:
		if d.Negative {
			return "-Inf"
		}
		return "Inf"
	case NaN:
		return "NaN"
	case NaNSignaling:
		return "sNaN"
	}
	sign := ""
	if d.Negative {
		sign = "-"
	}
	return fmt.Sprintf("%s%sE%d", sign, d.Coeff.String(), d.Exponent)
}

func TestMax(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "2", "2E0"},
		{"2", "1", "2E0"},
		// Numerically equal: the larger total ordering wins.
		{"1.0", "1.00", "10E-1"},
		{"1.00", "1.0", "10E-1"},
		{"3.0", "3", "3E0"},
		// Signed zeros: +0 is the greater in the total ordering.
		{"0", "-0", "0E0"},
		{"-0", "0", "0E0"},
		{"-1.0", "-1.00", "-100E-2"},
		{"7", "-7", "7E0"},
		{"-2", "1", "1E0"},
		{"0.5", "-0.5", "5E-1"},
		{"1E+2", "1E-2", "1E2"},
		{"1E-391", "-1E-391", "1E-391"},
		// Infinities.
		{"Infinity", "1", "Inf"},
		{"-Infinity", "1", "1E0"},
		{"Infinity", "-Infinity", "Inf"},
		// A quiet NaN is treated as absent.
		{"NaN", "1", "1E0"},
		{"1", "NaN", "1E0"},
		{"NaN", "NaN", "NaN"},
		// A signaling NaN yields NaN.
		{"sNaN", "1", "NaN"},
		{"1", "sNaN", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.Max(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("Max(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "2", "1E0"},
		{"2", "1", "1E0"},
		{"1.0", "1.00", "100E-2"},
		{"3.0", "3", "30E-1"},
		{"0", "-0", "-0E0"},
		{"-0", "0", "-0E0"},
		{"-1.0", "-1.00", "-10E-1"},
		{"7", "-7", "-7E0"},
		{"-2", "1", "-2E0"},
		{"1E+2", "1E-2", "1E-2"},
		{"1E-391", "-1E-391", "-1E-391"},
		{"Infinity", "1", "1E0"},
		{"-Infinity", "1", "-Inf"},
		{"Infinity", "-Infinity", "-Inf"},
		{"NaN", "1", "1E0"},
		{"1", "NaN", "1E0"},
		{"NaN", "NaN", "NaN"},
		{"sNaN", "1", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.Min(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("Min(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestMaxAbs(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "-2", "-2E0"},
		{"-2", "1", "-2E0"},
		{"1.0", "1.00", "10E-1"},
		{"7", "-7", "7E0"},
		// Equal magnitudes fall back to the signed total ordering.
		{"1E-391", "-1E-391", "1E-391"},
		{"-1.0", "1.00", "100E-2"},
		{"0", "-0", "0E0"},
		{"-0", "0", "0E0"},
		{"Infinity", "1", "Inf"},
		{"-Infinity", "1", "-Inf"},
		{"NaN", "1", "1E0"},
		{"sNaN", "1", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.MaxAbs(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("MaxAbs(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestMinAbs(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "-2", "1E0"},
		{"-2", "1", "1E0"},
		{"1.0", "1.00", "100E-2"},
		{"7", "-7", "-7E0"},
		{"1E-391", "-1E-391", "-1E-391"},
		{"0", "-0", "-0E0"},
		{"-0", "0", "-0E0"},
		{"Infinity", "1", "1E0"},
		{"-Infinity", "1", "1E0"},
		{"Infinity", "-Infinity", "-Inf"},
		{"NaN", "1", "1E0"},
		{"sNaN", "1", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.MinAbs(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("MinAbs(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestNextPlus(t *testing.T) {
	tests := []struct {
		x, expected string
	}{
		{"0", "1E-391"},
		{"-0", "1E-391"},
		{"1", "100000001E-8"},
		{"-1", "-999999999E-9"},
		{"1.0", "100000001E-8"},
		{"0.999999999", "100000000E-8"},
		{"9.99999999E+384", "Inf"},
		{"-9.99999999E+384", "-999999998E376"},
		{"Infinity", "Inf"},
		{"-Infinity", "-999999999E376"},
		{"1E-391", "2E-391"},
		{"-1E-391", "-0E-391"},
		{"NaN", "NaN"},
		{"sNaN", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.NextPlus(d, newDecimal(t, c, tc.x)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("NextPlus(%s) = %s, want %s", tc.x, got, tc.expected)
			}
		})
	}
}

func TestNextMinus(t *testing.T) {
	tests := []struct {
		x, expected string
	}{
		{"0", "-1E-391"},
		{"-0", "-1E-391"},
		{"1", "999999999E-9"},
		{"-1", "-100000001E-8"},
		{"0.999999999", "999999998E-9"},
		{"9.99999999E+384", "999999998E376"},
		{"-9.99999999E+384", "-Inf"},
		{"Infinity", "999999999E376"},
		{"-Infinity", "-Inf"},
		{"1E-391", "0E-391"},
		{"-1E-391", "-2E-391"},
		{"NaN", "NaN"},
		{"sNaN", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.NextMinus(d, newDecimal(t, c, tc.x)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("NextMinus(%s) = %s, want %s", tc.x, got, tc.expected)
			}
		})
	}
}

func TestNextToward(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "2", "100000001E-8"},
		{"1", "0", "999999999E-9"},
		// Equal operands: x keeps its value, taking the sign of y.
		{"1", "1", "1E0"},
		{"1.0", "1", "10E-1"},
		{"-1", "-2", "-100000001E-8"},
		{"0", "1", "1E-391"},
		{"0", "-1", "-1E-391"},
		{"1", "-0", "999999999E-9"},
		{"-1E-391", "0", "-0E-391"},
		{"1E-391", "0", "0E-391"},
		{"Infinity", "1", "999999999E376"},
		{"-Infinity", "1", "-999999999E376"},
		{"NaN", "1", "NaN"},
		{"1", "sNaN", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.NextToward(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("NextToward(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestLogB(t *testing.T) {
	tests := []struct {
		x, expected string
	}{
		{"0", "-Inf"},
		{"-0", "-Inf"},
		{"1", "0E0"},
		{"-1", "0E0"},
		{"250", "2E0"},
		{"0.03", "-2E0"},
		{"1.0", "0E0"},
		{"-2.5", "0E0"},
		{"9.99999999E+384", "384E0"},
		{"1E-391", "-391E0"},
		{"Infinity", "Inf"},
		{"-Infinity", "Inf"},
		{"NaN", "NaN"},
		{"sNaN", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.LogB(d, newDecimal(t, c, tc.x)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("LogB(%s) = %s, want %s", tc.x, got, tc.expected)
			}
		})
	}
}

func TestScaleB(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "0", "1E0"},
		{"1", "1", "1E1"},
		{"1", "-1", "1E-1"},
		{"250", "2", "250E2"},
		{"0.03", "2", "3E0"},
		{"1.00", "-2", "100E-4"},
		{"-2.5", "1", "-25E0"},
		{"0", "5", "0E5"},
		{"1E-391", "5", "1E-386"},
		// Non-integer or out-of-range y is invalid.
		{"1", "1.5", "NaN"},
		{"1", "800000", "NaN"},
		{"1", "-800000", "NaN"},
		{"Infinity", "2", "Inf"},
		{"NaN", "1", "NaN"},
		{"sNaN", "1", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.ScaleB(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("ScaleB(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestCopySign(t *testing.T) {
	tests := []struct {
		x, y, expected string
	}{
		{"1", "-1", "-1E0"},
		{"-1", "1", "1E0"},
		{"1.0", "-0", "-10E-1"},
		{"-2.5", "1", "25E-1"},
		{"0", "-0", "-0E0"},
		{"-0", "0", "0E0"},
		{"250", "-2.5", "-250E0"},
		{"Infinity", "-1", "-Inf"},
		{"NaN", "-1", "NaN"},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			if _, err := c.CopySign(d, newDecimal(t, c, tc.x), newDecimal(t, c, tc.y)); err != nil {
				t.Fatal(err)
			}
			if got := canon(d); got != tc.expected {
				t.Errorf("CopySign(%s, %s) = %s, want %s", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestSameQuantum(t *testing.T) {
	tests := []struct {
		x, y     string
		expected bool
	}{
		{"1", "2", true},
		{"1.0", "1.00", false},
		{"1.0", "1.0", true},
		{"0", "-0", true},
		{"1E+2", "1E-2", false},
		{"Infinity", "-Infinity", true},
		{"NaN", "sNaN", true},
		{"NaN", "1", false},
		{"Infinity", "1", false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
			if got := x.SameQuantum(y); got != tc.expected {
				t.Errorf("SameQuantum(%s, %s) = %v, want %v", tc.x, tc.y, got, tc.expected)
			}
		})
	}
}

func TestCmpTotalMag(t *testing.T) {
	tests := []struct {
		x, y     string
		expected int
	}{
		{"1", "2", -1},
		{"1.0", "1.00", 1},
		{"1.00", "1.0", -1},
		{"-1.0", "1.00", 1},
		{"0", "-0", 0},
		{"7", "-7", 0},
		{"-2", "1", 1},
		{"Infinity", "-Infinity", 0},
		{"NaN", "1", 1},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
			// Ensure the operands are not mutated.
			xb, yb := canon(x), canon(y)
			if got := x.CmpTotalMag(y); got != tc.expected {
				t.Errorf("CmpTotalMag(%s, %s) = %d, want %d", tc.x, tc.y, got, tc.expected)
			}
			if canon(x) != xb || canon(y) != yb {
				t.Errorf("CmpTotalMag mutated an operand: %s,%s -> %s,%s", xb, yb, canon(x), canon(y))
			}
		})
	}
}

// TestOrderingConditions checks the conditions (flags) that the ordering and
// adjacency operations raise, independent of their numeric result.
func TestOrderingConditions(t *testing.T) {
	const signals = InvalidOperation | DivisionByZero
	tests := []struct {
		op, x, y string
		expected Condition
	}{
		{"Max", "sNaN", "1", InvalidOperation},
		{"Max", "1", "sNaN", InvalidOperation},
		{"Max", "NaN", "1", 0},
		{"Max", "1", "2", 0},
		{"Min", "sNaN", "1", InvalidOperation},
		{"Min", "1", "NaN", 0},
		{"MaxAbs", "1", "sNaN", InvalidOperation},
		{"MinAbs", "sNaN", "1", InvalidOperation},
		{"NextPlus", "sNaN", "", InvalidOperation},
		{"NextPlus", "NaN", "", 0},
		{"NextPlus", "1", "", 0},
		{"NextMinus", "sNaN", "", InvalidOperation},
		{"NextToward", "sNaN", "1", InvalidOperation},
		{"NextToward", "1", "sNaN", InvalidOperation},
		{"NextToward", "1", "2", 0},
		{"LogB", "0", "", DivisionByZero},
		{"LogB", "-0", "", DivisionByZero},
		{"LogB", "sNaN", "", InvalidOperation},
		{"LogB", "250", "", 0},
		{"LogB", "Infinity", "", 0},
		{"ScaleB", "1", "1.5", InvalidOperation},
		{"ScaleB", "1", "800000", InvalidOperation},
		{"ScaleB", "1", "-800000", InvalidOperation},
		{"ScaleB", "sNaN", "1", InvalidOperation},
		{"ScaleB", "1", "NaN", 0},
		{"ScaleB", "1", "2", 0},
	}
	c := orderingCtx()
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s,%s,%s", tc.op, tc.x, tc.y), func(t *testing.T) {
			d := new(Decimal)
			x := newDecimal(t, c, tc.x)
			var res Condition
			var err error
			switch tc.op {
			case "Max":
				res, err = c.Max(d, x, newDecimal(t, c, tc.y))
			case "Min":
				res, err = c.Min(d, x, newDecimal(t, c, tc.y))
			case "MaxAbs":
				res, err = c.MaxAbs(d, x, newDecimal(t, c, tc.y))
			case "MinAbs":
				res, err = c.MinAbs(d, x, newDecimal(t, c, tc.y))
			case "NextPlus":
				res, err = c.NextPlus(d, x)
			case "NextMinus":
				res, err = c.NextMinus(d, x)
			case "NextToward":
				res, err = c.NextToward(d, x, newDecimal(t, c, tc.y))
			case "LogB":
				res, err = c.LogB(d, x)
			case "ScaleB":
				res, err = c.ScaleB(d, x, newDecimal(t, c, tc.y))
			default:
				t.Fatalf("unknown op %s", tc.op)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := res & signals; got != tc.expected {
				t.Errorf("%s(%s, %s) raised %v, want %v", tc.op, tc.x, tc.y, got, tc.expected)
			}
		})
	}
}
