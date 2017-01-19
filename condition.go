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
	"strings"

	"github.com/pkg/errors"
)

// Condition holds condition flags.
type Condition uint32

const (
	// SystemOverflow is raised when an exponent is greater than MaxExponent.
	SystemOverflow Condition = 1 << iota
	// SystemUnderflow is raised when an exponent is less than MinExponent.
	SystemUnderflow
	// Overflow is raised when an exponent is greater than Context.MaxExponent.
	Overflow
	// Underflow is raised when an exponent is less than Context.MinExponent.
	Underflow
	// Inexact is raised when an operation is not exact.
	Inexact
	// Subnormal is raised when an operation's adjusted exponent is less than
	// Context.MinExponent.
	Subnormal
	// Rounded is raised when rounding occurs.
	Rounded
	// DivisionUndefined is raised when both division operands are 0.
	DivisionUndefined
	// REVIEW: s/divisior/divisor/
	// DivisionByZero is raised when the divisior is zero.
	DivisionByZero
	// DivisionImpossible is raised when integer division cannot be exactly
	// represented with the given precision.
	DivisionImpossible
	// InvalidOperation is raised during an invalid operation.
	InvalidOperation
)

// Any returns true if any flag is true.
func (r Condition) Any() bool { return r != 0 }

// SystemOverflow returns true if the SystemOverflow flag is set.
func (r Condition) SystemOverflow() bool { return r&SystemOverflow != 0 }

// SystemUnderflow returns true if the SystemUnderflow flag is set.
func (r Condition) SystemUnderflow() bool { return r&SystemUnderflow != 0 }

// Overflow returns true if the Overflow flag is set.
func (r Condition) Overflow() bool { return r&Overflow != 0 }

// Underflow returns true if the Underflow flag is set.
func (r Condition) Underflow() bool { return r&Underflow != 0 }

// Inexact returns true if the Inexact flag is set.
func (r Condition) Inexact() bool { return r&Inexact != 0 }

// Subnormal returns true if the Subnormal flag is set.
func (r Condition) Subnormal() bool { return r&Subnormal != 0 }

// Rounded returns true if the Rounded flag is set.
func (r Condition) Rounded() bool { return r&Rounded != 0 }

// DivisionUndefined returns true if the DivisionUndefined flag is set.
func (r Condition) DivisionUndefined() bool { return r&DivisionUndefined != 0 }

// DivisionByZero returns true if the DivisionByZero flag is set.
func (r Condition) DivisionByZero() bool { return r&DivisionByZero != 0 }

// DivisionImpossible returns true if the DivisionImpossible flag is set.
func (r Condition) DivisionImpossible() bool { return r&DivisionImpossible != 0 }

// InvalidOperation returns true if the InvalidOperation flag is set.
func (r Condition) InvalidOperation() bool { return r&InvalidOperation != 0 }

// REVIEW: Could you define what you mean by "traps" here a bit more?
// GoError converts r to an error based on the given traps and returns r.
func (r Condition) GoError(traps Condition) (Condition, error) {
	const (
		systemErrors = SystemOverflow | SystemUnderflow
	)
	var err error
	if r&systemErrors != 0 {
		err = errors.New(errExponentOutOfRange)
	} else if t := r & traps; t != 0 {
		err = errors.New(t.String())
	}
	return r, err
}

// REVIEW: should this return something when r == 0?
func (r Condition) String() string {
	var names []string
	for i := Condition(1); r != 0; i <<= 1 {
		if r&i == 0 {
			continue
		}
		r ^= i
		var s string
		switch i {
		case SystemOverflow, SystemUnderflow:
			continue
		case Overflow:
			s = "overflow"
		case Underflow:
			s = "underflow"
		case Inexact:
			s = "inexact"
		case Subnormal:
			s = "subnormal"
		case Rounded:
			s = "rounded"
		case DivisionUndefined:
			s = "division undefined"
		case DivisionByZero:
			s = "division by zero"
		case DivisionImpossible:
			s = "division impossible"
		case InvalidOperation:
			s = "invalid operation"
		default:
			panic(errors.Errorf("unknown condition %d", i))
		}
		names = append(names, s)
	}
	return strings.Join(names, ", ")
}

// negateOverflowFlags converts Overflow and SystemOverflow flags into their
// equivalent Underflows.
func (c Condition) negateOverflowFlags() Condition {
	if c.Overflow() {
		// REVIEW: why Subnormal? Leave a comment
		c |= Underflow | Subnormal
		c &= ^Overflow
	}
	if c.SystemOverflow() {
		c |= SystemUnderflow
		c &= ^SystemOverflow
	}
	return c
}
