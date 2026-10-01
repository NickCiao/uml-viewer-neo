// Package report sums up a day's sales for the owner.
package report

import (
	"fmt"
	"strings"
	"time"

	"bakery/money"
	"bakery/orders/cart"
)

// Sale is a paid cart. A refund is a sale with a negative Paid.
type Sale struct {
	Cart cart.Cart
	At   time.Time
	Paid money.Cents
}

// Takings is the money taken, less refunds.
func Takings(sales []Sale) money.Cents {
	var sum money.Cents
	for _, s := range sales {
		sum += s.Paid
	}
	return sum
}

// Daily is the end-of-day summary: takings, the busiest hour, the best
// seller, and a warning when refunds look high.
func Daily(sales []Sale) string {
	if len(sales) == 0 {
		return "No sales today.\n"
	}
	byHour := map[int]int{}
	byItem := map[string]int{}
	refunds := 0
	for _, s := range sales {
		if s.Paid < 0 {
			refunds++
			continue
		}
		byHour[s.At.Hour()]++
		for _, l := range s.Cart.Lines {
			byItem[l.Item.Name] += l.Count
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Takings: %s from %d sales\n", Takings(sales), len(sales)-refunds)
	busiest, most := -1, 0
	for h, n := range byHour {
		if n > most || n == most && h < busiest {
			busiest, most = h, n
		}
	}
	if busiest >= 0 {
		fmt.Fprintf(&b, "Busiest hour: %02d:00 (%d sales)\n", busiest, most)
	}
	best, sold := "", 0
	for name, n := range byItem {
		if n > sold || n == sold && name < best {
			best, sold = name, n
		}
	}
	if best != "" {
		fmt.Fprintf(&b, "Best seller: %s (%d)\n", best, sold)
	}
	if refunds*4 > len(sales) {
		fmt.Fprintf(&b, "Warning: %d of %d sales were refunded\n", refunds, len(sales))
	}
	return b.String()
}
