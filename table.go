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

import "math/big"

// digitsLookupTable is used to map binary digit counts to their corresponding
// decimal border values. The map relies on the proof that (without leading zeros)
// for any given number of binary digits r, such that the number represented is
// between 2^r and 2^(r+1)-1, there are only two possible decimal digit counts
// k and k+1 that the binary r digits could be representing.
//
// Using this proof, for a given digit count, the map will return the lower number
// of decimal digits (k) the binary digit count could represent, along with the
// value of the border between the two decimal digit counts (10^k).
const digitsTableSize = 128

var digitsLookupTable [digitsTableSize + 1]tableVal

type tableVal struct {
	digits int64
	border big.Int
}

func init() {
	curVal := big.NewInt(1)
	curExp := new(big.Int)
	for i := 1; i <= digitsTableSize; i++ {
		if i > 1 {
			curVal.Lsh(curVal, 1)
		}

		elem := &digitsLookupTable[i]
		elem.digits = int64(len(curVal.String()))

		elem.border.SetInt64(10)
		curExp.SetInt64(elem.digits)
		elem.border.Exp(&elem.border, curExp, nil)
	}
}

func lookupBits(bitLen int) (tableVal, bool) {
	if bitLen > 0 && bitLen < len(digitsLookupTable) {
		return digitsLookupTable[bitLen], true
	}
	return tableVal{}, false
}

// numDigits returns the number of decimal digits that make up
// big.Int value. The function first attempts to look this digit
// count up in the digitsLookupTable. If the value is not there,
// it defaults to constructing a string value for the big.Int and
// using this to determine the number of digits.
func (d *Decimal) numDigits() int64 {
	return numDigits(&d.Coeff)
}

func numDigits(b *big.Int) int64 {
	bl := b.BitLen()
	if bl == 0 {
		return 1
	}
	if val, ok := lookupBits(bl); ok {
		ab := new(big.Int).Abs(b)
		if ab.Cmp(&val.border) < 0 {
			return val.digits
		}
		return val.digits + 1
	}

	n := int64(float64(bl) / digitsToBitsRatio)
	a := new(big.Int).Abs(b)
	e := new(big.Int).Exp(bigTen, big.NewInt(n), nil)
	if a.Cmp(e) >= 0 {
		n++
	}
	return n
}
