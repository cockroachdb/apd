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
