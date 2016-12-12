// Package apd implements arbitrary-precision decimals.
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

	MaxExponent int32
	MinExponent int32
	Precision   uint32
	Rounding    Rounder
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
		if exp > math.MaxInt32 || exp < math.MinInt32 {
			return nil, errExponentOutOfRange
		}
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp -= len(s) - i - 1
		if exp > math.MaxInt32 || exp < math.MinInt32 {
			return nil, errExponentOutOfRange
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

var (
	errExponentOutOfRange        = errors.New("exponent out of range")
	errSqrtNegative              = errors.New("square root of negative number")
	errIntegerDivisionImpossible = errors.New("integer division impossible")
	errDivideByZero              = errors.New("divide by zero")
)

// setExponent sets d's Exponent to the sum of xs. Each value and the sum
// of xs must fit within an int32. An error occurs if the sum is outside of
// the MaxExponent or MinExponent range.
func (d *Decimal) setExponent(xs ...int64) error {
	var sum int64
	for _, x := range xs {
		if x > math.MaxInt32 || x < math.MinInt32 {
			return errExponentOutOfRange
		}
		sum += x
	}
	if sum > math.MaxInt32 || sum < math.MinInt32 {
		return errExponentOutOfRange
	}
	r := int32(sum)
	if d.MaxExponent != 0 || d.MinExponent != 0 {
		// For max/min exponent calculation, add in the number of digits for each power of 10.
		nr := sum + d.numDigits() - 1
		// Make sure it still fits in an int32 for comparison to Max/Min Exponent.
		if nr > math.MaxInt32 || nr < math.MinInt32 {
			return errExponentOutOfRange
		}
		v := int32(nr)
		if d.MaxExponent != 0 && v > d.MaxExponent ||
			d.MinExponent != 0 && v < d.MinExponent {
			return errExponentOutOfRange
		}
	}
	d.Exponent = r
	return nil
}

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
	if s > 10000 {
		return nil, nil, 0, errors.Wrapf(errExponentOutOfRange, "upscale: %d", s)
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

// Add sets d to the sum x+y.
func (d *Decimal) Add(x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Add")
	}
	d.Coeff.Add(a, b)
	d.Exponent = s
	return d.Round(d)
}

// Sub sets d to the difference x-y.
func (d *Decimal) Sub(x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Sub")
	}
	d.Coeff.Sub(a, b)
	d.Exponent = s
	return d.Round(d)
}

// Abs sets d to |x| (the absolute value of x).
func (d *Decimal) Abs(x *Decimal) error {
	d.Set(x)
	d.Coeff.Abs(&d.Coeff)
	return d.Round(d)
}

// Neg sets z to -x.
func (d *Decimal) Neg(x *Decimal) error {
	d.Set(x)
	d.Coeff.Neg(&d.Coeff)
	return d.Round(d)
}

// Mul sets d to the product x*y.
func (d *Decimal) Mul(x, y *Decimal) error {
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Mul")
	}
	d.Coeff.Mul(a, b)
	d.Exponent = s * 2
	return d.Round(d)
}

// Quo sets d to the quotient x/y for y != 0.
func (d *Decimal) Quo(x, y *Decimal) error {
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
	nf := d.Precision*2 + 8
	f := big.NewInt(int64(nf))
	e := new(big.Int).Exp(bigTen, f, nil)
	f.Mul(a, e)
	d.Coeff.Quo(f, b)
	if err := d.setExponent(-int64(nf)); err != nil {
		return err
	}
	return d.Round(d)
}

// QuoInteger sets d to the integer part of the quotient x/y. If the result
// cannot fit in d.Precision digits, an error is returned.
func (d *Decimal) QuoInteger(x, y *Decimal) error {
	if y.Coeff.Sign() == 0 {
		return errDivideByZero
	}
	a, b, _, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "QuoInteger")
	}
	d.Coeff.Quo(a, b)
	if d.numDigits() > int64(d.Precision) {
		return errIntegerDivisionImpossible
	}
	d.Exponent = 0
	return err
}

// Rem sets d to the remainder part of the quotient x/y. If
// the integer part cannot fit in d.Precision digits, an error is returned.
func (d *Decimal) Rem(x, y *Decimal) error {
	if y.Coeff.Sign() == 0 {
		return errDivideByZero
	}
	a, b, s, err := upscale(x, y)
	if err != nil {
		return errors.Wrap(err, "Rem")
	}
	tmp := new(big.Int)
	tmp.QuoRem(a, b, &d.Coeff)
	if numDigits(tmp) > int64(d.Precision) {
		return errIntegerDivisionImpossible
	}
	d.Exponent = s
	return d.Round(d)
}

// Sqrt sets d to the square root of x.
func (d *Decimal) Sqrt(x *Decimal) error {
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
	z.Precision = d.Precision*2 + 2
	err := z.Mul(x, decimalHalf)
	if err != nil {
		return errors.Wrap(err, "Sqrt")
	}

	// Iterate.
	tmp := new(Decimal)
	tmp.Precision = z.Precision
	for loop := newLoop("sqrt", z, 1); ; {
		err := tmp.Quo(x, z) // t = d / x_n
		if err != nil {
			return err
		}
		err = tmp.Add(tmp, z) // t = x_n + (d / x_n)
		if err != nil {
			return err
		}
		err = z.Mul(tmp, decimalHalf) // x_{n+1} = 0.5 * t
		if err != nil {
			return err
		}
		if done, err := loop.done(z); err != nil {
			return err
		} else if done {
			break
		}
	}

	return d.Round(z)
}

// Ln sets d to the natural log of x.
func (d *Decimal) Ln(x *Decimal) error {
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
	p := d.Precision
	if p < 15 {
		p = 15
	}
	p *= 4
	xr := &Decimal{Precision: p}

	fact := New(2, 0)
	var ed ErrDecimal

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
		xr.Precision += p
		ed.Sqrt(xr, xr)
		ed.Mul(fact, fact, decimalTwo)
	}
	if ed.Err != nil {
		return ed.Err
	}

	tmp1 := &Decimal{Precision: p}
	tmp2 := &Decimal{Precision: p}
	elem := &Decimal{Precision: p}
	numerator := &Decimal{Precision: p}
	z := &Decimal{Precision: p}

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
	if ed.Err != nil {
		return ed.Err
	}
	for loop := newLoop("log", z, 40); ; {
		// tmp1 = n, the i'th odd power: 3, 5, 7, 9, etc.
		tmp1.SetInt64(int64(loop.i)*2 + 3)
		// numerator = r^n
		ed.Mul(numerator, numerator, elem)
		// tmp2 = r^n / n
		ed.Quo(tmp2, numerator, tmp1)
		// z += r^n / n
		ed.Add(z, z, tmp2)
		if ed.Err != nil {
			return ed.Err
		}
		if done, err := loop.done(z); err != nil {
			return err
		} else if done {
			break
		}
		if ed.Err != nil {
			return ed.Err
		}
	}

	// Undo input range reduction.
	ed.Mul(z, z, fact)
	if ed.Err != nil {
		return ed.Err
	}

	// Round to the desired scale.
	return d.Round(z)
}

// Log10 sets d to the base 10 log of x.
func (d *Decimal) Log10(x *Decimal) error {
	z := &Decimal{Precision: d.Precision * 2}
	err := z.Ln(x)
	if err != nil {
		return errors.Wrap(err, "ln")
	}
	return d.Quo(z, decimalLog10)
}
