package python

import (
	"os"
	"testing"

	"uml-viewer-neo/internal/metrics"
)

func TestReadCoverageJSON(t *testing.T) {
	data, _ := os.ReadFile("testdata/coverage.json")
	cov, err := ReadCoverageJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cov.Files) != 2 || len(cov.Units) != 21 {
		t.Fatalf("files = %v, %d units", cov.Files, len(cov.Units))
	}
	if _, err := ReadCoverageJSON([]byte("not json")); err == nil {
		t.Fatal("a broken report must be an error")
	}
}

func TestScoresTheSampleRepo(t *testing.T) {
	scan, _ := Scan("testdata/shop")
	data, _ := os.ReadFile("testdata/coverage.json")
	cov, _ := Lang.Read(data, "testdata/shop", scan)
	checkout := metrics.Score(scan, &cov)["src/shop/checkout.py"]
	if run := checkout.Functions["run"]; run.Coverage != 1 || run.CRAP != 3 {
		t.Fatalf("run = %+v", run)
	}
	// never_called's def line ran at import; counting it would make this 0.5.
	if never := checkout.Functions["never_called"]; never.Coverage != 0 || never.CRAP != 2 {
		t.Fatalf("never_called = %+v", never)
	}
	if _, measured := metrics.Score(scan, &cov)["src/shop/cart/basket.py"]; measured {
		t.Fatal("basket.py is not in the report")
	}
}
