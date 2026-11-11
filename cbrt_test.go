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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apd

import (
	"fmt"
	"math/big"
	"testing"
)

func TestCbrtRounding(t *testing.T) {
	for _, tc := range []struct {
		mode               Rounder
		positive, negative string
	}{
		{RoundDown, "1.25", "-1.25"},
		{RoundUp, "1.26", "-1.26"},
		{RoundFloor, "1.25", "-1.26"},
		{RoundCeiling, "1.26", "-1.25"},
		{RoundHalfUp, "1.26", "-1.26"},
		{RoundHalfDown, "1.26", "-1.26"},
		{RoundHalfEven, "1.26", "-1.26"},
		{Round05Up, "1.26", "-1.26"},
	} {
		for _, negative := range []bool{false, true} {
			for _, alias := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/negative=%t/alias=%t", tc.mode, negative, alias), func(t *testing.T) {
					c := BaseContext.WithPrecision(3)
					c.Rounding = tc.mode
					x := New(2, 0)
					x.Negative = negative
					d := new(Decimal)
					if alias {
						d = x
					}
					flags, err := c.Cbrt(d, x)
					want := tc.positive
					if negative {
						want = tc.negative
					}
					if err != nil || flags != Inexact|Rounded || d.Cmp(newDecimal(t, testCtx, want)) != 0 {
						t.Fatalf("got %s, %s, %v; want %s, Inexact Rounded", d, flags, err, want)
					}
					c.Traps = Inexact
					x.SetFinite(2, 0)
					x.Negative = negative
					if flags, err = c.Cbrt(d, x); err == nil || flags != Inexact|Rounded {
						t.Fatalf("got %s, %v; expected trapped Inexact Rounded", flags, err)
					}
				})
			}
		}
	}
}

func TestCbrtExactRoundingBoundaries(t *testing.T) {
	for _, input := range []string{"8", "-8", "0.008", "-0.008", "125", "-125", "8000000", "-8000000"} {
		for mode := range roundings {
			for _, alias := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/alias=%t", input, mode, alias), func(t *testing.T) {
					c := BaseContext.WithPrecision(3)
					c.Rounding = mode
					c.Traps = Inexact | Rounded
					x := newDecimal(t, testCtx, input)
					d := new(Decimal)
					if alias {
						d = x
					}
					flags, err := c.Cbrt(d, x)
					if err != nil || flags != 0 {
						t.Fatalf("got %s, %s, %v; expected exact root", d, flags, err)
					}
					r, ok := new(big.Rat).SetString(d.String())
					if !ok {
						t.Fatal(d)
					}
					cube := new(big.Rat).Mul(new(big.Rat).Mul(r, r), r)
					want, _ := new(big.Rat).SetString(input)
					if cube.Cmp(want) != 0 {
						t.Fatalf("%s cubed is %s; want %s", d, cube, want)
					}
				})
			}
		}
	}
	for _, tc := range []struct {
		input          string
		up, down, even string
	}{
		{"1.976656374999", "1.25", "1.25", "1.25"},
		{"1.976656375", "1.26", "1.25", "1.26"},
		{"1.976656375001", "1.26", "1.26", "1.26"},
		{"1.929781125", "1.25", "1.24", "1.24"},
	} {
		for _, mode := range []Rounder{RoundHalfUp, RoundHalfDown, RoundHalfEven} {
			for _, negative := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/negative=%t", tc.input, mode, negative), func(t *testing.T) {
					want := tc.up
					if mode == RoundHalfDown {
						want = tc.down
					}
					if mode == RoundHalfEven {
						want = tc.even
					}
					input := tc.input
					if negative {
						input = "-" + input
						want = "-" + want
					}
					c := BaseContext.WithPrecision(3)
					c.Rounding = mode
					var d Decimal
					flags, err := c.Cbrt(&d, newDecimal(t, testCtx, input))
					if err != nil || flags != Inexact|Rounded || d.Cmp(newDecimal(t, testCtx, want)) != 0 {
						t.Fatalf("got %s, %s, %v; want %s, Inexact Rounded", &d, flags, err, want)
					}
				})
			}
		}
	}
}
