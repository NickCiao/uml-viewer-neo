package typescript

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
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

func TestScanMarksWhatOtherModulesCanCall(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.ts": `export function pub() {}
function priv() {}
export const arrow = () => 1;
const inner = () => 2;
function named() {}
const alsoNamed = () => 3;
export { named, alsoNamed as renamed };
export class Shop {
  constructor() {}
  add() {}
  private check() {}
  protected guard() {}
  #secret() {}
  static make() { return new Shop(); }
  static {}
  [key: string]: unknown;
}
class Hidden { open() {} }
export const api = { get() {}, ...{} };
const local = { put() {} };
export default function () {}
`,
		"b.js": "function one() {}\nfunction two() {}\nconst three = () => 3;\nfunction four() {}\n" +
			"module.exports = { one, alias: two };\nexports.three = three;\n",
		"c.ts": "function main() {}\nexport default main;\n",
		"d.js": "module.exports = class Exported { m() {} static s() {} };\nmodule.exports.Helper = class { h() {} };\n",
		"e.ts": "export const Foo = class { m() {} private p() {} };\nconst Bar = class { m() {} };\n" +
			"export default (class Named { n() {} });\nregister(class Arg { a() {} });\n",
		"n.ts": `export namespace Open { export function a() {} function b() {} }
namespace Closed { export function c() {} export class K { k() {} } }
export namespace Outer.Inner { export function d() {} }
namespace Hidden.Inner { export function e() {} }
export namespace Shell { export namespace Kept { export function g() {} } namespace Dropped { export function h() {} } }
namespace Named { export function i() {} }
export { Named };
declare module "ambient" { export function j(): void; }
`,
	}
	for name, src := range files {
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
			got[m.ID+" "+f.Name] = f.Exported
		}
	}
	want := map[string]bool{
		"a.ts pub": true, "a.ts priv": false, "a.ts arrow": true, "a.ts inner": false,
		"a.ts named": true, "a.ts alsoNamed": true, "a.ts Shop.constructor": true, "a.ts Shop.add": true,
		"a.ts Shop.check": false, "a.ts Shop.guard": false, "a.ts Shop.#secret": false, "a.ts Shop.make": true,
		"a.ts Hidden.open": false, "a.ts api.get": true, "a.ts local.put": false, "a.ts default": true,
		"b.js one": true, "b.js two": true, "b.js three": true, "b.js four": false,
		"c.ts main":       true,
		"d.js Exported.m": true, "d.js Exported.s": true, "d.js default.h": true,
		"e.ts Foo.m": true, "e.ts Foo.p": false, "e.ts Bar.m": false, "e.ts Named.n": true, "e.ts Arg.a": false,
		"n.ts Open.a": true, "n.ts Open.b": false, "n.ts Closed.c": false, "n.ts Closed.K.k": false,
		"n.ts Outer.Inner.d": true, "n.ts Hidden.Inner.e": false, "n.ts Shell.Kept.g": true,
		"n.ts Shell.Dropped.h": false, "n.ts Named.i": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exported = %v", got)
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
