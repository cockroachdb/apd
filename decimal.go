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

// Package apd implements arbitrary-precision decimals.
package apd

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Decimal is an arbitrary-precision decimal. Its value is:
//
//     Coeff * 10 ^ Exponent
//
type Decimal struct {
	Coeff    big.Int
	Exponent int32
}

// New creates a new decimal with the given coefficient and exponent.
func New(coeff int64, exponent int32) *Decimal {
	return &Decimal{
		Coeff:    *big.NewInt(coeff),
		Exponent: exponent,
	}
}

func newFromString(s string) (coeff *big.Int, exps []int64, err error) {
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "parse exponent: %s", s[i+1:])
		}
		exps = append(exps, exp)
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp := int64(len(s) - i - 1)
		exps = append(exps, -exp)
		s = s[:i] + s[i+1:]
	}
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, nil, errors.Errorf("parse mantissa: %s", s)
	}
	return i, exps, nil
}

// NewFromString creates a new decimal from s. It has no restrictions on
// exponents or precision.
func NewFromString(s string) (*Decimal, error) {
	i, exps, err := newFromString(s)
	if err != nil {
		return nil, err
	}
	d := &Decimal{
		Coeff: *i,
	}
	_, err = d.setExponent(&BaseContext, exps...).GoError(BaseContext.Traps)
	return d, err
}

// SetString set's d to s and returns d. It has no restrictions on exponents
// or precision.
func (d *Decimal) SetString(s string) (*Decimal, error) {
	i, exps, err := newFromString(s)
	if err != nil {
		return d, err
	}
	d.Coeff = *i
	_, err = d.setExponent(&BaseContext, exps...).GoError(BaseContext.Traps)
	return d, err
}

// NewFromString creates a new decimal from s. The returned Decimal has its
// exponents restricted by the context and its value rounded if it contains more
// digits than the context's precision.
func (c *Context) NewFromString(s string) (*Decimal, Condition, error) {
	i, exps, err := newFromString(s)
	if err != nil {
		return nil, 0, err
	}
	d := &Decimal{
		Coeff: *i,
	}
	res := d.setExponent(c, exps...)
	res |= c.round(d, d)
	_, err = res.GoError(c.Traps)
	return d, res, err
}

// String is a wrapper of ToSci.
func (d *Decimal) String() string {
	return d.ToSci()
}

// ToSci returns d in scientific notation if an exponent is needed.
func (d *Decimal) ToSci() string {
	s := d.Coeff.String()
	if s == "0" {
		return s
	}
	neg := d.Coeff.Sign() < 0
	if neg {
		s = s[1:]
	}
	adj := int(d.Exponent) + (len(s) - 1)
	if d.Exponent <= 0 && adj >= -6 {
		if d.Exponent < 0 {
			if left := -int(d.Exponent) - len(s); left > 0 {
				s = "0." + strings.Repeat("0", left) + s
			} else if left < 0 {
				offset := -left
				s = s[:offset] + "." + s[offset:]
			} else {
				s = "0." + s
			}
		}
	} else {
		dot := ""
		if len(s) > 1 {
			dot = "." + s[1:]
		}
		s = fmt.Sprintf("%s%sE%+d", s[:1], dot, adj)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ToStandard converts d to a standard notation string (i.e., no exponent
// part). This can result in long strings given large exponents.
func (d *Decimal) ToStandard() string {
	s := d.Coeff.String()
	if d.Exponent < 0 {
		if left := -int(d.Exponent) - len(s); left > 0 {
			s = "0." + strings.Repeat("0", left) + s
		} else if left < 0 {
			offset := -left
			s = s[:offset] + "." + s[offset:]
		} else {
			s = "0." + s
		}
	} else if d.Exponent > 0 {
		s += strings.Repeat("0", int(d.Exponent))
	}
	return s
}

// Set sets d's Coefficient and Exponent from x and returns d.
func (d *Decimal) Set(x *Decimal) *Decimal {
	d.Coeff.Set(&x.Coeff)
	d.Exponent = x.Exponent
	return d
}

// SetCoefficient sets d's Coefficient value to x and returns d. The Exponent
// is not changed.
func (d *Decimal) SetCoefficient(x int64) *Decimal {
	d.Coeff.SetInt64(x)
	return d
}

// SetExponent sets d's Exponent value to x and returns d.
func (d *Decimal) SetExponent(x int32) *Decimal {
	d.Exponent = x
	return d
}

// SetFloat64 sets d's Coefficient and Exponent to x and returns d. d will
// hold the exact value of f.
func (d *Decimal) SetFloat64(f float64) (*Decimal, error) {
	return d.SetString(strconv.FormatFloat(f, 'E', -1, 64))
}

// Int64 returns the int64 representation of x. If x cannot be represented in an int64, an error is returned.
func (d *Decimal) Int64() (int64, error) {
	integ, frac := new(Decimal), new(Decimal)
	d.Modf(integ, frac)
	if frac.Sign() != 0 {
		return 0, errors.Errorf("%s: has fractional part", d)
	}
	var ed ErrDecimal
	if integ.Cmp(New(math.MaxInt64, 0)) > 0 {
		return 0, errors.Errorf("%s: greater than max int64", d)
	}
	if integ.Cmp(New(math.MinInt64, 0)) < 0 {
		return 0, errors.Errorf("%s: less than min int64", d)
	}
	if err := ed.Err(); err != nil {
		return 0, err
	}
	v := integ.Coeff.Int64()
	for i := int32(0); i < integ.Exponent; i++ {
		v *= 10
	}
	return v, nil
}

// Float64 returns the float64 representation of x. This conversion may lose
// data (see strconv.ParseFloat for caveats).
func (d *Decimal) Float64() (float64, error) {
	return strconv.ParseFloat(d.String(), 64)
}

const (
	errExponentOutOfRange = "exponent out of range"
)

// setExponent sets d's Exponent to the sum of xs. Each value and the sum
// of xs must fit within an int32. An error occurs if the sum is outside of
// the MaxExponent or MinExponent range.
func (d *Decimal) setExponent(c *Context, xs ...int64) Condition {
	var sum int64
	for _, x := range xs {
		if x > MaxExponent {
			return SystemOverflow | Overflow
		}
		if x < MinExponent {
			return SystemUnderflow | Underflow
		}
		sum += x
	}
	r := int32(sum)

	// adj is the adjusted exponent: exponent + clength - 1
	adj := sum + d.NumDigits() - 1
	// Make sure it is less than the system limits.
	if adj > MaxExponent {
		return SystemOverflow | Overflow
	}
	if adj < MinExponent {
		return SystemUnderflow | Underflow
	}
	v := int32(adj)

	var res Condition
	// d is subnormal.
	if v < c.MinExponent {
		res |= Subnormal
		Etiny := c.MinExponent - (int32(c.Precision) - 1)
		// Only need to round if exponent < Etiny.
		if r < Etiny {
			// Round to keep any precision we can while changing the exponent to Etiny.
			np := v - Etiny + 1
			if np < 0 {
				np = 0
			}
			nc := c.WithPrecision(uint32(np))
			b := new(big.Int).Set(&d.Coeff)
			tmp := &Decimal{
				Coeff: *b,
			}
			// Ignore the Precision == 0 check by using the Rounding directly.
			res |= nc.Rounding(nc, tmp, tmp)
			if res.Inexact() {
				res |= Underflow
			}
			d.Coeff = tmp.Coeff
			r = Etiny
		}
	} else if v > c.MaxExponent {
		res |= Overflow
	}

	d.Exponent = r
	return res
}

const (
	// TODO(mjibson): MaxExponent is set because both upscale and Round
	// perform a calculation of 10^x, where x is an exponent. This is done by
	// big.Int.Exp. This restriction could be lifted if better algorithms were
	// determined during upscale and Round that don't need to perform Exp.

	// MaxExponent is the highest exponent supported. Exponents near this range will
	// perform very slowly (many seconds per operation).
	MaxExponent = 100000
	// MinExponent is the lowest exponent supported with the same limitations as
	// MaxExponent.
	MinExponent = -MaxExponent
)

// upscale converts a and b to big.Ints with the same scaling, and their
// scaling. An error can be produced if the resulting scale factor is out
// of range.
func upscale(a, b *Decimal) (*big.Int, *big.Int, int32, error) {
	if a.Exponent == b.Exponent {
		return &a.Coeff, &b.Coeff, a.Exponent, nil
	}
	swapped := false
	if a.Exponent < b.Exponent {
		swapped = true
		b, a = a, b
	}
	s := int64(a.Exponent) - int64(b.Exponent)
	// TODO(mjibson): figure out a better way to upscale numbers with highly
	// differing exponents.
	if s > MaxExponent {
		return nil, nil, 0, errors.New(errExponentOutOfRange)
	}
	y := big.NewInt(s)
	e := new(big.Int).Exp(bigTen, y, nil)
	y.Mul(&a.Coeff, e)
	x := &b.Coeff
	if swapped {
		x, y = y, x
	}
	return y, x, b.Exponent, nil
}

// Cmp compares d and x and returns:
//
//   -1 if d <  x
//    0 if d == x
//   +1 if d >  x
//
func (d *Decimal) Cmp(x *Decimal) int {
	// First compare signs.
	ds := d.Sign()
	xs := x.Sign()
	if ds < xs {
		return -1
	} else if ds > xs {
		return 1
	} else if ds == 0 && xs == 0 {
		return 0
	}

	// Next compare adjusted exponents.
	dn := d.NumDigits() + int64(d.Exponent)
	xn := x.NumDigits() + int64(x.Exponent)
	if dn < xn {
		// Swap in the negative case.
		if ds < 0 {
			return 1
		}
		return -1
	} else if dn > xn {
		if ds < 0 {
			return -1
		}
		return 1
	}

	// Now have to use aligned big.Ints. This function previously used upscale to
	// align in all cases, but that requires an error in the return value. upscale
	// does that so that it can fail if it needs to take the Exp of too-large a
	// number, which is very slow. The only way for that to happen here is for d
	// and x's coefficients to be of hugely differing values. That is practically
	// more difficult, so we are assuming the user is already comfortable with
	// slowness in those operations.

	// Convert to int64 to guarantee the following arithmetic will succeed.
	diff := int64(d.Exponent) - int64(x.Exponent)
	if diff < 0 {
		diff = -diff
	}
	y := big.NewInt(diff)
	e := new(big.Int).Exp(bigTen, y, nil)
	db := new(big.Int).Set(&d.Coeff)
	xb := new(big.Int).Set(&x.Coeff)
	if d.Exponent > x.Exponent {
		db.Mul(db, e)
	} else {
		xb.Mul(xb, e)
	}
	return db.Cmp(xb)
}

// Sign returns:
//
//	-1 if d <  0
//	 0 if d == 0
//	+1 if d >  0
//
func (d *Decimal) Sign() int {
	return d.Coeff.Sign()
}

// Modf sets integ to the integral part of d and frac to the fractional part
// such that d = integ+frac. If d is negative, both integ or frac will be
// either 0 or negative. integ.Exponent will be >= 0; frac.Exponent will be
// <= 0.
func (d *Decimal) Modf(integ, frac *Decimal) {
	// No fractional part.
	if d.Exponent > 0 {
		frac.Exponent = 0
		frac.SetCoefficient(0)
		integ.Set(d)
		return
	}
	nd := d.NumDigits()
	exp := -int64(d.Exponent)
	// d < 0 because exponent is larger than number of digits.
	if exp > nd {
		integ.Exponent = 0
		integ.SetCoefficient(0)
		frac.Set(d)
		return
	}

	y := big.NewInt(exp)
	e := new(big.Int).Exp(bigTen, y, nil)
	integ.Coeff.QuoRem(&d.Coeff, e, &frac.Coeff)
	integ.Exponent = 0
	frac.Exponent = d.Exponent
}

// Neg sets d to -x and returns d.
func (d *Decimal) Neg(x *Decimal) *Decimal {
	d.Set(x)
	d.Coeff.Neg(&d.Coeff)
	return d
}
