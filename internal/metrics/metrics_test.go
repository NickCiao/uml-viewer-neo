package metrics

import (
	"math"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestCRAP(t *testing.T) {
	if CRAP(3, 1) != 3 || CRAP(3, 0) != 12 || !near(CRAP(16, 3.0/7), 63.767) {
		t.Fatalf("CRAP: %v %v %v", CRAP(3, 1), CRAP(3, 0), CRAP(16, 3.0/7))
	}
}

func TestCoverageCountsOnlyTheBody(t *testing.T) {
	// export function total(…): number {  ← the declaration runs at import
	total := facts.Function{File: "src/pricing.ts", Start: 3, End: 5, CoverStart: 3, CoverCol: 46, CC: 1}
	units := []Unit{
		{File: "src/pricing.ts", Line: 3, Col: 0, Weight: 1, Hit: true},   // the declaration: not the body
		{File: "src/pricing.ts", Line: 3, Col: 46, Weight: 1, Hit: false}, // the body's first statement
		{File: "src/pricing.ts", Line: 4, Col: 2, Weight: 1, Hit: true},
		{File: "src/pricing.ts", Line: 6, Col: 0, Weight: 1, Hit: false}, // after the function
		{File: "src/other.ts", Line: 4, Col: 2, Weight: 1, Hit: false},   // another file
	}
	if got := FunctionCoverage(total, units); got != 0.5 {
		t.Fatalf("coverage = %v", got)
	}
}

func TestCoverageExcludesUnitsBelowCoverStart(t *testing.T) {
	// def run(…):  ← line 2, the def line runs at import
	//   pass      ← line 3, the body
	run := facts.Function{File: "a.py", Start: 2, End: 4, CoverStart: 3, CC: 1}
	units := []Unit{
		{File: "a.py", Line: 1, Col: NoCol, Weight: 1, Hit: true},  // earlier function
		{File: "a.py", Line: 2, Col: NoCol, Weight: 1, Hit: true},  // def line: not the body
		{File: "a.py", Line: 3, Col: NoCol, Weight: 1, Hit: false}, // body
	}
	if got := FunctionCoverage(run, units); got != 0 {
		t.Fatalf("coverage = %v, want 0 (only body counts, which is missed)", got)
	}
}

func TestCoverageWeighsUnitsAndAcceptsUnitsWithoutColumns(t *testing.T) {
	run := facts.Function{File: "shop.go", Start: 10, End: 18, CC: 3}
	units := []Unit{
		{File: "shop.go", Line: 11, Col: NoCol, Weight: 2, Hit: true},
		{File: "shop.go", Line: 13, Col: NoCol, Weight: 1, Hit: false},
	}
	if got := FunctionCoverage(run, units); !near(got, 2.0/3) {
		t.Fatalf("coverage = %v", got)
	}
}

func TestAFunctionWithNothingToCoverCountsAsCovered(t *testing.T) {
	if got := FunctionCoverage(facts.Function{File: "a.go", Start: 1, End: 1, CC: 1}, nil); got != 1 {
		t.Fatalf("coverage = %v", got)
	}
}

func TestStatsAndGrades(t *testing.T) {
	cases := []struct {
		crap  []float64
		mu, s float64
		grade facts.Grade
	}{
		{[]float64{8}, 8, 0, facts.Green},
		{[]float64{12, 2}, 7, 5, facts.Amber}, // μ + σ = 12 exactly is still amber
		{[]float64{30, 2}, 16, 14, facts.Red},
	}
	for _, c := range cases {
		st := StatsOf(c.crap)
		if !near(st.Mu, c.mu) || !near(st.Sigma, c.s) || GradeOf(st) != c.grade {
			t.Errorf("%v: stats %+v grade %s", c.crap, st, GradeOf(st))
		}
	}
	if StatsOf(nil) != nil || GradeOf(nil) != facts.Unlit {
		t.Fatal("no scores should mean no stats and an unlit lamp")
	}
}

func TestScoreLeavesFilesOutsideTheReportUnmeasured(t *testing.T) {
	scan := facts.Scan{Modules: []facts.Module{
		{ID: "a", Functions: []facts.Function{{Name: "A", File: "a.go", Start: 1, End: 3, CC: 2}}},
		{ID: "b", Functions: []facts.Function{{Name: "B", File: "b.go", Start: 1, End: 3, CC: 2}}},
		{ID: "c", Functions: []facts.Function{{Name: "C", File: "c.go", Start: 1, End: 3, CC: 5}}},
	}}
	cov := &Coverage{
		Files: map[string]bool{"a.go": true, "c.go": true},
		Units: []Unit{{File: "a.go", Line: 2, Col: NoCol, Weight: 1, Hit: true}},
	}
	scores := Score(scan, cov)
	if _, measured := scores["b"]; measured {
		t.Fatal("b.go is not in the report, so b must be unmeasured")
	}
	if a := scores["a"]; a.Grade != facts.Green || a.Functions["A"].Coverage != 1 {
		t.Fatalf("a = %+v", a)
	}
	// c.go is in the report but has no units: nothing to cover counts as covered, so CRAP = CC.
	if c := scores["c"]; c.Functions["C"].CRAP != 5 {
		t.Fatalf("c = %+v", c)
	}
	if len(Score(scan, nil)) != 0 {
		t.Fatal("no coverage means no scores")
	}
}

func TestBandsDescribeTheCutoffs(t *testing.T) {
	if !strings.Contains(Bands["green"], "8") || !strings.Contains(Bands["amber"], "12") || !strings.Contains(Bands["red"], "12") {
		t.Fatalf("bands = %v", Bands)
	}
	if Bands["amber"] != "over 8, up to 12" {
		t.Fatalf("Bands[amber] = %q, want %q", Bands["amber"], "over 8, up to 12")
	}
}
