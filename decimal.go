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

// Decimal is in arbitrary-precision decimal. Its value is:
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

// NewFromString creates a new decimal from s.
func NewFromString(s string) (*Decimal, error) {
	var err error

	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err = strconv.Atoi(s[i+1:])
		if err != nil {
			return nil, errors.Wrapf(err, "parse exponent: %s", s[i+1:])
		}
		if exp > MaxExponent || exp < MinExponent {
			return nil, errors.Errorf(errExponentOutOfRange, exp)
		}
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp -= len(s) - i - 1
		if exp > MaxExponent || exp < MinExponent {
			return nil, errors.Errorf(errExponentOutOfRange, exp)
		}
		s = s[:i] + s[i+1:]
	}
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, errors.Errorf("parse mantissa: %s", s)
	}
	return &Decimal{
		Coeff:    *i,
		Exponent: int32(exp),
	}, nil
}

func (d *Decimal) String() string {
	// forceExp is the number of trailing 0s or 0s after the decimal after which
	// a scientific notation is used instead.
	const forceExp = 1000

	s := d.Coeff.String()
	neg := d.Coeff.Sign() < 0
	if neg {
		s = s[1:]
	}
	nd := d.numDigits()
	e := int64(d.Exponent)
	if d.Exponent < 0 {
		if nd+e < -forceExp {
			s = fmt.Sprintf("%sE%d", s, d.Exponent)
		} else if left := -int(d.Exponent) - len(s); left > 0 {
			s = "0." + strings.Repeat("0", left) + s
		} else if left < 0 {
			offset := -left
			s = s[:offset] + "." + s[offset:]
		} else {
			s = "0." + s
		}
	} else if d.Exponent > 0 {
		if e > forceExp {
			s = fmt.Sprintf("%sE%d", s, d.Exponent)
		} else {
			s += strings.Repeat("0", int(d.Exponent))
		}
	}
	if neg {
		s = "-" + s
	}
	return s
}

// Set sets d's coefficient and exponent from x.
func (d *Decimal) Set(x *Decimal) *Decimal {
	d.Coeff.Set(&x.Coeff)
	d.Exponent = x.Exponent
	return d
}

// SetInt64 sets d.'s Coefficient value to x. The exponent is not changed.
func (d *Decimal) SetInt64(x int64) *Decimal {
	d.Coeff.SetInt64(x)
	return d
}

// Int64 returns the int64 representation of x. If x cannot be represented in an int64, an error is returned.
func (d *Decimal) Int64() (int64, error) {
	integ, frac := new(Decimal), new(Decimal)
	d.Modf(integ, frac)
	if frac.Sign() != 0 {
		return 0, errors.Errorf("%s: has fractional part", d)
	}
	var ed ErrDecimal
	if c := ed.Cmp(integ, New(math.MaxInt64, 0)); c > 0 {
		return 0, errors.Errorf("%s: greater than max int64", d)
	}
	if c := ed.Cmp(integ, New(math.MinInt64, 0)); c < 0 {
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

var (
	errExponentOutOfRange        = "exponent out of range: %v"
	errSqrtNegative              = errors.New("square root of negative number")
	errIntegerDivisionImpossible = errors.New("integer division impossible")
	errDivideByZero              = errors.New("divide by zero")
	errPowZeroNegative           = errors.New("zero raised to a negative power is undefined")
	errPowNegNonInteger          = errors.New("a negative number raised to a non-integer power yields a complex result")
)

// setExponent sets d's Exponent to the sum of xs. Each value and the sum
// of xs must fit within an int32. An error occurs if the sum is outside of
// the MaxExponent or MinExponent range.
func (d *Decimal) setExponent(c *Context, xs ...int64) error {
	var sum int64
	for _, x := range xs {
		if x > MaxExponent || x < MinExponent {
			return errors.Errorf(errExponentOutOfRange, x)
		}
		sum += x
	}
	if sum > MaxExponent || sum < MinExponent {
		return errors.Errorf(errExponentOutOfRange, sum)
	}
	r := int32(sum)
	if c.MaxExponent != 0 || c.MinExponent != 0 {
		// For max/min exponent calculation, add in the number of digits for each power of 10.
		nr := sum + d.numDigits() - 1
		// Make sure it still fits in an int32 for comparison to Max/Min Exponent.
		if nr > MaxExponent || nr < MinExponent {
			return errors.Errorf(errExponentOutOfRange, nr)
		}
		v := int32(nr)
		if c.MaxExponent != 0 && v > c.MaxExponent ||
			c.MinExponent != 0 && v < c.MinExponent {
			return errors.Errorf(errExponentOutOfRange, v)
		}
	}
	d.Exponent = r
	return nil
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
		return nil, nil, 0, errors.Errorf(errExponentOutOfRange, s)
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
func (d *Decimal) Cmp(x *Decimal) (int, error) {
	a, b, _, err := upscale(d, x)
	if err != nil {
		return 0, errors.Wrap(err, "Cmp")
	}
	return a.Cmp(b), nil
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
		frac.SetInt64(0)
		integ.Set(d)
		return
	}
	nd := d.numDigits()
	exp := -int64(d.Exponent)
	// d < 0 because exponent is larger than number of digits.
	if exp > nd {
		integ.Exponent = 0
		integ.SetInt64(0)
		frac.Set(d)
		return
	}

	y := big.NewInt(exp)
	e := new(big.Int).Exp(bigTen, y, nil)
	integ.Coeff.QuoRem(&d.Coeff, e, &frac.Coeff)
	integ.Exponent = 0
	frac.Exponent = d.Exponent
}
