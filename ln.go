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

import "errors"

// lnRounded encloses ln(x) between fixed-point integers, then rounds only when
// both endpoints give the same answer. A fixed number of guard digits is not
// enough: cancellation and results arbitrarily close to a rounding boundary
// can otherwise make the final answer depend on intermediate rounding.
func (c *Context) lnRounded(d, x *Decimal) (Condition, error) {
	if c.Precision == 0 {
		return 0, errors.New(errZeroPrecisionStr)
	}
	var z, numerator, denominator Decimal
	z.Set(x)
	var twos, tens int64
	negative := x.Cmp(decimalOne) < 0
	nearOne := x.Cmp(decimalHalf) >= 0 && x.Cmp(decimalTwo) <= 0
	if !nearOne {
		// x = z * 2^twos * 10^tens, with 1 <= z <= 2. All
		// reductions are exact. ln(10) = 3*ln(2) + ln(5/4).
		tens = x.NumDigits() - 1 + int64(x.Exponent)
		z.Exponent -= int32(tens)
		for z.Cmp(decimalTwo) > 0 {
			if _, err := BaseContext.Mul(&z, &z, decimalHalf); err != nil {
				return 0, err
			}
			twos++
		}
	}
	if _, err := BaseContext.Sub(&numerator, &z, decimalOne); err != nil {
		return 0, err
	}
	numerator.Abs(&numerator)
	if _, err := BaseContext.Add(&denominator, &z, decimalOne); err != nil {
		return 0, err
	}
	// Express |z-1|/(z+1) as a ratio of integers without rounding.
	var num, den, tmp BigInt
	// upscale returns new values rather than modifying its operands.
	n, q, _, err := upscale(&numerator, &denominator, &tmp)
	if err != nil {
		return 0, err
	}
	num.Set(n)
	den.Set(q)
	places := int64(c.Precision) + 8
	if nearOne {
		// Near one, ln(x) is of the same order as x-1. Keep relative
		// precision even when the input agrees with one for many digits.
		adjusted := numerator.NumDigits() + int64(numerator.Exponent) - 1
		if adjusted < 0 {
			places -= adjusted
		}
	}
	for {
		if places > MaxExponent {
			return 0, errors.New("ln: required working precision exceeds supported limit")
		}
		var scale BigInt
		scale.Exp(bigTen, NewBigInt(places), nil)
		lo, hi := lnSeriesBounds(&num, &den, &scale)
		if nearOne && negative {
			lo, hi = hi, lo
			lo.Neg(&lo)
			hi.Neg(&hi)
		}
		for _, term := range []struct{ numerator, denominator, coefficient int64 }{
			{1, 3, twos + 3*tens}, // ln(2)
			{1, 9, tens},          // ln(5/4)
		} {
			if term.coefficient == 0 {
				continue
			}
			a, b := lnSeriesBounds(NewBigInt(term.numerator), NewBigInt(term.denominator), &scale)
			if term.coefficient < 0 {
				a, b = b, a
			}
			factor := NewBigInt(term.coefficient)
			a.Mul(&a, factor)
			b.Mul(&b, factor)
			lo.Add(&lo, &a)
			hi.Add(&hi, &b)
		}
		var lower, upper Decimal
		lower.SetFinite(0, -int32(places))
		upper.SetFinite(0, -int32(places))
		lower.Negative = lo.Sign() < 0
		upper.Negative = hi.Sign() < 0
		lower.Coeff.Abs(&lo)
		upper.Coeff.Abs(&hi)
		lf := c.round(&lower, &lower)
		uf := c.round(&upper, &upper)
		if lower.CmpTotal(&upper) == 0 && lf == uf {
			d.Set(&lower)
			// For a finite decimal x != 1, ln(x) is irrational.
			return c.goError(lf | Inexact | Rounded)
		}
		// Refine an ambiguous enclosure rather than guessing which side of
		// a rounding boundary contains the exact value. Try the maximum
		// supported scale before rejecting a refinement that overshoots it.
		if places == MaxExponent {
			return 0, errors.New("ln: required working precision exceeds supported limit")
		}
		places *= 2
		if places > MaxExponent {
			places = MaxExponent
		}
	}
}

// lnSeriesBounds encloses 2*atanh(num/den), in units of 1/scale. Its
// precondition is 0 <= num/den <= 1/3. Every integer division is rounded
// outward, including the geometric bound on the unevaluated series tail.
func lnSeriesBounds(num, den, scale *BigInt) (lo, hi BigInt) {
	var yl, yu, rem, sl, su, pl, pu, t BigInt
	yl.Mul(num, scale)
	yl.QuoRem(&yl, den, &rem)
	yu.Set(&yl)
	if rem.Sign() != 0 {
		yu.Add(&yu, bigOne)
	}
	sl.Mul(&yl, &yl)
	sl.Quo(&sl, scale)
	su.Mul(&yu, &yu)
	ceilQuo(&su, &su, scale)
	pl.Set(&yl)
	pu.Set(&yu)
	for odd := int64(1); ; odd += 2 {
		divisor := NewBigInt(odd)
		t.Quo(&pl, divisor)
		lo.Add(&lo, &t)
		ceilQuo(&t, &pu, divisor)
		hi.Add(&hi, &t)
		pl.Mul(&pl, &sl)
		pl.Quo(&pl, scale)
		pu.Mul(&pu, &su)
		ceilQuo(&pu, &pu, scale)
		if pu.Cmp(bigTwo) <= 0 {
			lo.Mul(&lo, bigTwo)
			hi.Mul(&hi, bigTwo)
			// y <= 1/3, so the tail is <= 2*y^(odd+2)/
			// ((odd+2)*(1-y*y)) <= 4*y^(odd+2)/(odd+2).
			t.Mul(&pu, NewBigInt(4))
			ceilQuo(&t, &t, NewBigInt(odd+2))
			hi.Add(&hi, &t)
			return lo, hi
		}
	}
}

// ceilQuo sets z to ceil(x/y), for nonnegative x and positive y.
func ceilQuo(z, x, y *BigInt) {
	var rem BigInt
	z.QuoRem(x, y, &rem)
	if rem.Sign() != 0 {
		z.Add(z, bigOne)
	}
}
