// Command bakery rings up one sample order and prints the day's report.
package main

import (
	"fmt"
	"log"
	"time"

	"bakery/menu"
	"bakery/orders/cart"
	"bakery/orders/checkout"
	"bakery/report"
)

const sampleMenu = `
[[item]]
name = "croissant"
price = 350

[[item]]
name = "sourdough"
price = 600
`

func main() {
	m, err := menu.Parse(sampleMenu)
	if err != nil {
		log.Fatal(err)
	}
	var c cart.Cart
	c.Add(m["croissant"], 2)
	c.Add(m["sourdough"], 1)
	now := time.Now()
	bill := checkout.Pay(c, "EARLYBIRD", now)
	fmt.Printf("Total: %s (saved %s)\n\n", bill.Total, bill.Discount)
	fmt.Print(report.Daily([]report.Sale{{Cart: c, At: now, Paid: bill.Total}}))
}
