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

// This file implements the General Decimal Arithmetic ordering and adjacency
// operations: max, min, max-magnitude, min-magnitude, next-plus, next-minus,
// next-toward, logb, scaleb and copy-sign.

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

// tinyBelowEtiny sets t to a positive quantity one decimal place below the
// smallest subnormal, so that directed rounding of a value plus or minus t
// yields the adjacent representable number.
func (c *Context) tinyBelowEtiny(t *Decimal) {
	t.Form = Finite
	t.Negative = false
	t.Exponent = c.etiny() - 1
	t.Coeff.Set(bigOne)
}

// nextPlus sets d to the number adjacent to x in the direction of +Infinity,
// without raising any conditions. x must not be a NaN. It is the shared kernel
// of NextPlus and NextToward.
//
// Negative operands are handled by symmetry (nextPlus(x) == -nextMinus(-x)) so
// that the increment arithmetic always runs on a non-negative value. This
// avoids relying on directed rounding of negative subnormals, which apd rounds
// by magnitude regardless of sign.
func (c *Context) nextPlus(d, x *Decimal) {
	if x.Form == Infinite {
		if x.Negative {
			// The largest representable negative number.
			c.setNmax(d, true)
		} else {
			d.Set(decimalInfinity)
		}
		return
	}
	if x.Negative {
		var m Decimal
		m.Abs(x)
		c.nextDownPos(d, &m)
		d.Negative = !d.Negative
		return
	}
	c.nextUpPos(d, x)
}

// nextMinus sets d to the number adjacent to x in the direction of -Infinity,
// without raising any conditions. x must not be a NaN. It is the shared kernel
// of NextMinus and NextToward.
func (c *Context) nextMinus(d, x *Decimal) {
	if x.Form == Infinite {
		if x.Negative {
			d.Set(decimalInfinity)
			d.Negative = true
		} else {
			// The largest representable positive number.
			c.setNmax(d, false)
		}
		return
	}
	if x.Negative {
		var m Decimal
		m.Abs(x)
		c.nextUpPos(d, &m)
		d.Negative = !d.Negative
		return
	}
	c.nextDownPos(d, x)
}

// nextUpPos sets d to the neighbour of x toward +Infinity, where x is a
// non-negative finite value.
func (c *Context) nextUpPos(d, x *Decimal) {
	nc := *c
	nc.Rounding = RoundCeiling
	nc.Traps = 0
	// Round x toward +Infinity. If that changes its value, x was not
	// representable and the rounded value is the adjacent one.
	var fixed Decimal
	nc.round(&fixed, x)
	if fixed.Cmp(x) != 0 {
		d.Set(&fixed)
		return
	}
	// x is representable; adding a quantity below the smallest subnormal and
	// rounding toward +Infinity yields the next representable number. The
	// operands are non-negative, so the sum is non-negative.
	var tiny Decimal
	c.tinyBelowEtiny(&tiny)
	nc.Add(d, x, &tiny)
}

// nextDownPos sets d to the neighbour of x toward -Infinity, where x is a
// non-negative finite value.
func (c *Context) nextDownPos(d, x *Decimal) {
	if x.IsZero() {
		// The neighbour of zero toward -Infinity is the smallest negative
		// subnormal; this is exact.
		d.Form = Finite
		d.Negative = true
		d.Exponent = c.etiny()
		d.Coeff.Set(bigOne)
		return
	}
	nc := *c
	nc.Rounding = RoundFloor
	nc.Traps = 0
	var fixed Decimal
	nc.round(&fixed, x)
	if fixed.Form == Infinite {
		// apd rounds every overflow to Infinity, but rounding a too-large
		// positive value toward -Infinity clamps to the largest finite value.
		c.setNmax(&fixed, false)
	}
	if fixed.Cmp(x) != 0 {
		d.Set(&fixed)
		return
	}
	// x is a positive representable value; subtracting tiny keeps the
	// difference non-negative, so directed rounding stays on the positive side.
	var tiny Decimal
	c.tinyBelowEtiny(&tiny)
	nc.Sub(d, x, &tiny)
}

// NextPlus sets d to the smallest representable number larger than x. A NaN
// operand propagates, and a signaling NaN raises InvalidOperation. The
// operation itself raises no other conditions, even when the result overflows
// to Infinity.
func (c *Context) NextPlus(d, x *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, nil) {
		return c.setAsNaN(d, x, nil)
	}
	c.nextPlus(d, x)
	return 0, nil
}

// NextMinus sets d to the largest representable number smaller than x. NaN
// handling matches NextPlus.
func (c *Context) NextMinus(d, x *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, nil) {
		return c.setAsNaN(d, x, nil)
	}
	c.nextMinus(d, x)
	return 0, nil
}

// NextToward sets d to the number adjacent to x in the direction of y. If x and
// y are numerically equal, the result is x with the sign of y and no conditions
// are raised. Otherwise the result is the neighbour of x toward y; unlike
// NextPlus and NextMinus, this operation raises Overflow, Underflow, Inexact,
// Rounded, Subnormal and Clamped as appropriate. NaN handling matches NextPlus.
func (c *Context) NextToward(d, x, y *Decimal) (Condition, error) {
	if c.shouldSetAsNaN(x, y) {
		return c.setAsNaN(d, x, y)
	}
	cmp := x.Cmp(y)
	if cmp == 0 {
		// The result is x with the sign of y (copy-sign).
		d.Abs(x)
		d.Negative = y.Negative
		return 0, nil
	}
	if cmp < 0 {
		c.nextPlus(d, x)
	} else {
		c.nextMinus(d, x)
	}
	var res Condition
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
	d.Set(x)
	newExp := int64(x.Exponent) + n
	if newExp > MaxExponent || newExp < MinExponent {
		d.Set(decimalNaN)
		return c.goError(InvalidOperation)
	}
	d.Exponent = int32(newExp)
	// Rounding to the context finalizes the exponent, applying overflow,
	// underflow, subnormal and clamping just as GDA's fix step does.
	res := c.round(d, d)
	return c.goError(res)
}

// CopySign sets d to the magnitude of x combined with the sign of y. No
// rounding is performed and no conditions are raised, even for signaling NaN
// operands: only the sign is affected.
func (c *Context) CopySign(d, x, y *Decimal) (Condition, error) {
	d.Abs(x)
	d.Negative = y.Negative
	return 0, nil
}
