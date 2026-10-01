package facts

import (
	"encoding/json"
	"os"
	"testing"
)

func TestScanDecodesScannerOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ts-shop.json")
	if err != nil {
		t.Fatal(err)
	}
	var s Scan
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Lang != "typescript" || len(s.Modules) != 1 || len(s.Notes) != 1 {
		t.Fatalf("scan = %+v", s)
	}
	m := s.Modules[0]
	if m.ID != "src/pricing.ts" || m.Path != "pricing" || m.File != "src/pricing.ts" {
		t.Fatalf("module = %+v", m)
	}
	if len(m.Imports) != 3 || !m.Imports[0].Project || m.Imports[1].Module != "zod" || !m.Imports[2].Std {
		t.Fatalf("imports = %+v", m.Imports)
	}
	f := m.Functions[0]
	if f.Name != "total" || f.Start != 3 || f.End != 5 || f.CoverStart != 3 || f.CoverCol != 46 || f.CC != 1 {
		t.Fatalf("function = %+v", f)
	}
}

func TestSourceIsTheFileElseTheDirectory(t *testing.T) {
	if got := (Module{File: "src/a.ts"}).Source(); got != "src/a.ts" {
		t.Fatalf("got %q", got)
	}
	if got := (Module{Dir: "internal/cart"}).Source(); got != "internal/cart" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingToolErrorNamesTheTool(t *testing.T) {
	if got := (&MissingToolError{Tool: "node"}).Error(); got != "node is not installed or not on PATH" {
		t.Fatalf("got %q", got)
	}
}
