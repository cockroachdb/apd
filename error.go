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

// ErrDecimal performs operations on decimals and collects errors during
// operations. If an error is already set, the operation is skipped. Designed to
// be used for many operations in a row, with a single error check at the end.
type ErrDecimal struct {
	Err error
}

// Abs performs d.Abs(x).
func (e *ErrDecimal) Abs(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Abs(x)
}

// Add performs d.Add(x, y).
func (e *ErrDecimal) Add(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Add(x, y)
}

// Cmp returns 0 if Err is set. Otherwise returns a.Cmp(b).
func (e *ErrDecimal) Cmp(a, b *Decimal) int {
	if e.Err != nil {
		return 0
	}
	var c int
	c, e.Err = a.Cmp(b)
	return c
}

// Exp performs d.Exp(x).
func (e *ErrDecimal) Exp(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Exp(x)
}

// Int64 returns 0 if Err is set. Otherwise returns d.Int64().
func (e *ErrDecimal) Int64(d *Decimal) int64 {
	if e.Err != nil {
		return 0
	}
	var r int64
	r, e.Err = d.Int64()
	return r
}

// Ln performs d.Ln(x).
func (e *ErrDecimal) Ln(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Ln(x)
}

// Log10 performs d.Log10(x).
func (e *ErrDecimal) Log10(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Log10(x)
}

// Mul performs d.Mul(x, y).
func (e *ErrDecimal) Mul(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Mul(x, y)
}

// Neg performs d.Neg(x).
func (e *ErrDecimal) Neg(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Neg(x)
}

// Pow performs d.Pow(x, y).
func (e *ErrDecimal) Pow(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Pow(x, y)
}

// Quo performs d.Quo(x, y).
func (e *ErrDecimal) Quo(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Quo(x, y)
}

// QuoInteger performs d.QuoInteger(x, y).
func (e *ErrDecimal) QuoInteger(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.QuoInteger(x, y)
}

// Rem performs d.Rem(x, y).
func (e *ErrDecimal) Rem(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Rem(x, y)
}

// Round performs d.Round(x).
func (e *ErrDecimal) Round(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Round(x)
}

// Sqrt performs d.Sqrt(x).
func (e *ErrDecimal) Sqrt(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Sqrt(x)
}

// Sub performs d.Sub(x, y).
func (e *ErrDecimal) Sub(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Sub(x, y)
}
