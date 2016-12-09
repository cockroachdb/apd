package apd

import "math/big"

// Round rounds x with d's settings. The result is stored in d and returned. If
// d has zero Precision, no modification of x is done. If d has no Rounding
// specified, RoundDown is used.

func (d *Decimal) Round(x *Decimal) (*Decimal, error) {
	if d.Precision == 0 {
		return d.Set(x), nil
	}
	rounder := d.Rounding
	if rounder == nil {
		rounder = RoundDown
	}
	err := rounder(d, x)
	return d, err
}

// Round rounds x with d's precision settings and stores the result in d.
type Rounder func(d, x *Decimal) error

func RoundDown(d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		y.Div(&d.Coeff, e)
		d.Coeff.Set(y)
		err := d.addExponent(diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func RoundHalfUp(d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.DivMod(&d.Coeff, e, m)
		m.Mul(m, bigTwo)
		if m.Cmp(e) >= 0 {
			y.Add(y, bigOne)
		}
		d.Coeff.Set(y)
		err := d.addExponent(diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func RoundHalfEven(d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		m := new(big.Int)
		y.DivMod(&d.Coeff, e, m)
		m.Mul(m, bigTwo)
		if m.Cmp(e) > 0 {
			y.Add(y, bigOne)
		}
		d.Coeff.Set(y)
		err := d.addExponent(diff)
		if err != nil {
			return err
		}
	}
	return nil
}
