package apd

import (
	"fmt"
	"math/big"
	"testing"
)

var sqrtTests = []struct {
	sqrtFrom int
	prec     uint
	fmt      string
	expected string
}{
	{1, uint(1), "%.1f", "1.0"},
	{1, uint(10), "%.10f", "1.0000000000"},
	{2, uint(50), "%.50f", "1.41421356237309581160843663383275270462036132812500"},
	{2, uint(100), "%.100f", "1.4142135623730950488016887242091762499253118869660345518308661172390827687195269390940666198730468750"},
}

func TestSqrt(t *testing.T) {
	for _, test := range sqrtTests {
		from := big.NewFloat(float64(test.sqrtFrom))
		from.SetPrec(test.prec)
		actual := Sqrt(from)

		if actual.Prec() != from.Prec() {
			t.Fatalf("Sqrt(%d) expected prec %d, got prec %d", test.sqrtFrom, from.Prec(), actual.Prec())
		}

		if fmt.Sprintf(test.fmt, actual) != test.expected {
			t.Fatalf("Sqrt(%d) expected %s, got %s", test.sqrtFrom, test.expected, fmt.Sprintf(test.fmt, actual))
		}
	}
}
