package report

import "testing"

func TestTakingsSubtractsRefunds(t *testing.T) {
	if got := Takings([]Sale{{Paid: 700}, {Paid: 350}, {Paid: -350}}); got != 700 {
		t.Fatalf("got %d", got)
	}
}
