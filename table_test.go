package apd

import (
	"bytes"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

func TestNumDigits(t *testing.T) {
	runTest := func(start string, c byte) {
		buf := bytes.NewBufferString(start)
		var offset int
		if strings.HasPrefix(start, "-") {
			offset--
		}
		for i := 1; i < 1000; i++ {
			buf.WriteByte(c)
			bs := buf.String()
			t.Run(bs, func(t *testing.T) {
				d := newDecimal(t, bs)
				n := d.numDigits()
				e := int64(buf.Len() + offset)
				if n != e {
					t.Fatalf("%s ('%c'): expected %d, got %d", bs, c, e, n)
				}
			})
		}
	}
	runTest("", '9')
	runTest("1", '0')
	runTest("-", '9')
	runTest("-1", '0')
}

func TestDigitsLookupTable(t *testing.T) {
	// Make sure all elements in table make sense.
	min := new(big.Int)
	prevBorder := big.NewInt(0)
	for i := 1; i <= digitsTableSize; i++ {
		elem := digitsLookupTable[i]

		min.SetInt64(2)
		min.Exp(min, big.NewInt(int64(i-1)), nil)
		if minLen := int64(len(min.String())); minLen != elem.digits {
			t.Errorf("expected 2^%d to have %d digits, found %d", i, elem.digits, minLen)
		}

		if zeros := int64(strings.Count(elem.border.String(), "0")); zeros != elem.digits {
			t.Errorf("the %d digits for digitsLookupTable[%d] does not agree with the border %v", elem.digits, i, &elem.border)
		}

		if min.Cmp(&elem.border) >= 0 {
			t.Errorf("expected 2^%d = %v to be less than the border, found %v", i-1, min, &elem.border)
		}

		if elem.border.Cmp(prevBorder) > 0 {
			if min.Cmp(prevBorder) <= 0 {
				t.Errorf("expected 2^%d = %v to be greater than or equal to the border, found %v", i-1, min, prevBorder)
			}
			prevBorder = &elem.border
		}
	}

	// Throw random big.Ints at the table and make sure the
	// digit lengths line up.
	const randomTrials = 100
	for i := 0; i < randomTrials; i++ {
		a := big.NewInt(rand.Int63())
		b := big.NewInt(rand.Int63())
		a.Mul(a, b)

		d := &Decimal{Coeff: *a}
		tableDigits := d.numDigits()
		if actualDigits := int64(len(a.String())); actualDigits != tableDigits {
			t.Errorf("expected %d digits for %v, found %d", tableDigits, a, actualDigits)
		}
	}
}
