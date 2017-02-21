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

// New creates a new decimal with the given coefficient and exponent.
func New(coeff int64, exponent int32) *Decimal {
	return &Decimal{
		Coeff:    *big.NewInt(coeff),
		Exponent: exponent,
	}
}

// NewWithBigInt creates a new decimal with the given coefficient and exponent.
func NewWithBigInt(coeff *big.Int, exponent int32) *Decimal {
	return &Decimal{
		Coeff:    *coeff,
		Exponent: exponent,
	}
}

func (d *Decimal) setString(c *Context, s string) (Condition, error) {
	var exps []int64
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil {
			return 0, errors.Wrapf(err, "parse exponent: %s", s[i+1:])
		}
		exps = append(exps, exp)
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp := int64(len(s) - i - 1)
		exps = append(exps, -exp)
		s = s[:i] + s[i+1:]
	}
	if _, ok := d.Coeff.SetString(s, 10); !ok {
		return 0, errors.Errorf("parse mantissa: %s", s)
	}
	return c.goError(d.setExponent(c, 0, exps...))
}

// NewFromString creates a new decimal from s. It has no restrictions on
// exponents or precision.
func NewFromString(s string) (*Decimal, Condition, error) {
	return BaseContext.NewFromString(s)
}

// SetString sets d to s and returns d. It has no restrictions on exponents
// or precision.
func (d *Decimal) SetString(s string) (*Decimal, Condition, error) {
	return BaseContext.SetString(d, s)
}

// NewFromString creates a new decimal from s. The returned Decimal has its
// exponents restricted by the context and its value rounded if it contains more
// digits than the context's precision.
func (c *Context) NewFromString(s string) (*Decimal, Condition, error) {
	d := new(Decimal)
	return c.SetString(d, s)
}

// SetString sets d to s and returns d. The returned Decimal has its exponents
// restricted by the context and its value rounded if it contains more digits
// than the context's precision.
func (c *Context) SetString(d *Decimal, s string) (*Decimal, Condition, error) {
	res, err := d.setString(c, s)
	if err != nil {
		return nil, 0, err
	}
	res |= c.round(d, d)
	_, err = c.goError(res)
	return d, res, err
}

// String is a wrapper of ToSci.
func (d *Decimal) String() string {
	return d.ToSci()
}

// ToSci returns d in scientific notation if an exponent is needed.
func (d *Decimal) ToSci() string {
	// See: http://speleotrove.com/decimal/daconvs.html#reftostr
	const adjExponentLimit = -6

	s := d.Coeff.String()
	prefix := ""
	if d.Coeff.Sign() < 0 {
		prefix = "-"
		s = s[1:]
	}
	adj := int(d.Exponent) + (len(s) - 1)
	if d.Exponent <= 0 && adj >= adjExponentLimit {
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
	return prefix + s
}

// ToStandard converts d to a standard notation string (i.e., no exponent
// part). This can result in long strings given large exponents.
func (d *Decimal) ToStandard() string {
	s := d.Coeff.String()
	var neg string
	if strings.HasPrefix(s, "-") {
		neg = "-"
		s = s[1:]
	}
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
	return neg + s
}

// Set sets d's Coefficient and Exponent from x and returns d.
func (d *Decimal) Set(x *Decimal) *Decimal {
	if d == x {
		return d
	}
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
	_, _, err := d.SetString(strconv.FormatFloat(f, 'E', -1, 64))
	return d, err
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
	errExponentOutOfRangeStr = "exponent out of range"
)

// setExponent sets d's Exponent to the sum of xs. Each value and the sum
// of xs must fit within an int32. An error occurs if the sum is outside of
// the MaxExponent or MinExponent range. res is any Condition previously set
// for this operation, which can cause Underflow to be set if, for example,
// Inexact is already set.
func (d *Decimal) setExponent(c *Context, res Condition, xs ...int64) Condition {
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

	nd := d.NumDigits()
	// adj is the adjusted exponent: exponent + clength - 1
	adj := sum + nd - 1
	// Make sure it is less than the system limits.
	if adj > MaxExponent {
		return SystemOverflow | Overflow
	}
	if adj < MinExponent {
		return SystemUnderflow | Underflow
	}
	v := int32(adj)

	// d is subnormal.
	if v < c.MinExponent {
		if d.Sign() != 0 {
			res |= Subnormal
		}
		Etiny := c.MinExponent - (int32(c.Precision) - 1)
		// Only need to round if exponent < Etiny.
		if r < Etiny {
			// We need to take off (r - Etiny) digits. Split up d.Coeff into integer and
			// fractional parts and do operations similar Round. We avoid calling Round
			// directly because it calls setExponent and modifies the result's exponent
			// and coeff in ways that would be wrong here.
			b := new(big.Int).Set(&d.Coeff)
			tmp := &Decimal{
				Coeff:    *b,
				Exponent: r - Etiny,
			}
			integ, frac := new(Decimal), new(Decimal)
			tmp.Modf(integ, frac)
			frac.Abs(frac)
			if frac.Sign() != 0 {
				res |= Inexact
				if c.Rounding(&integ.Coeff, frac.Cmp(decimalHalf)) {
					if d.Sign() >= 0 {
						integ.Coeff.Add(&integ.Coeff, bigOne)
					} else {
						integ.Coeff.Sub(&integ.Coeff, bigOne)
					}
				}
			}
			if integ.Sign() == 0 {
				res |= Clamped
			}
			r = Etiny
			d.Coeff = integ.Coeff
			res |= Rounded
		}
	} else if v > c.MaxExponent {
		if d.Sign() == 0 {
			res |= Clamped
			r = c.MaxExponent
		} else {
			res |= Overflow
		}
	}

	if res.Inexact() && res.Subnormal() {
		res |= Underflow
	}

	d.Exponent = r
	return res
}

// upscale converts a and b to big.Ints with the same scaling. It returns
// them with this scaling, along with the scaling. An error can be produced
// if the resulting scale factor is out of range.
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
		return nil, nil, 0, errors.New(errExponentOutOfRangeStr)
	}
	x := new(big.Int)
	e := tableExp10(s, x)
	x.Mul(&a.Coeff, e)
	y := &b.Coeff
	if swapped {
		x, y = y, x
	}
	return x, y, b.Exponent, nil
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
	db := new(big.Int)
	e := tableExp10(diff, db)
	db.Set(&d.Coeff)
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

	e := tableExp10(exp, nil)
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

// Abs sets d to |x| and returns d.
func (d *Decimal) Abs(x *Decimal) *Decimal {
	d.Set(x)
	d.Coeff.Abs(&d.Coeff)
	return d
}

// Reduce sets d to x with all trailing zeros removed and returns d.
func (d *Decimal) Reduce(x *Decimal) *Decimal {
	neg := false
	switch x.Sign() {
	case 0:
		d.SetCoefficient(0)
		d.Exponent = 0
		return d
	case -1:
		neg = true
	}
	d.Set(x)

	// Use a uint64 for the division if possible.
	if d.Coeff.BitLen() <= 64 {
		i := d.Coeff.Uint64()
		e := d.Exponent
		for i >= 10000 && i%10000 == 0 {
			i /= 10000
			e += 4
		}
		for i%10 == 0 {
			i /= 10
			e++
		}
		if e != d.Exponent {
			d.Exponent = e
			d.Coeff.SetUint64(i)
			if neg {
				d.Coeff.Neg(&d.Coeff)
			}
		}
		return d
	}

	// Divide by 10 in a loop. In benchmarks of reduce0.decTest, this is 20%
	// faster than converting to a string and trimming the 0s from the end.
	z := new(big.Int).Set(&d.Coeff)
	r := new(big.Int)
	for {
		z.QuoRem(&d.Coeff, bigTen, r)
		if r.Sign() == 0 {
			d.Coeff.Set(z)
			d.Exponent++
		} else {
			break
		}
	}
	return d
}
