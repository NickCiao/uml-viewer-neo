package typescript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/metrics"
)

const reportDir = ".umlv/raw/ts-coverage"

// ReadIstanbul reads istanbul's coverage-final.json, which vitest and jest
// both write: one unit per statement, with its column, so a one-line arrow
// function's body can be told from its declaration. Paths are absolute;
// files outside root are ignored.
func ReadIstanbul(data []byte, root string) (metrics.Coverage, error) {
	var report map[string]struct {
		StatementMap map[string]struct {
			Start struct{ Line, Column int } `json:"start"`
		} `json:"statementMap"`
		S map[string]int `json:"s"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return metrics.Coverage{}, fmt.Errorf("istanbul report: %w", err)
	}
	cov := metrics.Coverage{Files: map[string]bool{}}
	for abs, file := range report {
		rel, ok := relative(root, abs)
		if !ok {
			continue
		}
		cov.Files[rel] = true
		for id, st := range file.StatementMap {
			cov.Units = append(cov.Units, metrics.Unit{File: rel, Line: st.Start.Line, Col: st.Start.Column, Weight: 1, Hit: file.S[id] > 0})
		}
	}
	sort.Slice(cov.Units, func(i, j int) bool {
		a, b := cov.Units[i], cov.Units[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return cov, nil
}

// relative is file relative to root with forward slashes, or false when it
// lies outside. Symlinks are resolved first: on macOS /tmp is /private/tmp.
func relative(root, file string) (string, bool) {
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if f, err := filepath.EvalSymlinks(file); err == nil {
		file = f
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func coverageCommand(root string) metrics.Command {
	c := metrics.Command{Report: reportDir + "/coverage-final.json"}
	switch testRunner(root) {
	case "vitest":
		c.Args = []string{"npx", "vitest", "run", "--coverage.enabled", "--coverage.reporter=json", "--coverage.reportsDirectory=" + reportDir}
	case "jest":
		c.Args = []string{"npx", "jest", "--coverage", "--coverageReporters=json", "--coverageDirectory=" + reportDir}
	}
	return c
}

// testRunner is "vitest" or "jest" when package.json depends on it.
func testRunner(root string) string {
	var pkg struct{ Dependencies, DevDependencies map[string]string }
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil || json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	for _, name := range []string{"vitest", "jest"} {
		if pkg.Dependencies[name] != "" || pkg.DevDependencies[name] != "" {
			return name
		}
	}
	return ""
}

func readCoverage(report []byte, root string, _ facts.Scan) (metrics.Coverage, error) {
	return ReadIstanbul(report, root)
}
