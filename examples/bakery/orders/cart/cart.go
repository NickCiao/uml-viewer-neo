// Package cart holds what a customer is buying.
package cart

import (
	"bakery/menu"
	"bakery/money"
)

// Line is one item in a cart and how many of it.
type Line struct {
	Item  menu.Item
	Count int
}

// Cart is a customer's order before they pay.
type Cart struct{ Lines []Line }

// Add puts n of item in the cart, adding to its line if it has one.
func (c *Cart) Add(item menu.Item, n int) {
	for i := range c.Lines {
		if c.Lines[i].Item.Name == item.Name {
			c.Lines[i].Count += n
			return
		}
	}
	c.Lines = append(c.Lines, Line{item, n})
}

// Total is what the cart costs before any discount.
func (c Cart) Total() money.Cents {
	var sum money.Cents
	for _, l := range c.Lines {
		sum += l.Item.Price * money.Cents(l.Count)
	}
	return sum
}
