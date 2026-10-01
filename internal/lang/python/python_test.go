package python

import (
	"testing"

	"uml-viewer-neo/internal/facts"
)

func TestScanShop(t *testing.T) {
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "python" || s.Prefix != "shop" || len(s.Notes) != 1 {
		t.Fatalf("scan = %+v", s)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	if len(byID) != 5 || byID["src/shop/__init__.py"].Path != "" || byID["src/shop/cart/basket.py"].Path != "cart/basket" {
		t.Fatalf("modules = %+v", s.Modules)
	}
	checkout := byID["src/shop/checkout.py"]
	var sawPricing, sawRequests bool
	for _, imp := range checkout.Imports {
		sawPricing = sawPricing || (imp.To == "src/shop/pricing.py" && imp.Project)
		sawRequests = sawRequests || (imp.Module == "requests" && !imp.Project && !imp.Std)
	}
	if !sawPricing || !sawRequests {
		t.Fatalf("imports = %+v", checkout.Imports)
	}
	run := checkout.Functions[0]
	if run.Name != "run" || run.Start != 15 || run.End != 20 || run.CoverStart != 16 || run.CC != 3 {
		t.Fatalf("run = %+v", run)
	}
}

func TestLangIsRecognisedByPyproject(t *testing.T) {
	if Lang.Name != "python" || Lang.Markers[0] != "pyproject.toml" || Lang.Markers[1] != "setup.py" {
		t.Fatalf("Lang = %+v", Lang)
	}
}
