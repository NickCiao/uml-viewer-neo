package typescript

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"uml-viewer-neo/internal/metrics"
)

func TestReadIstanbulKeepsColumnsAndDropsFilesOutsideTheRepo(t *testing.T) {
	data, _ := os.ReadFile("testdata/coverage-final.json")
	cov, err := ReadIstanbul(data, "/ROOT")
	if err != nil {
		t.Fatal(err)
	}
	if len(cov.Files) != 1 || !cov.Files["src/pricing.ts"] || len(cov.Units) != 4 {
		t.Fatalf("files = %v, %d units", cov.Files, len(cov.Units))
	}
	if u := cov.Units[3]; u.Line != 11 || u.Col != 35 || !u.Hit {
		t.Fatalf("last unit = %+v", u)
	}
}

func TestScoresObjectLiteralMethods(t *testing.T) {
	scan, _ := Scan("testdata/shop")
	data, _ := os.ReadFile("testdata/coverage-final.json")
	cov, _ := ReadIstanbul(data, "/ROOT")
	fns := metrics.Score(scan, &cov)["src/pricing.ts"].Functions
	if fns["total"].Coverage != 1 || fns["discounts.half"].Coverage != 0 || fns["discounts.none"].Coverage != 1 {
		t.Fatalf("functions = %+v", fns)
	}
}

func TestTheCoverageCommandFollowsTheTestRunner(t *testing.T) {
	for runner, want := range map[string]string{"vitest": "vitest", "jest": "jest", "": ""} {
		root := t.TempDir()
		deps := `{}`
		if runner != "" {
			deps = `{"devDependencies": {"` + runner + `": "1"}}`
		}
		os.WriteFile(filepath.Join(root, "package.json"), []byte(deps), 0o644)
		c := Lang.Coverage(root)
		if (want == "" && c.Args != nil) || (want != "" && !slices.Contains(c.Args, want)) {
			t.Errorf("%q: args = %v", runner, c.Args)
		}
		if c.Report != ".umlv/raw/ts-coverage/coverage-final.json" {
			t.Errorf("%q: report = %q", runner, c.Report)
		}
	}
}
