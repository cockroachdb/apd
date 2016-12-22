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
func (c *Context) Round(d, x *Decimal) error {
	if c.Precision == 0 {
		d.Set(x)
		err := d.setExponent(c, int64(d.Exponent))
		return err
	}
	rounder := c.Rounding
	if rounder == nil {
		rounder = RoundHalfUp
	}
	err := rounder(c, d, x)
	if err != nil {
		return err
	}
	return d.setExponent(c, int64(d.Exponent))
}

// Rounder sets d to rounded x.
type Rounder func(c *Context, d, x *Decimal) error

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
	// RoundUp rounds up if the digits > 0.
	RoundUp Rounder = roundUp
)

func roundDown(c *Context, d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		y.Quo(&d.Coeff, e)
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

// roundAddOne adds 1 to abs(b).
func roundAddOne(b *big.Int, diff *int64) {
	nd := numDigits(b)
	if b.Sign() >= 0 {
		b.Add(b, bigOne)
	} else {
		b.Sub(b, bigOne)
	}
	nd2 := numDigits(b)
	if nd2 > nd {
		b.Div(b, bigTen)
		*diff++
	}
}

func roundHalfUp(c *Context, d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		m.Abs(m)
		m.Mul(m, bigTwo)
		if m.Cmp(e) >= 0 {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundHalfEven(c *Context, d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		m.Abs(m)
		m.Mul(m, bigTwo)
		if c := m.Cmp(e); c > 0 {
			roundAddOne(y, &diff)
		} else if c == 0 {
			if y.Bit(0) == 1 {
				roundAddOne(y, &diff)
			}
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundHalfDown(c *Context, d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		m.Abs(m)
		m.Mul(m, bigTwo)
		if m.Cmp(e) > 0 {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundUp(c *Context, d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		if m.Sign() != 0 {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundFloor(c *Context, d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		if m.Sign() != 0 && y.Sign() < 0 {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundCeiling(c *Context, d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(c.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.QuoRem(&d.Coeff, e, m)
		if m.Sign() != 0 && y.Sign() >= 0 {
			roundAddOne(y, &diff)
		}
		d.Coeff.Set(y)
		err := d.setExponent(c, int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}
