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
	err error
	Ctx *Context
}

// Err returns the first error encountered.
func (e *ErrDecimal) Err() error {
	return e.err
}

// Abs performs e.Ctx.Abs(d, x) and returns d.
func (e *ErrDecimal) Abs(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Abs(d, x)
	return d
}

// Add performs e.Ctx.Add(d, x, y) and returns d.
func (e *ErrDecimal) Add(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Add(d, x, y)
	return d
}

// Cmp returns 0 if err is set. Otherwise returns e.Ctx.Cmp(a, b).
func (e *ErrDecimal) Cmp(a, b *Decimal) int {
	if e.Err() != nil {
		return 0
	}
	var c int
	c, e.err = a.Cmp(b)
	return c
}

// Exp performs e.Ctx.Exp(d, x) and returns d.
func (e *ErrDecimal) Exp(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Exp(d, x)
	return d
}

// Int64 returns 0 if err is set. Otherwise returns d.Int64().
func (e *ErrDecimal) Int64(d *Decimal) int64 {
	if e.Err() != nil {
		return 0
	}
	var r int64
	r, e.err = d.Int64()
	return r
}

// Ln performs e.Ctx.Ln(d, x) and returns d.
func (e *ErrDecimal) Ln(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Ln(d, x)
	return d
}

// Log10 performs d.Log10(x) and returns d.
func (e *ErrDecimal) Log10(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Log10(d, x)
	return d
}

// Mul performs e.Ctx.Mul(d, x, y) and returns d.
func (e *ErrDecimal) Mul(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Mul(d, x, y)
	return d
}

// Neg performs e.Ctx.Neg(d, x) and returns d.
func (e *ErrDecimal) Neg(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Neg(d, x)
	return d
}

// Pow performs e.Ctx.Pow(d, x, y) and returns d.
func (e *ErrDecimal) Pow(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Pow(d, x, y)
	return d
}

// Quo performs e.Ctx.Quo(d, x, y) and returns d.
func (e *ErrDecimal) Quo(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Quo(d, x, y)
	return d
}

// QuoInteger performs e.Ctx.QuoInteger(d, x, y) and returns d.
func (e *ErrDecimal) QuoInteger(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.QuoInteger(d, x, y)
	return d
}

// Rem performs e.Ctx.Rem(d, x, y) and returns d.
func (e *ErrDecimal) Rem(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Rem(d, x, y)
	return d
}

// Round performs e.Ctx.Round(d, x) and returns d.
func (e *ErrDecimal) Round(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Round(d, x).GoError()
	return d
}

// Sqrt performs e.Ctx.Sqrt(d, x) and returns d.
func (e *ErrDecimal) Sqrt(d, x *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Sqrt(d, x)
	return d
}

// Sub performs e.Ctx.Sub(d, x, y) and returns d.
func (e *ErrDecimal) Sub(d, x, y *Decimal) *Decimal {
	if e.Err() != nil {
		return d
	}
	e.err = e.Ctx.Sub(d, x, y)
	return d
}
