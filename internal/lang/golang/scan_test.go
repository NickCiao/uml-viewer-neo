package golang

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func parseFunc(t *testing.T, decls string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", "package p\n"+decls, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
}

func TestComplexityCountsEachDecisionPoint(t *testing.T) {
	for _, c := range []struct {
		body string
		want int
	}{
		{"", 1},
		{"if a {}", 2},
		{"if a && b {}", 3},
		{"if a || b {}", 3},
		{"if a != b {}", 2}, // only && and || are short-circuits
		{"for {}", 2},
		{"for range ch {}", 2},
		{"switch { case a: case b: default: }", 3}, // default is not a decision
		{"switch { default: }", 1},
		{"select { case <-ch: default: }", 2},
	} {
		fd := parseFunc(t, "func f(a, b bool, ch chan int) {"+c.body+"}")
		if got := complexity(fd); got != c.want {
			t.Errorf("%q: CC = %d, want %d", c.body, got, c.want)
		}
	}
}

func TestMethodNamesSeeThroughParenthesesAndNameNothingElse(t *testing.T) {
	fd := parseFunc(t, "type S struct{}\nfunc (s (*S)) A() {}")
	if got := funcName(fd); got != "S.A" {
		t.Fatalf("got %q", got)
	}
	if got := recvType(&ast.ArrayType{}); got != "?" {
		t.Fatalf("a receiver that is no type name is %q, want ?", got)
	}
}

func TestModulePathIsTheFirstPackagesModuleOrNothing(t *testing.T) {
	if got := modulePath(nil); got != "" {
		t.Fatalf("no packages: %q", got)
	}
	pkgs := []listed{
		{ImportPath: "a"},
		{ImportPath: "b", Module: &struct{ Path, Dir string }{}},
		{ImportPath: "c", Module: &struct{ Path, Dir string }{Path: "example.com/m"}},
		{ImportPath: "d", Module: &struct{ Path, Dir string }{Path: "example.com/other"}},
	}
	if got := modulePath(pkgs); got != "example.com/m" {
		t.Fatalf("got %q", got)
	}
}

func TestFunctionsSkipFilesThatDoNotParse(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{"bad.go": "package p\nfunc (", "good.go": "package p\nfunc Good() {}\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := functions(dir, listed{Dir: dir, GoFiles: []string{"bad.go", "good.go"}})
	if len(got) != 1 || got[0].Name != "Good" || got[0].File != "good.go" {
		t.Fatalf("got %+v", got)
	}
}

func TestScanNamesAMissingGoToolchain(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Scan(t.TempDir())
	var missing *facts.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "go" {
		t.Fatalf("err = %v", err)
	}
}

func TestScanReportsWhatGoListSaysWhenItFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\nbogus line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Scan(root)
	if err == nil || !strings.Contains(err.Error(), "go list") || !strings.Contains(err.Error(), "unknown directive") {
		t.Fatalf("err = %v", err)
	}
}

func TestScanRejectsGoListOutputItCannotRead(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\necho 'not json'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if _, err := Scan(t.TempDir()); err == nil {
		t.Fatal("garbage from go list must be an error, not an empty scan")
	}
}

func TestRequiredModuleNamesTheModuleOnOneGoModLine(t *testing.T) {
	for _, c := range []struct {
		line    string
		inBlock bool
		want    string
	}{
		{"github.com/a/b v1.0.0", true, "github.com/a/b"},
		{"", true, ""},
		{"require github.com/a/b v1.0.0", false, "github.com/a/b"},
		{"require (", false, ""},
		{"require", false, ""},
		{"module github.com/a/b", false, ""},
		{"github.com/a/b v1.0.0", false, ""}, // a bare line means nothing outside a block
	} {
		if got := requiredModule(c.line, c.inBlock); got != c.want {
			t.Errorf("requiredModule(%q, %v) = %q, want %q", c.line, c.inBlock, got, c.want)
		}
	}
}

func TestWithoutCommentTrimsAndDropsTheComment(t *testing.T) {
	for in, want := range map[string]string{
		"  a b // indirect": "a b",
		"// only a comment": "",
		"  plain  ":         "plain",
	} {
		if got := withoutComment(in); got != want {
			t.Errorf("withoutComment(%q) = %q, want %q", in, got, want)
		}
	}
}
