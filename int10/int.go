package int10

import (
	"math"
	"math/big"
	"strings"
)

// Int represents an unsigned, base-10, multi-precision integer. Each index is a single base-10 digit, in reverse order as written. That is, [0] is the 1s digit, [1] 10s, [2] 100s, etc. 0 is represented by nil or an empty slice.
type Int []Word

type Word uint8

const base = 10

// NewInt makes a new Int with value x.
func NewInt(x uint64) Int {
	if x == 0 {
		return nil
	}
	var arr [20]Word
	i := 0
	for ; x != 0; i++ {
		arr[i] = Word(x % base)
		x /= base
	}
	a := make(Int, i)
	copy(a, arr[:i])
	return a
}

// NewInt64 makes a new Int with value abs(x).
func NewInt64(x int64) Int {
	if x == 0 {
		return nil
	}
	if x >= 0 {
		return NewInt(uint64(x))
	}
	if x == math.MinInt64 {
		return NewInt(-math.MinInt64)
	}
	return NewInt(uint64(-x))
}

// NewIntBig makes a new Int with value abs(x).
func NewIntBig(x *big.Int) Int {
	s := x.String()
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	i, _ := NewIntString(s)
	return i
}

// NewIntString makes a new Int with value s. s must contain only characters 0-9. The second return value is false otherwise.
func NewIntString(s string) (Int, bool) {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return nil, true
	}
	x := make(Int, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return nil, false
		}
		x[len(x)-i-1] = Word(c - '0')
	}
	return x, true
}

// SetInt64 sets a to x.
func (a *Int) SetInt64(x int64) {
	*a = NewInt64(x)
}

// SetString sets a to s and returns whether s was a valid string.
func (a *Int) SetString(s string) bool {
	var ok bool
	*a, ok = NewIntString(s)
	return ok
}

// Set sets z to the value of x and returns z.
func (z *Int) Set(x *Int) *Int {
	if z != x {
		*z = append((*z)[:0], (*x)...)
	}
	return z
}

// SetInt sets z to x and returns z.
func (z *Int) SetInt(x uint64) *Int {
	*z = NewInt(x)
	return z
}

// Uint64 returns a as a uint64. If a cannot be represented in a uint64, it is undefined.
func (a Int) Uint64() uint64 {
	if len(a) == 0 {
		return 0
	}
	var x uint64
	var m uint64 = 1
	for _, d := range a {
		x += uint64(d) * m
		m *= 10
	}
	return x
}

// Int64 returns a as a int64. If a cannot be represented in a int64, it is undefined.
func (a Int) Int64() int64 {
	if len(a) == 0 {
		return 0
	}
	var x int64
	var m int64 = 1
	for _, d := range a {
		x += int64(d) * m
		m *= 10
	}
	return x
}

func (a Int) Cmp(b Int) int {
	if len(a) > len(b) {
		return 1
	}
	if len(b) > len(a) {
		return -1
	}
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

// Zero returns whether z is 0.
func (z Int) Zero() bool {
	for _, d := range z {
		if d != 0 {
			return false
		}
	}
	return true
}

// Equal returns whether a == b. a and b are required to not have any leading 0s.
func (a Int) Equal(b Int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func (z Int) String() string {
	if len(z) == 0 {
		return "0"
	}
	b := make([]byte, len(z))
	for i, v := range z {
		b[len(b)-i-1] = byte(v + '0')
	}
	return string(b)
}

// AddCarry sets z to x+y, with carry bit d. That is, x+y = z+d.
func (z *Int) AddCarry(x, y Int) (d bool) {
	return z.add(x, y, false)
}

// Add sets z to x+y.
func (z *Int) Add(x, y Int) {
	d := z.AddCarry(x, y)
	if d {
		// Since add omits leading zeros, we need to guarantee they are here.
		n := len(x)
		if len(y) > n {
			n = len(y)
		}
		zeroes := n - len(*z)
		*z = append(*z, make(Int, zeroes)...)
		*z = append(*z, 1)
	}
}

// Sub sets z to x-y. d is the borrow bit.
func (z *Int) Sub(x, y Int) (d bool) {
	return z.add(x, y, true)
}

// Diff sets z to the difference of x and y. That is, |x-y|. d is true if x-y < 0.
func (z *Int) Diff(x, y Int) (d bool) {
	d = z.add(x, y, true)
	if d {
		n := len(x)
		if len(y) > n {
			n = len(y)
		}
		c := append(make(Int, n), 1)
		z.Sub(c, *z)
	}
	return d
}

func (z *Int) add(x, y Int, sub bool) (d bool) {
	n := len(x)
	if len(y) > n {
		n = len(y)
	}
	if cap(*z) < n {
		*z = make(Int, 0, n)
	} else {
		*z = (*z)[:0]
	}
	if len(x) == 0 && !sub {
		*z = append(*z, y...)
		return false
	}
	if len(y) == 0 {
		*z = append(*z, x...)
		return false
	}
	var s, _d, t int16
	lastNonzero := -1
	for i := 0; i < n; i++ {
		if i >= len(x) {
			if sub {
				s = -int16(y[i])
			} else {
				s = int16(y[i])
			}
		} else if i >= len(y) {
			s = int16(x[i])
		} else if sub {
			s = int16(x[i]) - int16(y[i])
		} else {
			s = int16(x[i]) + int16(y[i])
		}
		s += _d
		if s < 0 {
			t = s + base
			_d = -1
		} else {
			t = s % base
			_d = s / base
		}
		if t != 0 {
			lastNonzero = i
		}
		*z = append(*z, Word(t))
	}
	*z = (*z)[:lastNonzero+1]
	return _d != 0
}

func (a Int) Mul(b Int) Int {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	var c Int
	for i, d := range b {
		t := a.mul(d)
		t.Mul10(i)
		c.Add(c, t)
	}
	return c
}

// Mul10 multiplies a by 10^n in place and returns a. If n < 0, a is truncated.
func (a *Int) Mul10(n int) *Int {
	if a.Zero() {
		return a
	}
	if n > 0 {
		*a = append(make(Int, n), *a...)
	} else if n <= -len(*a) {
		*a = (*a)[:0]
	} else if n < 0 {
		*a = (*a)[-n:]
	}
	return a
}

func (a Int) mul(b Word) Int {
	var t uint64
	var c, w Int
	for i, d := range a {
		t = uint64(d * b)
		w = NewInt(t)
		w.Mul10(i)
		c.Add(c, w)
	}
	return c
}

// Split sets frac to the lowest n digits of a and integ to the remainder. If
// n >= len(a), frac is set to a and integ is nil. integ and frac are shallow
// copies of a.
func (a Int) Split(n int) (integ, frac Int) {
	if n >= len(a) {
		return nil, a
	}
	return a[n:], a[:n]
}

// High returns the highest digit of a.
func (a Int) High() Word {
	if len(a) == 0 {
		return 0
	}
	return a[len(a)-1]
}

// Low returns the lowest digit of a.
func (a Int) Low() Word {
	if len(a) == 0 {
		return 0
	}
	return a[0]
}
