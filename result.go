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

import "errors"

type Result int32

const (
	SystemOverflow Result = 1 << iota
	SystemUnderflow
	Overflow
	Underflow
	Inexact
	Subnormal
	Rounded
)

func (r Result) Any() bool       { return r != 0 }
func (r Result) Overflow() bool  { return r&Overflow != 0 }
func (r Result) Underflow() bool { return r&Underflow != 0 }
func (r Result) Inexact() bool   { return r&Inexact != 0 }
func (r Result) Subnormal() bool { return r&Subnormal != 0 }
func (r Result) Rounded() bool   { return r&Rounded != 0 }

func (r Result) GoError() error {
	const (
		systemErrors = SystemOverflow | SystemUnderflow
		errorFields  = Underflow | Overflow | Subnormal
	)
	if r&systemErrors != 0 {
		return errors.New(errExponentOutOfRange)
	}
	if r&errorFields != 0 {
		return resultError(r)
	}
	return nil
}

type resultError Result

func (r resultError) Error() string {
	re := Result(r)
	switch {
	case re.Subnormal():
		return "subnormal"
	case re.Overflow():
		return "overflow"
	case re.Underflow():
		return "underflow"
	default:
		// In this case, a Result was returned or created instead of a nil error. This
		// should only occur if there's a bug in apd.
		panic("not an error")
	}
}
