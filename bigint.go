// Copyright 2022 The Cockroach Authors.
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
	"fmt"
	"math/big"
	"math/rand"
	"unsafe"
)

// BigInt is a wrapper around big.Int. It minimizes memory allocation by using
// an inline array to back the big.Int's variable-length "nat" slice when the
// integer's value is sufficiently small.
// The zero value is ready to use.
// The value must not be copied after first use.
type BigInt struct {
	// The wrapped big.Int. Methods should access this field through inner.
	_inner big.Int

	// The inlined backing array for (big.Int).abs when the value is small.
	//
	// Each BigInt maintains (through big.Int) an internal reference to a
	// variable-length integer value, which is represented by a []big.Word. The
	// _inline field and lazyInit method combine to allow BigInt to inline this
	// variable-length integer array within the BigInt struct when its value is
	// sufficiently small. In lazyInit, we point the _inner field's slice at the
	// _inline array. big.Int will avoid re-allocating this array until it is
	// provided with a value that exceeds the initial capacity.
	_inline [inlineWords]big.Word

	// Disallow copying of BigInt structs. The self-referencing pointer from
	// _inner to _inline makes this unsafe, as it could allow aliasing between
	// two BigInt structs which would be hidden from escape analysis due to the
	// call to noescape in lazyInit. If the first BigInt then fell out of scope
	// and was GCed, this could corrupt the state of the second BigInt.
	//
	// We use static and dynamic analysis to prevent copying of this struct,
	// which is similar to sync.Cond and strings.Builder.
	_noCopy noCopy
	_addr   *BigInt
}

// Set the inlineWords capacity to accommodate any value that would fit in a
// 128-bit integer (i.e. values up to 2^128 - 1).
const inlineWords = 2

// NewBigInt allocates and returns a new BigInt set to x.
//
// NOTE: BigInt jumps through hoops to avoid escaping to the heap. As such, most
// users of BigInt should not need this function. They should instead declare a
// zero-valued BigInt directly on the stack and interact with references to this
// stack-allocated value. Recall that the zero-valued BigInt is ready to use.
func NewBigInt(x int64) *BigInt {
	return new(BigInt).SetInt64(x)
}

func (b *BigInt) inner() *big.Int {
	b.copyCheck()
	return &b._inner
}

func (b *BigInt) innerOrNil() *big.Int {
	if b == nil {
		return nil
	}
	return b.inner()
}

// noescape hides a pointer from escape analysis. noescape is the identity
// function but escape analysis doesn't think the output depends on the input.
// noescape is inlined and currently compiles down to zero instructions.
//
// USE CAREFULLY!
//
// This was copied from strings.Builder, which has identical code which was
// itself copied from the runtime.
// For more, see issues #23382 and #7921 in github.com/golang/go.
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

// This was copied from (strings.Builder).copyCheck.
func (b *BigInt) copyCheck() {
	if b._addr == nil {
		// This hack works around a failing of Go's escape analysis that was
		// causing b to escape and be heap-allocated. See issue #23382 in
		// github.com/golang/go.
		b._addr = (*BigInt)(noescape(unsafe.Pointer(b)))
	} else if b._addr != b {
		panic("apd: illegal use of non-zero BigInt copied by value")
	}
}

// lazyInit lazily initializes a zero BigInt value. Use of the method is not
// required for correctness, but can improve performance by avoiding a separate
// heap allocation within the big.Int field.
func (b *BigInt) lazyInit() {
	if b._inner.Bits() == nil {
		// If the big.Int has a nil slice, point it at the inline array.

		// Before doing so, zero out the inline array, in case it had a value
		// previously. This is necessary in edge cases where _inner initially
		// uses the _inline array, then switches to a separate backing array,
		// then performs arithmetic which results in its value being set to 0.
		// In such cases, it is possible for _inner's slice to be reset to nil
		// because big.Int treats this the same as the value 0. If we did not
		// zero out the _inline slice here, we could unintentionally change its
		// value.
		b._inline = [inlineWords]big.Word{}

		// Point the big.Int at the inline array. When doing so, use noescape
		// for the same reason we did above in copyCheck. Go's escape analysis
		// struggles with self-referential pointers, and we don't want this
		// forcing the BigInt to escape to the heap.
		inline := (*[inlineWords]big.Word)(noescape(unsafe.Pointer(&b._inline[0])))
		b._inner.SetBits(inline[:0])
	}
}

///////////////////////////////////////////////////////////////////////////////
//                        big.Int API wrapper methods                        //
///////////////////////////////////////////////////////////////////////////////

// Abs calls (big.Int).Abs.
func (b *BigInt) Abs(x *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Abs(x.inner())
	return b
}

// Add calls (big.Int).Add.
func (b *BigInt) Add(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Add(x.inner(), y.inner())
	return b
}

// And calls (big.Int).And.
func (b *BigInt) And(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().And(x.inner(), y.inner())
	return b
}

// AndNot calls (big.Int).AndNot.
func (b *BigInt) AndNot(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().AndNot(x.inner(), y.inner())
	return b
}

// Append calls (big.Int).Append.
func (b *BigInt) Append(buf []byte, base int) []byte {
	return b.inner().Append(buf, base)
}

// Binomial calls (big.Int).Binomial.
func (b *BigInt) Binomial(n, k int64) *BigInt {
	b.lazyInit()
	b.inner().Binomial(n, k)
	return b
}

// Bit calls (big.Int).Bit.
func (b *BigInt) Bit(i int) uint {
	return b.inner().Bit(i)
}

// BitLen calls (big.Int).BitLen.
func (b *BigInt) BitLen() int {
	return b.inner().BitLen()
}

// Bits calls (big.Int).Bits.
func (b *BigInt) Bits() []big.Word {
	// Don't expose direct access to the big.Int's word slice.
	panic("unimplemented")
}

// Bytes calls (big.Int).Bytes.
func (b *BigInt) Bytes() []byte {
	return b.inner().Bytes()
}

// Cmp calls (big.Int).Cmp.
func (b *BigInt) Cmp(y *BigInt) (r int) {
	return b.inner().Cmp(y.inner())
}

// CmpAbs calls (big.Int).CmpAbs.
func (b *BigInt) CmpAbs(y *BigInt) int {
	return b.inner().CmpAbs(y.inner())
}

// Div calls (big.Int).Div.
func (b *BigInt) Div(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Div(x.inner(), y.inner())
	return b
}

// DivMod calls (big.Int).DivMod.
func (b *BigInt) DivMod(x, y, m *BigInt) (*BigInt, *BigInt) {
	b.lazyInit()
	m.lazyInit()
	b.inner().DivMod(x.inner(), y.inner(), m.inner())
	return b, m
}

// Exp calls (big.Int).Exp.
func (b *BigInt) Exp(x, y, m *BigInt) *BigInt {
	b.lazyInit()
	if b.inner().Exp(x.inner(), y.inner(), m.innerOrNil()) == nil {
		return nil
	}
	return b
}

// FillBytes calls (big.Int).FillBytes.
func (b *BigInt) FillBytes(buf []byte) []byte {
	return b.inner().FillBytes(buf)
}

// Format calls (big.Int).Format.
func (b *BigInt) Format(s fmt.State, ch rune) {
	b.innerOrNil().Format(s, ch)
}

// GCD calls (big.Int).GCD.
func (b *BigInt) GCD(x, y, m, n *BigInt) *BigInt {
	b.lazyInit()
	b.inner().GCD(x.innerOrNil(), y.innerOrNil(), m.inner(), n.inner())
	return b
}

// GobEncode calls (big.Int).GobEncode.
func (b *BigInt) GobEncode() ([]byte, error) {
	return b.innerOrNil().GobEncode()
}

// GobDecode calls (big.Int).GobDecode.
func (b *BigInt) GobDecode(buf []byte) error {
	b.lazyInit()
	return b.inner().GobDecode(buf)
}

// Int64 calls (big.Int).Int64.
func (b *BigInt) Int64() int64 {
	return b.inner().Int64()
}

// IsInt64 calls (big.Int).IsInt64.
func (b *BigInt) IsInt64() bool {
	return b.inner().IsInt64()
}

// IsUint64 calls (big.Int).IsUint64.
func (b *BigInt) IsUint64() bool {
	return b.inner().IsUint64()
}

// Lsh calls (big.Int).Lsh.
func (b *BigInt) Lsh(x *BigInt, n uint) *BigInt {
	b.lazyInit()
	b.inner().Lsh(x.inner(), n)
	return b
}

// MarshalJSON calls (big.Int).MarshalJSON.
func (b *BigInt) MarshalJSON() ([]byte, error) {
	return b.innerOrNil().MarshalJSON()
}

// MarshalText calls (big.Int).MarshalText.
func (b *BigInt) MarshalText() (text []byte, err error) {
	return b.innerOrNil().MarshalText()
}

// Mod calls (big.Int).Mod.
func (b *BigInt) Mod(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Mod(x.inner(), y.inner())
	return b
}

// ModInverse calls (big.Int).ModInverse.
func (b *BigInt) ModInverse(g, n *BigInt) *BigInt {
	b.lazyInit()
	if b.inner().ModInverse(g.inner(), n.inner()) == nil {
		return nil
	}
	return b
}

// ModSqrt calls (big.Int).ModSqrt.
func (b *BigInt) ModSqrt(x, p *BigInt) *BigInt {
	b.lazyInit()
	if b.inner().ModSqrt(x.inner(), p.inner()) == nil {
		return nil
	}
	return b
}

// Mul calls (big.Int).Mul.
func (b *BigInt) Mul(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Mul(x.inner(), y.inner())
	return b
}

// MulRange calls (big.Int).MulRange.
func (b *BigInt) MulRange(x, y int64) *BigInt {
	b.lazyInit()
	b.inner().MulRange(x, y)
	return b
}

// Neg calls (big.Int).Neg.
func (b *BigInt) Neg(x *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Neg(x.inner())
	return b
}

// Not calls (big.Int).Not.
func (b *BigInt) Not(x *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Not(x.inner())
	return b
}

// Or calls (big.Int).Or.
func (b *BigInt) Or(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Or(x.inner(), y.inner())
	return b
}

// ProbablyPrime calls (big.Int).ProbablyPrime.
func (b *BigInt) ProbablyPrime(n int) bool {
	return b.inner().ProbablyPrime(n)
}

// Quo calls (big.Int).Quo.
func (b *BigInt) Quo(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Quo(x.inner(), y.inner())
	return b
}

// QuoRem calls (big.Int).QuoRem.
func (b *BigInt) QuoRem(x, y, r *BigInt) (*BigInt, *BigInt) {
	b.lazyInit()
	r.lazyInit()
	b.inner().QuoRem(x.inner(), y.inner(), r.inner())
	return b, r
}

// Rand calls (big.Int).Rand.
func (b *BigInt) Rand(rnd *rand.Rand, n *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Rand(rnd, n.inner())
	return b
}

// Rem calls (big.Int).Rem.
func (b *BigInt) Rem(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Rem(x.inner(), y.inner())
	return b
}

// Rsh calls (big.Int).Rsh.
func (b *BigInt) Rsh(x *BigInt, n uint) *BigInt {
	b.lazyInit()
	b.inner().Rsh(x.inner(), n)
	return b
}

// Scan calls (big.Int).Scan.
func (b *BigInt) Scan(s fmt.ScanState, ch rune) error {
	b.lazyInit()
	return b.inner().Scan(s, ch)
}

// Set calls (big.Int).Set.
func (b *BigInt) Set(x *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Set(x.inner())
	return b
}

// SetBit calls (big.Int).SetBit.
func (b *BigInt) SetBit(x *BigInt, i int, v uint) *BigInt {
	b.lazyInit()
	b.inner().SetBit(x.inner(), i, v)
	return b
}

// SetBits calls (big.Int).SetBits.
func (b *BigInt) SetBits(_ []big.Word) *BigInt {
	// Don't expose direct access to the big.Int's word slice.
	panic("unimplemented")
}

// SetBytes calls (big.Int).SetBytes.
func (b *BigInt) SetBytes(buf []byte) *BigInt {
	b.lazyInit()
	b.inner().SetBytes(buf)
	return b
}

// SetInt64 calls (big.Int).SetInt64.
func (b *BigInt) SetInt64(x int64) *BigInt {
	b.lazyInit()
	b.inner().SetInt64(x)
	return b
}

// SetString calls (big.Int).SetString.
func (b *BigInt) SetString(s string, base int) (*BigInt, bool) {
	b.lazyInit()
	if _, ok := b.inner().SetString(s, base); !ok {
		return nil, false
	}
	return b, true
}

// SetUint64 calls (big.Int).SetUint64.
func (b *BigInt) SetUint64(x uint64) *BigInt {
	b.lazyInit()
	b.inner().SetUint64(x)
	return b
}

// Sign calls (big.Int).Sign.
func (b *BigInt) Sign() int {
	return b.inner().Sign()
}

// Sqrt calls (big.Int).Sqrt.
func (b *BigInt) Sqrt(x *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Sqrt(x.inner())
	return b
}

// String calls (big.Int).String.
func (b *BigInt) String() string {
	return b.inner().String()
}

// Sub calls (big.Int).Sub.
func (b *BigInt) Sub(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Sub(x.inner(), y.inner())
	return b
}

// Text calls (big.Int).Text.
func (b *BigInt) Text(base int) string {
	return b.inner().Text(base)
}

// TrailingZeroBits calls (big.Int).TrailingZeroBits.
func (b *BigInt) TrailingZeroBits() uint {
	return b.inner().TrailingZeroBits()
}

// Uint64 calls (big.Int).Uint64.
func (b *BigInt) Uint64() uint64 {
	return b.inner().Uint64()
}

// UnmarshalJSON calls (big.Int).UnmarshalJSON.
func (b *BigInt) UnmarshalJSON(text []byte) error {
	b.lazyInit()
	return b.inner().UnmarshalJSON(text)
}

// UnmarshalText calls (big.Int).UnmarshalText.
func (b *BigInt) UnmarshalText(text []byte) error {
	b.lazyInit()
	return b.inner().UnmarshalText(text)
}

// Xor calls (big.Int).Xor.
func (b *BigInt) Xor(x, y *BigInt) *BigInt {
	b.lazyInit()
	b.inner().Xor(x.inner(), y.inner())
	return b
}
