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

// NewErrDecimal creates a ErrDecimal with given context.
func NewErrDecimal(c *Context) *ErrDecimal {
	return &ErrDecimal{
		Ctx: c,
	}
}

// ErrDecimal performs operations on decimals and collects errors during
// operations. If an error is already set, the operation is skipped. Designed to
// be used for many operations in a row, with a single error check at the end.
type ErrDecimal struct {
	Err error
	Ctx *Context
}

// Abs performs e.Ctx.Abs(d, x).
func (e *ErrDecimal) Abs(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Abs(d, x)
}

// Add performs e.Ctx.Add(d, x, y).
func (e *ErrDecimal) Add(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Add(d, x, y)
}

// Cmp returns 0 if Err is set. Otherwise returns e.Ctx.Cmp(a, b).
func (e *ErrDecimal) Cmp(a, b *Decimal) int {
	if e.Err != nil {
		return 0
	}
	var c int
	c, e.Err = a.Cmp(b)
	return c
}

// Exp performs e.Ctx.Exp(d, x).
func (e *ErrDecimal) Exp(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Exp(d, x)
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

// Ln performs e.Ctx.Ln(d, x).
func (e *ErrDecimal) Ln(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Ln(d, x)
}

// Log10 performs d.Log10(x).
func (e *ErrDecimal) Log10(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Log10(d, x)
}

// Mul performs e.Ctx.Mul(d, x, y).
func (e *ErrDecimal) Mul(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Mul(d, x, y)
}

// Neg performs e.Ctx.Neg(d, x).
func (e *ErrDecimal) Neg(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Neg(d, x)
}

// Pow performs e.Ctx.Pow(d, x, y).
func (e *ErrDecimal) Pow(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Pow(d, x, y)
}

// Quo performs e.Ctx.Quo(d, x, y).
func (e *ErrDecimal) Quo(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Quo(d, x, y)
}

// QuoInteger performs e.Ctx.QuoInteger(d, x, y).
func (e *ErrDecimal) QuoInteger(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.QuoInteger(d, x, y)
}

// Rem performs e.Ctx.Rem(d, x, y).
func (e *ErrDecimal) Rem(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Rem(d, x, y)
}

// Round performs e.Ctx.Round(d, x).
func (e *ErrDecimal) Round(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Round(d, x)
}

// Sqrt performs e.Ctx.Sqrt(d, x).
func (e *ErrDecimal) Sqrt(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Sqrt(d, x)
}

// Sub performs e.Ctx.Sub(d, x, y).
func (e *ErrDecimal) Sub(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = e.Ctx.Sub(d, x, y)
}
