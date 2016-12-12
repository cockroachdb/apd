package apd

// ErrDecimal performs operations on decimals and collects errors during
// operations. If an error is already set, the operation is skipped. Designed to
// be used for many operations in a row, with a single error check at the end.
type ErrDecimal struct {
	Err error
}

// Cmp returns 0 if Err is set. Otherwise returns a.Cmp(b).
func (e *ErrDecimal) Cmp(a, b *Decimal) int {
	if e.Err != nil {
		return 0
	}
	var c int
	c, e.Err = a.Cmp(b)
	return c
}

// Sqrt performs d.Sqrt(x).
func (e *ErrDecimal) Sqrt(d, x *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Sqrt(x)
}

// Mul performs d.Mul(x, y).
func (e *ErrDecimal) Mul(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Mul(x, y)
}

// Add performs d.Add(x, y).
func (e *ErrDecimal) Add(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Add(x, y)
}

// Sub performs d.Sub(x, y).
func (e *ErrDecimal) Sub(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Sub(x, y)
}

// Quo performs d.Quo(x, y).
func (e *ErrDecimal) Quo(d, x, y *Decimal) {
	if e.Err != nil {
		return
	}
	e.Err = d.Quo(x, y)
}
