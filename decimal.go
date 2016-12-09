package apd

import (
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

	Precision uint32
	Rounding  Rounder
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
	d.Exponent = int32(r)
	return nil
}

var (
	bigOne = big.NewInt(1)
	bigTwo = big.NewInt(2)
	bigTen = big.NewInt(10)
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
	if s > math.MaxInt32 {
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

// numDigits returns the number of digits of d's coefficient.
func (d *Decimal) numDigits() int64 {
	const digitsToBitsRatio = math.Ln10 / math.Ln2

	numDigits := float64(d.Coeff.BitLen()) / digitsToBitsRatio
	return int64(numDigits) + 1
}

// Add sets d to the sum x+y and returns d.
func (d *Decimal) Add(x, y *Decimal) (*Decimal, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "add")
	}
	d.Coeff.Add(a, b)
	d.Exponent = s
	return d, nil
}
