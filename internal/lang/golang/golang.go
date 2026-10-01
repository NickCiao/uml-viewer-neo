package golang

import "uml-viewer-neo/internal/lang"

// Lang is Go: recognised by go.mod, scanned in-process.
var Lang = lang.Language{
	Name: "go", Markers: []string{"go.mod"}, Scan: Scan,
	Coverage: coverageCommand, Read: readCoverage,
	CoverageHint: "Go needs nothing extra; check that `go test ./...` passes.",
}
