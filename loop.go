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

import (
	"math"

	"github.com/pkg/errors"
)

// loop provides a looping structure that determines when a given computation
// on an *Decimal has converged. It was adapted from robpike.io/ivy/value's loop
// implementation.
type loop struct {
	c             *Context
	name          string   // The name of the function we are evaluating.
	i             uint64   // Loop count.
	maxIterations uint64   // When to give up.
	stallCount    int      // Iterations since |delta| changed more than threshold.
	arg           *Decimal // original argument to function; only used for diagnostic.
	prevZ         *Decimal // Result from the previous iteration.
	delta         *Decimal // |Change| from previous iteration.
	stallThresh   *Decimal // The maximum |delta| to be considered a stall.
}

const digitsToBitsRatio = math.Ln10 / math.Ln2

// newLoop returns a new loop checker. The arguments are the name
// of the function being evaluated, the argument to the function,
// and the desired scale of the result, and the iterations
// per bit.
func (c *Context) newLoop(name string, x *Decimal, itersPerBit int) *loop {
	bits := x.Coeff.BitLen()
	incrPrec := float64(c.Precision) + float64(x.Exponent)
	if incrPrec > 0 {
		bits += int(incrPrec * digitsToBitsRatio)
	}
	if scaleBits := int(float64(c.Precision) * digitsToBitsRatio); scaleBits > bits {
		bits = scaleBits
	}
	l := &loop{
		c:             c,
		name:          name,
		maxIterations: 10 + uint64(itersPerBit*bits),
	}
	l.arg = new(Decimal)
	l.arg.Set(x)
	l.stallThresh = New(1, -int32(c.Precision+1))
	l.prevZ = new(Decimal)
	l.delta = new(Decimal)
	return l
}

// done reports whether the loop is done. If it does not converge
// after the maximum number of iterations, it returns an error.
func (l *loop) done(z *Decimal) (bool, error) {
	l.c.Sub(l.delta, l.prevZ, z)
	switch l.delta.Sign() {
	case 0:
		return true, nil
	case -1:
		l.c.Neg(l.delta, l.delta)
	}
	if c, err := l.delta.Cmp(l.stallThresh); err != nil {
		return false, err
	} else if c < 0 {
		l.stallCount++
		if l.stallCount > 2 {
			return true, nil
		}
	} else {
		l.stallCount = 0
	}
	l.i++
	if l.i == l.maxIterations {
		return false, errors.Errorf("%s %s: did not converge after %d iterations; prev,last result %s,%s delta %s", l.name, l.arg.String(), l.maxIterations, z.String(), l.prevZ.String(), l.delta.String())
	}
	l.prevZ.Set(z)
	return false, nil
}
