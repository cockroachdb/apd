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
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"testing"
)

func TestDecomposerRoundTrip(t *testing.T) {
	list := []struct {
		N string // Name.
		S string // String value.
		E bool   // Expect an error.
	}{
		{N: "Zero", S: "0"},
		{N: "Normal-1", S: "123.456"},
		{N: "Normal-2", S: "-123.456"},
		{N: "NaN-1", S: "NaN"},
		{N: "NaN-2", S: "-NaN"},
		{N: "Infinity-1", S: "Infinity"},
		{N: "Infinity-2", S: "-Infinity"},
	}
	for _, item := range list {
		t.Run(item.N, func(t *testing.T) {
			d, _, err := NewFromString(item.S)
			if err != nil {
				t.Fatal(err)
			}
			set, set2 := &Decimal{}, &Decimal{}
			f, n, c, e := d.Decompose(nil)
			err = set.Compose(f, n, c, e)
			if err == nil && item.E {
				t.Fatal("expected error, got <nil>")
			}
			err = set2.Compose(f, n, c, e)
			if err == nil && item.E {
				t.Fatal("expected error, got <nil>")
			}
			if set.Cmp(set2) != 0 {
				t.Fatalf("composing the same value twice resulted in different values. set=%v set2=%v", set, set2)
			}
			if err != nil && !item.E {
				t.Fatalf("unexpected error: %v", err)
			}
			if set.Cmp(d) != 0 {
				t.Fatalf("values incorrect, got %v want %v (%s)", set, d, item.S)
			}
		})
	}
}

func TestDecomposerCompose(t *testing.T) {
	list := []struct {
		N string // Name.
		S string // String value.

		Form byte // Form
		Neg  bool
		Coef []byte // Coefficent
		Exp  int32

		Err bool // Expect an error.
	}{
		{N: "Zero", S: "0", Coef: nil, Exp: 0},
		{N: "Normal-1", S: "123.456", Coef: []byte{0x01, 0xE2, 0x40}, Exp: -3},
		{N: "Neg-1", S: "-123.456", Neg: true, Coef: []byte{0x01, 0xE2, 0x40}, Exp: -3},
		{N: "PosExp-1", S: "123456000", Coef: []byte{0x01, 0xE2, 0x40}, Exp: 3},
		{N: "PosExp-2", S: "-123456000", Neg: true, Coef: []byte{0x01, 0xE2, 0x40}, Exp: 3},
		{N: "AllDec-1", S: "0.123456", Coef: []byte{0x01, 0xE2, 0x40}, Exp: -6},
		{N: "AllDec-2", S: "-0.123456", Neg: true, Coef: []byte{0x01, 0xE2, 0x40}, Exp: -6},
		{N: "NaN-1", S: "NaN", Form: 2},
		{N: "NaN-2", S: "-NaN", Form: 2, Neg: true},
		{N: "Infinity-1", S: "Infinity", Form: 1},
		{N: "Infinity-2", S: "-Infinity", Form: 1, Neg: true},
	}

	for _, item := range list {
		t.Run(item.N, func(t *testing.T) {
			d, _, err := NewFromString(item.S)
			if err != nil {
				t.Fatal(err)
			}
			err = d.Compose(item.Form, item.Neg, item.Coef, item.Exp)
			if err != nil && !item.Err {
				t.Fatalf("unexpected error, got %v", err)
			}
			if item.Err {
				if err == nil {
					t.Fatal("expected error, got <nil>")
				}
				return
			}
			if s := fmt.Sprintf("%f", d); s != item.S {
				t.Fatalf("unexpected value, got %q want %q", s, item.S)
			}
		})
	}
}

func TestDecomposerDecompose_usesTheBufferForCoefficientWithSameSize(t *testing.T) {
	tests := []struct {
		value string
	}{
		{"0"},
		{"1"},
		{strconv.FormatUint(math.MaxUint32, 10)},
		{strconv.FormatUint(math.MaxUint64, 10)},
		{"18446744073709551616"}, // math.MaxUint64 + 1
		{"36893488147419103230"}, // math.MaxUint64 * 2
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			value, _, err := NewFromString(test.value)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			buffer := make([]byte, 0, (value.Coeff.BitLen()+8-1)/8)

			_, _, coef, _ := value.Decompose(buffer)
			if !bytes.Equal(coef, value.Coeff.Bytes()) {
				t.Fatalf("unexpected different coefficients: %s != %s", hex.EncodeToString(coef), hex.EncodeToString(value.Coeff.Bytes()))
			}

			var res BigInt
			res.SetBytes(coef)
			if res != value.Coeff {
				t.Fatal("unexpected different results")
			}
		})
	}
}

func TestDecomposerDecompose_usesTheBufferForCoefficientWithBiggerSize(t *testing.T) {
	tests := []struct {
		value string
	}{
		{"0"},
		{"1"},
		{strconv.FormatUint(math.MaxUint32, 10)},
		{strconv.FormatUint(math.MaxUint64, 10)},
		{"18446744073709551616"}, // math.MaxUint64 + 1
		{"36893488147419103230"}, // math.MaxUint64 * 2
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			value, _, err := NewFromString(test.value)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			buffer := make([]byte, 0, 64)

			_, _, coef, _ := value.Decompose(buffer)
			if !bytes.Equal(coef, value.Coeff.Bytes()) {
				t.Fatalf("unexpected different coefficients: %s != %s", hex.EncodeToString(coef), hex.EncodeToString(value.Coeff.Bytes()))
			}

			var res BigInt
			res.SetBytes(coef)
			if res != value.Coeff {
				t.Fatal("unexpected different results")
			}
		})
	}
}

func TestDecomposerDecompose_ignoresBufferIfItDoesNotFit(t *testing.T) {
	value := New(42, 0)
	buffer := make([]byte, 0)

	_, _, coef, _ := value.Decompose(buffer)
	if !bytes.Equal([]byte{42}, coef) {
		t.Fatal("unexpected different buffers", coef)
	}

	_, _, coef, _ = value.Decompose(nil)
	if !bytes.Equal([]byte{42}, coef) {
		t.Fatal("unexpected different buffers with <nil>", coef)
	}
}

func TestDecomposerDecompose_usesBufferWithNonZeroLength(t *testing.T) {
	value := New(42, 0)
	buffer := make([]byte, 4)

	_, _, coef, _ := value.Decompose(buffer)
	if !bytes.Equal([]byte{42}, coef) {
		t.Fatal("unexpected different buffers", coef)
	}
}

func TestDecomposerDecompose_usesBufferWithNonZeroCapacity(t *testing.T) {
	value := New(42, 0)
	buffer := make([]byte, 0, 4)

	_, _, coef, _ := value.Decompose(buffer)
	if !bytes.Equal([]byte{42}, coef) {
		t.Fatal("unexpected different buffers", coef)
	}
}

func TestDecomposerDecompose_extendsBufferWithNonZeroLength(t *testing.T) {
	value := New(math.MaxInt64, 0)
	buffer := make([]byte, 2)

	_, _, coef, _ := value.Decompose(buffer)
	if !bytes.Equal([]byte{127, 255, 255, 255, 255, 255, 255, 255}, coef) {
		t.Fatal("unexpected different buffers", coef)
	}
}

func BenchmarkDecomposerDecompose(b *testing.B) {
	b.Run("no allocation", func(b *testing.B) {
		buf := make([]byte, 0, 8)
		value := New(42, -1)

		for i := 0; i < b.N; i++ {
			_, _, _, _ = value.Decompose(buf)
		}
	})
	b.Run("one allocation", func(b *testing.B) {
		value := New(42, -1)

		for i := 0; i < b.N; i++ {
			_, _, _, _ = value.Decompose(nil)
		}
	})
}
