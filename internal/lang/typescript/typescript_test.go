package typescript

import (
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
)

func names(fs []facts.Function) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

func TestScanShopWithoutNodeModules(t *testing.T) {
	if _, err := os.Stat("testdata/shop/node_modules"); err == nil {
		t.Fatal("the sample repo must not have node_modules")
	}
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	if len(byID) != 5 || byID["src/index.ts"].Path != "" || byID["src/cart/index.ts"].Path != "cart" {
		t.Fatalf("modules = %+v", s.Modules)
	}
	checkout := byID["src/checkout.ts"]
	var sawAlias, sawStd, sawLib bool
	for _, imp := range checkout.Imports {
		sawAlias = sawAlias || (imp.To == "src/pricing.ts" && imp.Project) // via the @/ path alias
		sawStd = sawStd || (imp.To == "node:fs" && imp.Std)
		sawLib = sawLib || (imp.Module == "zod" && !imp.Std && !imp.Project)
	}
	if !sawAlias || !sawStd || !sawLib {
		t.Fatalf("imports = %+v", checkout.Imports)
	}
	if got := names(byID["src/pricing.ts"].Functions); len(got) != 3 || got[1] != "discounts.half" || got[2] != "discounts.none" {
		t.Fatalf("pricing functions = %v", got)
	}
	if got := names(byID["src/index.ts"].Functions); len(got) != 1 || got[0] != "default.fetch" {
		t.Fatalf("index functions = %v", got)
	}
	total := byID["src/pricing.ts"].Functions[0]
	if total.Start != 3 || total.CoverStart != 3 || total.CoverCol != 46 {
		t.Fatalf("total = %+v", total)
	}
}

func TestTheEmbeddedCompilerIsPinned(t *testing.T) {
	gz, err := gzip.NewReader(bytes.NewReader(compilerGz))
	if err != nil {
		t.Fatal(err)
	}
	probe := []byte(`process.stdout.write(JSON.stringify({lang: ts.version, prefix: "", modules: []}))`)
	s, err := lang.RunScript("node", program(gz, probe), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "5.9.3" {
		t.Fatalf("TypeScript %s", s.Lang)
	}
}
