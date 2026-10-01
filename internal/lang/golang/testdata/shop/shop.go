package shop

import (
	"fmt"

	"github.com/acme/shop/internal/cart"
)

// Run has three paths: one for the function, one loop, one condition.
func Run(items []string) {
	c := cart.New()
	for _, it := range items {
		if it != "" {
			c.Add(it)
		}
	}
	fmt.Println(c.Len())
}
