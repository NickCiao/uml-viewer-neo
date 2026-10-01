// Package checkout turns a cart into a bill.
package checkout

import (
	"time"

	"bakery/money"
	"bakery/orders/cart"
)

// Bill is what the customer pays.
type Bill struct {
	Subtotal, Discount, Total money.Cents
}

// Pay bills a cart, applying the discount code if it gives one.
func Pay(c cart.Cart, code string, at time.Time) Bill {
	d := Discount(c, code, at)
	return Bill{Subtotal: c.Total(), Discount: d, Total: c.Total() - d}
}

// Discount is what a code takes off a cart paid for at a given time.
func Discount(c cart.Cart, code string, at time.Time) money.Cents {
	total := c.Total()
	switch code {
	case "":
		return 0
	case "EARLYBIRD":
		if at.Hour() < 9 {
			return total.Percent(20)
		}
	case "DOZEN":
		for _, l := range c.Lines {
			if l.Count >= 12 {
				return total.Percent(10)
			}
		}
	case "BIRTHDAY":
		if total >= 1000 {
			return 500
		}
	case "STAFF":
		return total.Percent(50)
	}
	return 0
}
