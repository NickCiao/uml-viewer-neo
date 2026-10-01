package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func scanShop(t *testing.T) map[string]facts.Module {
	t.Helper()
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "go" || s.Prefix != "github.com/acme/shop" {
		t.Fatalf("scan = %+v", s)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	return byID
}

func TestScanFindsPackagesAndTheirImports(t *testing.T) {
	byID := scanShop(t)
	root, cart := byID["github.com/acme/shop"], byID["github.com/acme/shop/internal/cart"]
	if root.Path != "" || root.Name != "shop" || cart.Path != "internal/cart" || cart.Dir != "internal/cart" {
		t.Fatalf("root = %+v, cart = %+v", root, cart)
	}
	var sawCart, sawFmt bool
	for _, imp := range root.Imports {
		sawCart = sawCart || (imp.To == "github.com/acme/shop/internal/cart" && imp.Project)
		sawFmt = sawFmt || (imp.To == "fmt" && imp.Std)
	}
	if !sawCart || !sawFmt {
		t.Fatalf("imports = %+v", root.Imports)
	}
}

func TestScanFindsFunctionsWithLinesAndComplexity(t *testing.T) {
	byID := scanShop(t)
	run := byID["github.com/acme/shop"].Functions[0]
	if run.Name != "Run" || run.File != "shop.go" || run.Start != 10 || run.End != 18 || run.CC != 3 {
		t.Fatalf("Run = %+v", run)
	}
	var names []string
	for _, f := range byID["github.com/acme/shop/internal/cart"].Functions {
		names = append(names, f.Name)
	}
	if got := len(names); got != 4 || names[0] != "New" || names[1] != "Cart.Add" || names[3] != "Cart.Full" {
		t.Fatalf("cart functions = %v", names)
	}
}

func TestScanLeavesOutTests(t *testing.T) {
	for _, m := range scanShop(t) {
		for _, f := range m.Functions {
			if len(f.Name) > 4 && f.Name[:4] == "Test" {
				t.Fatalf("test function %s was scanned", f.Name)
			}
		}
	}
}

func TestImportsResolveLibrariesToTheirModule(t *testing.T) {
	got := imports([]string{"github.com/spf13/pflag/sub", "fmt", "github.com/acme/shop/x", "C", "golang.org/x/tools/go/ast"},
		"github.com/acme/shop", []string{"github.com/spf13/pflag", "golang.org/x/tools"})
	want := []facts.Import{
		{To: "github.com/spf13/pflag/sub", Module: "github.com/spf13/pflag"},
		{To: "fmt", Std: true, Module: "fmt"},
		{To: "github.com/acme/shop/x", Project: true},
		{To: "golang.org/x/tools/go/ast", Module: "golang.org/x/tools"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestReadRequiresReadsBlocksAndSingleLines(t *testing.T) {
	gomod := filepath.Join(t.TempDir(), "go.mod")
	text := "module x\n\nrequire github.com/a/b v1.0.0 // indirect\n\nrequire (\n\tgolang.org/x/tools v0.1.0\n\t// a comment\n\tgithub.com/spf13/pflag v1.0.5\n)\n"
	if err := os.WriteFile(gomod, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{"github.com/spf13/pflag", "golang.org/x/tools", "github.com/a/b"} // longest first
	if got := readRequires(gomod); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if readRequires(filepath.Join(t.TempDir(), "missing")) != nil {
		t.Fatal("a missing go.mod has no requires")
	}
}

func TestMethodNamesUseTheReceiverTypeEvenWhenGeneric(t *testing.T) {
	src := "package p\ntype S[T any] struct{}\nfunc (s *S[T]) A() {}\ntype M[K, V any] struct{}\nfunc (m M[K, V]) B() {}\nfunc C() {}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			got = append(got, funcName(fd))
		}
	}
	if !reflect.DeepEqual(got, []string{"S.A", "M.B", "C"}) {
		t.Fatalf("got %v", got)
	}
}

func TestLangIsRecognisedByGoMod(t *testing.T) {
	if Lang.Name != "go" || Lang.Markers[0] != "go.mod" || Lang.Scan == nil {
		t.Fatalf("Lang = %+v", Lang)
	}
}
