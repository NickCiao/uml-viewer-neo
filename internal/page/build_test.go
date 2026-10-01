package page

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"uml-viewer-neo/internal/facts"
)

var update = flag.Bool("update", false, "rewrite web/testdata/shop.page.json")

const golden = "../../web/testdata/shop.page.json"

func project(id string) facts.Import { return facts.Import{To: id, Project: true} }

func shopScan() facts.Scan {
	return facts.Scan{Lang: "typescript", Notes: []string{"Left out 1 file."}, Modules: []facts.Module{
		{ID: "src/index.ts", Path: "", Name: "shop", File: "src/index.ts", Imports: []facts.Import{project("src/checkout.ts")}},
		{ID: "src/checkout.ts", Path: "checkout", Name: "checkout", File: "src/checkout.ts",
			Imports: []facts.Import{project("src/cart/basket.ts"), project("src/pricing.ts"),
				{To: "zod", Module: "zod"}, {To: "node:fs", Std: true, Module: "node:fs"}, {To: "left-pad", Module: "left-pad"}},
			Functions: []facts.Function{
				{Name: "run", File: "src/checkout.ts", Start: 9, End: 17, CC: 5},
				{Name: "neverCalled", File: "src/checkout.ts", Start: 19, End: 19, CC: 1}}},
		{ID: "src/pricing.ts", Path: "pricing", Name: "pricing", File: "src/pricing.ts",
			Imports:   []facts.Import{project("src/cart/basket.ts")},
			Functions: []facts.Function{{Name: "total", File: "src/pricing.ts", Start: 3, End: 5, CC: 1}}},
		{ID: "src/cart/index.ts", Path: "cart", Name: "cart", File: "src/cart/index.ts",
			Imports: []facts.Import{project("src/cart/basket.ts")}},
		{ID: "src/cart/basket.ts", Path: "cart/basket", Name: "basket", File: "src/cart/basket.ts",
			Functions: []facts.Function{
				{Name: "Basket.add", File: "src/cart/basket.ts", Start: 4, End: 6, CC: 3},
				{Name: "Basket.full", File: "src/cart/basket.ts", Start: 8, End: 8, CC: 9}}},
	}}
}

func shopScores() facts.Scores {
	return facts.Scores{
		"src/checkout.ts": {Grade: facts.Red, Stats: &facts.Stats{Mu: 16, Sigma: 14, Max: 30},
			Functions: map[string]facts.FunctionScore{"run": {Coverage: 0, CRAP: 30}, "neverCalled": {Coverage: 0, CRAP: 2}}},
		"src/pricing.ts": {Grade: facts.Green, Stats: &facts.Stats{Mu: 1, Sigma: 0, Max: 1},
			Functions: map[string]facts.FunctionScore{"total": {Coverage: 1, CRAP: 1}}},
		"src/cart/basket.ts": {Grade: facts.Amber, Stats: &facts.Stats{Mu: 6.5625, Sigma: 2.4375, Max: 9},
			Functions: map[string]facts.FunctionScore{"Basket.add": {Coverage: 0.5, CRAP: 4.125}, "Basket.full": {Coverage: 1, CRAP: 9}}},
	}
}

func shopOptions() Options {
	return Options{Repo: "shop", ScannedAt: "2026-09-30T20:00:00Z", CoverageAt: "2026-09-30T19:59:00Z",
		EditorPrefix: "vscode://file/repo/", Libraries: []string{"zod"},
		Bands: map[string]string{"green": "8 or less", "amber": "over 8, up to 12", "red": "over 12"}}
}

func find(p facts.Page, id string) facts.PageModule {
	for _, m := range p.Modules {
		if m.ID == id {
			return m
		}
	}
	return facts.PageModule{}
}

func TestBuildResolvesUsesToModulesAndListedLibrariesOnly(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	if got := find(p, "src/checkout.ts").Uses; !reflect.DeepEqual(got, []string{"lib:zod", "src/cart/basket.ts", "src/pricing.ts"}) {
		t.Fatalf("uses = %v", got)
	}
	if !reflect.DeepEqual(p.Libraries, []facts.Library{{ID: "lib:zod", Name: "zod"}}) {
		t.Fatalf("libraries = %v", p.Libraries)
	}
}

func TestBuildPlacesModulesInTheFolderTree(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	if !sort.SliceIsSorted(p.Modules, func(i, j int) bool { return p.Modules[i].ID < p.Modules[j].ID }) {
		t.Fatal("modules are not sorted by ID")
	}
	if got := find(p, "src/index.ts").Tree; !reflect.DeepEqual(got, []string{"shop"}) {
		t.Fatalf("root module tree = %v", got)
	}
	if got := find(p, "src/cart/basket.ts").Tree; !reflect.DeepEqual(got, []string{"cart", "basket"}) {
		t.Fatalf("basket tree = %v", got)
	}
}

func TestBuildCarriesScoresAndLeavesUnmeasuredModulesUnlit(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	checkout := find(p, "src/checkout.ts")
	if checkout.Grade != facts.Red || checkout.Stats.Mu != 16 {
		t.Fatalf("checkout = %+v", checkout)
	}
	run := checkout.Functions[0]
	if run.Name != "run" || run.Line != 9 || run.CRAP == nil || *run.CRAP != 30 || run.Coverage == nil || *run.Coverage != 0 {
		t.Fatalf("run = %+v", run)
	}
	cart := find(p, "src/cart/index.ts")
	if cart.Grade != facts.Unlit || cart.Stats != nil {
		t.Fatalf("cart = %+v", cart)
	}
}

func TestLibraryCoversSubPackages(t *testing.T) {
	cases := []struct {
		imp  facts.Import
		libs []string
		want string
	}{
		{facts.Import{To: "ai/test", Module: "ai/test"}, []string{"ai"}, "ai"},
		{facts.Import{To: "aim", Module: "aim"}, []string{"ai"}, ""},
		{facts.Import{To: "google.cloud.storage", Module: "google.cloud.storage"}, []string{"google"}, "google"},
		{facts.Import{To: "github.com/spf13/pflag"}, []string{"github.com/spf13/pflag"}, "github.com/spf13/pflag"},
	}
	for _, c := range cases {
		if got, _ := library(c.imp, c.libs); got != c.want {
			t.Errorf("library(%v, %v) = %q, want %q", c.imp, c.libs, got, c.want)
		}
	}
}

func TestBuildMatchesTheGoldenFile(t *testing.T) {
	got, err := json.MarshalIndent(Build(shopScan(), shopScores(), shopOptions()), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/page -run Golden -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("page data changed. If intended: go test ./internal/page -run Golden -update, then node --test web/")
	}
}
