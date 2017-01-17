// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is adapted from https://github.com/robpike/ivy/blob/master/value/loop.go.

package apd

import (
	"math"

	"github.com/pkg/errors"
)

type loop struct {
	c             *Context
	name          string   // The name of the function we are evaluating.
	i             uint64   // Loop count.
	maxIterations uint64   // When to give up.
	stallCount    int      // Iterations since |delta| changed more than threshold.
	arg           *Decimal // original argument to function; only used for diagnostic.
	prevZ         *Decimal // Result from the previous iteration.
	delta         *Decimal // |Change| from previous iteration.
	prevDelta     *Decimal // The maximum |delta| to be considered a stall.
}

const digitsToBitsRatio = math.Ln10 / math.Ln2

// newLoop returns a new loop checker. The arguments are the name
// of the function being evaluated, the argument to the function, and
// the maximum number of iterations to perform before giving up.
// The last number in terms of iterations per digit, so the caller can
// ignore the precision setting.
func (c *Context) newLoop(name string, x *Decimal, itersPerDigit int) *loop {
	return &loop{
		c:             c,
		name:          name,
		arg:           new(Decimal).Set(x),
		maxIterations: 10 + uint64(itersPerDigit*int(c.Precision)),
		prevZ:         new(Decimal),
		delta:         new(Decimal),
		prevDelta:     new(Decimal),
	}
}

// done reports whether the loop is done. If it does not converge
// after the maximum number of iterations, it returns an error.
func (l *loop) done(z *Decimal) (bool, error) {
	l.c.Sub(l.delta, l.prevZ, z)
	if l.delta.Sign() == 0 {
		return true, nil
	}
	if l.delta.Sign() < 0 {
		// Convergence can oscillate when the calculation is nearly
		// done and we're running out of bits. This stops that.
		// See next comment.
		l.delta.Neg(l.delta)
	}
	if l.delta.Cmp(l.prevDelta) == 0 {
		// In freaky cases (like e**3) we can hit the same large positive
		// and then  large negative value (4.5, -4.5) so we count a few times
		// to see that it really has stalled. Avoids having to do hard math,
		// but it means we may iterate a few extra times. Usually, though,
		// iteration is stopped by the zero check above, so this is fine.
		l.stallCount++
		if l.stallCount > 3 {
			// Convergence has stopped.
			return true, nil
		}
	} else {
		l.stallCount = 0
	}
	l.i++
	if l.i == l.maxIterations {
		return false, errors.Errorf("%s %s: did not converge after %d iterations; prev,last result %s,%s delta %s", l.name, l.arg.String(), l.maxIterations, z, l.prevZ, l.delta)
	}
	l.prevDelta.Set(l.delta)
	l.prevZ.Set(z)
	return false, nil
}
