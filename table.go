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

import "github.com/cockroachdb/apd/int10"

// NumDigits returns the number of decimal digits of d.Coeff.
func (d *Decimal) NumDigits() int64 {
	return NumDigits(d.Coeff)
}

// NumDigits returns the number of decimal digits of b.
func NumDigits(i int10.Int) int64 {
	n := len(i)
	if n == 0 {
		return 1
	}
	return int64(n)
}

// tableExp10 returns 10^x for x >= 0, looked up from a table when
// possible. This returned value must not be mutated. tmp is used as an
// intermediate variable, but may be nil.
func tableExp10(x int, tmp int10.Int) int10.Int {
	i := int10.NewInt(1)
	i.Mul10(x)
	return i
}
