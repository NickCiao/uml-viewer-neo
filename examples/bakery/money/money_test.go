package money

import "testing"

func TestString(t *testing.T) {
	for c, want := range map[Cents]string{450: "$4.50", 5: "$0.05", -1200: "-$12.00"} {
		if got := c.String(); got != want {
			t.Errorf("%d: got %q, want %q", c, got, want)
		}
	}
}

func TestPercent(t *testing.T) {
	if got := Cents(999).Percent(20); got != 199 {
		t.Errorf("got %d", got)
	}
}
