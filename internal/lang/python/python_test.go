package python

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestScanMarksWhatOtherModulesAreMeantToCall(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"pkg/__init__.py": "",
		"pkg/mod.py": "def run(): pass\ndef _helper(): pass\n" +
			"class Shop:\n    def __init__(self): pass\n    def add(self): pass\n" +
			"    def _check(self): pass\n    def __mangled(self): pass\n" +
			"class _Hidden:\n    def visible(self): pass\n",
		"pkg/listed.py": "__all__ = [\"kept\"]\ndef kept(): pass\ndef dropped(): pass\n",
		"pkg/grown.py": "__all__: list[str] = [\"a\"] + [\"b\"]\n__all__ += [\"c\"]\n__all__.extend((\"d\",))\n" +
			"__all__.append(\"e\")\ndef a(): pass\ndef b(): pass\ndef c(): pass\ndef d(): pass\ndef e(): pass\ndef f(): pass\n",
	}
	for name, src := range files {
		os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755)
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range s.Modules {
		for _, f := range m.Functions {
			got[f.Name] = f.Exported
		}
	}
	want := map[string]bool{"run": true, "_helper": false, "Shop.__init__": true, "Shop.add": true,
		"Shop._check": false, "Shop.__mangled": false, "_Hidden.visible": false, "kept": true, "dropped": false,
		"a": true, "b": true, "c": true, "d": true, "e": true, "f": false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exported = %v", got)
	}
}

func TestLangIsRecognisedByPyproject(t *testing.T) {
	if Lang.Name != "python" || Lang.Markers[0] != "pyproject.toml" || Lang.Markers[1] != "setup.py" {
		t.Fatalf("Lang = %+v", Lang)
	}
}
