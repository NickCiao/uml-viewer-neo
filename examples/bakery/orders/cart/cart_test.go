package cart

import (
	"testing"

	"bakery/menu"
)

func TestAddAndTotal(t *testing.T) {
	var c Cart
	c.Add(menu.Item{Name: "croissant", Price: 350}, 2)
	c.Add(menu.Item{Name: "sourdough", Price: 600}, 1)
	c.Add(menu.Item{Name: "croissant", Price: 350}, 1)
	if len(c.Lines) != 2 || c.Total() != 1650 {
		t.Fatalf("got %+v, total %d", c.Lines, c.Total())
	}
}
