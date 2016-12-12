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
	"testing"
)

var rounders = map[string]Rounder{
	"down":      RoundDown,
	"half_up":   RoundHalfUp,
	"half_even": RoundHalfEven,
}

func TestRound(t *testing.T) {

	tests := map[string][]struct {
		x string
		p uint32
		r string
	}{
		"down": {
			{x: "12", p: 0, r: "12"},
			{x: "12", p: 1, r: "10"},
			{x: "12", p: 2, r: "12"},
			{x: "12", p: 3, r: "12"},
			{x: "1234.5678e10", p: 5, r: "12345000000000"},
		},
		"half_up": {
			{x: "14", p: 1, r: "10"},
			{x: "15", p: 1, r: "20"},
			{x: "16", p: 1, r: "20"},
			{x: "149", p: 1, r: "100"},
			{x: "150", p: 1, r: "200"},
			{x: "151", p: 1, r: "200"},
			{x: "149", p: 2, r: "150"},
			{x: "150", p: 2, r: "150"},
			{x: "151", p: 2, r: "150"},
			{x: "154", p: 2, r: "150"},
			{x: "155", p: 2, r: "160"},
			{x: "156", p: 2, r: "160"},
		},
		"half_even": {
			{x: "14", p: 1, r: "10"},
			{x: "15", p: 1, r: "20"},
			{x: "16", p: 1, r: "20"},
			{x: "24", p: 1, r: "20"},
			{x: "25", p: 1, r: "20"},
			{x: "26", p: 1, r: "30"},
			{x: "149", p: 1, r: "100"},
			{x: "150", p: 1, r: "200"},
			{x: "151", p: 1, r: "200"},
			{x: "149", p: 2, r: "150"},
			{x: "150", p: 2, r: "150"},
			{x: "151", p: 2, r: "150"},
			{x: "154", p: 2, r: "150"},
			{x: "155", p: 2, r: "160"},
			{x: "156", p: 2, r: "160"},
		},
	}
	for rname, tcs := range tests {
		rounder := rounders[rname]
		if rounder == nil {
			t.Fatal(rname)
		}
		t.Run(rname, func(t *testing.T) {
			for _, tc := range tcs {
				t.Run(fmt.Sprintf("%s, %d", tc.x, tc.p), func(t *testing.T) {
					x := newDecimal(t, tc.x)
					d := new(Decimal)
					d.Precision = tc.p
					d.Rounding = rounder
					err := d.Round(x)
					if err != nil {
						t.Fatal(err)
					}
					r := d.String()
					if r != tc.r {
						t.Fatalf("expected %s, got %s", tc.r, r)
					}
				})
			}
		})
	}
}
