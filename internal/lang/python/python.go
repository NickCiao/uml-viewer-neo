// Package python scans Python projects with Python's own ast module, by
// piping the embedded pyscan.py into python3.
package python

import (
	"bytes"
	_ "embed"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
)

//go:embed pyscan.py
var script []byte

// Lang is Python: recognised by pyproject.toml or setup.py.
var Lang = lang.Language{Name: "python", Markers: []string{"pyproject.toml", "setup.py"}, Scan: Scan}

// Scan reports the modules, imports and functions of the Python project at root.
func Scan(root string) (facts.Scan, error) {
	return lang.RunScript("python3", bytes.NewReader(script), root)
}
