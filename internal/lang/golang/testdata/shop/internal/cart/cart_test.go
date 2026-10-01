package cart

import "testing"

func TestFull(t *testing.T) {
	c := New()
	c.Add("a")
	c.Add("b")
	if c.Full() {
		t.Fatal("two items is not full")
	}
	c.Add("c")
	if !c.Full() {
		t.Fatal("three items is full")
	}
}
