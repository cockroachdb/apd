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

// Round sets d to rounded x, rounded to the precision specified by c. If c
// has zero precision, no rounding will occur. If c has no Rounding specified,
// RoundHalfUp is used.
func (c *Context) Round(d, x *Decimal) (Condition, error) {
	return c.goError(c.round(d, x))
}

func (c *Context) round(d, x *Decimal) Condition {
	if c.Precision == 0 {
		d.Set(x)
		return d.setExponent(c, 0, int64(d.Exponent))
	}
	rounder := c.rounding()
	res := rounder.Round(c, d, x)
	return res
}

func (c *Context) rounding() Rounder {
	if c.Rounding == nil {
		return roundHalfUp
	}
	return c.Rounding
}

// Rounder defines a function that returns true if 1 should be added to the
// absolute value of a number being rounded. result is the result to which
// the 1 would be added. half is -1 if the discarded digits are < 0.5, 0 if =
// 0.5, or 1 if > 0.5.
type Rounder func(result *big.Int, half int) bool

// Round sets d to rounded x.
func (r Rounder) Round(c *Context, d, x *Decimal) Condition {
	d.Set(x)
	nd := x.NumDigits()
	xs := x.Sign()
	var res Condition

	// adj is the adjusted exponent: exponent + clength - 1
	if adj := int64(x.Exponent) + nd - 1; xs != 0 && adj < int64(c.MinExponent) {
		// Subnormal is defined before rounding.
		res |= Subnormal
		// setExponent here to prevent double-rounded subnormals.
		res |= d.setExponent(c, res, int64(d.Exponent))
		return res
	}

	diff := nd - int64(c.Precision)
	if diff > 0 {
		if diff > MaxExponent {
			return SystemOverflow | Overflow
		}
		if diff < MinExponent {
			return SystemUnderflow | Underflow
		}
		res |= Rounded
		y := new(big.Int)
		e := tableExp10(diff, y)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		if m.Sign() != 0 {
			res |= Inexact
			m.Abs(m)
			discard := NewWithBigInt(m, int32(-diff))
			if r(y, discard.Cmp(decimalHalf)) {
				roundAddOne(y, &diff, xs)
			}
		}
		d.Coeff = *y
	} else {
		diff = 0
	}
	res |= d.setExponent(c, res, int64(d.Exponent), diff)
	return res
}

// roundAddOne adds 1 to abs(b). sign is the sign of the rounded number.
func roundAddOne(b *big.Int, diff *int64, sign int) {
	nd := NumDigits(b)
	if sign >= 0 {
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
	// Round05Up rounds zero or five away from 0; same as round-up, except that
	// rounding up only occurs if the digit to be rounded up is 0 or 5.
	Round05Up Rounder = round05Up
)

func roundDown(result *big.Int, half int) bool {
	return false
}

func roundUp(result *big.Int, half int) bool {
	return true
}

func round05Up(result *big.Int, half int) bool {
	z := new(big.Int)
	z.Rem(result, bigFive)
	if z.Sign() == 0 {
		return true
	}
	z.Rem(result, bigTen)
	return z.Sign() == 0
}

func roundHalfUp(result *big.Int, half int) bool {
	return half >= 0
}

func roundHalfEven(result *big.Int, half int) bool {
	if half > 0 {
		return true
	}
	if half < 0 {
		return false
	}
	return result.Bit(0) == 1
}

func roundHalfDown(result *big.Int, half int) bool {
	return half > 0
}

func roundFloor(result *big.Int, half int) bool {
	return result.Sign() < 0
}

func roundCeiling(result *big.Int, half int) bool {
	return result.Sign() >= 0
}
