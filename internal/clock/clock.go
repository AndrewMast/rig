// Package clock is the time IO seam, so commands that stamp or date things stay
// testable with a frozen clock.
package clock

import "time"

// Clock yields the current time.
type Clock interface {
	Now() time.Time
}

// Real is the system clock.
type Real struct{}

// Now returns time.Now().
func (Real) Now() time.Time { return time.Now() }

// Fixed is a frozen clock for tests.
type Fixed struct{ T time.Time }

// Now returns the fixed time.
func (f Fixed) Now() time.Time { return f.T }
