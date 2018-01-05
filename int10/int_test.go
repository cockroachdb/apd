package int10

import (
	"fmt"
	"math"
	"math/big"
	"testing"
)

func (z Int) V(t *testing.T) {
	for _, d := range z {
		if d >= base {
			t.Fatalf("bad digit: %d", d)
		}
	}
	if len(z) > 0 && z[len(z)-1] == 0 {
		t.Fatal("trailing zero")
	}
}

func TestNewInt(t *testing.T) {
	tests := []uint64{
		0,
		1,
		2,
		9,
		10,
		11,
		100,
		1000,
		234567,
		math.MaxUint64,
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			a := NewInt(tc)
			if !a.Equal(NewInt(tc)) {
				t.Fatal("expected equal")
			}
			a.V(t)
			i := a.Uint64()
			if i != tc {
				t.Fatalf("got %d (%v), expected %v", i, a, tc)
			}
			got := a.String()
			s := fmt.Sprint(tc)
			if s != got {
				t.Fatalf("got %s, expected %s", got, s)
			}
		})
	}
}

func TestNewInt64(t *testing.T) {
	tests := map[int64]uint64{
		0:             0,
		-1:            1,
		1:             1,
		math.MaxInt64: math.MaxInt64,
		math.MinInt64: -math.MinInt64,
	}
	for tc, expect := range tests {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			a := NewInt64(tc)
			a.V(t)
			i := a.Uint64()
			if i != expect {
				t.Fatalf("got %d, expected %d", i, expect)
			}
		})
	}
}
func TestNewIntBig(t *testing.T) {
	tests := map[string]string{
		"0":  "0",
		"-0": "0",
		"1":  "1",
		"-1": "1",
		"1234145435656745634324524536456745634": "1234145435656745634324524536456745634",
	}
	for tc, expect := range tests {
		t.Run(tc, func(t *testing.T) {
			var b big.Int
			i, ok := b.SetString(tc, 10)
			if !ok {
				t.Fatal("bad string")
			}
			a := NewIntBig(i)
			a.V(t)
			s := a.String()
			if s != expect {
				t.Fatalf("got %s, expected %s", s, expect)
			}
		})
	}
}
func TestNewIntString(t *testing.T) {
	tests := []struct {
		s   string
		err bool
	}{
		{
			s: "0",
		},
		{
			s: "1",
		},
		{
			s:   "-1",
			err: true,
		},
		{
			s: "123",
		},
		{
			s: "349857598452734538945230",
		},
		{
			s:   "e",
			err: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			i, ok := NewIntString(tc.s)
			if !ok {
				if !tc.err {
					t.Fatal("unexpected error")
				}
				return
			}
			i.V(t)
			s := i.String()
			if s != tc.s {
				t.Fatalf("got %s, expected %s", s, tc.s)
			}
		})
	}
}

func TestIntEqual(t *testing.T) {
	tests := []struct {
		a, b  uint64
		equal bool
	}{
		{
			a:     1,
			b:     1,
			equal: true,
		},
		{
			a:     1,
			equal: false,
		},
		{
			a:     123,
			b:     1234,
			equal: false,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d = %d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			a.V(t)
			b.V(t)
			eq := a.Equal(b)
			if eq != tc.equal {
				t.Fatalf("got %v, expected %v", eq, tc.equal)
			}
		})
	}
}

func TestIntAdd(t *testing.T) {
	tests := []struct {
		a, b, c uint64
		d       bool
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			c: 1,
		},
		{
			a: 349482367,
			b: 23442,
			c: 349505809,
		},
		{
			a: 321,
			b: 148247592,
			c: 148247913,
		},
		{
			a: 9,
			b: 9,
			c: 8,
			d: true,
		},
		{
			a: 10,
			b: 10,
			c: 20,
		},
		{
			a: 9999,
			b: 1,
			c: 0,
			d: true,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d+%d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			c := NewInt(tc.c)
			a.V(t)
			b.V(t)
			c.V(t)
			d := a.AddCarry(a, b)
			if !a.Equal(c) {
				t.Fatalf("%s != %s", a, c)
			}
			if d != tc.d {
				t.Fatalf("%t != %t", d, tc.d)
			}
		})
	}
}

func TestIntSub(t *testing.T) {
	tests := []struct {
		a, b, c uint64
		d       bool
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			c: 1,
		},
		{
			a: 349482367,
			b: 23442,
			c: 349458925,
		},
		{
			a: 321,
			b: 148247592,
			c: 851752729,
			d: true,
		},
		{
			a: 9,
			b: 9,
			c: 0,
		},
		{
			a: 3,
			b: 4,
			c: 9,
			d: true,
		},
		{
			a: 20,
			b: 32,
			c: 88,
			d: true,
		},
		{
			a: 0,
			b: 1,
			c: 9,
			d: true,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d-%d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			c := NewInt(tc.c)
			a.V(t)
			b.V(t)
			c.V(t)
			var z Int
			d := z.Sub(a, b)
			z.V(t)
			if !z.Equal(c) {
				t.Fatalf("%s != %s", z, c)
			}
			if d != tc.d {
				t.Fatalf("%t != %t", d, tc.d)
			}
		})
	}
}

func TestIntMul(t *testing.T) {
	tests := []struct {
		a, b, c uint64
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			b: 0,
			c: 0,
		},
		{
			a: 10,
			b: 1,
			c: 10,
		},
		{
			a: 10,
			b: 100,
			c: 1000,
		},
		{
			a: 9,
			b: 9,
			c: 81,
		},
		{
			a: 3,
			b: 4,
			c: 12,
		},
		{
			a: 20,
			b: 32,
			c: 640,
		},
		{
			a: 46820,
			b: 56282,
			c: 2635123240,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d*%d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			c := NewInt(tc.c)
			res := a.Mul(b)
			if !res.Equal(c) {
				t.Fatalf("%s != %s", res, c)
			}
		})
	}
}

func TestIntAddCarry(t *testing.T) {
	tests := []struct {
		a, b, c uint64
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			b: 0,
			c: 1,
		},
		{
			a: 10,
			b: 1,
			c: 11,
		},
		{
			a: 9,
			b: 9,
			c: 18,
		},
		{
			a: 1000,
			b: 9000,
			c: 10000,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d+%d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			c := NewInt(tc.c)
			var z Int
			z.Add(a, b)
			if !z.Equal(c) {
				t.Fatalf("got %s, expected %s", z, c)
			}
		})
	}
}

func TestIntMul10(t *testing.T) {
	tests := []struct {
		a, c uint64
		b    int
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			b: 0,
			c: 1,
		},
		{
			a: 1,
			b: 1,
			c: 10,
		},
		{
			a: 10,
			b: 10,
			c: 100000000000,
		},
		{
			a: 9,
			b: 9,
			c: 9000000000,
		},
		{
			a: 1234,
			b: -1,
			c: 123,
		},
		{
			a: 1234,
			b: -2,
			c: 12,
		},
		{
			a: 1234,
			b: -3,
			c: 1,
		},
		{
			a: 1234,
			b: 0,
			c: 1234,
		},
		{
			a: 1234,
			b: -4,
			c: 0,
		},
		{
			a: 1234,
			b: -5,
			c: 0,
		},
		{
			a: 1,
			b: 2,
			c: 100,
		},
		{
			a: 0,
			b: 1,
			c: 0,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d*10^%d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			c := NewInt(tc.c)
			a.Mul10(tc.b)
			a.V(t)
			if !a.Equal(c) {
				t.Fatalf("got %s, expected %s", a, c)
			}
		})
	}
}

func TestIntCmp(t *testing.T) {
	tests := []struct {
		a, b uint64
		c    int
	}{
		{
			a: 0,
			b: 0,
			c: 0,
		},
		{
			a: 1,
			b: 0,
			c: 1,
		},
		{
			a: 1,
			b: 1,
			c: 0,
		},
		{
			a: 1,
			b: 10,
			c: -1,
		},
		{
			a: 10,
			b: 1,
			c: 1,
		},
		{
			a: 2,
			b: 1,
			c: 1,
		},
		{
			a: 1,
			b: 2,
			c: -1,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d, %d", tc.a, tc.b), func(t *testing.T) {
			a := NewInt(tc.a)
			b := NewInt(tc.b)
			c := a.Cmp(b)
			if tc.c != c {
				t.Fatalf("got %d, expected %d", c, tc.c)
			}
		})
	}
}

func TestIntSplit(t *testing.T) {
	tests := []struct {
		a, integ, frac uint64
		n              int
	}{
		{
			a:     123456,
			n:     2,
			integ: 1234,
			frac:  56,
		},
		{
			a:    123456,
			n:    10,
			frac: 123456,
		},
		{
			a: 0,
			n: 1,
		},
		{
			a:     1,
			n:     0,
			integ: 1,
		},
		{
			a:    1,
			n:    1,
			frac: 1,
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d, %d", tc.a, tc.n), func(t *testing.T) {
			a := NewInt(tc.a)
			integ, frac := a.Split(tc.n)
			if integ.Uint64() != tc.integ {
				t.Fatalf("got %s, expected %d", integ, tc.integ)
			}
			if frac.Uint64() != tc.frac {
				t.Fatalf("got %s, expected %d", frac, tc.frac)
			}
		})
	}
}
