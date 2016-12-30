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
	"math/big"

	"github.com/pkg/errors"
)

// Context maintains options for Decimal operations.
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
}

// BaseContext is a useful default Context.
var BaseContext = Context{
	// Disable rounding.
	Precision: 0,
	// MaxExponent and MinExponent are set to the packages's limits.
	MaxExponent: MaxExponent,
	MinExponent: MinExponent,
}

// WithPrecision returns a copy of c but with the specified precision.
func (c *Context) WithPrecision(p uint32) Context {
	r := *c
	r.Precision = p
	return r
}

// Add sets d to the sum x+y.
func (c *Context) Add(d, x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Add")
	}
	d.Coeff.Add(a, b)
	d.Exponent = s
	return c.Round(d, d).GoError()
}

// Sub sets d to the difference x-y.
func (c *Context) Sub(d, x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Sub")
	}
	d.Coeff.Sub(a, b)
	d.Exponent = s
	return c.Round(d, d).GoError()
}

// Abs sets d to |x| (the absolute value of x).
func (c *Context) Abs(d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Abs(&d.Coeff)
	return c.Round(d, d).GoError()
}

// Neg sets z to -x.
func (c *Context) Neg(d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Neg(&d.Coeff)
	return c.Round(d, d).GoError()
}

// Mul sets d to the product x*y.
func (c *Context) Mul(d, x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Mul")
	}
	d.Coeff.Mul(a, b)
	d.Exponent = s * 2
	return c.Round(d, d).GoError()
}

// Quo sets d to the quotient x/y for y != 0. c's Precision must be > 0.
func (c *Context) Quo(d, x, y *Decimal) error {
	if c.Precision == 0 {
		// 0 precision is disallowed because we compute the required number of digits
		// during the 10**x calculation using the precision.
		return errors.New("Quo requires a Context with > 0 Precision")
	}

	if y.Coeff.Sign() == 0 {
		return errDivideByZero
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Quo")
	}

	// In order to compute the decimal remainder part, add enough 0s to the
	// numerator to accurately round with the given precision.
	// TODO(mjibson): determine a better algorithm for this instead of p*2+8.
	nc := BaseContext.WithPrecision(c.Precision*2 + 8)
	f := big.NewInt(int64(nc.Precision))
	e := new(big.Int).Exp(bigTen, f, nil)
	f.Mul(a, e)
	d.Coeff.Quo(f, b)
	if err := d.setExponent(c, -int64(nc.Precision)).GoError(); err != nil {
		return err
	}
	return c.Round(d, d).GoError()
}

// QuoInteger sets d to the integer part of the quotient x/y. If the result
// cannot fit in d.Precision digits, an error is returned.
func (c *Context) QuoInteger(d, x, y *Decimal) error {
	if y.Coeff.Sign() == 0 {
		return errDivideByZero
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "QuoInteger")
	}
	d.Coeff.Quo(a, b)
	if d.numDigits() > int64(c.Precision) {
		return errIntegerDivisionImpossible
	}
	d.Exponent = 0
	return err
}

// Rem sets d to the remainder part of the quotient x/y. If
// the integer part cannot fit in d.Precision digits, an error is returned.
func (c *Context) Rem(d, x, y *Decimal) error {
	if y.Coeff.Sign() == 0 {
		return errDivideByZero
	}
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Rem")
	}
	tmp := new(big.Int)
	tmp.QuoRem(a, b, &d.Coeff)
	if numDigits(tmp) > int64(c.Precision) {
		return errIntegerDivisionImpossible
	}
	d.Exponent = s
	return c.Round(d, d).GoError()
}

// Sqrt sets d to the square root of x.
func (c *Context) Sqrt(d, x *Decimal) error {
	// The square root calculation is implemented using Newton's Method.
	// We start with an initial estimate for sqrt(d), and then iterate:
	//     x_{n+1} = 1/2 * ( x_n + (d / x_n) ).

	// Validate the sign of x.
	switch x.Coeff.Sign() {
	case -1:
		return errSqrtNegative
	case 0:
		d.Coeff.SetInt64(0)
		d.Exponent = 0
		return nil
	}

	// Use half as the initial estimate.
	z := new(Decimal)
	nc := BaseContext.WithPrecision(c.Precision*2 + 2)
	ed := NewErrDecimal(&nc)
	ed.Mul(z, x, decimalHalf)

	// Iterate.
	tmp := new(Decimal)
	for loop := nc.newLoop("sqrt", z, 1); ; {
		ed.Quo(tmp, x, z)           // t = d / x_n
		ed.Add(tmp, tmp, z)         // t = x_n + (d / x_n)
		ed.Mul(z, tmp, decimalHalf) // x_{n+1} = 0.5 * t

		if err := ed.Err(); err != nil {
			return err
		}
		if done, err := loop.done(z); err != nil {
			return err
		} else if done {
			break
		}
	}

	if err := ed.Err(); err != nil {
		return err
	}
	return c.Round(d, z).GoError()
}

// Ln sets d to the natural log of x.
func (c *Context) Ln(d, x *Decimal) error {
	// Validate the sign of x.
	if x.Sign() <= 0 {
		return errors.Errorf("natural log of non-positive value: %s", x)
	}

	// Attempt to make our precision high enough so that intermediate calculations
	// will produce enough data to have a correct output at the end. The constants
	// here were found experimentally and are sufficient to pass many of the
	// GDA tests, however this may still fail to produce accurate results for
	// some inputs.
	// TODO(mjibson): figure out an algorithm that can correctly determine this
	// for all inputs.
	p := c.Precision
	if p < 15 {
		p = 15
	}
	p *= 4
	nc := BaseContext.WithPrecision(p)
	xr := new(Decimal)

	fact := New(2, 0)
	ed := NewErrDecimal(&nc)

	// Use the Taylor series approximation:
	//
	//   r = (x - 1) / (x + 1)
	//   ln(x) = 2 * [ r + r^3 / 3 + r^5 / 5 + ... ]

	// The taylor series of ln(x) converges much faster if 0.9 < x < 1.1. We
	// can use the logarithmic identity:
	// log_b (sqrt(x)) = log_b (x) / 2
	// Thus, successively square-root x until it is in that region. Keep track
	// of how many square-rootings were done using fact and multiply at the end.
	xr.Set(x)
	for ed.Cmp(xr, decimalZeroPtNine) < 0 || ed.Cmp(xr, decimalOnePtOne) > 0 {
		nc.Precision += p
		ed.Sqrt(xr, xr)
		ed.Mul(fact, fact, decimalTwo)
	}
	if err := ed.Err(); err != nil {
		return err
	}

	tmp1 := new(Decimal)
	tmp2 := new(Decimal)
	elem := new(Decimal)
	numerator := new(Decimal)
	z := new(Decimal)

	// tmp1 = x + 1
	ed.Add(tmp1, xr, decimalOne)
	// tmp2 = x - 1
	ed.Sub(tmp2, xr, decimalOne)
	// elem = r = (x - 1) / (x + 1)
	ed.Quo(elem, tmp2, tmp1)
	// z will be the result. Initialize to elem.
	z.Set(elem)
	numerator.Set(elem)
	// elem = r^2 = ((x - 1) / (x + 1)) ^ 2
	// Used since the series uses only odd powers of z.
	ed.Mul(elem, elem, elem)
	tmp1.Exponent = 0
	if err := ed.Err(); err != nil {
		return err
	}
	for loop := nc.newLoop("log", z, 40); ; {
		// tmp1 = n, the i'th odd power: 3, 5, 7, 9, etc.
		tmp1.SetInt64(int64(loop.i)*2 + 3)
		// numerator = r^n
		ed.Mul(numerator, numerator, elem)
		// tmp2 = r^n / n
		ed.Quo(tmp2, numerator, tmp1)
		// z += r^n / n
		ed.Add(z, z, tmp2)
		if done, err := loop.done(z); err != nil {
			return err
		} else if done {
			break
		}
		if err := ed.Err(); err != nil {
			return err
		}
	}

	// Undo input range reduction.
	ed.Mul(z, z, fact)
	if err := ed.Err(); err != nil {
		return err
	}

	// Round to the desired scale.
	return c.Round(d, z).GoError()
}

// Log10 sets d to the base 10 log of x.
func (c *Context) Log10(d, x *Decimal) error {
	nc := BaseContext.WithPrecision(c.Precision * 2)
	z := new(Decimal)
	err := nc.Ln(z, x)
	if err != nil {
		return errors.Wrap(err, "ln")
	}
	return c.Quo(d, z, decimalLog10)
}

// Exp sets d = e**n.
func (c *Context) Exp(d, n *Decimal) error {
	// We are computing (e^n) by splitting n into an integer and a float (e.g
	// 3.1 ==> x = 3, y = 0.1), this allows us to write e^n = e^(x+y) = e^x * e^y

	integ := new(Decimal)
	frac := new(Decimal)
	n.Modf(integ, frac)

	if integ.Exponent > 0 {
		y := big.NewInt(int64(integ.Exponent))
		e := new(big.Int).Exp(bigTen, y, nil)
		integ.Coeff.Mul(&integ.Coeff, e)
		integ.Exponent = 0
	}

	z := new(Decimal)
	nc := BaseContext.WithPrecision(c.Precision * 2)
	if err := nc.integerPower(z, decimalE, &integ.Coeff); err != nil {
		return errors.Wrap(err, "IntegerPower")
	}
	return c.smallExp(d, z, frac)
}

// smallExp sets d = x * e**y. It should be used with small y values only
// (|y| < 1).
func (c *Context) smallExp(d, x, y *Decimal) error {
	n := new(Decimal)
	e := x.Exponent
	if e < 0 {
		e = -e
	}
	nc := BaseContext.WithPrecision((uint32(x.numDigits()) + uint32(e)))
	if p := c.Precision * 2; nc.Precision < p {
		nc.Precision = p
	}
	ed := NewErrDecimal(&nc)
	z := d
	tmp := new(Decimal)
	z.Set(x)
	tmp.Set(x)
	for loop := nc.newLoop("exp", z, 1); ; {
		if err := ed.Err(); err != nil {
			return err
		}
		if done, err := loop.done(z); err != nil {
			return err
		} else if done {
			break
		}
		ed.Add(n, n, decimalOne)
		ed.Mul(tmp, tmp, y)
		ed.Quo(tmp, tmp, n)
		ed.Add(z, z, tmp)
	}
	if err := ed.Err(); err != nil {
		return err
	}
	return c.Round(d, z).GoError()
}

// integerPower sets d = x**y.
// For integers we use exponentiation by squaring.
// See: https://en.wikipedia.org/wiki/Exponentiation_by_squaring
func (c *Context) integerPower(d, x *Decimal, y *big.Int) error {
	b := new(big.Int).Set(y)
	neg := b.Sign() < 0
	if neg {
		b.Abs(b)
	}

	nc := BaseContext.WithPrecision(c.Precision * 2)
	n, z := new(Decimal), d
	n.Set(x)
	z.Set(decimalOne)
	ed := NewErrDecimal(&nc)
	for b.Sign() > 0 {
		if b.Bit(0) == 1 {
			ed.Mul(z, z, n)
		}
		b.Rsh(b, 1)

		ed.Mul(n, n, n)
		if err := ed.Err(); err != nil {
			return err
		}
	}

	if neg {
		e := z.Exponent
		if e < 0 {
			e = -e
		}
		qc := nc.WithPrecision((uint32(z.numDigits()) + uint32(e)) * 2)
		ed.Ctx = &qc
		ed.Quo(z, decimalOne, z)
		ed.Ctx = &nc
	}
	return ed.Err()
}

// Pow sets d = x**y.
func (c *Context) Pow(d, x, y *Decimal) error {
	// x ** 1 == x
	if p, err := y.Cmp(decimalOne); err != nil {
		return err
	} else if p == 0 {
		return c.Round(d, x).GoError()
	}
	// 1 ** x == 1
	if p, err := x.Cmp(decimalOne); err != nil {
		return err
	} else if p == 0 {
		return c.Round(d, x).GoError()
	}

	// maxPrecision is the largest number of decimal digits (sum of number of
	// digits before and after the decimal point) before an errArgumentTooLarge
	// is returned for any computation.
	const maxPrecision = 2000

	// Check if y is of type int.
	tmp := new(Decimal)
	if err := c.Abs(tmp, y); err != nil {
		return errors.Wrap(err, "Abs")
	}
	integ, frac := new(Decimal), new(Decimal)
	tmp.Modf(integ, frac)
	isInt := frac.Sign() == 0

	xs := x.Sign()
	if xs == 0 {
		switch y.Sign() {
		case 0:
			d.Set(decimalOne)
			return nil
		case 1:
			d.Set(decimalZero)
			return nil
		default: // -1
			// undefined for y < 0
			return errors.Wrapf(errPowZeroNegative, "%s", y)
		}
	}

	neg := xs < 0

	if !isInt && neg {
		return errors.Wrapf(errPowNegNonInteger, "%s**%s", x, y)
	}

	// Exponent Precision Explanation (RaduBerinde):
	// Say we compute the Log with a scale of k. That means that the result we
	// get is:
	//   ln x +/- 10^-k
	// This leads to an error of y * 10^-k in the exponent, which leads to a
	// multiplicative error of e^(y*10^-k) in the result. For small values of u,
	// e^u can be approximated by 1 + u, so for large k that error is around 1
	// + y*10^-k. So the additive error will be x^y * y * 10^-k, and we want
	// this to be less than 10^-s. This approximately means that k has to be
	// s + the number of digits before the decimal point in x^y (where s =
	// d.Precision). Which roughly is
	//   s + <the number of digits before decimal point in x> * y

	x.Modf(tmp, frac)
	// numDigits = <the number of digits before decimal point in x>
	numDigits := tmp.numDigits()

	var ed ErrDecimal

	// numDigits *= y
	numDigits *= ed.Int64(integ)
	// numDigits += s
	numDigits += int64(c.Precision)
	numDigits += 2

	if numDigits < 0 || numDigits > maxPrecision {
		return errors.New(errExponentOutOfRange)
	}
	nc := BaseContext.WithPrecision(uint32(numDigits))
	ed.Ctx = &nc

	ed.Abs(tmp, x)
	ed.Ln(tmp, tmp)
	ed.Mul(tmp, tmp, y)
	ed.Exp(tmp, tmp)

	if neg && integ.Coeff.Bit(0) == 1 && integ.Exponent == 0 {
		ed.Neg(tmp, tmp)
	}

	if err := ed.Err(); err != nil {
		return err
	}
	return c.Round(d, tmp).GoError()
}
