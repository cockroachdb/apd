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

// SetInt sets d.'s Coefficient value to x. The exponent is not changed.
func (d *Decimal) SetInt64(x int64) *Decimal {
	d.Coeff.SetInt64(x)
	return d
}

var ErrExponentOutOfRange = errors.New("exponent out of range")

// setExponent sets d's Exponent to the sum of xs. Each value and the sum
// of xs must fit within an int32. An error occurs if the sum is outside of
// the MaxExponent or MinExponent range.
func (d *Decimal) setExponent(xs ...int64) error {
	var sum int64
	for _, x := range xs {
		if x > math.MaxInt32 || x < math.MinInt32 {
			return ErrExponentOutOfRange
		}
		sum += x
	}
	if sum > math.MaxInt32 || sum < math.MinInt32 {
		return ErrExponentOutOfRange
	}
	r := int32(sum)
	if d.MaxExponent != 0 || d.MinExponent != 0 {
		// For max/min exponent calculation, add in the number of digits for each power of 10.
		nr := sum + d.numDigits() - 1
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
	d.Exponent = r
	return nil
}

var (
	bigZero     = big.NewInt(0)
	bigOne      = big.NewInt(1)
	bigTwo      = big.NewInt(2)
	bigTen      = big.NewInt(10)
	decimalHalf = New(5, -1)
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

// Sign returns:
//
//	-1 if d <  0
//	 0 if d == 0
//	+1 if d >  0
//
func (d *Decimal) Sign() int {
	return d.Coeff.Sign()
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

// Abs sets d to |x| (the absolute value of x) and returns d.
func (d *Decimal) Abs(x *Decimal) (*Decimal, error) {
	d.Set(x)
	d.Coeff.Abs(&d.Coeff)
	return d.Round(d)
}

// Neg sets z to -x and returns z.
func (d *Decimal) Neg(x *Decimal) (*Decimal, error) {
	d.Set(x)
	d.Coeff.Neg(&d.Coeff)
	return d.Round(d)
}

// Mul sets d to the product x*y and returns d.
func (d *Decimal) Mul(x, y *Decimal) (*Decimal, error) {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "Mul")
	}
	d.Coeff.Mul(a, b)
	d.Exponent = s * 2
	return d.Round(d)
}

var ErrDivideByZero = errors.New("divide by zero")

// Quo sets d to the quotient x/y for y != 0 and returns d.
func (d *Decimal) Quo(x, y *Decimal) (*Decimal, error) {
	if y.Coeff.Sign() == 0 {
		return nil, ErrDivideByZero
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "Quo")
	}

	// In order to compute the decimal remainder part, add enough 0s to the
	// numerator to accurately round with the given precision.
	// TODO(mjibson): determine a better algorithm for this instead of p*2+8.
	nf := d.Precision*2 + 8
	f := big.NewInt(int64(nf))
	e := new(big.Int).Exp(bigTen, f, nil)
	f.Mul(a, e)
	d.Coeff.Quo(f, b)
	if err := d.setExponent(-int64(nf)); err != nil {
		return nil, err
	}
	return d.Round(d)
}

var ErrIntegerDivisionImpossible = errors.New("integer division impossible")

// QuoInteger sets d to the integer part of the quotient x/y and returns
// d. If the result cannot fit in d.Precision digits, an error is returned.
func (d *Decimal) QuoInteger(x, y *Decimal) (*Decimal, error) {
	if y.Coeff.Sign() == 0 {
		return nil, ErrDivideByZero
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "QuoInteger")
	}
	d.Coeff.Quo(a, b)
	if d.numDigits() > int64(d.Precision) {
		return nil, ErrIntegerDivisionImpossible
	}
	d.Exponent = 0
	return nil, err
}

// Rem sets d to the remainder part of the quotient x/y and returns d. If
// the integer part cannot fit in d.Precision digits, an error is returned.
func (d *Decimal) Rem(x, y *Decimal) (*Decimal, error) {
	if y.Coeff.Sign() == 0 {
		return nil, ErrDivideByZero
	}
	a, b, s, err := upscale(x, y)
	if err != nil {
		return nil, errors.Wrap(err, "Rem")
	}
	tmp := new(big.Int)
	tmp.QuoRem(a, b, &d.Coeff)
	if numDigits(tmp) > int64(d.Precision) {
		return nil, ErrIntegerDivisionImpossible
	}
	d.Exponent = s
	return d.Round(d)
}

var ErrSqrtNegative = errors.New("square root of negative number")

// Sqrt set d to the square root of x and returns d.
func (d *Decimal) Sqrt(x *Decimal) (*Decimal, error) {
	// The square root calculation is implemented using Newton's Method.
	// We start with an initial estimate for sqrt(d), and then iterate:
	//     x_{n+1} = 1/2 * ( x_n + (d / x_n) ).

	// Validate the sign of x.
	switch x.Coeff.Sign() {
	case -1:
		return nil, ErrSqrtNegative
	case 0:
		d.Coeff.SetInt64(0)
		d.Exponent = 0
		return d, nil
	}

	// Use half as the initial estimate.
	z := new(Decimal)
	z.Precision = d.Precision*2 + 2
	_, err := z.Mul(x, decimalHalf)
	if err != nil {
		return nil, errors.Wrap(err, "Sqrt")
	}

	// Iterate.
	tmp := new(Decimal)
	tmp.Precision = z.Precision
	for loop := newLoop("sqrt", z, 1); ; {
		_, err := tmp.Quo(x, z) // t = d / x_n
		if err != nil {
			return nil, err
		}
		_, err = tmp.Add(tmp, z) // t = x_n + (d / x_n)
		if err != nil {
			return nil, err
		}
		_, err = z.Mul(tmp, decimalHalf) // x_{n+1} = 0.5 * t
		if err != nil {
			return nil, err
		}
		if done, err := loop.done(z); err != nil {
			return nil, err
		} else if done {
			break
		}
	}

	return d.Round(z)
}
