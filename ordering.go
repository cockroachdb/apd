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

import "errors"

// This file implements the General Decimal Arithmetic ordering and adjacency
// operations: max, min, max-magnitude, min-magnitude, next-plus, next-minus,
// next-toward, logb and scaleb. copy-sign is Decimal.CopySign.

// maxMinSetAsNaN applies the NaN rules that are shared by Max, Min, MaxAbs and
// MinAbs. Unlike most operations, these treat a quiet NaN operand as absent:
// the other (numeric) operand is returned. Only a signaling NaN raises
// InvalidOperation, and two quiet NaNs produce a NaN. It reports whether it
// handled the inputs (i.e. at least one operand was a NaN); if so, d has been
// set and the returned Condition and error should be returned to the caller.
func (c *Context) maxMinSetAsNaN(d, x, y *Decimal) (bool, Condition, error) {
	xNaN := x.Form == NaN || x.Form == NaNSignaling
	yNaN := y.Form == NaN || y.Form == NaNSignaling
	if !xNaN && !yNaN {
		return false, 0, nil
	}
	// A signaling NaN in either operand, or a NaN in both, follows the standard
	// NaN handling: signaling raises InvalidOperation and quiets, otherwise the
	// first NaN propagates.
	if x.Form == NaNSignaling || y.Form == NaNSignaling || (xNaN && yNaN) {
		res, err := c.setAsNaN(d, x, y)
		return true, res, err
	}
	// Exactly one operand is a quiet NaN; return the other, rounded to context.
	other := y
	if yNaN {
		other = x
	}
	res := c.selectResult(d, other)
	resErr, err := c.goError(res)
	return true, resErr, err
}

// selectResult sets d to the chosen operand, rounding a finite value to the
// context. Special values (infinities and NaNs) are copied unchanged, matching
// the GDA rule that these selection operations do not otherwise alter their
// operands.
func (c *Context) selectResult(d, chosen *Decimal) Condition {
	if chosen.Form != Finite {
		d.Set(chosen)
		return 0
	}
	return c.round(d, chosen)
}

// Max sets d to the larger of x and y and returns d. If the operands compare
// equal numerically, the one with the larger total order (see CmpTotal) is
// returned; for example Max(1.0, 1.00) is 1.0. A quiet NaN operand is treated
// as absent, so the other operand is returned; a signaling NaN raises
// InvalidOperation.
func (c *Context) Max(d, x, y *Decimal) (Condition, error) {
	if handled, res, err := c.maxMinSetAsNaN(d, x, y); handled {
		return res, err
	}
	cmp := x.Cmp(y)
	if cmp == 0 {
		cmp = x.CmpTotal(y)
	}
	chosen := x
	if cmp < 0 {
		chosen = y
	}
	return c.goError(c.selectResult(d, chosen))
}

// Min sets d to the smaller of x and y. If the operands compare equal
// numerically, the one with the smaller total order is returned. NaN handling
// is the same as Max.
func (c *Context) Min(d, x, y *Decimal) (Condition, error) {
	if handled, res, err := c.maxMinSetAsNaN(d, x, y); handled {
		return res, err
	}
	cmp := x.Cmp(y)
	if cmp == 0 {
		cmp = x.CmpTotal(y)
	}
	chosen := x
	if cmp > 0 {
		chosen = y
	}
	return c.goError(c.selectResult(d, chosen))
}

// MaxAbs sets d to whichever of x and y has the larger magnitude, preserving
// its sign. If the magnitudes are equal the total order of the signed operands
// is used as a tie-break, as in Max. NaN handling is the same as Max.
func (c *Context) MaxAbs(d, x, y *Decimal) (Condition, error) {
	if handled, res, err := c.maxMinSetAsNaN(d, x, y); handled {
		return res, err
	}
	var xa, ya Decimal
	cmp := xa.Abs(x).Cmp(ya.Abs(y))
	if cmp == 0 {
		cmp = x.CmpTotal(y)
	}
	chosen := x
	if cmp < 0 {
		chosen = y
	}
	return c.goError(c.selectResult(d, chosen))
}

// MinAbs sets d to whichever of x and y has the smaller magnitude, preserving
// its sign. If the magnitudes are equal the total order of the signed operands
// is used as a tie-break, as in Min. NaN handling is the same as Max.
func (c *Context) MinAbs(d, x, y *Decimal) (Condition, error) {
	if handled, res, err := c.maxMinSetAsNaN(d, x, y); handled {
		return res, err
	}
	var xa, ya Decimal
	cmp := xa.Abs(x).Cmp(ya.Abs(y))
	if cmp == 0 {
		cmp = x.CmpTotal(y)
	}
	chosen := x
	if cmp > 0 {
		chosen = y
	}
	return c.goError(c.selectResult(d, chosen))
}

// setNmax sets d to the largest finite magnitude representable in the context,
// Nmax, with the given sign.
func (c *Context) setNmax(d *Decimal, negative bool) {
	var tmp BigInt
	d.Coeff.Sub(tableExp10(int64(c.Precision), &tmp), bigOne)
	d.Exponent = c.MaxExponent - int32(c.Precision) + 1
	d.Negative = negative
	d.Form = Finite
}

// nextMag sets d to the representable number adjacent to |x| in the direction
// away from zero if up is true, and toward zero otherwise. x must be finite and
// c.Precision non-zero. The result is non-negative, except that stepping down
// from zero yields the smallest negative subnormal.
//
// The step is computed directly on the coefficient: the representable numbers
// near |x| are the multiples of 10**q, where q is the exponent of the last
// digit |x| would have at full precision (but not less than Etiny). This takes
// O(Precision) work regardless of the exponents involved. The only possible
// conditions are the system limits, if the result's exponent is out of range.
func (c *Context) nextMag(d, x *Decimal, up bool) Condition {
	p := int64(c.Precision)
	etiny := int64(c.etiny())
	var k BigInt
	var q int64
	// Read before d is written, in case d and x are the same.
	zero := x.IsZero()
	if zero {
		k.SetInt64(1)
		q = etiny
	} else {
		nd := x.NumDigits()
		if adj := int64(x.Exponent) + nd - 1; adj > int64(c.MaxExponent) {
			// |x| is larger than Nmax.
			if up {
				d.Set(decimalInfinity)
			} else {
				c.setNmax(d, false)
			}
			return 0
		} else if q = adj - p + 1; q < etiny {
			q = etiny
		}
		// k = floor(|x| / 10**q); exact reports whether the division is exact.
		exact := true
		var tmp BigInt
		if shift := int64(x.Exponent) - q; shift >= 0 {
			// q >= adj-p+1, so shift <= p-nd.
			k.Mul(&x.Coeff, tableExp10(shift, &tmp))
		} else if -shift > nd {
			// 0 < |x| < 10**q.
			exact = false
		} else {
			var rem BigInt
			k.QuoRem(&x.Coeff, tableExp10(-shift, &tmp), &rem)
			exact = rem.Sign() == 0
		}
		switch {
		case up:
			// The ceiling of a non-representable |x|, or the next multiple of
			// 10**q above a representable one: both are k+1.
			k.Add(&k, bigOne)
			if NumDigits(&k) > p {
				// k was 10**p-1, so the result is 10**(q+p).
				k.Quo(&k, bigTen)
				q++
			}
		case !exact:
			// The floor of a non-representable |x| is k.
		case q > etiny && k.Cmp(tableExp10(p-1, &tmp)) == 0:
			// |x| is a power of ten: the number below it has p nines and the
			// smaller exponent.
			k.Sub(tableExp10(p, &tmp), bigOne)
			q--
		default:
			k.Sub(&k, bigOne)
		}
	}
	if q+NumDigits(&k)-1 > int64(c.MaxExponent) {
		d.Set(decimalInfinity)
		return 0
	}
	if q < MinExponent {
		return SystemUnderflow | Underflow
	}
	if q > MaxExponent {
		return SystemOverflow | Overflow
	}
	d.Coeff.Set(&k)
	d.Exponent = int32(q)
	d.Negative = zero && !up
	d.Form = Finite
	return 0
}

// nextPlus sets d to the number adjacent to x in the direction of +Infinity.
// x must not be a NaN. It is the shared kernel of NextPlus and NextToward.
func (c *Context) nextPlus(d, x *Decimal) Condition {
	if x.Form == Infinite {
		if x.Negative {
			// The largest representable negative number.
			c.setNmax(d, true)
		} else {
			d.Set(decimalInfinity)
		}
		return 0
	}
	if !x.Negative {
		return c.nextMag(d, x, true)
	}
	// nextPlus(x) == -nextMinus(-x).
	res := c.nextMag(d, x, false)
	d.Negative = !d.Negative
	return res
}

// nextMinus sets d to the number adjacent to x in the direction of -Infinity.
// x must not be a NaN. It is the shared kernel of NextMinus and NextToward.
func (c *Context) nextMinus(d, x *Decimal) Condition {
	if x.Form == Infinite {
		if x.Negative {
			d.Set(decimalInfinity)
			d.Negative = true
		} else {
			// The largest representable positive number.
			c.setNmax(d, false)
		}
		return 0
	}
	if !x.Negative {
		return c.nextMag(d, x, false)
	}
	// nextMinus(x) == -nextPlus(-x).
	res := c.nextMag(d, x, true)
	d.Negative = !d.Negative
	return res
}

// NextPlus sets d to the smallest representable number larger than x. A NaN
// operand propagates, and a signaling NaN raises InvalidOperation. The
// operation itself raises no other conditions, even when the result overflows
// to Infinity. c must have a non-zero Precision.
func (c *Context) NextPlus(d, x *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, nil) {
		return c.setAsNaN(d, x, nil)
	}
	if c.Precision == 0 {
		return 0, errors.New(errZeroPrecisionStr)
	}
	return c.goError(c.nextPlus(d, x))
}

// NextMinus sets d to the largest representable number smaller than x. NaN
// handling and the Precision requirement match NextPlus.
func (c *Context) NextMinus(d, x *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, nil) {
		return c.setAsNaN(d, x, nil)
	}
	if c.Precision == 0 {
		return 0, errors.New(errZeroPrecisionStr)
	}
	return c.goError(c.nextMinus(d, x))
}

// NextToward sets d to the number adjacent to x in the direction of y. If x and
// y are numerically equal, the result is x with the sign of y and no conditions
// are raised. Otherwise the result is the neighbour of x toward y; unlike
// NextPlus and NextMinus, this operation raises Overflow, Underflow, Inexact,
// Rounded, Subnormal and Clamped as appropriate. NaN handling and the Precision
// requirement match NextPlus.
func (c *Context) NextToward(d, x, y *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, y) {
		return c.setAsNaN(d, x, y)
	}
	if c.Precision == 0 {
		return 0, errors.New(errZeroPrecisionStr)
	}
	cmp := x.Cmp(y)
	if cmp == 0 {
		d.CopySign(x, y)
		return 0, nil
	}
	var res Condition
	if cmp < 0 {
		res = c.nextPlus(d, x)
	} else {
		res = c.nextMinus(d, x)
	}
	if res != 0 {
		return c.goError(res)
	}
	if d.Form == Infinite {
		res = Overflow | Inexact | Rounded
	} else if adj := int64(d.Exponent) + d.NumDigits() - 1; adj < int64(c.MinExponent) {
		res = Underflow | Subnormal | Inexact | Rounded
		if d.IsZero() {
			res |= Clamped
		}
	}
	return c.goError(res)
}

// LogB sets d to the adjusted exponent of x: the integer floor(log10(|x|)),
// expressed as a decimal. LogB(0) is -Infinity and raises DivisionByZero;
// LogB(±Infinity) is +Infinity. A signaling NaN raises InvalidOperation.
func (c *Context) LogB(d, x *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, nil) {
		return c.setAsNaN(d, x, nil)
	}
	if x.Form == Infinite {
		d.Set(decimalInfinity)
		return 0, nil
	}
	if x.IsZero() {
		d.Set(decimalInfinity)
		d.Negative = true
		return c.goError(DivisionByZero)
	}
	adj := int64(x.Exponent) + x.NumDigits() - 1
	d.SetInt64(adj)
	return c.goError(c.round(d, d))
}

// ScaleB sets d to x * 10**y, where y must be an integer. y is required to have
// exponent zero and to satisfy |y| <= 2*(MaxExponent+Precision); otherwise the
// result is NaN and InvalidOperation is raised. The result is rounded to the
// context and may overflow, underflow or become subnormal. NaN handling follows
// the standard rules (signaling NaN raises InvalidOperation).
func (c *Context) ScaleB(d, x, y *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, y) {
		return c.setAsNaN(d, x, y)
	}
	// y must be an integer represented with a zero exponent, within the
	// permitted range.
	lim := 2 * (int64(c.MaxExponent) + int64(c.Precision))
	if y.Form != Finite || y.Exponent != 0 || !y.Coeff.IsInt64() {
		d.Set(decimalNaN)
		return c.goError(InvalidOperation)
	}
	n := y.Coeff.Int64()
	if y.Negative {
		n = -n
	}
	if n < -lim || n > lim {
		d.Set(decimalNaN)
		return c.goError(InvalidOperation)
	}
	if x.Form == Infinite {
		d.Set(x)
		return 0, nil
	}
	// Beyond the system exponent limits the result cannot be held, as with
	// every other operation.
	newExp := int64(x.Exponent) + n
	if newExp > MaxExponent {
		return c.goError(SystemOverflow | Overflow)
	}
	if newExp < MinExponent {
		return c.goError(SystemUnderflow | Underflow)
	}
	d.Set(x)
	d.Exponent = int32(newExp)
	// Rounding to the context finalizes the exponent, applying overflow,
	// underflow, subnormal and clamping just as GDA's fix step does.
	return c.goError(c.round(d, d))
}
