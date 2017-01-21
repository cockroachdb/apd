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
	"math"
	"math/big"

	"github.com/pkg/errors"
)

// Context maintains options for Decimal operations. It can safely be used
// concurrently, but not modified concurrently.
type Context struct {
	// Precision is the number of places to round during rounding.
	Precision uint32
	// Rounding specifies the Rounder to use during rounding. RoundHalfUp is used if
	// nil.
	Rounding Rounder
	// MaxExponent specifies the largest effective exponent. The
	// effective exponent is the value of the Decimal in scientific notation. That
	// is, for 10e2, the effective exponent is 3 (1.0e3). Zero (0) is not a special
	// value; it does not disable this check.
	MaxExponent int32
	// MinExponent is similar to MaxExponent, but for the smallest effective
	// exponent.
	MinExponent int32
	// Traps are the conditions which will trigger an error result if the
	// corresponding Flag condition occurred.
	Traps Condition
}

const (
	// DefaultTraps is the default trap set used by BaseContext.
	DefaultTraps = SystemOverflow |
		SystemUnderflow |
		Overflow |
		Underflow |
		Subnormal |
		DivisionUndefined |
		DivisionByZero |
		DivisionImpossible |
		InvalidOperation

	errZeroPrecisionStr = "Context may not have 0 Precision for this operation"
)

// BaseContext is a useful default Context. Should not be mutated.
var BaseContext = Context{
	// Disable rounding.
	Precision: 0,
	// MaxExponent and MinExponent are set to the packages's limits.
	MaxExponent: MaxExponent,
	MinExponent: MinExponent,
	// Default error conditions.
	Traps: DefaultTraps,
}

// WithPrecision returns a copy of c but with the specified precision.
func (c *Context) WithPrecision(p uint32) *Context {
	r := *c
	r.Precision = p
	return &r
}

// goError converts flags into an error based on c.Traps.
func (c *Context) goError(flags Condition) (Condition, error) {
	return flags.GoError(c.Traps)
}

// Add sets d to the sum x+y.
func (c *Context) Add(d, x, y *Decimal) (Condition, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return 0, errors.Wrap(err, "Add")
	}
	d.Coeff.Add(a, b)
	d.Exponent = s
	return c.Round(d, d)
}

// Sub sets d to the difference x-y.
func (c *Context) Sub(d, x, y *Decimal) (Condition, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return 0, errors.Wrap(err, "Sub")
	}
	d.Coeff.Sub(a, b)
	d.Exponent = s
	return c.Round(d, d)
}

// Abs sets d to |x| (the absolute value of x).
func (c *Context) Abs(d, x *Decimal) (Condition, error) {
	d.Set(x)
	d.Coeff.Abs(&d.Coeff)
	return c.Round(d, d)
}

// Neg sets d to -x.
func (c *Context) Neg(d, x *Decimal) (Condition, error) {
	d.Neg(x)
	return c.Round(d, d)
}

// Mul sets d to the product x*y.
func (c *Context) Mul(d, x, y *Decimal) (Condition, error) {
	d.Coeff.Mul(&x.Coeff, &y.Coeff)
	res := d.setExponent(c, 0, int64(x.Exponent), int64(y.Exponent))
	res |= c.round(d, d)
	return c.goError(res)
}

// Quo sets d to the quotient x/y for y != 0. c.Precision must be > 0. If an
// exact division is required, use a context with high precision and verify
// it was exact by checking the Inexact flag on the return Condition.
func (c *Context) Quo(d, x, y *Decimal) (Condition, error) {
	if c.Precision == 0 {
		// 0 precision is disallowed because we compute the required number of digits
		// during the 10**x calculation using the precision.
		return 0, errors.New(errZeroPrecisionStr)
	}
	if c.Precision > 5000 {
		return 0, errors.New("Quo requires Precision <= 5000")
	}

	if y.Coeff.Sign() == 0 {
		// TODO(mjibson): correctly set Inf and NaN here.
		var res Condition
		if x.Coeff.Sign() == 0 {
			res |= DivisionUndefined
		} else {
			res |= DivisionByZero
		}
		return c.goError(res)
	}
	// An integer variable, adjust, is initialized to 0.
	var adjust int64
	// The result coefficient is initialized to 0.
	quo := new(Decimal)
	var res Condition
	var diff int64
	if x.Coeff.Sign() != 0 {
		dividend := new(big.Int).Abs(&x.Coeff)
		divisor := new(big.Int).Abs(&y.Coeff)

		// The operand coefficients are adjusted so that the coefficient of the
		// dividend is greater than or equal to the coefficient of the divisor and
		// is also less than ten times the coefficient of the divisor, thus:

		// While the coefficient of the dividend is less than the coefficient of
		// the divisor it is multiplied by 10 and adjust is incremented by 1.
		for dividend.Cmp(divisor) < 0 {
			dividend.Mul(dividend, bigTen)
			adjust++
		}

		// While the coefficient of the dividend is greater than or equal to ten
		// times the coefficient of the divisor the coefficient of the divisor is
		// multiplied by 10 and adjust is decremented by 1.
		for tmp := new(big.Int); ; {
			tmp.Mul(divisor, bigTen)
			if dividend.Cmp(tmp) < 0 {
				break
			}
			divisor.Set(tmp)
			adjust--
		}

		prec := int64(c.Precision)

		// The following steps are then repeated until the division is complete:
		for {
			// While the coefficient of the divisor is smaller than or equal to the
			// coefficient of the dividend the former is subtracted from the latter and
			// the coefficient of the result is incremented by 1.
			for divisor.Cmp(dividend) <= 0 {
				dividend.Sub(dividend, divisor)
				quo.Coeff.Add(&quo.Coeff, bigOne)
			}

			// If the coefficient of the dividend is now 0 and adjust is greater than
			// or equal to 0, or if the coefficient of the result has precision digits,
			// the division is complete.
			if (dividend.Sign() == 0 && adjust >= 0) || quo.NumDigits() == prec {
				break
			}

			// Otherwise, the coefficients of the result and the dividend are multiplied
			// by 10 and adjust is incremented by 1.
			quo.Coeff.Mul(&quo.Coeff, bigTen)
			dividend.Mul(dividend, bigTen)
			adjust++
		}

		// Use the adjusted exponent to determine if we are Subnormal. If so,
		// don't round.
		adj := int64(x.Exponent) + int64(-y.Exponent) - adjust + quo.NumDigits() - 1
		// Any remainder (the final coefficient of the dividend) is recorded and
		// taken into account for rounding.
		if dividend.Sign() != 0 && adj >= int64(c.MinExponent) {
			res |= Inexact | Rounded
			dividend.Mul(dividend, bigTwo)
			half := dividend.Cmp(divisor)
			rounding := c.rounding()
			if rounding(&quo.Coeff, half) {
				roundAddOne(&quo.Coeff, &diff)
			}
		}
	}

	// The exponent of the result is computed by subtracting the sum of the
	// original exponent of the divisor and the value of adjust at the end of
	// the coefficient calculation from the original exponent of the dividend.
	res |= quo.setExponent(c, res, int64(x.Exponent), int64(-y.Exponent), -adjust, diff)

	// The sign of the result is the exclusive or of the signs of the operands.
	if xn, yn := x.Sign() == -1, y.Sign() == -1; xn != yn {
		quo.Coeff.Neg(&quo.Coeff)
	}

	d.Set(quo)
	return c.goError(res)
}

// QuoInteger sets d to the integer part of the quotient x/y. If the result
// cannot fit in d.Precision digits, an error is returned.
func (c *Context) QuoInteger(d, x, y *Decimal) (Condition, error) {
	var res Condition
	if y.Coeff.Sign() == 0 {
		// TODO(mjibson): correctly set Inf and NaN here (since this is Integer
		// division, may be different or not apply like in Quo).
		if x.Coeff.Sign() == 0 {
			res |= DivisionUndefined
		} else {
			res |= DivisionByZero
		}
		return c.goError(res)
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return 0, errors.Wrap(err, "QuoInteger")
	}
	d.Coeff.Quo(a, b)
	if d.NumDigits() > int64(c.Precision) {
		res |= DivisionImpossible
	}
	d.Exponent = 0
	return c.goError(res)
}

// Rem sets d to the remainder part of the quotient x/y. If
// the integer part cannot fit in d.Precision digits, an error is returned.
func (c *Context) Rem(d, x, y *Decimal) (Condition, error) {
	var res Condition
	if y.Coeff.Sign() == 0 {
		// TODO(mjibson): correctly set Inf and NaN here (since this is Remainder
		// division, may be different or not apply like in Quo).
		if x.Coeff.Sign() == 0 {
			res |= DivisionUndefined
		} else {
			res |= InvalidOperation
		}
		return c.goError(res)
	}
	a, b, s, err := upscale(x, y)
	if err != nil {
		return 0, errors.Wrap(err, "Rem")
	}
	tmp := new(big.Int)
	tmp.QuoRem(a, b, &d.Coeff)
	if NumDigits(tmp) > int64(c.Precision) {
		res |= DivisionImpossible
	}
	d.Exponent = s
	res |= c.round(d, d)
	return c.goError(res)
}

// Sqrt sets d to the square root of x.
func (c *Context) Sqrt(d, x *Decimal) (Condition, error) {
	// See: Properly Rounded Variable Precision Square Root by T. E. Hull
	// and A. Abrham, ACM Transactions on Mathematical Software, Vol 11 #3,
	// pp229–237, ACM, September 1985.

	switch x.Coeff.Sign() {
	case -1:
		res := InvalidOperation
		return c.goError(res)
	case 0:
		d.Coeff.SetInt64(0)
		d.Exponent = 0
		return 0, nil
	}

	f := new(Decimal).Set(x)
	nd := x.NumDigits()
	e := nd + int64(x.Exponent)
	f.Exponent = int32(-nd)
	approx := new(Decimal)
	nc := c.WithPrecision(c.Precision)
	ed := NewErrDecimal(nc)
	if e%2 == 0 {
		approx.SetCoefficient(819).SetExponent(-3)
		ed.Mul(approx, approx, f)
		ed.Add(approx, approx, New(259, -3))
	} else {
		f.Exponent--
		e++
		approx.SetCoefficient(259).SetExponent(-2)
		ed.Mul(approx, approx, f)
		ed.Add(approx, approx, New(819, -4))
	}

	p := uint32(3)
	tmp := new(Decimal)
	// The algorithm in the paper says to use c.Precision + 2. 7 instead of 2
	// here allows all of the non-extended tests to pass without allowing 1ulp
	// of error or ignoring the Inexact flag, similary to the Quo precision
	// increase. This does mean that there are probably some inputs for which
	// Sqrt is 1ulp off or will incorrectly mark things as Inexact or exact.
	for maxp := c.Precision + 7; p != maxp; {
		p = 2*p - 2
		if p > maxp {
			p = maxp
		}
		nc.Precision = p
		// tmp = f / approx
		ed.Quo(tmp, f, approx)
		// tmp = approx + f / approx
		ed.Add(tmp, tmp, approx)
		// approx = 0.5 * (approx + f / approx)
		ed.Mul(approx, tmp, decimalHalf)
	}
	p = c.Precision
	nc.Precision = p
	dp := int32(p)
	approxsubhalf := new(Decimal)
	ed.Sub(approxsubhalf, approx, New(5, -1-dp))
	nc.Rounding = RoundUp
	ed.Mul(approxsubhalf, approxsubhalf, approxsubhalf)
	if approxsubhalf.Cmp(f) > 0 {
		ed.Sub(approx, approx, New(1, -dp))
	} else {
		approxaddhalf := new(Decimal)
		ed.Add(approxaddhalf, approx, New(5, -1-dp))
		nc.Rounding = RoundDown
		ed.Mul(approxaddhalf, approxaddhalf, approxaddhalf)
		if approxaddhalf.Cmp(f) < 0 {
			ed.Add(approx, approx, New(1, -dp))
		}
	}

	if err := ed.Err(); err != nil {
		return 0, err
	}

	d.Set(approx)
	d.Exponent += int32(e / 2)
	nc.Precision = c.Precision
	nc.Rounding = RoundHalfEven
	return nc.Round(d, d)
}

// Cbrt sets d to the cube root of x.
func (c *Context) Cbrt(d, x *Decimal) (Condition, error) {
	// The cube root calculation is implemented using Newton-Raphson
	// method. We start with an initial estimate for cbrt(d), and
	// then iterate:
	//     x_{n+1} = 1/3 * ( 2 * x_n + (d / x_n / x_n) ).

	// Validate the sign of x.
	switch x.Coeff.Sign() {
	case -1:
		res := InvalidOperation
		return c.goError(res)
	case 0:
		d.Coeff.SetInt64(0)
		d.Exponent = 0
		return 0, nil
	}

	z := new(Decimal).Set(x)
	nc := BaseContext.WithPrecision(c.Precision*2 + 2)
	ed := NewErrDecimal(nc)
	exp8 := 0

	// Follow Ken Turkowski paper:
	// https://people.freebsd.org/~lstewart/references/apple_tr_kt32_cuberoot.pdf
	//
	// Computing the cube root of any number is reduced to computing
	// the cube root of a number between 0.125 and 1. After the next loops,
	// x = z * 8^exp8 will hold.
	for z.Cmp(decimalOneEighth) < 0 {
		exp8--
		ed.Mul(z, z, decimalEight)
	}

	for z.Cmp(decimalOne) > 0 {
		exp8++
		ed.Mul(z, z, decimalOneEighth)
	}

	// Use this polynomial to approximate the cube root between 0.125 and 1.
	// z = (-0.46946116 * z + 1.072302) * z + 0.3812513
	// It will serve as an initial estimate, hence the precision of this
	// computation may only impact performance, not correctness.
	z0 := new(Decimal).Set(z)
	ed.Mul(z, z, decimalCbrtC1)
	ed.Add(z, z, decimalCbrtC2)
	ed.Mul(z, z, z0)
	ed.Add(z, z, decimalCbrtC3)

	for ; exp8 < 0; exp8++ {
		ed.Mul(z, z, decimalHalf)
	}

	for ; exp8 > 0; exp8-- {
		ed.Mul(z, z, decimalTwo)
	}

	z0.Set(z)

	// Loop until convergence.
	for loop := nc.newLoop("cbrt", z, 1); ; {
		// z = (2.0 * z0 +  x / (z0 * z0) ) / 3.0;
		z.Set(z0)
		ed.Mul(z, z, z0)
		ed.Quo(z, x, z)
		ed.Add(z, z, z0)
		ed.Add(z, z, z0)
		ed.Quo(z, z, decimalThree)

		if err := ed.Err(); err != nil {
			return 0, err
		}
		if done, err := loop.done(z); err != nil {
			return 0, err
		} else if done {
			break
		}
		z0.Set(z)
	}

	if err := ed.Err(); err != nil {
		return 0, err
	}
	return c.Round(d, z)
}

// Ln sets d to the natural log of x.
func (c *Context) Ln(d, x *Decimal) (Condition, error) {
	// See: On the Use of Iteration Methods for Approximating the Natural
	// Logarithm, James F. Epperson, The American Mathematical Monthly, Vol. 96,
	// No. 9, November 1989, pp. 831-835.

	if x.Sign() <= 0 {
		res := InvalidOperation
		return c.goError(res)
	}

	if x.Cmp(decimalOne) == 0 {
		d.Set(decimalZero)
		return 0, nil
	}

	// decNumber's decLnOp uses max(c.Precision, x.NumDigits, 7) + 2.
	p := c.Precision
	if p < 7 {
		p = 7
	}
	if nd := uint32(x.NumDigits()); p < nd {
		p = nd
	}
	p += 2

	// However, we must add another 5 to make all the tests pass. I think this
	// means there could be some inputs that will be rounded wrong.
	p += 5

	nc := c.WithPrecision(p)
	nc.Rounding = RoundHalfEven
	ed := NewErrDecimal(nc)

	tmp1 := new(Decimal)
	tmp2 := new(Decimal)
	tmp3 := new(Decimal)
	tmp4 := new(Decimal)

	z := new(Decimal).Set(x)

	// Reduce input range to [1/2, 1].
	m := new(Decimal)
	for z.Cmp(decimalHalf) < 0 {
		ed.Sub(m, m, decimalOne)
		ed.Mul(z, z, decimalTwo)
	}
	for z.Cmp(decimalOne) > 0 {
		ed.Add(m, m, decimalOne)
		ed.Mul(z, z, decimalHalf)
	}

	// Compute the initial estimate using the Hermite interpolation.

	// tmp1 = z - 1/2
	ed.Sub(tmp1, z, decimalHalf)
	// tmp1 = C (z - 1/2)
	ed.Mul(tmp1, tmp1, decimalLnHermiteC)
	// tmp1 = B + C (z - 1/2)
	ed.Add(tmp1, tmp1, decimalLnHermiteB)
	// tmp2 = z - 1
	ed.Sub(tmp2, z, decimalOne)
	// tmp1 = (z - 1) (B + C (z - 1/2)
	ed.Mul(tmp1, tmp1, tmp2)
	// tmp1 = A + (z - 1) (B + C (z - 1/2))
	ed.Add(tmp1, tmp1, decimalOne)
	// tmp1 = (z - 1) (A + (z - 1) (B + C (z - 1/2)))
	ed.Mul(tmp1, tmp1, tmp2)

	// Use Halley's Iteration.
	for loop := nc.newLoop("ln", x, 1); ; {
		// tmp1 = a_n (either from initial estimate or last iteration)

		// tmp2 = exp(a_n)
		ed.Exp(tmp2, tmp1)
		// tmp3 = exp(a_n) - z
		ed.Sub(tmp3, tmp2, z)
		// tmp4 = exp(a_n) + z
		ed.Add(tmp4, tmp2, z)
		// tmp2 = (exp(a_n) - z) / (exp(a_n) + z)
		ed.Quo(tmp2, tmp3, tmp4)
		// tmp2 = 2 * (exp(a_n) - z) / (exp(a_n) + z)
		ed.Mul(tmp2, tmp2, decimalTwo)
		// tmp2 = a_n+1 = a_n - 2 * (exp(a_n) - z) / (exp(a_n) + z)
		ed.Sub(tmp2, tmp1, tmp2)

		if done, err := loop.done(tmp2); err != nil {
			return 0, err
		} else if done {
			break
		}
		if err := ed.Err(); err != nil {
			return 0, err
		}

		tmp1.Set(tmp2)
	}

	// tmp2 = m * ln(2)
	// Disable Subnormal because decimalLog2 is so long.
	nc.Traps ^= Subnormal
	ed.Mul(tmp2, m, decimalLog2)
	ed.Add(tmp1, tmp1, tmp2)

	if err := ed.Err(); err != nil {
		return 0, err
	}
	res := c.round(d, tmp1)
	res |= Inexact
	return c.goError(res)
}

// Log10 sets d to the base 10 log of x.
func (c *Context) Log10(d, x *Decimal) (Condition, error) {
	if x.Sign() <= 0 {
		res := InvalidOperation
		return c.goError(res)
	}

	if x.Cmp(decimalOne) == 0 {
		d.Set(decimalZero)
		return 0, nil
	}

	// TODO(mjibson): This is exact under some conditions.
	res := Inexact

	// decNumber's decLnOp uses max(c.Precision, x.NumDigits + t) + 3, where t
	// is the number of digits in the exponent. However, they hardcode t to just 6.
	p := c.Precision
	if nd := uint32(x.NumDigits() + 6); p < nd {
		p = nd
	}
	p += 3

	nc := BaseContext.WithPrecision(p)
	nc.Rounding = RoundHalfEven
	z := new(Decimal)
	_, err := nc.Ln(z, x)
	if err != nil {
		return 0, errors.Wrap(err, "ln")
	}
	nc.Precision = c.Precision
	qr, err := nc.Quo(d, z, decimalLog10)
	if err != nil {
		return 0, err
	}
	res |= qr
	return c.goError(res)
}

// Exp sets d = e**x.
func (c *Context) Exp(d, x *Decimal) (Condition, error) {
	// See: Variable Precision Exponential Function, T. E. Hull and A. Abrham, ACM
	// Transactions on Mathematical Software, Vol 12 #2, pp79-91, ACM, June 1986.

	if x.Coeff.Sign() == 0 {
		d.Set(decimalOne)
		return 0, nil
	}

	if c.Precision == 0 {
		return 0, errors.New(errZeroPrecisionStr)
	}

	nc := c.WithPrecision(c.Precision)
	nc.Rounding = RoundHalfEven
	res := Inexact | Rounded

	// Stage 1
	cp := int64(c.Precision)
	tmp1 := new(Decimal).Abs(x)
	tmp2 := New(cp*23, 0)
	// if abs(x) > 23*currentprecision; assert false
	if tmp1.Cmp(tmp2) > 0 {
		res |= Overflow
		if x.Sign() < 0 {
			res = res.negateOverflowFlags()
		}
		return c.goError(res)
	}
	// if abs(x) <= setexp(.9, -currentprecision); then result 1
	tmp2.SetCoefficient(9).SetExponent(int32(-cp) - 1)
	if tmp1.Cmp(tmp2) <= 0 {
		d.Set(decimalOne)
		return c.goError(res)
	}

	// Stage 2
	// Add x.NumDigits because the paper assumes that x.Coeff [0.1, 1).
	t := x.Exponent + int32(x.NumDigits())
	if t < 0 {
		t = 0
	}
	k := New(1, t)
	r := new(Decimal)
	if _, err := nc.Quo(r, x, k); err != nil {
		return 0, errors.Wrap(err, "QuoInteger")
	}
	ra := new(Decimal).Abs(r)
	p := cp + int64(t) + 2

	// Stage 3
	rf, err := ra.Float64()
	if err != nil {
		return 0, errors.Wrap(err, "r.Float64")
	}
	pf := float64(p)
	nf := math.Ceil((1.435*pf - 1.182) / math.Log10(pf/rf))
	if nf > 1000 || math.IsNaN(nf) {
		return 0, errors.New("too many iterations")
	}
	n := int64(nf)

	// Stage 4
	nc.Precision = uint32(p)
	ed := NewErrDecimal(nc)
	sum := New(1, 0)
	tmp2.Exponent = 0
	for i := n - 1; i > 0; i-- {
		tmp2.SetCoefficient(i)
		// tmp1 = r / i
		ed.Quo(tmp1, r, tmp2)
		// sum = sum * r / i
		ed.Mul(sum, tmp1, sum)
		// sum = sum + 1
		ed.Add(sum, sum, decimalOne)
	}
	if err != ed.Err() {
		return 0, err
	}

	// sum ** k
	_, ki, err := exp10(int64(t))
	if err != nil {
		return 0, errors.Wrap(err, "ki")
	}
	if _, err := nc.integerPower(d, sum, ki); err != nil {
		return 0, errors.Wrap(err, "integer power")
	}
	nc.Precision = c.Precision
	res |= nc.round(d, d)
	return c.goError(res)
}

// integerPower sets d = x**y.
func (c *Context) integerPower(d, x *Decimal, y *big.Int) (Condition, error) {
	// See: https://en.wikipedia.org/wiki/Exponentiation_by_squaring.

	b := new(big.Int).Set(y)
	neg := b.Sign() < 0
	if neg {
		b.Abs(b)
	}

	n, z := new(Decimal), d
	n.Set(x)
	z.Set(decimalOne)
	ed := NewErrDecimal(c)
	for b.Sign() > 0 {
		if b.Bit(0) == 1 {
			ed.Mul(z, z, n)
		}
		b.Rsh(b, 1)

		ed.Mul(n, n, n)
		if err := ed.Err(); err != nil {
			// In the negative case, convert overflow to underflow.
			if neg {
				ed.Flags = ed.Flags.negateOverflowFlags()
			}
			return ed.Flags, err
		}
	}

	if neg {
		e := z.Exponent
		if e < 0 {
			e = -e
		}
		qc := c.WithPrecision((uint32(z.NumDigits()) + uint32(e)) * 2)
		ed.Ctx = qc
		ed.Quo(z, decimalOne, z)
		ed.Ctx = c
	}
	return ed.Flags, ed.Err()
}

// Pow sets d = x**y.
func (c *Context) Pow(d, x, y *Decimal) (Condition, error) {
	// x ** 1 == x
	if y.Cmp(decimalOne) == 0 {
		return c.Round(d, x)
	}
	// 1 ** x == 1
	if x.Cmp(decimalOne) == 0 {
		return c.Round(d, x)
	}

	// Check if y is of type int.
	tmp := new(Decimal)
	if _, err := c.Abs(tmp, y); err != nil {
		return 0, errors.Wrap(err, "Abs")
	}
	integ, frac := new(Decimal), new(Decimal)
	tmp.Modf(integ, frac)
	yIsInt := frac.Sign() == 0

	xs := x.Sign()
	ys := y.Sign()

	if xs == 0 {
		switch ys {
		case 0:
			d.Set(decimalOne)
			return 0, nil
		case 1:
			d.Set(decimalZero)
			return 0, nil
		default: // -1
			res := InvalidOperation
			return c.goError(res)

		}
	}
	if ys == 0 {
		d.Set(decimalOne)
		return 0, nil
	}

	if (xs == 0 && ys == 0) || (xs < 0 && !yIsInt) {
		res := InvalidOperation
		return c.goError(res)
	}

	// decNumber sets the precision to be max(x digits + exponent, c.Precision)
	// + 4. 6 is used as the exponent digits.
	p := c.Precision
	if nd := uint32(x.NumDigits()) + 6; p < nd {
		p = nd
	}
	p += 4

	nc := BaseContext.WithPrecision(p)
	ed := NewErrDecimal(nc)

	ed.Abs(tmp, x)
	ed.Ln(tmp, tmp)
	ed.Mul(tmp, tmp, y)
	ed.Exp(tmp, tmp)

	if xs < 0 && integ.Coeff.Bit(0) == 1 && integ.Exponent == 0 {
		ed.Neg(tmp, tmp)
	}

	if err := ed.Err(); err != nil {
		return ed.Flags, err
	}
	res := c.round(d, tmp)
	if !yIsInt {
		res |= Inexact
	}
	return c.goError(res)
}

// Quantize sets d to the value of v with x's Exponent.
func (c *Context) Quantize(d, v, x *Decimal) (Condition, error) {
	res := c.quantize(d, v, x)
	if nd := d.NumDigits(); nd > int64(c.Precision) {
		res |= InvalidOperation
	}
	res |= c.round(d, d)
	return c.goError(res)
}

func (c *Context) quantize(d, v, e *Decimal) Condition {
	d.Coeff.Set(&v.Coeff)
	diff := e.Exponent - v.Exponent
	var res Condition
	var offset int32
	if diff < 0 {
		if diff < MinExponent {
			return SystemUnderflow | Underflow
		}
		y := big.NewInt(-int64(diff))
		e := new(big.Int).Exp(bigTen, y, nil)
		d.Coeff.Mul(&d.Coeff, e)
	} else if diff > 0 {
		p := int32(d.NumDigits()) - diff
		if p < 0 {
			if d.Sign() != 0 {
				d.Coeff.SetInt64(0)
				res = Inexact | Rounded
			}
		} else {
			nc := c.WithPrecision(uint32(p))
			neg := d.Sign() < 0
			// Avoid the c.Precision == 0 check.
			res = nc.Rounding.Round(nc, d, d)
			offset = d.Exponent - diff
			// TODO(mjibson): There may be a bug in roundAddOne or roundFunc that
			// unexpectedly removes a negative sign when converting from -9 to -10. This
			// check is needed until it is fixed.
			if neg && d.Sign() > 0 {
				d.Coeff.Neg(&d.Coeff)
			}
		}
	}
	d.Exponent = e.Exponent + offset
	return res
}

func (c *Context) toIntegral(d, x *Decimal) Condition {
	res := c.quantize(d, x, decimalOne)
	// TODO(mjibson): trim here, once trim is in
	return res
}

// ToIntegral sets d to integral value of x. Inexact and Rounded flags are ignored and removed.
func (c *Context) ToIntegral(d, x *Decimal) (Condition, error) {
	res := c.toIntegral(d, x)
	res &= ^(Inexact | Rounded)
	return c.goError(res)
}

// ToIntegralX sets d to integral value of x.
func (c *Context) ToIntegralX(d, x *Decimal) (Condition, error) {
	res := c.toIntegral(d, x)
	return c.goError(res)
}

// Ceil sets d to the smallest integer >= x.
func (c *Context) Ceil(d, x *Decimal) (Condition, error) {
	frac := new(Decimal)
	x.Modf(d, frac)
	if frac.Sign() > 0 {
		return c.Add(d, d, decimalOne)
	}
	return 0, nil
}

// Floor sets d to the largest integer <= x.
func (c *Context) Floor(d, x *Decimal) (Condition, error) {
	frac := new(Decimal)
	x.Modf(d, frac)
	if frac.Sign() < 0 {
		return c.Sub(d, d, decimalOne)
	}
	return 0, nil
}

// Reduce sets d to x with all trailing zeros removed.
func (c *Context) Reduce(d, x *Decimal) (Condition, error) {
	d.Reduce(x)
	return c.Round(d, d)
}

// exp10 returns x, 1*10^x. An error is returned if x is too large.
func exp10(x int64) (f, exp *big.Int, err error) {
	if x > MaxExponent || x < MinExponent {
		return nil, nil, errors.New(errExponentOutOfRangeStr)
	}
	f = big.NewInt(x)
	return f, new(big.Int).Exp(bigTen, f, nil), nil
}
