// Package money counts in cents, so sums never drift.
package money

import "fmt"

// Cents is an amount of money in cents.
type Cents int64

// String formats c as dollars, like "$4.50".
func (c Cents) String() string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s$%d.%02d", sign, c/100, c%100)
}

// Percent is p percent of c, rounded down.
func (c Cents) Percent(p int) Cents { return c * Cents(p) / 100 }
