package int10

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
)

func TestBig(t *testing.T) {
	for i := 0; i < 1e3; i++ {
		x := getBigString()
		y := getBigString()
		t.Run(fmt.Sprintf("%s, %s", x, y), func(t *testing.T) {
			t.Parallel()
			testBig(t, x, y)
		})
	}
}

func getBigString() string {
	n := rand.Intn(100)
	var s string
	if n == 0 {
		s = "0"
	} else {
		b := make([]byte, n)
		for j := range b {
			b[j] = '0' + byte(rand.Intn(10))
		}
		s = string(b)
	}
	return s
}

func testBig(t *testing.T, x, y string) {
	var bx, by, bz big.Int
	if _, ok := bx.SetString(x, 10); !ok {
		t.Fatal(x)
	}
	if _, ok := by.SetString(y, 10); !ok {
		t.Fatal(y)
	}
	ix, ok := NewIntString(x)
	if !ok {
		t.Fatal(x)
	}
	iy, ok := NewIntString(y)
	if !ok {
		t.Fatal(y)
	}
	var iz Int

	ops := []string{
		"+",
		"-",
		"*",
	}
	bfns := []func(*big.Int, *big.Int) *big.Int{
		bz.Add,
		func(x, y *big.Int) *big.Int {
			bz.Sub(x, y)
			bz.Abs(&bz)
			return &bz
		},
		bz.Mul,
	}
	ifns := []func(Int, Int){
		iz.Add,
		func(x, y Int) {
			iz.Diff(x, y)
		},
		func(x, y Int) {
			iz = ix.Mul(iy)
		},
	}

	for i, bfn := range bfns {
		t.Run(ops[i], func(t *testing.T) {
			bfn(&bx, &by)
			ifns[i](ix, iy)
			bs := bz.String()
			is := iz.String()
			if bs != is {
				t.Fatalf("got %s, want %s", is, bs)
			}
		})
	}
}
