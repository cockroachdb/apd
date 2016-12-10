package apd

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Decimal is in arbitrary-precision decimal. It's value is:
//
//     coeff * 10 ^ exponent
//
// All arithmetic operations on a Decimal are subject to the result's
// Precision and Rounding settings. RoundDown is the default Rounding.

type Decimal struct {
	Coeff    big.Int
	Exponent int32

	MaxExponent int32
	MinExponent int32
	Precision   uint32
	Rounding    Rounder
}

func New(i int64, scale int32) *Decimal {
	return &Decimal{
		Coeff:    *big.NewInt(i),
		Exponent: scale,
	}
}

func NewFromString(s string) (*Decimal, error) {
	var err error

	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err = strconv.Atoi(s[i+1:])
		if err != nil {
			return nil, errors.Wrapf(err, "parse exponent: %s", s[i+1:])
		}
		if exp > math.MaxInt32 || exp < math.MinInt32 {
			return nil, ErrExponentOutOfRange
		}
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp -= len(s) - i - 1
		if exp > math.MaxInt32 || exp < math.MinInt32 {
			return nil, ErrExponentOutOfRange
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
	s := d.Coeff.String()
	neg := d.Coeff.Sign() < 0
	if neg {
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
	if neg {
		s = "-" + s
	}
	return s
}

func (d *Decimal) GoString() string {
	return fmt.Sprintf(`{Coeff: %s, Exponent: %d, MaxExponent: %d, MinExponent: %d, Precision: %d}`, d.Coeff.String(), d.Exponent, d.MaxExponent, d.MinExponent, d.Precision)
}

func (d *Decimal) Set(x *Decimal) *Decimal {
	d.Coeff.Set(&x.Coeff)
	d.Exponent = x.Exponent
	return d
}

var ErrExponentOutOfRange = errors.New("exponent out of range")

// addExponent adds x to d's Exponent and checks that it is in range.
func (d *Decimal) addExponent(x int64) error {
	if x > math.MaxInt32 || x < math.MinInt32 {
		return ErrExponentOutOfRange
	}
	// Now both d.Exponent and x are guaranteed to fit in an int32, so we can
	// add them without overflow.
	r := int64(d.Exponent) + int64(x)
	if r > math.MaxInt32 || r < math.MinInt32 {
		return ErrExponentOutOfRange
	}
	if d.MaxExponent != 0 || d.MinExponent != 0 {
		// For max/min exponent calculation, add in the number of digits for each power of 10.
		nr := r + d.numDigits() - 1
		// Make sure it still fits in an int32 for comparison to Max/Min Exponent.
		if nr > math.MaxInt32 || nr < math.MinInt32 {
			return ErrExponentOutOfRange
		}
		v := int32(nr)
		if d.MaxExponent != 0 && v > d.MaxExponent ||
			d.MinExponent != 0 && v < d.MinExponent {
			return ErrExponentOutOfRange
		}
	}
	d.Exponent = int32(r)
	return nil
}

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
	bigTwo  = big.NewInt(2)
	bigTen  = big.NewInt(10)
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
	if s > 1000 {
		return nil, nil, 0, ErrExponentOutOfRange
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

// Add sets d to the sum x+y and returns d.
func (d *Decimal) Add(x, y *Decimal) (*Decimal, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "Add")
	}
	d.Coeff.Add(a, b)
	d.Exponent = s
	return d.Round(d)
}

// Sub sets d to the difference x-y and returns d.
func (d *Decimal) Sub(x, y *Decimal) (*Decimal, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "Sub")
	}
	d.Coeff.Sub(a, b)
	d.Exponent = s
	return d.Round(d)
}
