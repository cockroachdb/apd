package apd

import (
	"math"
	"math/big"
)

// Sqrt returns a big.Float representation of the square root of
// z. Precision is the same as the one of the argument. The function
// returns nil if z is negative, returns ±0 when z = ±0, and +Inf when z =
// +Inf.
func Sqrt(z *big.Float) *big.Float {

	// panic on negative z
	if z.Sign() == -1 {
		return nil
	}

	// √±0 = ±0
	if z.Sign() == 0 {
		return big.NewFloat(float64(z.Sign()))
	}

	// √+Inf  = +Inf
	if z.IsInf() {
		return big.NewFloat(math.Inf(+1))
	}

	// Compute √(a·2**b) as
	//   √(a)·2**b/2       if b is even
	//   √(2a)·2**b/2      if b > 0 is odd
	//   √(0.5a)·2**b/2    if b < 0 is odd
	//
	// The difference in the odd exponent case is due to the fact that
	// exp/2 is rounded in different directions when exp is negative.
	mant := new(big.Float)
	exp := z.MantExp(mant)
	switch exp % 2 {
	case 1:
		mant.Mul(big.NewFloat(2), mant)
	case -1:
		mant.Mul(big.NewFloat(0.5), mant)
	}

	// Solving x² - z = 0 directly requires a Quo call, but it's
	// faster for small precisions.
	//
	// Solving 1/x² - z = 0 avoids the Quo call and is much faster for
	// high precisions.
	//
	// Use sqrtDirect for prec <= 128 and sqrtInverse for prec > 128.
	var x *big.Float
	if z.Prec() <= 128 {
		x = sqrtDirect(mant)
	} else {
		x = sqrtInverse(mant)
	}

	// re-attach the exponent and return
	return x.SetMantExp(x, exp/2)

}

// compute √z using newton to solve
// t² - z = 0 for t
func sqrtDirect(z *big.Float) *big.Float {
	// f(t)/f'(t) = 0.5(t² - z)/t
	half := big.NewFloat(0.5)
	f := func(t *big.Float) *big.Float {
		x := new(big.Float).Mul(t, t) // x = t²
		x.Sub(x, z)                   // x = t² - z
		x.Mul(half, x)                // x = 0.5(t² - z)
		return x.Quo(x, t)            // return x = 0.5(t² - z)/t
	}

	// initial guess
	zf, _ := z.Float64()
	guess := big.NewFloat(math.Sqrt(zf))

	return newton(f, guess, z.Prec())
}

// compute √z using newton to solve
// 1/t² - z = 0 for x and then inverting.
func sqrtInverse(z *big.Float) *big.Float {
	// f(t)/f'(t) = -0.5t(1 - zt²)
	nhalf := big.NewFloat(-0.5)
	one := big.NewFloat(1)
	f := func(t *big.Float) *big.Float {
		u := new(big.Float)
		u.Mul(t, t)                     // u = t²
		u.Mul(u, z)                     // u = zt²
		u.Sub(one, u)                   // u = 1 - zt²
		u.Mul(u, nhalf)                 // u = -0.5(1 - zt²)
		return new(big.Float).Mul(t, u) // x = -0.5t(1 - zt²)
	}

	// initial guess
	zf, _ := z.Float64()
	guess := big.NewFloat(1 / math.Sqrt(zf))

	// There's another operation after newton,
	// so we need to force it to return at least
	// a few guard digits. Use 32.
	x := newton(f, guess, z.Prec()+32)
	return x.Mul(z, x).SetPrec(z.Prec())
}

// returns an approximate (to precision dPrec) solution to
//    f(t) = 0
// using the Newton Method.
// fOverDf needs to be a fuction returning f(t)/f'(t).
// t must not be changed by fOverDf.
// guess is the initial guess (and it's not preserved).
func newton(fOverDf func(z *big.Float) *big.Float, guess *big.Float, dPrec uint) *big.Float {

	prec, guard := guess.Prec(), uint(64)
	guess.SetPrec(prec + guard)

	for prec < 2*dPrec {
		guess.Sub(guess, fOverDf(guess))
		prec *= 2
		guess.SetPrec(prec + guard)
	}

	return guess.SetPrec(dPrec)
}
