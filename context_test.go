// Copyright 2026 The Cockroach Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apd

import (
	"testing"
	"time"
)

// TestLnSeriesWithSubnormalTerms verifies that the power series of Ln
// converges when the terms of the series are below the exponent range of the
// context. The terms are much smaller than the result, so a term that is
// subnormal in the context's range must not stop the series: the values are
// the ones Python's decimal gives for the same context.
func TestLnSeriesWithSubnormalTerms(t *testing.T) {
	tests := []struct {
		x    string
		want string
	}{
		{x: "0.95", want: "-0.05129329439"},
		{x: "0.9", want: "-0.1053605157"},
		{x: "1.1", want: "0.09531017980"},
		{x: "1.001", want: "0.0009995003331"},
		{x: "0.99999", want: "-0.00001000005000"},
		{x: "9E8", want: "20.61790532"},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			c := Context{
				Precision:   10,
				MaxExponent: 999,
				MinExponent: -5,
				Traps:       DefaultTraps,
				Rounding:    RoundHalfEven,
			}
			x, _, err := NewFromString(tc.x)
			if err != nil {
				t.Fatal(err)
			}
			var d Decimal
			var cond Condition
			// The series has to converge. The deadline turns a hang into a
			// failure instead of a test that never ends.
			done := make(chan error, 1)
			go func() {
				var err error
				cond, err = c.Ln(&d, x)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Ln did not return")
			}
			if s := d.Text('g'); s != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, s)
			}
			if cond != Inexact|Rounded {
				t.Fatalf("expected %s, got %s", Inexact|Rounded, cond)
			}
		})
	}
}

// TestLnInexactTrap verifies that the Inexact trap of the context is
// honoured: the series converges, and the trap fires on the result.
func TestLnInexactTrap(t *testing.T) {
	c := Context{
		Precision:   10,
		MaxExponent: 999,
		MinExponent: -5,
		Traps:       DefaultTraps | Inexact,
		Rounding:    RoundHalfEven,
	}
	x, _, err := NewFromString("0.95")
	if err != nil {
		t.Fatal(err)
	}
	var d Decimal
	done := make(chan error, 1)
	go func() {
		_, err := c.Ln(&d, x)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the Inexact trap to fire")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Ln did not return")
	}
}
