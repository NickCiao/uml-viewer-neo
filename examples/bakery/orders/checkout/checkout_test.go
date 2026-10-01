package checkout

import (
	"testing"
	"time"

	"bakery/menu"
	"bakery/orders/cart"
)

func cartOf(n int) cart.Cart {
	var c cart.Cart
	c.Add(menu.Item{Name: "croissant", Price: 350}, n)
	return c
}

func at(hour int) time.Time { return time.Date(2026, 10, 1, hour, 0, 0, 0, time.UTC) }

func TestPayWithoutACode(t *testing.T) {
	if b := Pay(cartOf(2), "", at(10)); b.Total != 700 || b.Discount != 0 {
		t.Fatalf("got %+v", b)
	}
}

func TestEarlyBirdOnlyBeforeNine(t *testing.T) {
	if d := Discount(cartOf(2), "EARLYBIRD", at(8)); d != 140 {
		t.Fatalf("at 8: %d", d)
	}
	if d := Discount(cartOf(2), "EARLYBIRD", at(9)); d != 0 {
		t.Fatalf("at 9: %d", d)
	}
}

func TestDozenNeedsTwelveOfOneThing(t *testing.T) {
	if d := Discount(cartOf(12), "DOZEN", at(10)); d != 420 {
		t.Fatalf("12: %d", d)
	}
	if d := Discount(cartOf(11), "DOZEN", at(10)); d != 0 {
		t.Fatalf("11: %d", d)
	}
}

func TestBirthdayNeedsTenDollars(t *testing.T) {
	if d := Discount(cartOf(3), "BIRTHDAY", at(10)); d != 500 {
		t.Fatalf("$10.50: %d", d)
	}
	if d := Discount(cartOf(2), "BIRTHDAY", at(10)); d != 0 {
		t.Fatalf("$7.00: %d", d)
	}
}
