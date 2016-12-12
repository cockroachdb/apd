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

import "testing"

// Appease the unused test.
// TODO(mjibson): actually test all the ErrDecimal methods.
func TestErrDecimal(t *testing.T) {
	var ed ErrDecimal
	a := New(1, 0)
	ed.Abs(a, a)
	ed.Exp(a, a)
	ed.Ln(a, a)
	ed.Log10(a, a)
	ed.Neg(a, a)
	ed.QuoInteger(a, a, a)
	ed.Rem(a, a, a)
}
