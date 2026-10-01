package python

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/metrics"
)

// ReadCoverageJSON reads coverage.py's JSON report, which pytest-cov also
// writes: one unit per line, without columns. Paths are relative to where
// the tests ran, the repo root.
func ReadCoverageJSON(data []byte) (metrics.Coverage, error) {
	var report struct {
		Files map[string]struct {
			Executed []int `json:"executed_lines"`
			Missing  []int `json:"missing_lines"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return metrics.Coverage{}, fmt.Errorf("coverage.py report: %w", err)
	}
	names := make([]string, 0, len(report.Files))
	for name := range report.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	cov := metrics.Coverage{Files: map[string]bool{}}
	for _, name := range names {
		file, lines := filepath.ToSlash(name), report.Files[name]
		cov.Files[file] = true
		for _, l := range lines.Executed {
			cov.Units = append(cov.Units, metrics.Unit{File: file, Line: l, Col: metrics.NoCol, Weight: 1, Hit: true})
		}
		for _, l := range lines.Missing {
			cov.Units = append(cov.Units, metrics.Unit{File: file, Line: l, Col: metrics.NoCol, Weight: 1})
		}
	}
	return cov, nil
}

func coverageCommand(string) metrics.Command {
	return metrics.Command{
		Args:   []string{"python3", "-m", "pytest", "-q", "--cov", "--cov-report", "json:{report}"},
		Env:    []string{"COVERAGE_FILE=.umlv/raw/.coverage"},
		Report: ".umlv/raw/coverage.json",
	}
}

func readCoverage(report []byte, _ string, _ facts.Scan) (metrics.Coverage, error) {
	return ReadCoverageJSON(report)
}
