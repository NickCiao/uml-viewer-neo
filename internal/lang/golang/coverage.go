package golang

import (
	"regexp"
	"strconv"
	"strings"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/metrics"
)

var block = regexp.MustCompile(`^(.+):((\d+)\.\d+,\d+\.\d+) (\d+) (\d+)$`)

// ReadProfile reads a `go test -coverprofile` file. Paths are import paths;
// prefix (the module path) makes them repo-relative. With -coverpkg each
// block appears once per test binary, so blocks merge: hit if any run hit it.
func ReadProfile(data []byte, prefix string) (metrics.Coverage, error) {
	cov := metrics.Coverage{Files: map[string]bool{}}
	seen := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		m := block.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !strings.HasPrefix(m[1], prefix+"/") {
			continue
		}
		file := strings.TrimPrefix(m[1], prefix+"/")
		hit := m[5] != "0"
		cov.Files[file] = true
		if i, ok := seen[file+":"+m[2]]; ok {
			cov.Units[i].Hit = cov.Units[i].Hit || hit
			continue
		}
		start, _ := strconv.Atoi(m[3])
		stmts, _ := strconv.Atoi(m[4])
		seen[file+":"+m[2]] = len(cov.Units)
		cov.Units = append(cov.Units, metrics.Unit{File: file, Line: start, Col: metrics.NoCol, Weight: stmts, Hit: hit})
	}
	return cov, nil
}

func coverageCommand(string) metrics.Command {
	return metrics.Command{
		Args:   []string{"go", "test", "./...", "-coverpkg=./...", "-coverprofile", "{report}"},
		Report: ".umlv/raw/cover.out",
	}
}

func readCoverage(report []byte, _ string, scan facts.Scan) (metrics.Coverage, error) {
	return ReadProfile(report, scan.Prefix)
}
