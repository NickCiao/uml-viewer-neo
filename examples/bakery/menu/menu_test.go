package menu

import "testing"

func TestParse(t *testing.T) {
	m, err := Parse("[[item]]\nname = \"croissant\"\nprice = 350\n")
	if err != nil || m["croissant"].Price != 350 {
		t.Fatalf("got %v, %v", m, err)
	}
}

func TestParseRejectsBrokenText(t *testing.T) {
	if _, err := Parse("[[item"); err == nil {
		t.Fatal("no error")
	}
}
