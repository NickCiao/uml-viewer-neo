package web

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestStyleUsesTheDesignPaletteAndNoGlow(t *testing.T) {
	css, err := fs.ReadFile(FS, "style.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, hex := range []string{"#1C1A17", "#2E2924", "#534C43", "#8A8070", "#E8DCC4", "#A89E8C",
		"#C9A86A", "#3B8A4E", "#E0A030", "#E8503A", "#221F1B"} {
		if !strings.Contains(string(css), hex) {
			t.Errorf("style.css lacks %s from the Look table", hex)
		}
	}
	for _, m := range regexp.MustCompile(`font-size: (\d+)px`).FindAllStringSubmatch(string(css), -1) {
		if n, _ := strconv.Atoi(m[1]); n < 11 {
			t.Errorf("font-size %spx is below the 11px floor", m[1])
		}
	}
	for _, banned := range []string{"text-shadow", "blur(", "gradient("} {
		if strings.Contains(string(css), banned) {
			t.Errorf("style.css uses %s; the look has no glow", banned)
		}
	}
}
