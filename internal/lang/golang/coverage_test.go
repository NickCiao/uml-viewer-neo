package golang

import (
	"os"
	"testing"

	"uml-viewer-neo/internal/metrics"
)

func TestReadProfileMergesBlocksAcrossTestBinaries(t *testing.T) {
	data, _ := os.ReadFile("testdata/cover.out")
	cov, err := ReadProfile(data, "github.com/acme/shop")
	if err != nil {
		t.Fatal(err)
	}
	if len(cov.Files) != 2 || !cov.Files["shop.go"] || !cov.Files["internal/cart/cart.go"] {
		t.Fatalf("files = %v", cov.Files)
	}
	if len(cov.Units) != 8 {
		t.Fatalf("%d units, want 8 after merging", len(cov.Units))
	}
	for _, u := range cov.Units {
		if !u.Hit || u.Col != metrics.NoCol {
			t.Fatalf("unit %+v: every block ran in at least one binary", u)
		}
	}
	if u := cov.Units[0]; u.File != "shop.go" || u.Line != 11 || u.Weight != 2 {
		t.Fatalf("first unit = %+v", u)
	}
}

func TestScoresTheSampleRepo(t *testing.T) {
	scan, _ := Scan("testdata/shop")
	data, _ := os.ReadFile("testdata/cover.out")
	cov, _ := Lang.Read(data, "testdata/shop", scan)
	scores := metrics.Score(scan, &cov)
	run := scores["github.com/acme/shop"].Functions["Run"]
	if run.Coverage != 1 || run.CRAP != 3 {
		t.Fatalf("Run = %+v", run)
	}
	if c := Lang.Coverage("testdata/shop"); c.Report != ".umlv/raw/cover.out" || c.Args[0] != "go" {
		t.Fatalf("command = %+v", c)
	}
}
