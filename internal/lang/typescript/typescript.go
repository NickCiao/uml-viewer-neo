// Package typescript scans TypeScript and JavaScript projects with
// TypeScript's own parser and resolver. umlv carries a pinned copy of the
// compiler and pipes it into node ahead of tsscan.js, so no project needs
// node_modules and nothing is downloaded.
package typescript

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
)

//go:embed tsscan.js
var scanner []byte

//go:embed typescript.js.gz
var compilerGz []byte

// The compiler expects to be loaded by require(), which hands it a
// module.exports to fill. The wrapper gives it a private one and keeps the
// result as `ts`.
const (
	wrapperStart = "const ts = (function () { const module = { exports: {} }; const exports = module.exports;\n"
	wrapperEnd   = "\nreturn module.exports; })();\n"
)

// Lang is TypeScript and JavaScript: recognised by tsconfig.json or package.json.
var Lang = lang.Language{
	Name: "typescript", Markers: []string{"tsconfig.json", "package.json"}, Scan: Scan,
	Coverage: coverageCommand, Read: readCoverage,
	CoverageHint: "vitest needs @vitest/coverage-v8 installed (npm install -D @vitest/coverage-v8); jest has coverage built in.",
}

// Scan reports the modules, imports and functions of the project at root.
func Scan(root string) (facts.Scan, error) {
	compiler, err := gzip.NewReader(bytes.NewReader(compilerGz))
	if err != nil {
		return facts.Scan{}, fmt.Errorf("embedded TypeScript is unreadable: %w", err)
	}
	defer compiler.Close()
	return lang.RunScript("node", program(compiler, scanner), root)
}

func program(compiler io.Reader, script []byte) io.Reader {
	return io.MultiReader(
		strings.NewReader(wrapperStart), compiler,
		strings.NewReader(wrapperEnd), bytes.NewReader(script))
}
