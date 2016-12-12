package apd

import "math/big"

// Round rounds x with d's settings. The result is stored in d and returned. If
// d has zero Precision, no modification of x is done. If d has no Rounding
// specified, RoundDown is used.
func (d *Decimal) Round(x *Decimal) error {
	if d.Precision == 0 {
		d.Set(x)
		err := d.setExponent(int64(d.Exponent))
		return err
	}
	rounder := d.Rounding
	if rounder == nil {
		rounder = RoundDown
	}
	err := rounder(d, x)
	if err != nil {
		return err
	}
	return d.setExponent(int64(d.Exponent))
}

// Rounder rounds x with d's precision settings and stores the result in d.
type Rounder func(d, x *Decimal) error

var (
	// RoundDown rounds toward 0; truncate.
	RoundDown Rounder = roundDown
	// RoundHalfUp rounds up if the digits are >= 0.5.
	RoundHalfUp Rounder = roundHalfUp
	// RoundHalfEven rounds up if the digits are > 0.5. If the digits are equal
	// to 0.5, it rounds up if the previous digit is odd, always producing an
	// even digit.
	RoundHalfEven Rounder = roundHalfEven
)

func roundDown(d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
		y := big.NewInt(diff)
		e := new(big.Int).Exp(bigTen, y, nil)
		y.Quo(&d.Coeff, e)
		d.Coeff.Set(y)
		err := d.setExponent(int64(d.Exponent), diff)
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

func roundHalfUp(d, x *Decimal) error {
	d.Set(x)
	d.Coeff.Add(&d.Coeff, bigZero)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
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
		err := d.setExponent(int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func roundHalfEven(d, x *Decimal) error {
	d.Set(x)
	nd := x.numDigits()
	if diff := nd - int64(d.Precision); diff > 0 {
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
		err := d.setExponent(int64(d.Exponent), diff)
		if err != nil {
			return err
		}
	}
	return nil
}
