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

import "math/big"

// Round sets d to rounded x. The result is stored in d and returned. If
// d has zero Precision, no modification of x is done. If d has no Rounding
// specified, RoundHalfUp is used.
func (c *Context) Round(d, x *Decimal) (Condition, error) {
	return c.round(d, x).GoError(c.Traps)
}

func (c *Context) round(d, x *Decimal) Condition {
	if c.Precision == 0 {
		d.Set(x)
		return d.setExponent(c, int64(d.Exponent))
	}
	rounder := c.Rounding
	if rounder == nil {
		rounder = roundHalfUp
	}
	res := rounder(c, d, x)
	res |= d.setExponent(c, int64(d.Exponent))
	return res
}

// Rounder sets d to rounded x.
type Rounder func(c *Context, d, x *Decimal) Condition

var (
	// RoundDown rounds toward 0; truncate.
	RoundDown Rounder = roundDown
	// RoundHalfUp rounds up if the digits are >= 0.5.
	RoundHalfUp Rounder = roundHalfUp
	// RoundHalfEven rounds up if the digits are > 0.5. If the digits are equal
	// to 0.5, it rounds up if the previous digit is odd, always producing an
	// even digit.
	RoundHalfEven Rounder = roundHalfEven
	// RoundCeiling towards +Inf: rounds up if digits are > 0 and the number
	// is positive.
	RoundCeiling Rounder = roundCeiling
	// RoundFloor towards -Inf: rounds up if digits are > 0 and the number
	// is negative.
	RoundFloor Rounder = roundFloor
	// RoundHalfDown rounds up if the digits are > 0.5.
	RoundHalfDown Rounder = roundHalfDown
	// RoundUp rounds away from 0.
	RoundUp Rounder = roundUp
)

func roundDown(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		return false
	})
}

func roundHalfUp(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		m.Abs(m)
		m.Mul(m, bigTwo)
		return m.Cmp(e) >= 0
	})
}

func roundHalfEven(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		m.Abs(m)
		m.Mul(m, bigTwo)
		if c := m.Cmp(e); c > 0 {
			return true
		} else if c == 0 && y.Bit(0) == 1 {
			return true
		}
		return false
	})
}

func roundHalfDown(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		m.Abs(m)
		m.Mul(m, bigTwo)
		return m.Cmp(e) > 0
	})
}

func roundUp(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		return m.Sign() != 0
	})
}

func roundFloor(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		return m.Sign() != 0 && y.Sign() < 0
	})
}

func roundCeiling(c *Context, d, x *Decimal) Condition {
	return roundFunc(c, d, x, func(m, y, e *big.Int) bool {
		return m.Sign() != 0 && y.Sign() >= 0
	})
}

func roundFunc(c *Context, d, x *Decimal, f func(m, y, e *big.Int) bool) Condition {
	d.Set(x)
	nd := x.NumDigits()
	var res Condition
	if diff := nd - int64(c.Precision); diff > 0 {
		if diff > MaxExponent {
			return SystemOverflow | Overflow
		}
		if diff < MinExponent {
			return SystemUnderflow | Underflow
		}
		tmp := new(Decimal).Set(x)
		res |= Rounded
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		if f(m, y, e) {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		res |= d.setExponent(c, int64(d.Exponent), diff)
		if d.Cmp(tmp) != 0 {
			res |= Inexact
		}
	}
	return res
}

// roundAddOne adds 1 to abs(b).
func roundAddOne(b *big.Int, diff *int64) {
	nd := NumDigits(b)
	if b.Sign() >= 0 {
		b.Add(b, bigOne)
	} else {
		b.Sub(b, bigOne)
	}
	nd2 := NumDigits(b)
	if nd2 > nd {
		b.Quo(b, bigTen)
		*diff++
	}
}
