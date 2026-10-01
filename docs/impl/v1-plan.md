# umlv v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `umlv`, a Go CLI that scans a Go, TypeScript or Python repo and writes one self-contained HTML page (plus `data.json`) showing its modules, imports and CRAP grades, replacing the Clojure fork.

**Architecture:** Go gathers facts (scanners, coverage, CRAP, grades, policy) and writes page data; the page's plain JavaScript files build views (folding, arrows, badges, ELK.js layout, SVG). esbuild's Go API bundles the page code into the HTML each run. Each language lives in its own package under `internal/lang/`.

**Tech Stack:** Go 1.27; `github.com/BurntSushi/toml`; `github.com/evanw/esbuild`; plain ES-module JavaScript tested with `node --test`; ELK.js and TypeScript 5.9.3 vendored.

**Spec:** `docs/design/architecture.md`. Read it before starting any task; this plan argues from it.

## Global Constraints

- Module path `uml-viewer-neo`; `go` directive `1.27`.
- Go dependencies: only `github.com/BurntSushi/toml` and `github.com/evanw/esbuild`. Nothing else.
- JavaScript: no npm dependencies, no lockfile. `web/package.json` is exactly `{"private": true, "type": "module"}`. Tests run with `node --test web/`.
- Vendored: ELK.js as `web/vendor/elk.bundled.cjs` (with its EPL-2.0 licence and version recorded in `web/vendor/README.md`); TypeScript 5.9.3 as `internal/lang/typescript/typescript.js.gz` (with its Apache-2.0 licence).
- `umlv` writes only inside `<repo>/.umlv/`: `policy.toml`, `index.html`, `data.json`, `raw/`.
- Grades: μ + σ (population σ) of a module's measured CRAP scores; green ≤ 8, amber ≤ 12, red > 12, unlit when nothing measured. These numbers live only in `internal/metrics`.
- CRAP = `CC² × (1 − coverage)³ + CC`. CC = 1 + one per `if`, conditional expression, loop, `case`, `catch`/`except`, `&&`, `||`, `??`.
- Colours are exactly the hex values in the spec's Look table.
- Packages under `internal/` return errors and never print or exit; only `cmd/umlv` prints and sets exit codes.
- Every code change is test-first: write the failing test, watch it fail, make it pass.
- Before every commit: `go vet ./... && go test ./... && node --test web/` all pass.
- Commits carry no AI co-author line. Never add a git remote or push; the repo is private.

## Review Focus

1. **Names that contain markup** (`<`, `&`, quotes) in a repo, module or function name: the page shows them as text and nothing breaks out of the data `<script>` tag. Tests in Task 10 and Task 11.
2. **A repo path with a space** (`~/My Repo`): scanners receive it as one argument and source links still open. Tests in Task 2 and Task 6.
3. **Import cycles** (A imports B imports A): two arrows, no crash, layout still succeeds. Tests in Task 8 and Task 9.
4. **A crowded folder** (60 boxes, 150 arrows, the size of a real repo's top level): layout finishes within 3 seconds. Test in Task 9.
5. **A re-scan that removed the folder the URL points into**: the page opens at the top with a one-line notice instead of a blank canvas. Test in Task 17.

## Files

| Path | Responsibility |
|---|---|
| `cmd/umlv/main.go` | Flags, the language list, wiring, printing, exit codes |
| `internal/facts/facts.go` | Shared types: scan facts, scores, grades, page data, `MissingToolError` |
| `internal/lang/lang.go` | `Language`, `Detect`, `ByName`, `RunScript` |
| `internal/lang/golang/` | Go scanner (ported `goscan`), cover-profile reader, coverage command |
| `internal/lang/python/` | `pyscan.py` embedded, coverage.py JSON reader, coverage command |
| `internal/lang/typescript/` | `tsscan.js` and pinned TypeScript embedded, istanbul reader, coverage command |
| `internal/metrics/` | Coverage units, `Run`, CRAP, function coverage, stats, grades, `Score` |
| `internal/policy/policy.go` | Default, write, load and query `policy.toml` |
| `internal/page/build.go` | Scan + scores + options → `facts.Page` |
| `internal/page/render.go` | Bundle the page code and write the HTML |
| `web/embed.go` | Embeds the page files for `internal/page` |
| `web/model.js` | Page data + folder → boxes, arrows, badges, lamps, box geometry |
| `web/layout.js` | Boxes and arrows → positions and routes via ELK.js |
| `web/render.js` | Positions → diagram SVG; header and card HTML |
| `web/app.js` | Events, state, URL fragment, zoom and pan |
| `web/style.css`, `web/index.html` | The look; the page template |
| `web/testdata/shop.page.json` | Golden page data written by Go, read by the JS tests |
| `skills/umlv/SKILL.md` | The agent skill |

Each `internal/lang/<name>/testdata/` holds that language's sample repo and captured reports.

---

### Task 1: Module scaffold and shared types

**Files:**
- Create: `go.mod`, `.gitignore`, `internal/facts/facts.go`, `internal/facts/facts_test.go`, `internal/facts/testdata/ts-shop.json`

**Interfaces:**
- Produces: package `facts` with `Import`, `Function`, `Module` (+ `Source()`), `Scan`, `Grade` (`Green`, `Amber`, `Red`, `Unlit`), `Stats`, `FunctionScore`, `ModuleScore`, `Scores`, `Page`, `PageModule`, `PageFunction`, `Library`, `MissingToolError`.

- [ ] **Step 1: Create the module and ignore file**

```bash
cd ~/Documents/Repos/uml-viewer-neo
go mod init uml-viewer-neo
go mod edit -go=1.27
printf '.umlv/\n' > .gitignore
```

- [ ] **Step 2: Write the fixture** `internal/facts/testdata/ts-shop.json` (a trimmed copy of real `tsscan.js` output):

```json
{"lang":"typescript","prefix":"","notes":["Left out 1 file."],"modules":[
 {"ns":"src/pricing.ts","path":"pricing","name":"pricing","file":"src/pricing.ts",
  "imports":[{"to":"src/cart/basket.ts","project":true},
             {"to":"zod","project":false,"module":"zod"},
             {"to":"node:fs","project":false,"std":true,"module":"node:fs"}],
  "functions":[{"name":"total","file":"src/pricing.ts","start":3,"end":5,"cover_start":3,"cover_col":46,"cc":1}]}]}
```

- [ ] **Step 3: Write the failing test** `internal/facts/facts_test.go`

```go
package facts

import (
	"encoding/json"
	"os"
	"testing"
)

func TestScanDecodesScannerOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ts-shop.json")
	if err != nil {
		t.Fatal(err)
	}
	var s Scan
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Lang != "typescript" || len(s.Modules) != 1 || len(s.Notes) != 1 {
		t.Fatalf("scan = %+v", s)
	}
	m := s.Modules[0]
	if m.ID != "src/pricing.ts" || m.Path != "pricing" || m.File != "src/pricing.ts" {
		t.Fatalf("module = %+v", m)
	}
	if len(m.Imports) != 3 || !m.Imports[0].Project || m.Imports[1].Module != "zod" || !m.Imports[2].Std {
		t.Fatalf("imports = %+v", m.Imports)
	}
	f := m.Functions[0]
	if f.Name != "total" || f.Start != 3 || f.End != 5 || f.CoverStart != 3 || f.CoverCol != 46 || f.CC != 1 {
		t.Fatalf("function = %+v", f)
	}
}

func TestSourceIsTheFileElseTheDirectory(t *testing.T) {
	if got := (Module{File: "src/a.ts"}).Source(); got != "src/a.ts" {
		t.Fatalf("got %q", got)
	}
	if got := (Module{Dir: "internal/cart"}).Source(); got != "internal/cart" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingToolErrorNamesTheTool(t *testing.T) {
	if got := (&MissingToolError{Tool: "node"}).Error(); got != "node is not installed or not on PATH" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 4: Run it to see it fail**

Run: `go test ./internal/facts/`
Expected: FAIL to compile, `undefined: Scan`.

- [ ] **Step 5: Write** `internal/facts/facts.go`

```go
// Package facts is the vocabulary every other package shares: what a scanner
// reports, the scores computed from coverage, and the page data embedded in
// index.html and written to data.json.
package facts

import "fmt"

// Import is one import of a module, as a scanner reports it. A project import
// names another module's ID in To; anything else is a library or the standard
// library, named by its import path and, when the scanner knows it, its
// package name in Module.
type Import struct {
	To      string `json:"to"`
	Project bool   `json:"project"`
	Std     bool   `json:"std,omitempty"`
	Module  string `json:"module,omitempty"`
}

// Function is one function or method. CoverStart and CoverCol, when set, are
// where its body starts: coverage before that point belongs to the
// declaration, not the body.
type Function struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	CoverStart int    `json:"cover_start,omitempty"`
	CoverCol   int    `json:"cover_col,omitempty"`
	CC         int    `json:"cc"`
}

// Module is the unit a language imports. ID is what imports point at; Path
// is its place in the folder tree ("" for the repo's root module).
type Module struct {
	ID        string     `json:"ns"`
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	File      string     `json:"file,omitempty"`
	Dir       string     `json:"dir,omitempty"`
	Imports   []Import   `json:"imports"`
	Functions []Function `json:"functions"`
}

// Source is the repo-relative path a source link should open: the module's
// file, or for a Go package its directory.
func (m Module) Source() string {
	if m.File != "" {
		return m.File
	}
	return m.Dir
}

// Scan is everything one scanner reports about a repo.
type Scan struct {
	Lang    string   `json:"lang"`
	Prefix  string   `json:"prefix"`
	Modules []Module `json:"modules"`
	Notes   []string `json:"notes,omitempty"`
}

// Grade is a module's lamp.
type Grade string

const (
	Green Grade = "green"
	Amber Grade = "amber"
	Red   Grade = "red"
	Unlit Grade = "unlit"
)

// Stats summarises a module's measured CRAP scores.
type Stats struct {
	Mu    float64 `json:"mu"`
	Sigma float64 `json:"sigma"`
	Max   float64 `json:"max"`
}

// FunctionScore is one measured function.
type FunctionScore struct {
	Coverage float64
	CRAP     float64
}

// ModuleScore is one module with at least one measured function. Functions
// is keyed by function name; a function missing from it was not measured.
type ModuleScore struct {
	Grade     Grade
	Stats     *Stats
	Functions map[string]FunctionScore
}

// Scores is keyed by module ID. A module missing from it is unlit.
type Scores map[string]ModuleScore

// Page is the document embedded in index.html and written to data.json.
// model.js is its only reader in the page.
type Page struct {
	Repo         string            `json:"repo"`
	ScannedAt    string            `json:"scannedAt"`
	CoverageAt   string            `json:"coverageAt,omitempty"`
	EditorPrefix string            `json:"editorPrefix"`
	Bands        map[string]string `json:"bands,omitempty"`
	Modules      []PageModule      `json:"modules"`
	Libraries    []Library         `json:"libraries"`
	Notes        []string          `json:"notes,omitempty"`
}

// PageModule is a module as the page sees it. Tree is its folder path, ending
// with its own name; Uses holds module IDs and library IDs ("lib:<name>").
type PageModule struct {
	ID        string         `json:"id"`
	Tree      []string       `json:"tree"`
	Name      string         `json:"name"`
	Source    string         `json:"source"`
	Grade     Grade          `json:"grade"`
	Stats     *Stats         `json:"stats,omitempty"`
	Functions []PageFunction `json:"functions"`
	Uses      []string       `json:"uses"`
}

// PageFunction is a function as the page sees it; Coverage and CRAP are
// absent when it was not measured.
type PageFunction struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	CC       int      `json:"cc"`
	Coverage *float64 `json:"coverage,omitempty"`
	CRAP     *float64 `json:"crap,omitempty"`
}

// Library is an outside library drawn as an oval.
type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MissingToolError means a program umlv needs to run is not installed.
type MissingToolError struct{ Tool string }

func (e *MissingToolError) Error() string {
	return fmt.Sprintf("%s is not installed or not on PATH", e.Tool)
}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/facts/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod .gitignore internal/facts
git commit -m "Add the shared facts types."
```

---

### Task 2: Languages and the script runner

**Files:**
- Create: `internal/lang/lang.go`, `internal/lang/lang_test.go`

**Interfaces:**
- Consumes: `facts.Scan`, `facts.MissingToolError`.
- Produces: `type Language struct { Name string; Markers []string; Scan func(root string) (facts.Scan, error) }` (Task 15 adds three coverage fields); `var ErrNoLanguage error`; `func Detect(root string, langs []Language) (Language, error)`; `func ByName(name string, langs []Language) (Language, error)`; `func RunScript(interpreter string, script io.Reader, root string) (facts.Scan, error)`.

- [ ] **Step 1: Write the failing tests** `internal/lang/lang_test.go`

```go
package lang

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

var goLang = Language{Name: "go", Markers: []string{"go.mod"}}
var tsLang = Language{Name: "typescript", Markers: []string{"tsconfig.json", "package.json"}}

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectChecksLanguagesInOrder(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "package.json")
	touch(t, dir, "go.mod")
	l, err := Detect(dir, []Language{goLang, tsLang})
	if err != nil || l.Name != "go" {
		t.Fatalf("got %q, %v", l.Name, err)
	}
	os.Remove(filepath.Join(dir, "go.mod"))
	l, err = Detect(dir, []Language{goLang, tsLang})
	if err != nil || l.Name != "typescript" {
		t.Fatalf("got %q, %v", l.Name, err)
	}
}

func TestDetectNamesTheMarkersItLookedFor(t *testing.T) {
	_, err := Detect(t.TempDir(), []Language{goLang, tsLang})
	if !errors.Is(err, ErrNoLanguage) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"go.mod", "tsconfig.json", "package.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%q does not mention %s", err, want)
		}
	}
}

func TestByName(t *testing.T) {
	l, err := ByName("typescript", []Language{goLang, tsLang})
	if err != nil || l.Name != "typescript" {
		t.Fatalf("got %q, %v", l.Name, err)
	}
	_, err = ByName("cobol", []Language{goLang, tsLang})
	if err == nil || !strings.Contains(err.Error(), "go, typescript") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunScriptPipesTheScriptAndPassesTheRootAsOneArgument(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `import json, sys; print(json.dumps({"lang": "x", "prefix": sys.argv[1], "modules": []}))`
	s, err := RunScript("python3", strings.NewReader(script), root)
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "x" || s.Prefix != root {
		t.Fatalf("scan = %+v", s)
	}
}

func TestRunScriptReportsTheScriptsErrorOutput(t *testing.T) {
	script := `import sys; sys.stderr.write("boom"); sys.exit(3)`
	_, err := RunScript("python3", strings.NewReader(script), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunScriptNamesAMissingInterpreter(t *testing.T) {
	_, err := RunScript("umlv-no-such-tool", strings.NewReader(""), t.TempDir())
	var missing *facts.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "umlv-no-such-tool" {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/lang/`
Expected: FAIL to compile, `undefined: Language`.

- [ ] **Step 3: Write** `internal/lang/lang.go`

```go
// Package lang holds what every supported language provides, and the code
// that runs a scanner script written in another language.
package lang

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"uml-viewer-neo/internal/facts"
)

// Language is everything umlv knows about one language.
type Language struct {
	Name    string
	Markers []string
	Scan    func(root string) (facts.Scan, error)
}

var ErrNoLanguage = errors.New("no language recognised")

// Detect returns the first language, in the order given, with a marker file
// at root.
func Detect(root string, langs []Language) (Language, error) {
	var looked []string
	for _, l := range langs {
		for _, m := range l.Markers {
			if _, err := os.Stat(filepath.Join(root, m)); err == nil {
				return l, nil
			}
			looked = append(looked, m)
		}
	}
	return Language{}, fmt.Errorf("%w in %s (looked for %s)", ErrNoLanguage, root, strings.Join(looked, ", "))
}

// ByName returns the language called name.
func ByName(name string, langs []Language) (Language, error) {
	var names []string
	for _, l := range langs {
		if l.Name == name {
			return l, nil
		}
		names = append(names, l.Name)
	}
	return Language{}, fmt.Errorf("unknown language %q (choose %s)", name, strings.Join(names, ", "))
}

// RunScript runs `<interpreter> - <root>` with script on standard input and
// decodes the scan facts it prints. Nothing is written to disk.
func RunScript(interpreter string, script io.Reader, root string) (facts.Scan, error) {
	if _, err := exec.LookPath(interpreter); err != nil {
		return facts.Scan{}, &facts.MissingToolError{Tool: interpreter}
	}
	cmd := exec.Command(interpreter, "-", root)
	cmd.Dir = root
	cmd.Stdin = script
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return facts.Scan{}, fmt.Errorf("%s scanner failed (%v): %s", interpreter, err, strings.TrimSpace(errOut.String()))
	}
	var scan facts.Scan
	if err := json.Unmarshal(out.Bytes(), &scan); err != nil {
		return facts.Scan{}, fmt.Errorf("%s scanner printed unreadable JSON: %w", interpreter, err)
	}
	return scan, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/lang/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/lang
git commit -m "Detect languages and run scanner scripts over standard input."
```

---

### Task 3: The Go scanner

**Files:**
- Create: `internal/lang/golang/scan.go` (ported from the fork's `goscan`), `internal/lang/golang/golang.go`, `internal/lang/golang/scan_test.go`, `internal/lang/golang/testdata/shop/` (copied)

**Interfaces:**
- Consumes: `lang.Language`, `facts.*`.
- Produces: `func Scan(root string) (facts.Scan, error)`; `var Lang lang.Language` (name `go`, marker `go.mod`).

- [ ] **Step 1: Copy the scanner and the sample repo from the fork**

```bash
mkdir -p internal/lang/golang/testdata
cp -R ../uml-viewer-polyglot/spec/fixtures/go/shop internal/lang/golang/testdata/shop
cp ../uml-viewer-polyglot/resources/helpers/goscan/main.go internal/lang/golang/scan.go
```

- [ ] **Step 2: Write the failing tests** `internal/lang/golang/scan_test.go`

```go
package golang

import (
	"testing"

	"uml-viewer-neo/internal/facts"
)

func scanShop(t *testing.T) map[string]facts.Module {
	t.Helper()
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "go" || s.Prefix != "github.com/acme/shop" {
		t.Fatalf("scan = %+v", s)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	return byID
}

func TestScanFindsPackagesAndTheirImports(t *testing.T) {
	byID := scanShop(t)
	root, cart := byID["github.com/acme/shop"], byID["github.com/acme/shop/internal/cart"]
	if root.Path != "" || root.Name != "shop" || cart.Path != "internal/cart" || cart.Dir != "internal/cart" {
		t.Fatalf("root = %+v, cart = %+v", root, cart)
	}
	var sawCart, sawFmt bool
	for _, imp := range root.Imports {
		sawCart = sawCart || (imp.To == "github.com/acme/shop/internal/cart" && imp.Project)
		sawFmt = sawFmt || (imp.To == "fmt" && imp.Std)
	}
	if !sawCart || !sawFmt {
		t.Fatalf("imports = %+v", root.Imports)
	}
}

func TestScanFindsFunctionsWithLinesAndComplexity(t *testing.T) {
	byID := scanShop(t)
	run := byID["github.com/acme/shop"].Functions[0]
	if run.Name != "Run" || run.File != "shop.go" || run.Start != 10 || run.End != 18 || run.CC != 3 {
		t.Fatalf("Run = %+v", run)
	}
	var names []string
	for _, f := range byID["github.com/acme/shop/internal/cart"].Functions {
		names = append(names, f.Name)
	}
	if got := len(names); got != 4 || names[0] != "New" || names[1] != "Cart.Add" || names[3] != "Cart.Full" {
		t.Fatalf("cart functions = %v", names)
	}
}

func TestScanLeavesOutTests(t *testing.T) {
	for _, m := range scanShop(t) {
		for _, f := range m.Functions {
			if len(f.Name) > 4 && f.Name[:4] == "Test" {
				t.Fatalf("test function %s was scanned", f.Name)
			}
		}
	}
}

func TestLangIsRecognisedByGoMod(t *testing.T) {
	if Lang.Name != "go" || Lang.Markers[0] != "go.mod" || Lang.Scan == nil {
		t.Fatalf("Lang = %+v", Lang)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/lang/golang/`
Expected: FAIL to compile (`scan.go` is still `package main`).

- [ ] **Step 4: Port** `internal/lang/golang/scan.go`

Make exactly these edits:

1. Replace the file's opening comment and `package main` with:

```go
// Package golang scans Go modules. Packages and imports come from `go list`,
// so build constraints and the module path are whatever the toolchain says.
// Functions, their line ranges and cyclomatic complexity come from go/parser.
package golang
```

2. Delete the `Import`, `Function`, `Module` and `Scan` type declarations. Add `"uml-viewer-neo/internal/facts"` to the imports, and qualify every use: `Import` → `facts.Import`, `Function` → `facts.Function`, `Module{` → `facts.Module{`, and in the module literal rename the field `NS:` → `ID:`.

3. Replace `func main()` and `func fail(...)` with:

```go
// Scan reports the packages, imports and functions of the Go module at root.
func Scan(root string) (facts.Scan, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return facts.Scan{}, &facts.MissingToolError{Tool: "go"}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return facts.Scan{}, err
	}
	pkgs, err := goList(root)
	if err != nil {
		return facts.Scan{}, err
	}
	prefix := modulePath(pkgs)
	requires := readRequires(filepath.Join(root, "go.mod"))

	scan := facts.Scan{Lang: "go", Prefix: prefix, Modules: []facts.Module{}}
	for _, p := range pkgs {
		if len(p.GoFiles)+len(p.CgoFiles) == 0 {
			continue
		}
		rel, _ := filepath.Rel(root, p.Dir)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		scan.Modules = append(scan.Modules, facts.Module{
			ID:        p.ImportPath,
			Path:      rel,
			Name:      p.Name,
			Dir:       rel,
			Imports:   imports(p.Imports, prefix, requires),
			Functions: functions(root, p),
		})
	}
	return scan, nil
}
```

4. Run `go vet ./internal/lang/golang/` and delete any import it reports as unused.

- [ ] **Step 5: Write** `internal/lang/golang/golang.go`

```go
package golang

import "uml-viewer-neo/internal/lang"

// Lang is Go: recognised by go.mod, scanned in-process.
var Lang = lang.Language{Name: "go", Markers: []string{"go.mod"}, Scan: Scan}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/lang/golang/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/lang/golang
git commit -m "Scan Go modules in-process with go list and go/parser."
```

---

### Task 4: The Python scanner

**Files:**
- Create: `internal/lang/python/python.go`, `internal/lang/python/pyscan.py` (copied), `internal/lang/python/python_test.go`, `internal/lang/python/testdata/shop/` (copied)

**Interfaces:**
- Consumes: `lang.RunScript`, `lang.Language`.
- Produces: `func Scan(root string) (facts.Scan, error)`; `var Lang lang.Language` (name `python`, markers `pyproject.toml`, `setup.py`).

- [ ] **Step 1: Copy the scanner and the sample repo**

```bash
mkdir -p internal/lang/python/testdata
cp ../uml-viewer-polyglot/resources/helpers/pyscan.py internal/lang/python/pyscan.py
cp -R ../uml-viewer-polyglot/spec/fixtures/python/shop internal/lang/python/testdata/shop
```

- [ ] **Step 2: Write the failing tests** `internal/lang/python/python_test.go`

```go
package python

import (
	"testing"

	"uml-viewer-neo/internal/facts"
)

func TestScanShop(t *testing.T) {
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "python" || s.Prefix != "shop" || len(s.Notes) != 1 {
		t.Fatalf("scan = %+v", s)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	if len(byID) != 5 || byID["src/shop/__init__.py"].Path != "" || byID["src/shop/cart/basket.py"].Path != "cart/basket" {
		t.Fatalf("modules = %+v", s.Modules)
	}
	checkout := byID["src/shop/checkout.py"]
	var sawPricing, sawRequests bool
	for _, imp := range checkout.Imports {
		sawPricing = sawPricing || (imp.To == "src/shop/pricing.py" && imp.Project)
		sawRequests = sawRequests || (imp.Module == "requests" && !imp.Project && !imp.Std)
	}
	if !sawPricing || !sawRequests {
		t.Fatalf("imports = %+v", checkout.Imports)
	}
	run := checkout.Functions[0]
	if run.Name != "run" || run.Start != 15 || run.End != 20 || run.CoverStart != 16 || run.CC != 3 {
		t.Fatalf("run = %+v", run)
	}
}

func TestLangIsRecognisedByPyproject(t *testing.T) {
	if Lang.Name != "python" || Lang.Markers[0] != "pyproject.toml" || Lang.Markers[1] != "setup.py" {
		t.Fatalf("Lang = %+v", Lang)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/lang/python/`
Expected: FAIL to compile, `undefined: Scan`.

- [ ] **Step 4: Write** `internal/lang/python/python.go`

```go
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
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/lang/python/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/lang/python
git commit -m "Scan Python projects by piping pyscan.py into python3."
```

---

### Task 5: The TypeScript scanner with a pinned compiler

**Files:**
- Create: `internal/lang/typescript/typescript.go`, `internal/lang/typescript/tsscan.js` (copied, edited), `internal/lang/typescript/typescript.js.gz`, `internal/lang/typescript/TYPESCRIPT-LICENSE.txt`, `internal/lang/typescript/typescript_test.go`, `internal/lang/typescript/testdata/shop/` (copied)

**Interfaces:**
- Consumes: `lang.RunScript`, `lang.Language`.
- Produces: `func Scan(root string) (facts.Scan, error)`; `var Lang lang.Language` (name `typescript`, markers `tsconfig.json`, `package.json`).

The scanner and sample repo come from the fork's branch `claude/quizzical-hermann-4eec91`, which adds object-literal methods. If that branch is gone, find the commit with `git -C ../uml-viewer-polyglot log --all --oneline --grep "object literals"` and use its hash instead.

- [ ] **Step 1: Copy the scanner, the sample repo and the compiler**

```bash
mkdir -p internal/lang/typescript/testdata
BR=claude/quizzical-hermann-4eec91
git -C ../uml-viewer-polyglot show "${BR}:resources/helpers/tsscan.js" > internal/lang/typescript/tsscan.js
git -C ../uml-viewer-polyglot archive --format=tar "$BR" spec/fixtures/typescript/shop \
  | tar -x --strip-components=3 -C internal/lang/typescript/testdata
grep '"version"' ../spaced-repetition/node_modules/typescript/package.json   # must print 5.9.3
gzip -9 -c ../spaced-repetition/node_modules/typescript/lib/typescript.js > internal/lang/typescript/typescript.js.gz
cp ../spaced-repetition/node_modules/typescript/LICENSE.txt internal/lang/typescript/TYPESCRIPT-LICENSE.txt
```

- [ ] **Step 2: Write the failing tests** `internal/lang/typescript/typescript_test.go`

```go
package typescript

import (
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
)

func names(fs []facts.Function) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

func TestScanShopWithoutNodeModules(t *testing.T) {
	if _, err := os.Stat("testdata/shop/node_modules"); err == nil {
		t.Fatal("the sample repo must not have node_modules")
	}
	s, err := Scan("testdata/shop")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]facts.Module{}
	for _, m := range s.Modules {
		byID[m.ID] = m
	}
	if len(byID) != 5 || byID["src/index.ts"].Path != "" || byID["src/cart/index.ts"].Path != "cart" {
		t.Fatalf("modules = %+v", s.Modules)
	}
	checkout := byID["src/checkout.ts"]
	var sawAlias, sawStd, sawLib bool
	for _, imp := range checkout.Imports {
		sawAlias = sawAlias || (imp.To == "src/pricing.ts" && imp.Project) // via the @/ path alias
		sawStd = sawStd || (imp.To == "node:fs" && imp.Std)
		sawLib = sawLib || (imp.Module == "zod" && !imp.Std && !imp.Project)
	}
	if !sawAlias || !sawStd || !sawLib {
		t.Fatalf("imports = %+v", checkout.Imports)
	}
	if got := names(byID["src/pricing.ts"].Functions); len(got) != 3 || got[1] != "discounts.half" || got[2] != "discounts.none" {
		t.Fatalf("pricing functions = %v", got)
	}
	if got := names(byID["src/index.ts"].Functions); len(got) != 1 || got[0] != "default.fetch" {
		t.Fatalf("index functions = %v", got)
	}
	total := byID["src/pricing.ts"].Functions[0]
	if total.Start != 3 || total.CoverStart != 3 || total.CoverCol != 46 {
		t.Fatalf("total = %+v", total)
	}
}

func TestTheEmbeddedCompilerIsPinned(t *testing.T) {
	gz, err := gzip.NewReader(bytes.NewReader(compilerGz))
	if err != nil {
		t.Fatal(err)
	}
	probe := []byte(`process.stdout.write(JSON.stringify({lang: ts.version, prefix: "", modules: []}))`)
	s, err := lang.RunScript("node", program(gz, probe), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.Lang != "5.9.3" {
		t.Fatalf("TypeScript %s", s.Lang)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/lang/typescript/`
Expected: FAIL to compile, `undefined: Scan`.

- [ ] **Step 4: Edit** `internal/lang/typescript/tsscan.js`

1. In the header comment, replace the two lines starting `// TypeScript itself comes from the project…` with:

```js
// TypeScript arrives as `ts`, defined by the wrapper umlv pipes in ahead of
// this script (see typescript.go). Nothing is looked up in node_modules.
```

2. Delete the whole `function loadTypeScript(root) { … }`.
3. In `main()`, delete the line `const ts = loadTypeScript(root);`.

- [ ] **Step 5: Write** `internal/lang/typescript/typescript.go`

```go
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
var Lang = lang.Language{Name: "typescript", Markers: []string{"tsconfig.json", "package.json"}, Scan: Scan}

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
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/lang/typescript/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/lang/typescript
git commit -m "Scan TypeScript with a pinned compiler piped into node, so no repo needs node_modules."
```

---

### Task 6: The policy file

**Files:**
- Create: `internal/policy/policy.go`, `internal/policy/policy_test.go`
- Modify: `go.mod`, `go.sum` (add `github.com/BurntSushi/toml`)

**Interfaces:**
- Consumes: `facts.Scan`.
- Produces: `type Policy struct { Libraries []string; Editor string; Coverage struct { Command []string; Report string } }`; `const File = ".umlv/policy.toml"`; `func Default(scan facts.Scan) Policy`; `func TopLibraries(scan facts.Scan, n int) []string`; `func Write(root string, p Policy) error`; `func Load(root string) (p Policy, ok bool, err error)`; `func (p Policy) EditorPrefix(root string) string`.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/BurntSushi/toml@latest
```

- [ ] **Step 2: Write the failing tests** `internal/policy/policy_test.go`

```go
package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func lib(name string) facts.Import { return facts.Import{To: name, Module: name} }

func TestTopLibrariesCountsModulesNotImports(t *testing.T) {
	scan := facts.Scan{Modules: []facts.Module{
		{ID: "a", Imports: []facts.Import{lib("zod"), lib("zod"), lib("katex"), {To: "fmt", Std: true}, {To: "b", Project: true}}},
		{ID: "b", Imports: []facts.Import{lib("katex")}},
		{ID: "c", Imports: []facts.Import{lib("marked")}},
	}}
	if got := TopLibraries(scan, 2); !reflect.DeepEqual(got, []string{"katex", "marked"}) {
		t.Fatalf("got %v", got)
	}
}

func TestWriteThenLoadRoundTrips(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := Load(root); ok || err != nil {
		t.Fatalf("missing policy: ok=%v err=%v", ok, err)
	}
	want := Policy{Libraries: []string{"zod", "@scope/pkg"}, Editor: "cursor"}
	if err := Write(root, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Load(root)
	if err != nil || !ok || !reflect.DeepEqual(got.Libraries, want.Libraries) || got.Editor != "cursor" {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	text, _ := os.ReadFile(filepath.Join(root, File))
	if !strings.Contains(string(text), "# [coverage]") {
		t.Fatalf("written policy has no commented coverage example:\n%s", text)
	}
}

func TestLoadGivesTheLineOfASyntaxError(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(root, File), []byte("editor = \"vscode\"\nlibraries = [\n"), 0o644)
	_, _, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "policy.toml:") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsAnUnknownEditor(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(root, File), []byte("editor = \"emacs\"\n"), 0o644)
	if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "emacs") {
		t.Fatalf("err = %v", err)
	}
}

func TestEditorPrefixEscapesSpaces(t *testing.T) {
	got := Policy{Editor: "vscode"}.EditorPrefix("/Users/nick/My Repo")
	if got != "vscode://file/Users/nick/My%20Repo/" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/policy/`
Expected: FAIL to compile, `undefined: TopLibraries`.

- [ ] **Step 4: Write** `internal/policy/policy.go`

```go
// Package policy reads and writes .umlv/policy.toml: the one file in .umlv/
// that belongs to the user. umlv writes it once and never overwrites it.
package policy

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"uml-viewer-neo/internal/facts"
)

const File = ".umlv/policy.toml"

type Policy struct {
	Libraries []string `toml:"libraries"`
	Editor    string   `toml:"editor"`
	Coverage  struct {
		Command []string `toml:"command"`
		Report  string   `toml:"report"`
	} `toml:"coverage"`
}

// Default is the policy written on a repo's first scan.
func Default(scan facts.Scan) Policy {
	return Policy{Libraries: TopLibraries(scan, 8), Editor: "vscode"}
}

// TopLibraries returns the n outside libraries imported by the most modules,
// most first, ties by name.
func TopLibraries(scan facts.Scan, n int) []string {
	count := map[string]int{}
	for _, m := range scan.Modules {
		seen := map[string]bool{}
		for _, imp := range m.Imports {
			if imp.Project || imp.Std {
				continue
			}
			name := imp.Module
			if name == "" {
				name = imp.To
			}
			if !seen[name] {
				seen[name] = true
				count[name]++
			}
		}
	}
	names := make([]string, 0, len(count))
	for name := range count {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if count[names[i]] != count[names[j]] {
			return count[names[i]] > count[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) > n {
		names = names[:n]
	}
	return names
}

// Write writes p to root's policy file, with comments explaining each key.
func Write(root string, p Policy) error {
	path := filepath.Join(root, File)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	quoted := make([]string, len(p.Libraries))
	for i, l := range p.Libraries {
		quoted[i] = strconv.Quote(l)
	}
	text := "# umlv policy. Written once; edit freely, umlv never overwrites it.\n\n" +
		"# Outside libraries drawn as ovals. A name also covers its sub-packages.\n" +
		"libraries = [" + strings.Join(quoted, ", ") + "]\n\n" +
		"# Editor for source links: \"vscode\" or \"cursor\".\n" +
		"editor = " + strconv.Quote(p.Editor) + "\n\n" +
		"# To replace the language's coverage command, uncomment and edit.\n" +
		"# {report} in the command becomes the report path.\n" +
		"# [coverage]\n# command = [\"make\", \"cover\"]\n# report = \"build/cover.out\"\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

// Load reads root's policy. A missing file is not an error: ok is false.
func Load(root string) (p Policy, ok bool, err error) {
	path := filepath.Join(root, File)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return Policy{}, false, nil
	}
	if _, err := toml.DecodeFile(path, &p); err != nil {
		var perr toml.ParseError
		if errors.As(err, &perr) {
			return Policy{}, true, fmt.Errorf("%s:%d: %s", path, perr.Position.Line, perr.Message)
		}
		return Policy{}, true, fmt.Errorf("%s: %w", path, err)
	}
	if p.Editor == "" {
		p.Editor = "vscode"
	}
	if p.Editor != "vscode" && p.Editor != "cursor" {
		return Policy{}, true, fmt.Errorf("%s: editor must be \"vscode\" or \"cursor\", not %q", path, p.Editor)
	}
	return p, true, nil
}

// EditorPrefix is the start of a source link: append a repo-relative path,
// then ":<line>".
func (p Policy) EditorPrefix(root string) string {
	return p.Editor + "://file" + (&url.URL{Path: filepath.ToSlash(root)}).EscapedPath() + "/"
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/policy/`
Expected: PASS. If `toml.ParseError` is not matched by `errors.As` in the installed version, read `go doc github.com/BurntSushi/toml.ParseError` and match the type it documents; the test only requires `policy.toml:<line>` in the message.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/policy
git commit -m "Write, load and query the policy file."
```

---

### Task 7: Page data, and the golden file both languages share

**Files:**
- Create: `internal/page/build.go`, `internal/page/build_test.go`, `web/testdata/shop.page.json` (written by the test)

**Interfaces:**
- Consumes: `facts.Scan`, `facts.Scores`, `facts.Page`.
- Produces: `type Options struct { Repo, ScannedAt, CoverageAt, EditorPrefix string; Libraries []string; Bands map[string]string }`; `func Build(scan facts.Scan, scores facts.Scores, o Options) facts.Page`; the golden file `web/testdata/shop.page.json` that Tasks 8–10 read. Test helpers `shopScan()`, `shopScores()`, `shopOptions()` in `build_test.go`, reused by Task 11.

- [ ] **Step 1: Write the failing tests** `internal/page/build_test.go`

```go
package page

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"uml-viewer-neo/internal/facts"
)

var update = flag.Bool("update", false, "rewrite web/testdata/shop.page.json")

const golden = "../../web/testdata/shop.page.json"

func project(id string) facts.Import { return facts.Import{To: id, Project: true} }

func shopScan() facts.Scan {
	return facts.Scan{Lang: "typescript", Notes: []string{"Left out 1 file."}, Modules: []facts.Module{
		{ID: "src/index.ts", Path: "", Name: "shop", File: "src/index.ts", Imports: []facts.Import{project("src/checkout.ts")}},
		{ID: "src/checkout.ts", Path: "checkout", Name: "checkout", File: "src/checkout.ts",
			Imports: []facts.Import{project("src/cart/basket.ts"), project("src/pricing.ts"),
				{To: "zod", Module: "zod"}, {To: "node:fs", Std: true, Module: "node:fs"}, {To: "left-pad", Module: "left-pad"}},
			Functions: []facts.Function{
				{Name: "run", File: "src/checkout.ts", Start: 9, End: 17, CC: 5},
				{Name: "neverCalled", File: "src/checkout.ts", Start: 19, End: 19, CC: 1}}},
		{ID: "src/pricing.ts", Path: "pricing", Name: "pricing", File: "src/pricing.ts",
			Imports:   []facts.Import{project("src/cart/basket.ts")},
			Functions: []facts.Function{{Name: "total", File: "src/pricing.ts", Start: 3, End: 5, CC: 1}}},
		{ID: "src/cart/index.ts", Path: "cart", Name: "cart", File: "src/cart/index.ts",
			Imports: []facts.Import{project("src/cart/basket.ts")}},
		{ID: "src/cart/basket.ts", Path: "cart/basket", Name: "basket", File: "src/cart/basket.ts",
			Functions: []facts.Function{
				{Name: "Basket.add", File: "src/cart/basket.ts", Start: 4, End: 6, CC: 3},
				{Name: "Basket.full", File: "src/cart/basket.ts", Start: 8, End: 8, CC: 9}}},
	}}
}

func shopScores() facts.Scores {
	return facts.Scores{
		"src/checkout.ts": {Grade: facts.Red, Stats: &facts.Stats{Mu: 16, Sigma: 14, Max: 30},
			Functions: map[string]facts.FunctionScore{"run": {Coverage: 0, CRAP: 30}, "neverCalled": {Coverage: 0, CRAP: 2}}},
		"src/pricing.ts": {Grade: facts.Green, Stats: &facts.Stats{Mu: 1, Sigma: 0, Max: 1},
			Functions: map[string]facts.FunctionScore{"total": {Coverage: 1, CRAP: 1}}},
		"src/cart/basket.ts": {Grade: facts.Amber, Stats: &facts.Stats{Mu: 6.5625, Sigma: 2.4375, Max: 9},
			Functions: map[string]facts.FunctionScore{"Basket.add": {Coverage: 0.5, CRAP: 4.125}, "Basket.full": {Coverage: 1, CRAP: 9}}},
	}
}

func shopOptions() Options {
	return Options{Repo: "shop", ScannedAt: "2026-09-30T20:00:00Z", CoverageAt: "2026-09-30T19:59:00Z",
		EditorPrefix: "vscode://file/repo/", Libraries: []string{"zod"},
		Bands: map[string]string{"green": "8 or less", "amber": "over 8, up to 12", "red": "over 12"}}
}

func find(p facts.Page, id string) facts.PageModule {
	for _, m := range p.Modules {
		if m.ID == id {
			return m
		}
	}
	return facts.PageModule{}
}

func TestBuildResolvesUsesToModulesAndListedLibrariesOnly(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	if got := find(p, "src/checkout.ts").Uses; !reflect.DeepEqual(got, []string{"lib:zod", "src/cart/basket.ts", "src/pricing.ts"}) {
		t.Fatalf("uses = %v", got)
	}
	if !reflect.DeepEqual(p.Libraries, []facts.Library{{ID: "lib:zod", Name: "zod"}}) {
		t.Fatalf("libraries = %v", p.Libraries)
	}
}

func TestBuildPlacesModulesInTheFolderTree(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	if got := find(p, "src/index.ts").Tree; !reflect.DeepEqual(got, []string{"shop"}) {
		t.Fatalf("root module tree = %v", got)
	}
	if got := find(p, "src/cart/basket.ts").Tree; !reflect.DeepEqual(got, []string{"cart", "basket"}) {
		t.Fatalf("basket tree = %v", got)
	}
}

func TestBuildCarriesScoresAndLeavesUnmeasuredModulesUnlit(t *testing.T) {
	p := Build(shopScan(), shopScores(), shopOptions())
	checkout := find(p, "src/checkout.ts")
	if checkout.Grade != facts.Red || checkout.Stats.Mu != 16 {
		t.Fatalf("checkout = %+v", checkout)
	}
	run := checkout.Functions[0]
	if run.Name != "run" || run.Line != 9 || run.CRAP == nil || *run.CRAP != 30 || run.Coverage == nil || *run.Coverage != 0 {
		t.Fatalf("run = %+v", run)
	}
	cart := find(p, "src/cart/index.ts")
	if cart.Grade != facts.Unlit || cart.Stats != nil {
		t.Fatalf("cart = %+v", cart)
	}
}

func TestLibraryCoversSubPackages(t *testing.T) {
	cases := []struct {
		imp  facts.Import
		libs []string
		want string
	}{
		{facts.Import{To: "ai/test", Module: "ai/test"}, []string{"ai"}, "ai"},
		{facts.Import{To: "aim", Module: "aim"}, []string{"ai"}, ""},
		{facts.Import{To: "google.cloud.storage", Module: "google.cloud.storage"}, []string{"google"}, "google"},
		{facts.Import{To: "github.com/spf13/pflag"}, []string{"github.com/spf13/pflag"}, "github.com/spf13/pflag"},
	}
	for _, c := range cases {
		if got, _ := library(c.imp, c.libs); got != c.want {
			t.Errorf("library(%v, %v) = %q, want %q", c.imp, c.libs, got, c.want)
		}
	}
}

func TestBuildMatchesTheGoldenFile(t *testing.T) {
	got, err := json.MarshalIndent(Build(shopScan(), shopScores(), shopOptions()), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/page -run Golden -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("page data changed. If intended: go test ./internal/page -run Golden -update, then node --test web/")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/page/`
Expected: FAIL to compile, `undefined: Build`.

- [ ] **Step 3: Write** `internal/page/build.go`

```go
// Package page turns scan facts and scores into the page: the data the page
// reads, and the HTML that carries it along with the bundled page code.
package page

import (
	"sort"
	"strings"

	"uml-viewer-neo/internal/facts"
)

// Options are the parts of the page that do not come from the scan.
// Libraries and EditorPrefix are the policy, already resolved; Bands
// describes each grade's cut-off in words for the card.
type Options struct {
	Repo, ScannedAt, CoverageAt, EditorPrefix string
	Libraries                                 []string
	Bands                                     map[string]string
}

// Build makes the page data. Policy is applied here, so the page knows
// nothing about it: unlisted libraries and the standard library are dropped.
func Build(scan facts.Scan, scores facts.Scores, o Options) facts.Page {
	ids := map[string]bool{}
	for _, m := range scan.Modules {
		ids[m.ID] = true
	}
	used := map[string]bool{}
	p := facts.Page{
		Repo: o.Repo, ScannedAt: o.ScannedAt, CoverageAt: o.CoverageAt,
		EditorPrefix: o.EditorPrefix, Bands: o.Bands, Notes: scan.Notes,
		Modules: []facts.PageModule{}, Libraries: []facts.Library{},
	}
	for _, m := range scan.Modules {
		score, measured := scores[m.ID]
		pm := facts.PageModule{
			ID: m.ID, Tree: tree(m), Name: m.Name, Source: m.Source(), Grade: facts.Unlit,
			Functions: []facts.PageFunction{}, Uses: uses(m, ids, o.Libraries, used),
		}
		if measured {
			pm.Grade, pm.Stats = score.Grade, score.Stats
		}
		for _, f := range m.Functions {
			pf := facts.PageFunction{Name: f.Name, File: f.File, Line: f.Start, CC: f.CC}
			if fs, ok := score.Functions[f.Name]; ok {
				cov, crap := fs.Coverage, fs.CRAP
				pf.Coverage, pf.CRAP = &cov, &crap
			}
			pm.Functions = append(pm.Functions, pf)
		}
		p.Modules = append(p.Modules, pm)
	}
	sort.Slice(p.Modules, func(i, j int) bool { return p.Modules[i].ID < p.Modules[j].ID })
	for _, l := range o.Libraries {
		if used[l] {
			p.Libraries = append(p.Libraries, facts.Library{ID: "lib:" + l, Name: l})
		}
	}
	return p
}

// tree is a module's folder path ending with its own name. The repo's root
// module has an empty path and goes under its name.
func tree(m facts.Module) []string {
	if m.Path == "" {
		return []string{m.Name}
	}
	return strings.Split(m.Path, "/")
}

func uses(m facts.Module, ids map[string]bool, libs []string, used map[string]bool) []string {
	set := map[string]bool{}
	for _, imp := range m.Imports {
		switch {
		case imp.Project:
			if ids[imp.To] && imp.To != m.ID {
				set[imp.To] = true
			}
		case !imp.Std:
			if lib, ok := library(imp, libs); ok {
				set["lib:"+lib] = true
				used[lib] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// library is the listed library an import belongs to: its own name, or a
// listed name it sits under ("ai" covers "ai/test", "google" covers
// "google.cloud.storage").
func library(imp facts.Import, libs []string) (string, bool) {
	name := imp.Module
	if name == "" {
		name = imp.To
	}
	for _, l := range libs {
		if name == l || strings.HasPrefix(name, l+"/") || strings.HasPrefix(name, l+".") {
			return l, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Write the golden file and run the tests**

Run: `go test ./internal/page/ -run Golden -update && go test ./internal/page/`
Expected: PASS. Open `web/testdata/shop.page.json` and check by eye: five modules sorted by ID, `src/checkout.ts` uses `["lib:zod", "src/cart/basket.ts", "src/pricing.ts"]`, `libraries` is only `zod`.

- [ ] **Step 5: Commit**

```bash
git add internal/page web/testdata
git commit -m "Build page data from scan facts and scores; write the golden file the page tests read."
```

---

### Task 8: `model.js`: boxes, arrows, badges and lamps for one folder

**Files:**
- Create: `web/package.json`, `web/model.js`, `web/model.test.js`

**Interfaces:**
- Consumes: page data shaped like `web/testdata/shop.page.json`.
- Produces: `GEOMETRY`; `worstGrade(grades) → grade`; `shorten(name, max = 24) → string`; `folderExists(page, folder) → boolean`; `viewAt(page, folder) → { folder, boxes, libraries, arrows, counts }`, where a box is `{ key, name, kind ('module'|'folder'|'both'), label, detail, moduleId, grade, stats, modules, measured, uses: {on, off}, usedBy: {on, off}, width, height }`, a library is `{ key, id, name, width, height }`, an arrow is `{ from, to }` (box or library keys), and `counts` is `{ red, amber, green, unlit }`. A folder is an array of path segments; `[]` is the top.

- [ ] **Step 1: Create** `web/package.json`

```json
{"private": true, "type": "module"}
```

- [ ] **Step 2: Write the failing tests** `web/model.test.js`

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt, worstGrade, shorten, folderExists, GEOMETRY } from './model.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));
const box = (view, key) => view.boxes.find((b) => b.key === key);

test('the top level shows one box per top-level name, plus used libraries', () => {
  const view = viewAt(page, []);
  assert.deepEqual(view.boxes.map((b) => b.key), ['cart', 'checkout', 'pricing', 'shop']);
  assert.deepEqual(view.libraries.map((l) => l.key), ['lib:zod']);
});

test('a folder that is also a module is one box with its worst measured lamp', () => {
  const cart = box(viewAt(page, []), 'cart');
  assert.equal(cart.kind, 'both');
  assert.equal(cart.moduleId, 'src/cart/index.ts');
  assert.equal(cart.grade, 'amber');
  assert.equal(cart.label, 'cart/ ›');
  assert.equal(cart.detail, '2 modules · 1 measured');
  assert.equal(cart.stats, null);
});

test('a plain module box carries its stats', () => {
  const checkout = box(viewAt(page, []), 'checkout');
  assert.equal(checkout.kind, 'module');
  assert.equal(checkout.grade, 'red');
  assert.equal(checkout.stats.mu, 16);
});

test('arrows are merged to one per pair of boxes and skip a box importing itself', () => {
  const arrows = viewAt(page, []).arrows.map((a) => `${a.from}>${a.to}`).sort();
  assert.deepEqual(arrows, ['checkout>cart', 'checkout>lib:zod', 'checkout>pricing', 'pricing>cart', 'shop>checkout']);
});

test('inside a folder, its own module is a box and outside partners become badge counts', () => {
  const view = viewAt(page, ['cart']);
  assert.deepEqual(view.boxes.map((b) => b.key), ['cart', 'cart/basket']);
  assert.equal(box(view, 'cart').kind, 'module');
  const basket = box(view, 'cart/basket');
  assert.deepEqual(basket.usedBy, { on: ['cart'], off: ['checkout', 'pricing'] });
  assert.deepEqual(basket.uses, { on: [], off: [] });
  assert.deepEqual(view.libraries, []);
});

test('the legend counts boxes at this level by lamp', () => {
  assert.deepEqual(viewAt(page, []).counts, { red: 1, amber: 1, green: 1, unlit: 1 });
});

test('an import cycle gives an arrow each way', () => {
  const cycle = { libraries: [], modules: [
    { id: 'a', tree: ['a'], grade: 'unlit', uses: ['b'], functions: [] },
    { id: 'b', tree: ['b'], grade: 'unlit', uses: ['a'], functions: [] }] };
  assert.deepEqual(viewAt(cycle, []).arrows.map((x) => x.from + x.to).sort(), ['ab', 'ba']);
});

test('worstGrade ignores unlit unless nothing is lit', () => {
  assert.equal(worstGrade(['unlit', 'green']), 'green');
  assert.equal(worstGrade(['green', 'red', 'amber']), 'red');
  assert.equal(worstGrade(['unlit']), 'unlit');
  assert.equal(worstGrade([]), 'unlit');
});

test('shorten keeps both ends of a long name', () => {
  const s = shorten('abcdefghijklmnopqrstuvwxyz0123');
  assert.equal(s.length, 24);
  assert.ok(s.startsWith('abcdefghijkl') && s.endsWith('z0123') && s.includes('…'));
  assert.equal(shorten('short'), 'short');
});

test('folderExists only for folders with modules below them', () => {
  assert.equal(folderExists(page, []), true);
  assert.equal(folderExists(page, ['cart']), true);
  assert.equal(folderExists(page, ['pricing']), false);
  assert.equal(folderExists(page, ['gone']), false);
});

test('boxes are sized to fit their text', () => {
  const cart = box(viewAt(page, []), 'cart');
  assert.ok(cart.width >= 'cart/ ›'.length * GEOMETRY.nameW + 2 * GEOMETRY.padX);
  assert.equal(cart.height, 2 * GEOMETRY.line + 2 * GEOMETRY.padY);
  assert.equal(box(viewAt(page, []), 'shop').height, GEOMETRY.line + 2 * GEOMETRY.padY);
});
```

- [ ] **Step 3: Run them to see them fail**

Run: `node --test web/`
Expected: FAIL, `Cannot find module '.../web/model.js'`.

- [ ] **Step 4: Write** `web/model.js`

```js
// model.js: page data and a folder in; boxes, merged arrows, badge counts,
// folder lamps and box sizes out. No DOM and no layout here.

const ORDER = ['unlit', 'green', 'amber', 'red'];

// Box geometry in pixels, shared with render.js. Text is monospaced, so a
// box's width follows from its character count.
export const GEOMETRY = { padX: 12, padY: 10, line: 18, lamp: 10, gap: 8, nameW: 7.8, detailW: 7.2, libH: 28, libPad: 16 };

export function worstGrade(grades) {
  let worst = 'unlit';
  for (const g of grades) if (ORDER.indexOf(g) > ORDER.indexOf(worst)) worst = g;
  return worst;
}

export function shorten(name, max = 24) {
  if (name.length <= max) return name;
  const keep = max - 1;
  const head = Math.ceil(keep / 2);
  return name.slice(0, head) + '…' + name.slice(name.length - (keep - head));
}

const keyOf = (segs) => segs.join('/');
const under = (tree, folder) => folder.every((s, i) => tree[i] === s);

export function folderExists(page, folder) {
  return folder.length === 0 || page.modules.some((m) => m.tree.length > folder.length && under(m.tree, folder));
}

function size(label, detail) {
  const g = GEOMETRY;
  const text = Math.max(label.length * g.nameW, detail.length * g.detailW);
  return { width: Math.ceil(g.padX + g.lamp + g.gap + text + g.padX), height: (detail ? 2 : 1) * g.line + 2 * g.padY };
}

export function viewAt(page, folder) {
  const depth = folder.length;
  const byId = new Map(page.modules.map((m) => [m.id, m]));
  const libs = new Map(page.libraries.map((l) => [l.id, l]));

  // The box a module belongs to at this level, or null when it is outside.
  // A folder's own module (cart/index.ts inside cart/) is a box of its own.
  const boxKey = (m) => {
    if (!under(m.tree, folder)) return null;
    if (m.tree.length === depth) return keyOf(m.tree);
    return keyOf(m.tree.slice(0, depth + 1));
  };
  // How a partner outside this folder is named: its top-level name, or at
  // depth n its first n segments.
  const outsideKey = (m) => keyOf(m.tree.slice(0, Math.max(1, depth)));

  const boxes = new Map();
  for (const m of page.modules) {
    const key = boxKey(m);
    if (key === null) continue;
    if (!boxes.has(key)) {
      boxes.set(key, { key, name: key.split('/').pop(), module: null, members: [], folder: false,
        uses: { on: new Set(), off: new Set() }, usedBy: { on: new Set(), off: new Set() } });
    }
    const b = boxes.get(key);
    b.members.push(m);
    if (keyOf(m.tree) === key) b.module = m;
    else b.folder = true;
  }

  const arrows = new Map();
  const shownLibs = new Map();
  for (const m of page.modules) {
    const from = boxKey(m);
    for (const target of m.uses) {
      if (libs.has(target)) {
        if (from === null) continue;
        shownLibs.set(target, libs.get(target));
        boxes.get(from).uses.on.add(target);
        arrows.set(`${from}\n${target}`, { from, to: target });
        continue;
      }
      const t = byId.get(target);
      if (!t) continue;
      const to = boxKey(t);
      if (from !== null && to !== null) {
        if (from === to) continue;
        boxes.get(from).uses.on.add(to);
        boxes.get(to).usedBy.on.add(from);
        arrows.set(`${from}\n${to}`, { from, to });
      } else if (from !== null) {
        boxes.get(from).uses.off.add(outsideKey(t));
      } else if (to !== null) {
        boxes.get(to).usedBy.off.add(outsideKey(m));
      }
    }
  }

  const counts = { red: 0, amber: 0, green: 0, unlit: 0 };
  const list = [...boxes.values()].sort((a, b) => a.key.localeCompare(b.key)).map((b) => {
    const grades = b.members.map((m) => m.grade);
    const measured = grades.filter((g) => g !== 'unlit').length;
    const label = shorten(b.name) + (b.folder ? '/ ›' : '');
    const detail = b.folder ? `${b.members.length} modules · ${measured} measured` : '';
    const grade = worstGrade(grades);
    counts[grade] += 1;
    return {
      key: b.key, name: b.name, kind: b.folder ? (b.module ? 'both' : 'folder') : 'module',
      label, detail, moduleId: b.module ? b.module.id : null, grade,
      stats: b.module && !b.folder ? b.module.stats || null : null,
      modules: b.members.length, measured,
      uses: { on: [...b.uses.on].sort(), off: [...b.uses.off].sort() },
      usedBy: { on: [...b.usedBy.on].sort(), off: [...b.usedBy.off].sort() },
      ...size(label, detail),
    };
  });
  const libraries = [...shownLibs.values()].sort((a, b) => a.id.localeCompare(b.id)).map((l) => ({
    key: l.id, id: l.id, name: l.name,
    width: Math.ceil(l.name.length * GEOMETRY.detailW + 2 * GEOMETRY.libPad), height: GEOMETRY.libH,
  }));
  return { folder: [...folder], boxes: list, libraries, arrows: [...arrows.values()], counts };
}
```

- [ ] **Step 5: Run the tests**

Run: `node --test web/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/model.js web/model.test.js
git commit -m "model.js: fold modules into boxes for a folder, merge arrows, count badges and lamps."
```

---

### Task 9: `layout.js` with ELK.js

**Files:**
- Create: `web/vendor/elk.bundled.cjs`, `web/vendor/ELK-LICENSE` (whatever name the package uses), `web/vendor/README.md`, `web/layout.js`, `web/layout.test.js`

**Interfaces:**
- Consumes: a view from `viewAt`.
- Produces: `async layout(view) → { width, height, nodes: { [key]: { x, y, w, h } }, edges: [{ from, to, points: [[x, y], …] }] }`.

- [ ] **Step 1: Vendor ELK.js**

This downloads one npm package (about 2 MB). Ask Nick before running it.

```bash
mkdir -p web/vendor && cd web/vendor
V=$(npm view elkjs version)            # the latest release; record it below
npm pack "elkjs@$V"
tar -tzf "elkjs-$V.tgz" | grep -i -E 'licen|elk.bundled'
tar -xzf "elkjs-$V.tgz" package/lib/elk.bundled.js
mv package/lib/elk.bundled.js elk.bundled.cjs
tar -xzf "elkjs-$V.tgz" "$(tar -tzf "elkjs-$V.tgz" | grep -i licen | head -1)"
mv package/LICEN* ELK-LICENSE
rm -rf package "elkjs-$V.tgz"
cd ../..
```

Then write `web/vendor/README.md`, filling in the version printed above:

```markdown
# Vendored code

- `elk.bundled.cjs`: `lib/elk.bundled.js` from elkjs <version>, renamed `.cjs` so Node
  loads it as CommonJS inside this ES-module package. Eclipse Public License 2.0,
  see `ELK-LICENSE`. To upgrade, repeat Task 9 Step 1 of docs/impl/v1-plan.md.
```

- [ ] **Step 2: Write the failing tests** `web/layout.test.js`

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt } from './model.js';
import { layout } from './layout.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));
const overlap = (a, b) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

test('every box and library gets a place and none overlap', async () => {
  const view = viewAt(page, []);
  const pos = await layout(view);
  const keys = [...view.boxes, ...view.libraries].map((n) => n.key);
  assert.deepEqual(Object.keys(pos.nodes).sort(), keys.sort());
  for (const a of keys) for (const b of keys) if (a < b) assert.ok(!overlap(pos.nodes[a], pos.nodes[b]), `${a} overlaps ${b}`);
  assert.ok(pos.width > 0 && pos.height > 0);
});

test('importers sit above what they import, and every arrow has a route', async () => {
  const view = viewAt(page, []);
  const pos = await layout(view);
  for (const e of pos.edges) {
    assert.ok(pos.nodes[e.from].y + pos.nodes[e.from].h <= pos.nodes[e.to].y, `${e.from} is not above ${e.to}`);
    assert.ok(e.points.length >= 2);
  }
});

test('an import cycle still lays out', async () => {
  const cycle = { libraries: [], modules: [
    { id: 'a', tree: ['a'], grade: 'unlit', uses: ['b'], functions: [] },
    { id: 'b', tree: ['b'], grade: 'unlit', uses: ['a'], functions: [] }] };
  const pos = await layout(viewAt(cycle, []));
  assert.equal(pos.edges.length, 2);
});

test('a crowded folder of 60 boxes and about 150 arrows lays out within 3 seconds', async () => {
  const modules = Array.from({ length: 60 }, (_, i) => ({
    id: `m${i}`, tree: [`module${String(i).padStart(2, '0')}`], grade: 'unlit', functions: [],
    uses: [1, 7, 13].map((k) => `m${(i * k + 5) % 60}`).filter((u) => u !== `m${i}`),
  }));
  const started = performance.now();
  const pos = await layout(viewAt({ libraries: [], modules }, []));
  assert.equal(Object.keys(pos.nodes).length, 60);
  assert.ok(performance.now() - started < 3000, `took ${Math.round(performance.now() - started)} ms`);
});

test('an empty folder lays out to nothing', async () => {
  const pos = await layout(viewAt({ libraries: [], modules: [] }, []));
  assert.deepEqual(pos.nodes, {});
});
```

- [ ] **Step 3: Run them to see them fail**

Run: `node --test web/`
Expected: FAIL, `Cannot find module '.../web/layout.js'`.

- [ ] **Step 4: Write** `web/layout.js`

```js
// layout.js: boxes and arrows in; positions and orthogonal arrow routes out.
// Importers sit above what they import, so arrows point down.
import ELK from './vendor/elk.bundled.cjs';

const elk = new ELK();

const OPTIONS = {
  'elk.algorithm': 'layered',
  'elk.direction': 'DOWN',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.spacing.nodeNode': '32',
  'elk.layered.spacing.nodeNodeBetweenLayers': '56',
  'elk.layered.spacing.edgeNodeBetweenLayers': '20',
};

export async function layout(view) {
  const children = [...view.boxes, ...view.libraries].map((n) => ({ id: n.key, width: n.width, height: n.height }));
  if (children.length === 0) return { width: 0, height: 0, nodes: {}, edges: [] };
  const edges = view.arrows.map((a, i) => ({ id: `e${i}`, sources: [a.from], targets: [a.to] }));
  const out = await elk.layout({ id: 'root', layoutOptions: OPTIONS, children, edges });
  const nodes = {};
  for (const c of out.children) nodes[c.id] = { x: c.x, y: c.y, w: c.width, h: c.height };
  return {
    width: out.width, height: out.height, nodes,
    edges: (out.edges || []).map((e) => ({ from: e.sources[0], to: e.targets[0], points: route(e) })),
  };
}

function route(edge) {
  const s = edge.sections && edge.sections[0];
  if (!s) return [];
  return [s.startPoint, ...(s.bendPoints || []), s.endPoint].map((p) => [p.x, p.y]);
}
```

- [ ] **Step 5: Run the tests**

Run: `node --test web/`
Expected: PASS. If the "above" test fails only for an arrow into a library oval placed on the same layer, raise `elk.layered.spacing.nodeNodeBetweenLayers` no further; instead check the arrow's direction in `viewAt` before changing the test.

- [ ] **Step 6: Commit**

```bash
git add web/vendor web/layout.js web/layout.test.js
git commit -m "layout.js: lay boxes out top-down with ELK.js and route arrows at right angles."
```

---

### Task 10: `render.js`: the diagram, the header and the card as markup

**Files:**
- Create: `web/render.js`, `web/render.test.js`

**Interfaces:**
- Consumes: `viewAt`, `GEOMETRY` from `model.js`; positions from `layout`.
- Produces: `esc(s)`, `badgeText(on, off)`, `renderDiagram(view, pos, ui) → svg string`, `renderHeader(page, view, notice, ui) → html string`, `renderCard(page, view, key) → html string`. `ui` is `{ selected: key|null, arrows: boolean }`. Click targets carry `data-key` (boxes, libraries), `data-folder` (breadcrumb), `data-open` (card's Open button), `data-toggle-arrows` (header button).

- [ ] **Step 1: Write the failing tests** `web/render.test.js`

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt } from './model.js';
import { esc, badgeText, renderDiagram, renderHeader, renderCard } from './render.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));

// Positions without ELK: boxes in a row, each arrow a straight line.
function fakePos(view) {
  const nodes = {};
  let x = 0;
  for (const n of [...view.boxes, ...view.libraries]) { nodes[n.key] = { x, y: 0, w: n.width, h: n.height }; x += n.width + 20; }
  const edges = view.arrows.map((a) => ({ from: a.from, to: a.to, points: [[nodes[a.from].x, 40], [nodes[a.to].x, 0]] }));
  return { width: x, height: 60, nodes, edges };
}
const top = viewAt(page, []);
const on = { selected: null, arrows: true };

test('esc makes markup inert', () => {
  assert.equal(esc(`<a href="x">&'`), '&lt;a href=&quot;x&quot;&gt;&amp;&#39;');
});

test('badges show on-screen partners, then outside ones, and hide zero', () => {
  assert.equal(badgeText(3, 2), '3+2');
  assert.equal(badgeText(3, 0), '3');
  assert.equal(badgeText(0, 2), '+2');
  assert.equal(badgeText(0, 0), '');
});

test('the diagram has a box per key, a lamp per box and an arrow per pair', () => {
  const svg = renderDiagram(top, fakePos(top), on);
  for (const b of top.boxes) assert.ok(svg.includes(`data-key="${b.key}"`));
  assert.ok(svg.includes('class="lamp lamp-red"'));
  assert.equal((svg.match(/class="arrow"/g) || []).length, top.arrows.length);
  assert.ok(svg.includes('checkout → pricing'));
});

test('selecting a box highlights its arrows and fades and dims the rest', () => {
  const svg = renderDiagram(top, fakePos(top), { selected: 'pricing', arrows: true });
  assert.equal((svg.match(/class="arrow hi"/g) || []).length, 2); // checkout→pricing, pricing→cart
  assert.equal((svg.match(/class="arrow faded"/g) || []).length, top.arrows.length - 2);
  assert.ok(/class="box selected" data-key="pricing"/.test(svg));
  assert.ok(/class="box dim" data-key="shop"/.test(svg));
});

test('with arrows off only the selected box keeps its arrows', () => {
  const svg = renderDiagram(top, fakePos(top), { selected: 'pricing', arrows: false });
  assert.equal((svg.match(/class="arrow/g) || []).length, 2);
  assert.equal((renderDiagram(top, fakePos(top), { selected: null, arrows: false }).match(/class="arrow/g) || []).length, 0);
});

test('names are escaped in the diagram and the card', () => {
  const evil = structuredClone(page);
  evil.modules.find((m) => m.id === 'src/pricing.ts').tree = ['<img src=x onerror=alert(1)>'];
  const view = viewAt(evil, []);
  const svg = renderDiagram(view, fakePos(view), on);
  assert.ok(!svg.includes('<img') && svg.includes('&lt;img'));
  assert.ok(!renderCard(evil, view, view.boxes.find((b) => b.name.startsWith('<')).key).includes('<img'));
});

test('an empty folder says so', () => {
  const empty = viewAt({ libraries: [], modules: [] }, []);
  assert.ok(renderDiagram(empty, { width: 0, height: 0, nodes: {}, edges: [] }, on).includes('Nothing to show'));
});

test('the header has a breadcrumb, a legend with counts, and when it was measured', () => {
  const html = renderHeader(page, viewAt(page, ['cart']), null, on);
  assert.ok(html.includes('data-folder=""') && html.includes('data-folder="cart"'));
  assert.ok(html.includes('1 medium') && html.includes('1 not measured'));
  assert.ok(html.includes('coverage 2026-09-30 19:59'));
});

test('before any coverage the legend says how to get it, and a notice shows', () => {
  const html = renderHeader({ ...page, coverageAt: '' }, top, 'Folder gone/ was removed.', on);
  assert.ok(html.includes('umlv --metrics'));
  assert.ok(html.includes('class="notice"'));
});

test('a module card spells out its grade and lists functions worst first', () => {
  const html = renderCard(page, top, 'checkout');
  assert.ok(html.includes('Grade: red (μ + σ = 30.0; red is over 12)'));
  assert.ok(html.indexOf('>run<') < html.indexOf('>neverCalled<'));
  assert.ok(html.includes('class="worst"'));
  assert.ok(html.includes('href="vscode://file/repo/src/checkout.ts:9"'));
});

test('a folder card counts what was measured and offers Open', () => {
  const html = renderCard(page, top, 'cart');
  assert.ok(html.includes('lamp-dot lamp-amber'));
  assert.ok(html.includes('1 of 2 measured'));
  assert.ok(html.includes('data-open="cart"'));
  assert.ok(html.includes('basket'));
});

test('source links encode spaces', () => {
  const spaced = structuredClone(page);
  spaced.modules.find((m) => m.id === 'src/pricing.ts').source = 'src/my file.ts';
  assert.ok(renderCard(spaced, viewAt(spaced, []), 'pricing').includes('src/my%20file.ts'));
});
```

- [ ] **Step 2: Run them to see them fail**

Run: `node --test web/`
Expected: FAIL, `Cannot find module '.../web/render.js'`.

- [ ] **Step 3: Write** `web/render.js`

```js
// render.js: positions in; SVG for the diagram and HTML for the header and
// card out, as strings. No DOM here, so all of it runs under node --test.
import { GEOMETRY as G, viewAt } from './model.js';

const ESCAPES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
export const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ESCAPES[c]);
const fmt = (n) => n.toFixed(1);
const displayName = (key) => (key.startsWith('lib:') ? key.slice(4) : key);

export function badgeText(on, off) {
  if (on && off) return `${on}+${off}`;
  if (on) return `${on}`;
  if (off) return `+${off}`;
  return '';
}

const MARKER = '<marker id="head" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" ' +
  'markerUnits="userSpaceOnUse" orient="auto"><path class="head" d="M0,0 L8,4 L0,8 z"/></marker>';
const MARGIN = 24;

export function renderDiagram(view, pos, ui) {
  if (view.boxes.length === 0) return '<p class="empty">Nothing to show here.</p>';
  const sel = ui.selected;
  const selBox = view.boxes.find((b) => b.key === sel) || null;
  const selLib = view.libraries.find((l) => l.key === sel) || null;
  const partners = new Set(selBox ? [...selBox.uses.on, ...selBox.usedBy.on]
    : selLib ? view.boxes.filter((b) => b.uses.on.includes(sel)).map((b) => b.key) : []);
  const nameOf = (key) => {
    const b = view.boxes.find((x) => x.key === key);
    return b ? b.name + (b.kind === 'module' ? '' : '/') : displayName(key);
  };
  const dim = (key) => sel && key !== sel && !partners.has(key);

  const arrows = pos.edges.map((e) => {
    const touches = sel && (e.from === sel || e.to === sel);
    if (!ui.arrows && !touches) return '';
    const cls = touches ? 'arrow hi' : sel ? 'arrow faded' : 'arrow';
    const d = 'M' + e.points.map(([x, y]) => `${x},${y}`).join(' L');
    return `<g class="edge"><title>${esc(nameOf(e.from))} → ${esc(nameOf(e.to))}</title>` +
      `<path class="hit" d="${d}"/><path class="${cls}" d="${d}" marker-end="url(#head)"/></g>`;
  }).join('');

  const boxes = view.boxes.map((b) => {
    const p = pos.nodes[b.key];
    const cls = ['box', b.kind === 'module' ? '' : 'folder', b.key === sel ? 'selected' : '', dim(b.key) ? 'dim' : '']
      .filter(Boolean).join(' ');
    const cy = G.padY + G.line / 2;
    const tx = G.padX + G.lamp + G.gap;
    const usedBy = badgeText(b.usedBy.on.length, b.usedBy.off.length);
    const uses = badgeText(b.uses.on.length, b.uses.off.length);
    return `<g class="${cls}" data-key="${esc(b.key)}" transform="translate(${p.x},${p.y})">` +
      `<rect width="${p.w}" height="${p.h}" rx="2"/>` +
      `<circle class="lamp lamp-${b.grade}" cx="${G.padX + G.lamp / 2}" cy="${cy}" r="${G.lamp / 2}"><title>${esc(lampText(b))}</title></circle>` +
      `<text class="name" x="${tx}" y="${cy + 4.5}"><title>${esc(b.name)}</title>${esc(b.label)}</text>` +
      (b.detail ? `<text class="detail" x="${tx}" y="${cy + G.line + 4}">${esc(b.detail)}</text>` : '') +
      (usedBy ? `<text class="badge" x="${p.w - 4}" y="-5" text-anchor="end"><title>${esc('used by: ' + partnerNames(b.usedBy))}</title>${usedBy}</text>` : '') +
      (uses ? `<text class="badge" x="${p.w - 4}" y="${p.h + 14}" text-anchor="end"><title>${esc('uses: ' + partnerNames(b.uses))}</title>${uses}</text>` : '') +
      '</g>';
  }).join('');

  const libs = view.libraries.map((l) => {
    const p = pos.nodes[l.key];
    const cls = ['lib', l.key === sel ? 'selected' : '', dim(l.key) ? 'dim' : ''].filter(Boolean).join(' ');
    return `<g class="${cls}" data-key="${esc(l.key)}" transform="translate(${p.x},${p.y})">` +
      `<ellipse cx="${p.w / 2}" cy="${p.h / 2}" rx="${p.w / 2}" ry="${p.h / 2}"/>` +
      `<text x="${p.w / 2}" y="${p.h / 2 + 4}" text-anchor="middle">${esc(l.name)}</text></g>`;
  }).join('');

  const vb = `${-MARGIN} ${-MARGIN} ${pos.width + 2 * MARGIN} ${pos.height + 2 * MARGIN}`;
  return `<svg xmlns="http://www.w3.org/2000/svg" class="diagram" viewBox="${vb}" preserveAspectRatio="xMidYMin meet">` +
    `<defs>${MARKER}</defs><g class="arrows">${arrows}</g>${boxes}${libs}</svg>`;
}

function lampText(b) {
  if (b.grade === 'unlit') return 'Not measured';
  if (b.stats) return `Grade: ${b.grade} (μ + σ = ${fmt(b.stats.mu + b.stats.sigma)})`;
  return `Worst measured: ${b.grade} (${b.measured} of ${b.modules} measured)`;
}

function partnerNames(side) {
  return [...side.on.map(displayName), ...side.off.map((k) => `${k} (outside)`)].join(', ');
}

const stamp = (iso) => (iso ? iso.replace('T', ' ').slice(0, 16) : '');

export function renderHeader(page, view, notice, ui) {
  const crumbs = [`<a data-folder="">${esc(page.repo)}</a>`].concat(view.folder.map((seg, i) =>
    `<a data-folder="${esc(view.folder.slice(0, i + 1).join('/'))}">${esc(seg)}/</a>`)).join(' ');
  const c = view.counts;
  const legend = page.coverageAt
    ? [['red', 'high'], ['amber', 'medium'], ['green', 'low'], ['unlit', 'not measured']]
        .map(([g, label]) => `<span><span class="lamp-dot lamp-${g}"></span>${c[g]} ${label}</span>`).join(' · ')
    : '<span><span class="lamp-dot lamp-unlit"></span>not measured: run <code>umlv --metrics</code></span>';
  const times = `scanned ${esc(stamp(page.scannedAt))}` +
    (page.coverageAt ? ` · coverage ${esc(stamp(page.coverageAt))}` : ' · no coverage yet');
  return `<nav class="crumbs">${crumbs}</nav>` +
    `<div class="legend">${legend}<span>→ imports</span>` +
    `<button data-toggle-arrows>arrows ${ui.arrows ? 'on' : 'off'}</button></div>` +
    `<div class="times">${times}</div>` +
    (notice ? `<p class="notice">${esc(notice)}</p>` : '');
}

export function renderCard(page, view, key) {
  const lib = view.libraries.find((l) => l.key === key);
  if (lib) {
    const users = view.boxes.filter((b) => b.uses.on.includes(key)).map((b) => b.key);
    return `<h2>${esc(lib.name)}</h2><p class="path">outside library</p>` + partners('Used by', { on: users, off: [] });
  }
  const b = view.boxes.find((x) => x.key === key);
  if (!b) return '';
  const m = b.moduleId ? page.modules.find((x) => x.id === b.moduleId) : null;
  let html = `<h2><span class="lamp-dot lamp-${b.grade}"></span>${esc(b.name)}${b.kind === 'module' ? '' : '/'}</h2>`;
  if (m) html += moduleSection(page, m);
  if (b.kind !== 'module') html += folderSection(page, b);
  return html + partners('Uses', b.uses) + partners('Used by', b.usedBy);
}

const link = (page, path, line) => esc(page.editorPrefix + encodeURI(path) + (line ? `:${line}` : ''));

function moduleSection(page, m) {
  let html = `<a class="path" href="${link(page, m.source)}">${esc(m.source || '.')}</a>`;
  if (!m.stats) return html + '<p>Grade: not measured</p>';
  const band = (page.bands || {})[m.grade] || '';
  html += `<p>Grade: ${m.grade} (μ + σ = ${fmt(m.stats.mu + m.stats.sigma)}; ${m.grade} is ${esc(band)})</p>`;
  html += `<p class="stats">CRAP mean ${fmt(m.stats.mu)} · spread ${fmt(m.stats.sigma)} · worst ${fmt(m.stats.max)}</p>`;
  const fns = [...m.functions].sort((a, b) => (b.crap ?? -1) - (a.crap ?? -1));
  const rows = fns.map((f, i) =>
    `<tr${i === 0 && f.crap != null ? ' class="worst"' : ''}>` +
    `<td><a href="${link(page, f.file, f.line)}">${esc(f.name)}</a></td>` +
    `<td>${f.crap != null ? fmt(f.crap) : '–'}</td><td>${f.cc}</td>` +
    `<td>${f.coverage != null ? Math.round(f.coverage * 100) + '%' : '–'}</td></tr>`).join('');
  return html + '<h3>Functions</h3><table><thead><tr><th>function</th><th>crap</th><th>cc</th><th>cov</th></tr></thead>' +
    `<tbody>${rows}</tbody></table>`;
}

function folderSection(page, b) {
  const children = viewAt(page, b.key.split('/')).boxes.filter((c) => c.key !== b.key);
  const items = children.map((c) => `<li><span class="lamp-dot lamp-${c.grade}"></span>${esc(c.label)}</li>`).join('');
  return `<p>${b.measured} of ${b.modules} measured</p><button data-open="${esc(b.key)}">Open</button>` +
    `<h3>Contents</h3><ul>${items}</ul>`;
}

function partners(title, side) {
  if (side.on.length + side.off.length === 0) return '';
  const items = side.on.map((k) => `<li>${esc(displayName(k))}</li>`).join('') +
    side.off.map((k) => `<li class="off">${esc(k)} (outside)</li>`).join('');
  return `<h3>${title}</h3><ul>${items}</ul>`;
}
```

- [ ] **Step 4: Run the tests**

Run: `node --test web/`
Expected: PASS.

- [ ] **Step 5: Update the design's dependency table**

`render.js` reads box geometry from `model.js`, `model.js` now sizes boxes, and `render.js` reads a few page-data fields itself (repo, times, editor prefix, bands, a module's functions). In `docs/design/architecture.md`:

- Code organisation: end the `model.js` row's job with "…badge counts, folder lamps and box sizes out", and set the `render.js` row's "Depends on" to "`model.js`".
- Contracts, Page data: change "One Go type owns this shape and `model.js` is its only reader." to "One Go type owns this shape; `model.js` and `render.js` are its only readers."

- [ ] **Step 6: Commit**

```bash
git add web/render.js web/render.test.js docs/design/architecture.md
git commit -m "render.js: draw the diagram, header and card as escaped markup strings."
```

---

### Task 11: Bundle the page code into `index.html`

**Files:**
- Create: `web/embed.go`, `web/index.html`, `web/style.css`, `web/app.js` (first version), `internal/page/render.go`, `internal/page/render_test.go`
- Modify: `go.mod`, `go.sum` (add `github.com/evanw/esbuild`)

**Interfaces:**
- Consumes: `facts.Page`; the page files.
- Produces: `package web` with `var FS embed.FS`; `func Render(p facts.Page) ([]byte, error)` in `internal/page`; `export function start(doc, win)` in `web/app.js`.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/evanw/esbuild@latest
```

- [ ] **Step 2: Write the failing tests** `internal/page/render_test.go`

```go
package page

import (
	"encoding/json"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func dataOf(t *testing.T, html string) facts.Page {
	t.Helper()
	const open = `<script type="application/json" id="data">`
	i := strings.Index(html, open)
	if i < 0 {
		t.Fatal("no data script")
	}
	rest := html[i+len(open):]
	var p facts.Page
	if err := json.Unmarshal([]byte(rest[:strings.Index(rest, "</script>")]), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderEmbedsTheDataAndTheBundledCode(t *testing.T) {
	out, err := Render(Build(shopScan(), shopScores(), shopOptions()))
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if p := dataOf(t, html); p.Repo != "shop" || len(p.Modules) != 5 {
		t.Fatalf("data = %+v", p)
	}
	if !strings.Contains(html, "<title>shop · umlv</title>") || !strings.Contains(html, "--charcoal: #1C1A17") {
		t.Fatal("title or styles missing")
	}
	if len(html) < 1_000_000 {
		t.Fatalf("page is %d bytes; ELK.js should make it over 1 MB", len(html))
	}
}

func TestRenderKeepsMarkupInNamesInert(t *testing.T) {
	scan := shopScan()
	scan.Modules[0].Name = `</script><script>alert(1)</script>`
	p := Build(scan, nil, shopOptions())
	p.Repo = "<b>repo</b>"
	out, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if strings.Contains(html, "<script>alert") {
		t.Fatal("a name broke out of the data script")
	}
	if !strings.Contains(html, "&lt;b&gt;repo&lt;/b&gt; · umlv") {
		t.Fatal("the title is not escaped")
	}
	if dataOf(t, html).Repo != "<b>repo</b>" {
		t.Fatal("the data no longer round-trips")
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/page/`
Expected: FAIL to compile, `undefined: Render`.

- [ ] **Step 4: Write the page files**

`web/embed.go`:

```go
// Package web holds the page: its template, styles and code, embedded so
// internal/page can bundle them into each index.html.
package web

import "embed"

//go:embed index.html style.css app.js model.js layout.js render.js vendor/elk.bundled.cjs
var FS embed.FS
```

`web/index.html`:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{TITLE}} · umlv</title>
<style>{{CSS}}</style>
</head>
<body>
<header id="header"></header>
<main id="stage"><div id="diagram"></div><aside id="card" hidden></aside></main>
<script type="application/json" id="data">{{DATA}}</script>
<script>{{JS}}</script>
</body>
</html>
```

`web/style.css` (the spec's Look section; Task 18 checks it on screen):

```css
:root {
  --charcoal: #1C1A17; --panel: #2E2924; --rule: #534C43; --arrow: #8A8070;
  --cream: #E8DCC4; --cream-dim: #A89E8C; --brass: #C9A86A;
  --lamp-green: #3B8A4E; --lamp-amber: #E0A030; --lamp-red: #E8503A; --lamp-unlit: #221F1B;
  --mono: ui-monospace, Menlo, Consolas, monospace;
}
* { box-sizing: border-box; }
html, body { margin: 0; height: 100%; background: var(--charcoal); color: var(--cream); font: 13px/1.4 var(--mono); }
body { display: flex; flex-direction: column; }
button { font: inherit; font-size: 12px; color: var(--brass); background: none; border: 1px solid var(--rule); border-radius: 2px; padding: 1px 8px; cursor: pointer; }
code { color: var(--cream); }

#header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 24px; padding: 10px 16px; background: var(--panel); border-bottom: 1px solid var(--rule); }
.crumbs a { color: var(--brass); cursor: pointer; text-decoration: none; }
.crumbs a:last-child { color: var(--cream); }
.legend { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; font-size: 12px; color: var(--cream-dim); }
.times { margin-left: auto; font-size: 12px; color: var(--cream-dim); }
.notice { flex-basis: 100%; margin: 0; font-size: 12px; color: var(--brass); }

#stage { flex: 1; display: flex; min-height: 0; }
#diagram { flex: 1; min-width: 0; overflow: hidden; cursor: grab; }
#diagram svg { display: block; width: 100%; height: 100%; }
#diagram .empty { padding: 24px; color: var(--cream-dim); }
#card { width: 380px; overflow-y: auto; padding: 16px; background: var(--panel); border-left: 1px solid var(--rule); }
#card[hidden] { display: none; }

.box rect { fill: var(--panel); stroke: var(--rule); stroke-width: 1px; vector-effect: non-scaling-stroke; }
.box.selected rect { stroke: var(--cream); stroke-width: 2px; }
.box .name { fill: var(--cream); font-size: 13px; }
.box.selected .name { font-weight: bold; }
.box .detail, .badge { fill: var(--cream-dim); font-size: 12px; }
.box, .lib { cursor: pointer; }
.dim { opacity: 0.45; }
.lib ellipse { fill: none; stroke: var(--rule); stroke-dasharray: 4 3; vector-effect: non-scaling-stroke; }
.lib.selected ellipse { stroke: var(--cream); stroke-dasharray: none; }
.lib text { fill: var(--cream-dim); font-size: 12px; }
.arrow { fill: none; stroke: var(--arrow); stroke-width: 1px; vector-effect: non-scaling-stroke; }
.arrow.hi { stroke: var(--cream); stroke-width: 1.5px; }
.arrow.faded { opacity: 0.25; }
.hit { fill: none; stroke: transparent; stroke-width: 8px; vector-effect: non-scaling-stroke; }
.head { fill: var(--arrow); }

.lamp { vector-effect: non-scaling-stroke; }
.lamp-green { fill: var(--lamp-green); }
.lamp-amber { fill: var(--lamp-amber); }
.lamp-red { fill: var(--lamp-red); stroke: var(--cream); stroke-width: 1.5px; }
.lamp-unlit { fill: var(--lamp-unlit); stroke: var(--rule); stroke-width: 1px; }
.lamp-dot { display: inline-block; width: 10px; height: 10px; margin-right: 5px; border-radius: 50%; vertical-align: -1px; }
.lamp-dot.lamp-green { background: var(--lamp-green); }
.lamp-dot.lamp-amber { background: var(--lamp-amber); }
.lamp-dot.lamp-red { background: var(--lamp-red); box-shadow: 0 0 0 1.5px var(--cream); }
.lamp-dot.lamp-unlit { background: var(--lamp-unlit); box-shadow: inset 0 0 0 1px var(--rule); }

#card h2 { margin: 0 0 4px; font-size: 15px; font-weight: bold; }
#card h3 { margin: 18px 0 6px; font-size: 11px; font-weight: normal; letter-spacing: 0.08em; text-transform: uppercase; color: var(--brass); }
#card p { margin: 6px 0; }
#card a { color: var(--cream); }
#card .path, #card .stats { font-size: 12px; color: var(--cream-dim); }
#card table { width: 100%; border-collapse: collapse; font-size: 12px; }
#card th { font-weight: normal; text-align: right; color: var(--brass); border-bottom: 1px solid var(--rule); }
#card td { padding: 2px 0 2px 8px; text-align: right; border-bottom: 1px solid var(--rule); }
#card th:first-child, #card td:first-child { padding-left: 0; text-align: left; }
#card tr.worst td:first-child a { color: var(--brass); }
#card ul { margin: 0; padding: 0; list-style: none; }
#card li.off { color: var(--cream-dim); }
```

`web/app.js` (first version: draw a folder, open folders, follow the breadcrumb; Task 17 replaces it):

```js
// app.js: wiring between the page's DOM and the model, layout and render.
import { viewAt } from './model.js';
import { layout } from './layout.js';
import { renderDiagram, renderHeader } from './render.js';

export function start(doc) {
  const page = JSON.parse(doc.getElementById('data').textContent);
  const ui = { folder: [], selected: null, arrows: true };
  const draw = async () => {
    const view = viewAt(page, ui.folder);
    const pos = await layout(view);
    doc.getElementById('header').innerHTML = renderHeader(page, view, null, ui);
    doc.getElementById('diagram').innerHTML = renderDiagram(view, pos, ui);
  };
  doc.addEventListener('dblclick', (e) => {
    const box = e.target.closest('.box.folder');
    if (box) { ui.folder = box.dataset.key.split('/'); draw(); }
  });
  doc.addEventListener('click', (e) => {
    const crumb = e.target.closest('[data-folder]');
    if (crumb) { ui.folder = crumb.dataset.folder ? crumb.dataset.folder.split('/') : []; draw(); }
  });
  draw();
}
```

- [ ] **Step 5: Write** `internal/page/render.go`

```go
package page

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/web"
)

const entry = "import { start } from './app.js'; start(document, window);"

// Render writes the page: the template with the styles, the page data and
// the bundled page code inside it.
func Render(p facts.Page) ([]byte, error) {
	data, err := json.Marshal(p) // escapes <, > and &, so no name can close the script tag
	if err != nil {
		return nil, err
	}
	js, err := bundle()
	if err != nil {
		return nil, err
	}
	css, err := fs.ReadFile(web.FS, "style.css")
	if err != nil {
		return nil, err
	}
	tmpl, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		return nil, err
	}
	fill := strings.NewReplacer( // one pass: inserted text is never re-scanned
		"{{TITLE}}", html.EscapeString(p.Repo),
		"{{CSS}}", string(css),
		"{{DATA}}", string(data),
		"{{JS}}", strings.ReplaceAll(js, "</script", `<\/script`),
	)
	return []byte(fill.Replace(string(tmpl))), nil
}

// bundle joins app.js and everything it imports into one script, reading
// the files from the embedded web.FS rather than from disk.
func bundle() (string, error) {
	res := api.Build(api.BuildOptions{
		Stdin:             &api.StdinOptions{Contents: entry, ResolveDir: "/", Sourcefile: "entry.js", Loader: api.LoaderJS},
		Bundle:            true,
		Write:             false,
		Format:            api.FormatIIFE,
		Target:            api.ES2020,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		Plugins:           []api.Plugin{embedded(web.FS)},
	})
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("bundling the page code: %s", res.Errors[0].Text)
	}
	return string(res.OutputFiles[0].Contents), nil
}

func embedded(files fs.FS) api.Plugin {
	return api.Plugin{Name: "embedded", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `^\.\.?/`}, func(a api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: path.Join(a.ResolveDir, a.Path), Namespace: "web"}, nil
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "web"}, func(a api.OnLoadArgs) (api.OnLoadResult, error) {
			src, err := fs.ReadFile(files, strings.TrimPrefix(a.Path, "/"))
			if err != nil {
				return api.OnLoadResult{}, err
			}
			text := string(src)
			return api.OnLoadResult{Contents: &text, ResolveDir: path.Dir(a.Path), Loader: api.LoaderJS}, nil
		})
	}}
}
```

- [ ] **Step 6: Run the tests**

Run: `gofmt -w internal/page && go test ./internal/page/ && node --test web/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum web internal/page
git commit -m "Bundle the page code with esbuild into a self-contained index.html."
```

---

### Task 12: The `umlv` command, and a first look

**Files:**
- Create: `cmd/umlv/main.go`, `cmd/umlv/main_test.go`

**Interfaces:**
- Consumes: `lang.Detect`, `lang.ByName`, the three `Lang` values, `policy.*`, `page.Build`, `page.Render`.
- Produces: `func run(args []string, stdout, stderr io.Writer, open func(string) error, now func() time.Time) int`; the binary `umlv`.

- [ ] **Step 1: Write the failing tests** `cmd/umlv/main_test.go`

```go
package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uml-viewer-neo/internal/facts"
)

func copyShop(t *testing.T) string {
	t.Helper()
	src := "../../internal/lang/golang/testdata/shop"
	dst := filepath.Join(t.TempDir(), "my shop") // a space on purpose
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

var fixed = func() time.Time { return time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC) }

func umlv(t *testing.T, args ...string) (code int, stdout, stderr string, opened []string) {
	t.Helper()
	var out, errOut bytes.Buffer
	open := func(p string) error { opened = append(opened, p); return nil }
	code = run(args, &out, &errOut, open, fixed)
	return code, out.String(), errOut.String(), opened
}

func TestScansARepoAndWritesThePageAndData(t *testing.T) {
	dir := copyShop(t)
	code, out, errOut, opened := umlv(t, "--no-open", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, f := range []string{".umlv/index.html", ".umlv/data.json", ".umlv/policy.toml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s missing", f)
		}
	}
	var p facts.Page
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/data.json"))
	if err := json.Unmarshal(b, &p); err != nil || len(p.Modules) != 2 || p.Repo != "my shop" || p.ScannedAt != "2026-09-30T20:00:00Z" {
		t.Fatalf("data = %+v, err = %v", p, err)
	}
	if !strings.Contains(out, "2 modules, none measured") || !strings.Contains(out, ".umlv/index.html") {
		t.Fatalf("summary = %q", out)
	}
	if len(opened) != 0 {
		t.Fatal("opened a browser despite --no-open")
	}
}

func TestOpensThePageByDefault(t *testing.T) {
	dir := copyShop(t)
	if code, _, errOut, opened := umlv(t, dir); code != 0 || len(opened) != 1 || !strings.HasSuffix(opened[0], "index.html") {
		t.Fatalf("exit %d, opened %v: %s", code, opened, errOut)
	}
}

func TestKeepsAnEditedPolicy(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("editor = \"cursor\"\n"), 0o644)
	if code, _, errOut, _ := umlv(t, "--no-open", dir); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/policy.toml"))
	if string(b) != "editor = \"cursor\"\n" {
		t.Fatal("umlv overwrote the policy")
	}
}

func TestStopsWhenNoLanguageIsRecognised(t *testing.T) {
	code, _, errOut, _ := umlv(t, "--no-open", t.TempDir())
	if code != 1 || !strings.Contains(errOut, "--lang") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestRejectsAnUnknownLanguage(t *testing.T) {
	if code, _, errOut, _ := umlv(t, "--lang", "cobol", t.TempDir()); code != 2 || !strings.Contains(errOut, "cobol") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestStopsOnABrokenPolicyWithItsLine(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("editor = \"vscode\"\nlibraries = [\n"), 0o644)
	if code, _, errOut, _ := umlv(t, "--no-open", dir); code != 1 || !strings.Contains(errOut, "policy.toml:") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./cmd/umlv/`
Expected: FAIL to compile, `undefined: run`.

- [ ] **Step 3: Write** `cmd/umlv/main.go`

Keep every function small: Task 20 holds this repo to its own grades, and a long `run()` would grade red even fully tested.

```go
// Command umlv scans a Go, TypeScript or Python repo and writes one HTML
// page of its modules, imports and grades to <repo>/.umlv/index.html, with
// the same data in .umlv/data.json.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
	"uml-viewer-neo/internal/lang/golang"
	"uml-viewer-neo/internal/lang/python"
	"uml-viewer-neo/internal/lang/typescript"
	"uml-viewer-neo/internal/page"
	"uml-viewer-neo/internal/policy"
)

// languages, in the order detection tries them.
var languages = []lang.Language{golang.Lang, python.Lang, typescript.Lang}

const usage = "usage: umlv [--metrics] [--lang go|typescript|python] [--no-open] [repo]"

type options struct {
	root, lang      string
	metrics, noOpen bool
}

// usageError is a mistake on the command line, exit code 2. msg is empty
// when the flag package has already explained it.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, openBrowser, time.Now))
}

func run(args []string, stdout, stderr io.Writer, open func(string) error, now func() time.Time) int {
	o, err := parseArgs(args, stderr)
	if err != nil {
		return fail(err, stderr)
	}
	out, p, err := generate(o, stderr, now)
	if err != nil {
		return fail(err, stderr)
	}
	fmt.Fprintln(stdout, summary(p, out))
	openPage(o, out, open, stderr)
	return 0
}

func parseArgs(args []string, stderr io.Writer) (options, error) {
	var o options
	flags := flag.NewFlagSet("umlv", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, usage); flags.PrintDefaults() }
	flags.BoolVar(&o.metrics, "metrics", false, "run the repo's tests with coverage first")
	flags.StringVar(&o.lang, "lang", "", "go, typescript or python; overrides detection")
	flags.BoolVar(&o.noOpen, "no-open", false, "write the page without opening a browser")
	if err := flags.Parse(args); err != nil {
		return o, usageError{}
	}
	if flags.NArg() > 1 {
		return o, usageError{usage}
	}
	root, err := filepath.Abs(flags.Arg(0)) // "" means the current directory
	if err != nil {
		return o, err
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return o, fmt.Errorf("%s is not a directory", root)
	}
	o.root = root
	return o, nil
}

func fail(err error, stderr io.Writer) int {
	var u usageError
	if errors.As(err, &u) {
		if u.msg != "" {
			fmt.Fprintln(stderr, "umlv:", u.msg)
		}
		return 2
	}
	fmt.Fprintln(stderr, "umlv:", explain(err))
	return 1
}

// generate scans the repo, applies its policy and writes the page.
func generate(o options, stderr io.Writer, now func() time.Time) (string, facts.Page, error) {
	l, err := pickLanguage(o.root, o.lang)
	if err != nil {
		return "", facts.Page{}, err
	}
	scan, err := l.Scan(o.root)
	if err != nil {
		return "", facts.Page{}, err
	}
	for _, n := range scan.Notes {
		fmt.Fprintln(stderr, "umlv: note:", n)
	}
	pol, err := loadPolicy(o.root, scan)
	if err != nil {
		return "", facts.Page{}, err
	}
	p := page.Build(scan, facts.Scores{}, page.Options{
		Repo: filepath.Base(o.root), ScannedAt: now().UTC().Format(time.RFC3339),
		EditorPrefix: pol.EditorPrefix(o.root), Libraries: pol.Libraries,
	})
	out, err := write(o.root, p)
	return out, p, err
}

func pickLanguage(root, name string) (lang.Language, error) {
	if name != "" {
		l, err := lang.ByName(name, languages)
		if err != nil {
			return l, usageError{err.Error()}
		}
		return l, nil
	}
	l, err := lang.Detect(root, languages)
	if err != nil {
		return l, fmt.Errorf("%w; pass --lang to choose one", err)
	}
	return l, nil
}

// loadPolicy reads the repo's policy, writing the default on a first scan.
func loadPolicy(root string, scan facts.Scan) (policy.Policy, error) {
	p, ok, err := policy.Load(root)
	if err != nil || ok {
		return p, err
	}
	p = policy.Default(scan)
	return p, policy.Write(root, p)
}

var installHints = map[string]string{
	"go":      "install Go from https://go.dev/dl",
	"node":    "install Node.js from https://nodejs.org",
	"python3": "install Python 3 from https://www.python.org/downloads",
}

// explain adds an install hint when the error is a missing tool.
func explain(err error) string {
	var missing *facts.MissingToolError
	if errors.As(err, &missing) && installHints[missing.Tool] != "" {
		return fmt.Sprintf("%v; %s", err, installHints[missing.Tool])
	}
	return err.Error()
}

// write puts index.html and data.json in root's .umlv/ and returns the page's path.
func write(root string, p facts.Page) (string, error) {
	dir := filepath.Join(root, ".umlv")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	html, err := page.Render(p)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	out := filepath.Join(dir, "index.html")
	return out, os.WriteFile(out, html, 0o644)
}

func summary(p facts.Page, out string) string {
	n := map[facts.Grade]int{}
	for _, m := range p.Modules {
		n[m.Grade]++
	}
	if n[facts.Unlit] == len(p.Modules) {
		return fmt.Sprintf("umlv: %d modules, none measured (run with --metrics) → %s", len(p.Modules), out)
	}
	return fmt.Sprintf("umlv: %d modules: %d red, %d amber, %d green, %d unlit → %s",
		len(p.Modules), n[facts.Red], n[facts.Amber], n[facts.Green], n[facts.Unlit], out)
}

func openPage(o options, out string, open func(string) error, stderr io.Writer) {
	if o.noOpen {
		return
	}
	if err := open(out); err != nil {
		fmt.Fprintf(stderr, "umlv: could not open a browser (%v); open %s yourself\n", err, out)
	}
}

func openBrowser(path string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, path).Start()
}
```

- [ ] **Step 4: Run the tests**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: First look**

```bash
go run ./cmd/umlv --no-open ../spaced-repetition
```

Expected: a summary line like `umlv: 60 modules, none measured (run with --metrics) → …/spaced-repetition/.umlv/index.html`. Open that file in the browser pane (`file://` URL) and check: the top level shows boxes for `routes/ ›`, `tutor/ ›`, `db` and the rest; arrows point down; double-clicking `routes/ ›` opens it; the breadcrumb goes back. Save a screenshot to `docs/impl/screens/first-look.png`. This is a smoke check, not the visual review (Task 19).

- [ ] **Step 6: Commit**

```bash
git add cmd docs/impl/screens
git commit -m "Add the umlv command: scan, keep the policy, write index.html and data.json, open the page."
```

---

### Task 13: CRAP, stats and grades

**Files:**
- Create: `internal/metrics/metrics.go`, `internal/metrics/metrics_test.go`

**Interfaces:**
- Consumes: `facts.Function`, `facts.Scan`, `facts.Stats`, `facts.Grade`, `facts.Scores`.
- Produces: `const NoCol = -1`; `type Unit struct { File string; Line, Col, Weight int; Hit bool }`; `type Coverage struct { Files map[string]bool; Units []Unit }`; `type Command struct { Args, Env []string; Report string }`; `var Bands map[string]string`; `func CRAP(cc int, coverage float64) float64`; `func FunctionCoverage(f facts.Function, units []Unit) float64`; `func StatsOf(crap []float64) *facts.Stats`; `func GradeOf(s *facts.Stats) facts.Grade`; `func Score(scan facts.Scan, cov *Coverage) facts.Scores`.

- [ ] **Step 1: Write the failing tests** `internal/metrics/metrics_test.go`

```go
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
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/metrics/`
Expected: FAIL to compile, `undefined: CRAP`.

- [ ] **Step 3: Write** `internal/metrics/metrics.go`

```go
// Package metrics turns coverage into scores: CRAP for each function, and
// for each module the mean, spread, worst and grade. The grade cut-offs live
// here and nowhere else.
package metrics

import (
	"math"

	"uml-viewer-neo/internal/facts"
)

// NoCol marks a unit from a report that gives no column.
const NoCol = -1

// Unit is one piece of a coverage report: a Go block, an istanbul statement
// or a coverage.py line. Weight is how many statements it stands for.
type Unit struct {
	File         string
	Line, Col    int
	Weight       int
	Hit          bool
}

// Coverage is one report: the files it covered, and their units. A file
// missing from Files was not measured at all.
type Coverage struct {
	Files map[string]bool
	Units []Unit
}

// Command runs a repo's tests with coverage. {report} in Args becomes the
// report's absolute path; Report is relative to the repo root.
type Command struct {
	Args, Env []string
	Report    string
}

const greenMax, amberMax = 8.0, 12.0

// Bands describes each grade's cut-off on μ + σ, for the card.
var Bands = map[string]string{"green": "8 or less", "amber": "over 8, up to 12", "red": "over 12"}

// CRAP is CC² × (1 − coverage)³ + CC, with coverage from 0 to 1.
func CRAP(cc int, coverage float64) float64 {
	c, miss := float64(cc), 1-coverage
	return c + c*c*miss*miss*miss
}

// covers says whether u belongs to f's body. Coverage before the body's
// start (a def line, the `const f =` of an arrow function) ran at import.
func covers(f facts.Function, u Unit) bool {
	from, col := f.CoverStart, f.CoverCol
	if from == 0 {
		from, col = f.Start, 0
	}
	if u.File != f.File || u.Line > f.End {
		return false
	}
	return u.Line > from || (u.Line == from && (u.Col == NoCol || u.Col >= col))
}

// FunctionCoverage is the weighted share of f's body that ran. A function
// with nothing to cover counts as covered.
func FunctionCoverage(f facts.Function, units []Unit) float64 {
	total, hit := 0, 0
	for _, u := range units {
		if covers(f, u) {
			total += u.Weight
			if u.Hit {
				hit += u.Weight
			}
		}
	}
	if total == 0 {
		return 1
	}
	return float64(hit) / float64(total)
}

// StatsOf is the mean, population standard deviation and worst of scores.
func StatsOf(crap []float64) *facts.Stats {
	if len(crap) == 0 {
		return nil
	}
	sum, worst := 0.0, crap[0]
	for _, c := range crap {
		sum += c
		worst = math.Max(worst, c)
	}
	mu := sum / float64(len(crap))
	sq := 0.0
	for _, c := range crap {
		sq += (c - mu) * (c - mu)
	}
	return &facts.Stats{Mu: mu, Sigma: math.Sqrt(sq / float64(len(crap))), Max: worst}
}

// GradeOf reads μ + σ against the cut-offs.
func GradeOf(s *facts.Stats) facts.Grade {
	switch {
	case s == nil:
		return facts.Unlit
	case s.Mu+s.Sigma <= greenMax:
		return facts.Green
	case s.Mu+s.Sigma <= amberMax:
		return facts.Amber
	default:
		return facts.Red
	}
}

// Score measures every function whose file is in the report. A module with
// no measured function is left out, which leaves its lamp unlit.
func Score(scan facts.Scan, cov *Coverage) facts.Scores {
	scores := facts.Scores{}
	if cov == nil {
		return scores
	}
	byFile := map[string][]Unit{}
	for _, u := range cov.Units {
		byFile[u.File] = append(byFile[u.File], u)
	}
	for _, m := range scan.Modules {
		fns := map[string]facts.FunctionScore{}
		var craps []float64
		for _, f := range m.Functions {
			if !cov.Files[f.File] {
				continue
			}
			c := FunctionCoverage(f, byFile[f.File])
			fns[f.Name] = facts.FunctionScore{Coverage: c, CRAP: CRAP(f.CC, c)}
			craps = append(craps, CRAP(f.CC, c))
		}
		if len(craps) > 0 {
			stats := StatsOf(craps)
			scores[m.ID] = facts.ModuleScore{Grade: GradeOf(stats), Stats: stats, Functions: fns}
		}
	}
	return scores
}
```

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/metrics && go test ./internal/metrics/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics
git commit -m "Compute CRAP, module stats and grades; files outside the report stay unmeasured."
```

---

### Task 14: Running a coverage command

**Files:**
- Create: `internal/metrics/run.go`, `internal/metrics/run_test.go`

**Interfaces:**
- Consumes: `Command`, `facts.MissingToolError`.
- Produces: `func Run(root string, c Command, out io.Writer) error`. A failing test run returns `*exec.ExitError`; a missing program returns `*facts.MissingToolError`.

- [ ] **Step 1: Write the failing tests** `internal/metrics/run_test.go`

```go
package metrics

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func TestRunWritesTheReportWhereTheCommandIsTold(t *testing.T) {
	root := t.TempDir()
	c := Command{Args: []string{"sh", "-c", "echo hi > {report}"}, Report: ".umlv/raw/out.txt"}
	if err := Run(root, c, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".umlv/raw/out.txt")); string(b) != "hi\n" {
		t.Fatalf("report = %q", b)
	}
}

func TestRunRemovesAStaleReportFirst(t *testing.T) {
	root := t.TempDir()
	report := filepath.Join(root, ".umlv/raw/out.txt")
	os.MkdirAll(filepath.Dir(report), 0o755)
	os.WriteFile(report, []byte("old"), 0o644)
	if err := Run(root, Command{Args: []string{"true"}, Report: ".umlv/raw/out.txt"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(report); err == nil {
		t.Fatal("a stale report would pass for a fresh one")
	}
}

func TestRunReturnsTheExitErrorAndKeepsWhatWasWritten(t *testing.T) {
	root := t.TempDir()
	c := Command{Args: []string{"sh", "-c", "echo partial > {report}; exit 3"}, Report: "r.txt"}
	err := Run(root, c, &bytes.Buffer{})
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "r.txt")); err != nil {
		t.Fatal("the partial report is gone")
	}
}

func TestRunAddsTheEnvironmentAndStreamsOutput(t *testing.T) {
	var out bytes.Buffer
	c := Command{Args: []string{"sh", "-c", `echo "$UMLV_TEST"`}, Env: []string{"UMLV_TEST=yes"}, Report: "r.txt"}
	if err := Run(t.TempDir(), c, &out); err != nil || !strings.Contains(out.String(), "yes") {
		t.Fatalf("out = %q, err = %v", out.String(), err)
	}
}

func TestRunNamesAMissingProgram(t *testing.T) {
	err := Run(t.TempDir(), Command{Args: []string{"umlv-no-such-tool"}, Report: "r.txt"}, &bytes.Buffer{})
	var missing *facts.MissingToolError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/metrics/`
Expected: FAIL to compile, `undefined: Run`.

- [ ] **Step 3: Write** `internal/metrics/run.go`

```go
package metrics

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"uml-viewer-neo/internal/facts"
)

// Run runs c in root, streaming the tool's output to out. It removes any old
// report first, so a run that writes nothing cannot pass off a stale one. A
// failing test run comes back as *exec.ExitError, and whatever report it
// wrote is kept.
func Run(root string, c Command, out io.Writer) error {
	if len(c.Args) == 0 {
		return errors.New("no coverage command")
	}
	if _, err := exec.LookPath(c.Args[0]); err != nil {
		return &facts.MissingToolError{Tool: c.Args[0]}
	}
	report := filepath.Join(root, c.Report)
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		return err
	}
	if err := os.Remove(report); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = strings.ReplaceAll(a, "{report}", report)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), c.Env...)
	cmd.Stdout, cmd.Stderr = out, out
	return cmd.Run()
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/metrics/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics
git commit -m "Run a repo's coverage command, clearing any stale report first."
```

---

### Task 15: Each language's coverage command and report reader

**Files:**
- Modify: `internal/lang/lang.go` (three fields), `internal/lang/golang/golang.go`, `internal/lang/python/python.go`, `internal/lang/typescript/typescript.go`
- Create: `internal/lang/golang/coverage.go`, `internal/lang/golang/coverage_test.go`, `internal/lang/golang/testdata/cover.out`, `internal/lang/python/coverage.go`, `internal/lang/python/coverage_test.go`, `internal/lang/python/testdata/coverage.json`, `internal/lang/typescript/coverage.go`, `internal/lang/typescript/coverage_test.go`, `internal/lang/typescript/testdata/coverage-final.json`

**Interfaces:**
- Consumes: `metrics.Command`, `metrics.Coverage`, `metrics.Unit`, `metrics.NoCol`, `metrics.Score`.
- Produces: `Language` gains `Coverage func(root string) metrics.Command`, `Read func(report []byte, root string, scan facts.Scan) (metrics.Coverage, error)`, `CoverageHint string`. Readers: `golang.ReadProfile(data []byte, prefix string) (metrics.Coverage, error)`, `python.ReadCoverageJSON(data []byte) (metrics.Coverage, error)`, `typescript.ReadIstanbul(data []byte, root string) (metrics.Coverage, error)`.

- [ ] **Step 1: Add the fields** to `Language` in `internal/lang/lang.go` (and import `uml-viewer-neo/internal/metrics`):

```go
type Language struct {
	Name    string
	Markers []string
	Scan    func(root string) (facts.Scan, error)
	// Coverage is the command that runs the repo's tests and writes a report.
	Coverage func(root string) metrics.Command
	// Read turns that report into coverage units.
	Read func(report []byte, root string, scan facts.Scan) (metrics.Coverage, error)
	// CoverageHint says what to install when no report appears.
	CoverageHint string
}
```

- [ ] **Step 2: Write the fixtures**

`internal/lang/golang/testdata/cover.out` is a real profile from `go test ./... -coverpkg=./... -coverprofile` on the sample repo. Note each block appears once per test binary:

```
mode: set
github.com/acme/shop/shop.go:11.2,12.27 2 1
github.com/acme/shop/shop.go:13.3,13.15 1 1
github.com/acme/shop/shop.go:14.4,15.1 1 1
github.com/acme/shop/shop.go:17.2,17.22 1 1
github.com/acme/shop/internal/cart/cart.go:7.20,7.36 1 1
github.com/acme/shop/internal/cart/cart.go:10.2,11.1 1 1
github.com/acme/shop/internal/cart/cart.go:13.27,13.48 1 1
github.com/acme/shop/internal/cart/cart.go:15.29,15.55 1 0
github.com/acme/shop/shop.go:11.2,12.27 2 0
github.com/acme/shop/shop.go:13.3,13.15 1 0
github.com/acme/shop/shop.go:14.4,15.1 1 0
github.com/acme/shop/shop.go:17.2,17.22 1 0
github.com/acme/shop/internal/cart/cart.go:7.20,7.36 1 1
github.com/acme/shop/internal/cart/cart.go:10.2,11.1 1 1
github.com/acme/shop/internal/cart/cart.go:13.27,13.48 1 0
github.com/acme/shop/internal/cart/cart.go:15.29,15.55 1 1
```

`internal/lang/python/testdata/coverage.json`, in coverage.py's JSON format (version 3):

```json
{"meta": {"format": 3, "version": "7.6.1"},
 "files": {
  "src/shop/pricing.py": {"executed_lines": [1, 4, 6], "missing_lines": [], "excluded_lines": [],
                          "summary": {"covered_lines": 3, "num_statements": 3}},
  "src/shop/checkout.py": {"executed_lines": [1, 2, 4, 15, 16, 17, 18, 20, 23], "missing_lines": [19, 24], "excluded_lines": [],
                           "summary": {"covered_lines": 9, "num_statements": 11}}},
 "totals": {"covered_lines": 12, "num_statements": 14}}
```

`internal/lang/typescript/testdata/coverage-final.json`, in istanbul's format. Paths are absolute; the test passes `/ROOT` as the repo root:

```json
{"/ROOT/src/pricing.ts": {"path": "/ROOT/src/pricing.ts",
  "statementMap": {
   "0": {"start": {"line": 3, "column": 0}, "end": {"line": 5, "column": 1}},
   "1": {"start": {"line": 4, "column": 2}, "end": {"line": 4, "column": 22}},
   "2": {"start": {"line": 7, "column": 0}, "end": {"line": 12, "column": 2}},
   "3": {"start": {"line": 9, "column": 4}, "end": {"line": 9, "column": 21}},
   "4": {"start": {"line": 11, "column": 35}, "end": {"line": 11, "column": 40}}},
  "s": {"0": 1, "1": 3, "2": 1, "3": 0, "4": 1},
  "fnMap": {}, "f": {}, "branchMap": {}, "b": {}},
 "/elsewhere/lib.ts": {"path": "/elsewhere/lib.ts",
  "statementMap": {"0": {"start": {"line": 1, "column": 0}, "end": {"line": 1, "column": 5}}},
  "s": {"0": 1}, "fnMap": {}, "f": {}, "branchMap": {}, "b": {}}}
```

- [ ] **Step 3: Write the failing tests**

`internal/lang/golang/coverage_test.go`:

```go
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
```

`internal/lang/python/coverage_test.go`:

```go
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
	if len(cov.Files) != 2 || len(cov.Units) != 14 {
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
	if run := checkout.Functions["run"]; run.Coverage != 0.8 { // the def line is not the body
		t.Fatalf("run = %+v", run)
	}
	if never := checkout.Functions["never_called"]; never.Coverage != 0 || never.CRAP != 2 {
		t.Fatalf("never_called = %+v", never)
	}
	if _, measured := metrics.Score(scan, &cov)["src/shop/cart/basket.py"]; measured {
		t.Fatal("basket.py is not in the report")
	}
}
```

`internal/lang/typescript/coverage_test.go`:

```go
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
	if len(cov.Files) != 1 || !cov.Files["src/pricing.ts"] || len(cov.Units) != 5 {
		t.Fatalf("files = %v, %d units", cov.Files, len(cov.Units))
	}
	if u := cov.Units[4]; u.Line != 11 || u.Col != 35 || !u.Hit {
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
```

- [ ] **Step 4: Run them to see them fail**

Run: `go test ./internal/lang/...`
Expected: FAIL to compile, `undefined: ReadProfile` (and the other two readers).

- [ ] **Step 5: Write the readers and commands**

`internal/lang/golang/coverage.go`:

```go
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
```

and change `Lang` in `internal/lang/golang/golang.go` to:

```go
var Lang = lang.Language{
	Name: "go", Markers: []string{"go.mod"}, Scan: Scan,
	Coverage: coverageCommand, Read: readCoverage,
	CoverageHint: "Go needs nothing extra; check that `go test ./...` passes.",
}
```

`internal/lang/python/coverage.go`:

```go
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
```

and change `Lang` in `internal/lang/python/python.go` to:

```go
var Lang = lang.Language{
	Name: "python", Markers: []string{"pyproject.toml", "setup.py"}, Scan: Scan,
	Coverage: coverageCommand, Read: readCoverage,
	CoverageHint: "pytest needs pytest-cov installed in the repo's Python environment (pip install pytest-cov).",
}
```

`internal/lang/typescript/coverage.go`:

```go
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
```

and change `Lang` in `internal/lang/typescript/typescript.go` to:

```go
var Lang = lang.Language{
	Name: "typescript", Markers: []string{"tsconfig.json", "package.json"}, Scan: Scan,
	Coverage: coverageCommand, Read: readCoverage,
	CoverageHint: "vitest needs @vitest/coverage-v8 installed (npm install -D @vitest/coverage-v8); jest has coverage built in.",
}
```

- [ ] **Step 6: Run the tests**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/lang
git commit -m "Give each language its coverage command and report reader."
```

---

### Task 16: `--metrics`, and reusing the last report

**Files:**
- Modify: `cmd/umlv/main.go`, `cmd/umlv/main_test.go`

**Interfaces:**
- Consumes: `Language.Coverage`, `Language.Read`, `Language.CoverageHint`, `metrics.Run`, `metrics.Score`, `metrics.Bands`, `policy.Policy.Coverage`.
- Produces: `--metrics` behaviour; `CoverageAt` and `Bands` in the page data; graded summary lines.

- [ ] **Step 1: Write the failing tests** (append to `cmd/umlv/main_test.go`)

```go
func readPage(t *testing.T, dir string) facts.Page {
	t.Helper()
	var p facts.Page
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/data.json"))
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMetricsRunsTheTestsAndLightsTheLamps(t *testing.T) {
	dir := copyShop(t)
	code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "2 modules: 0 red, 0 amber, 2 green, 0 unlit") {
		t.Fatalf("summary = %q", out)
	}
	p := readPage(t, dir)
	if p.CoverageAt == "" || p.Bands["red"] != "over 12" {
		t.Fatalf("coverageAt = %q, bands = %v", p.CoverageAt, p.Bands)
	}
}

func TestAPlainRunReusesTheLastReport(t *testing.T) {
	dir := copyShop(t)
	umlv(t, "--metrics", "--no-open", dir)
	if _, out, _, _ := umlv(t, "--no-open", dir); !strings.Contains(out, "2 green") {
		t.Fatalf("summary = %q", out)
	}
}

func TestFailingTestsWarnButStillWriteThePage(t *testing.T) {
	dir := copyShop(t)
	os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte("package shop\n\nimport \"testing\"\n\nfunc TestBad(t *testing.T) { t.Fatal(\"no\") }\n"), 0o644)
	code, _, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 || !strings.Contains(errOut, "tests exited with code") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestNoReportWarnsWithTheLanguagesHint(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("[coverage]\ncommand = [\"true\"]\n"), 0o644)
	code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 || !strings.Contains(errOut, "no coverage report") || !strings.Contains(errOut, "Go needs nothing extra") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "none measured") {
		t.Fatalf("summary = %q", out)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./cmd/umlv/`
Expected: FAIL: `TestMetricsRunsTheTestsAndLightsTheLamps` sees "none measured".

- [ ] **Step 3: Wire coverage into** `cmd/umlv/main.go`

Add `"strings"` and `"uml-viewer-neo/internal/metrics"` to the imports. In `generate`, replace the `page.Build(...)` call with:

```go
	scores, coverageAt := coverage(o, l, pol, scan, stderr)
	p := page.Build(scan, scores, page.Options{
		Repo: filepath.Base(o.root), ScannedAt: now().UTC().Format(time.RFC3339), CoverageAt: coverageAt,
		EditorPrefix: pol.EditorPrefix(o.root), Libraries: pol.Libraries, Bands: metrics.Bands,
	})
```

Add these functions:

```go
// coverage returns the scores, and when their report was written: from a
// fresh test run with --metrics, else from the last report on disk.
// Problems only warn; the lamps stay unlit.
func coverage(o options, l lang.Language, pol policy.Policy, scan facts.Scan, stderr io.Writer) (facts.Scores, string) {
	c := coverageCommand(l, pol, o.root)
	if o.metrics {
		runTests(o.root, c, stderr)
	}
	cov, at, err := readReport(o.root, l, c, scan)
	if err != nil {
		if o.metrics {
			fmt.Fprintf(stderr, "umlv: %v. %s\n", err, l.CoverageHint)
		}
		return facts.Scores{}, ""
	}
	return metrics.Score(scan, &cov), at
}

// coverageCommand is the language's command, with the policy's overrides.
func coverageCommand(l lang.Language, pol policy.Policy, root string) metrics.Command {
	c := l.Coverage(root)
	if len(pol.Coverage.Command) > 0 {
		c.Args = pol.Coverage.Command
	}
	if pol.Coverage.Report != "" {
		c.Report = pol.Coverage.Report
	}
	return c
}

func runTests(root string, c metrics.Command, stderr io.Writer) {
	if len(c.Args) == 0 {
		fmt.Fprintln(stderr, "umlv: no known way to run this repo's tests; set [coverage] in .umlv/policy.toml")
		return
	}
	fmt.Fprintln(stderr, "umlv: running", strings.Join(c.Args, " "))
	var exit *exec.ExitError
	if err := metrics.Run(root, c, stderr); errors.As(err, &exit) {
		fmt.Fprintf(stderr, "umlv: tests exited with code %d; using whatever report they wrote\n", exit.ExitCode())
	} else if err != nil {
		fmt.Fprintln(stderr, "umlv:", explain(err))
	}
}

// readReport reads the report c names, and when it was written.
func readReport(root string, l lang.Language, c metrics.Command, scan facts.Scan) (metrics.Coverage, string, error) {
	path := filepath.Join(root, c.Report)
	st, err := os.Stat(path)
	if c.Report == "" || err != nil {
		return metrics.Coverage{}, "", fmt.Errorf("no coverage report at %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return metrics.Coverage{}, "", err
	}
	cov, err := l.Read(data, root, scan)
	if err != nil {
		return metrics.Coverage{}, "", fmt.Errorf("could not read the coverage report: %w", err)
	}
	return cov, st.ModTime().UTC().Format(time.RFC3339), nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd
git commit -m "Add --metrics: run the tests, read the report, grade every module; reuse the last report otherwise."
```

---

### Task 17: Interactions: selection, Esc, Enter, the arrows switch, zoom, pan and the URL

**Files:**
- Modify: `web/app.js` (replace the first version entirely)
- Create: `web/app.test.js`

**Interfaces:**
- Consumes: `viewAt`, `folderExists`, `layout`, `renderDiagram`, `renderHeader`, `renderCard`.
- Produces: `parseHash(hash) → { folder, selected, arrows }`; `formatHash(state) → string`; `onEscape(state) → state`; `restore(page, state) → { state, notice }`; `zoomAt(viewBox, factor, fx, fy) → viewBox`; `panBy(viewBox, dx, dy) → viewBox`; `start(doc, win)`. A viewBox is `[x, y, width, height]`. The fragment is `#f=<folder>&s=<box key>&a=off`, every part optional.

- [ ] **Step 1: Write the failing tests** `web/app.test.js`

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseHash, formatHash, onEscape, restore, zoomAt, panBy } from './app.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));

test('the URL fragment round-trips the folder, the selection and the arrows switch', () => {
  const state = { folder: ['cart'], selected: 'cart/basket', arrows: false };
  assert.deepEqual(parseHash(formatHash(state)), state);
  assert.deepEqual(parseHash(''), { folder: [], selected: null, arrows: true });
  assert.deepEqual(parseHash('#f=cart&s=cart/basket'), { folder: ['cart'], selected: 'cart/basket', arrows: true });
  assert.equal(formatHash({ folder: [], selected: null, arrows: true }), '');
});

test('Esc closes the card first, then goes up a level, then does nothing', () => {
  const s = { folder: ['cart'], selected: 'cart/basket', arrows: true };
  const once = onEscape(s);
  assert.deepEqual(once, { ...s, selected: null });
  assert.deepEqual(onEscape(once).folder, []);
  const top = { folder: [], selected: null, arrows: true };
  assert.deepEqual(onEscape(top), top);
});

test('a link into a folder that a re-scan removed opens the top with a notice', () => {
  const { state, notice } = restore(page, { folder: ['gone'], selected: 'gone/x', arrows: false });
  assert.deepEqual(state, { folder: [], selected: null, arrows: false });
  assert.ok(notice.includes('gone/'));
  assert.equal(restore(page, { folder: ['cart'], selected: null, arrows: true }).notice, null);
});

test('zooming keeps the point under the pointer still, and panning moves the view', () => {
  assert.deepEqual(zoomAt([0, 0, 100, 100], 2, 50, 50), [25, 25, 50, 50]);
  assert.deepEqual(zoomAt([0, 0, 100, 100], 2, 0, 0), [0, 0, 50, 50]);
  assert.deepEqual(panBy([0, 0, 100, 100], 10, -5), [10, -5, 100, 100]);
});
```

- [ ] **Step 2: Run them to see them fail**

Run: `node --test web/`
Expected: FAIL, `does not provide an export named 'parseHash'`.

- [ ] **Step 3: Replace** `web/app.js`

```js
// app.js: the page's state and events. Everything it draws comes from
// model, layout and render; the pure helpers below are what the tests check.
import { viewAt, folderExists } from './model.js';
import { layout } from './layout.js';
import { renderDiagram, renderHeader, renderCard } from './render.js';

export function parseHash(hash) {
  const p = new URLSearchParams(String(hash || '').replace(/^#/, ''));
  const f = p.get('f') || '';
  return { folder: f ? f.split('/') : [], selected: p.get('s') || null, arrows: p.get('a') !== 'off' };
}

export function formatHash(state) {
  const p = new URLSearchParams();
  if (state.folder.length) p.set('f', state.folder.join('/'));
  if (state.selected) p.set('s', state.selected);
  if (!state.arrows) p.set('a', 'off');
  const s = p.toString();
  return s ? `#${s}` : '';
}

export function onEscape(state) {
  if (state.selected) return { ...state, selected: null };
  if (state.folder.length) return { ...state, folder: state.folder.slice(0, -1) };
  return state;
}

export function restore(page, state) {
  if (folderExists(page, state.folder)) return { state, notice: null };
  return {
    state: { ...state, folder: [], selected: null },
    notice: `${state.folder.join('/')}/ no longer exists in this scan; showing the top level.`,
  };
}

export function zoomAt([x, y, w, h], factor, fx, fy) {
  return [fx - (fx - x) / factor, fy - (fy - y) / factor, w / factor, h / factor];
}

export function panBy([x, y, w, h], dx, dy) {
  return [x + dx, y + dy, w, h];
}

export function start(doc, win) {
  const page = JSON.parse(doc.getElementById('data').textContent);
  const header = doc.getElementById('header');
  const diagram = doc.getElementById('diagram');
  const card = doc.getElementById('card');
  let { state, notice } = restore(page, parseHash(win.location.hash));
  let view = null;
  let pos = null;
  let zoomed = null; // a viewBox while zoomed or panned; null means fitted

  const svg = () => diagram.querySelector('svg');
  const fitted = () => svg().getAttribute('viewBox').split(' ').map(Number);
  const current = () => zoomed || fitted();
  const setView = (vb) => { zoomed = vb; svg().setAttribute('viewBox', vb.join(' ')); };

  async function draw() {
    view = viewAt(page, state.folder);
    const keys = [...view.boxes, ...view.libraries].map((n) => n.key);
    if (state.selected && !keys.includes(state.selected)) state = { ...state, selected: null };
    pos = await layout(view);
    zoomed = null; // fit to view on load and after every drill-down
    paint();
  }

  function paint() {
    header.innerHTML = renderHeader(page, view, notice, state);
    diagram.innerHTML = renderDiagram(view, pos, state);
    if (zoomed && svg()) svg().setAttribute('viewBox', zoomed.join(' '));
    card.hidden = !state.selected;
    card.innerHTML = state.selected ? renderCard(page, view, state.selected) : '';
    win.history.replaceState(null, '', formatHash(state) || win.location.pathname);
  }

  const open = (folder) => { state = { ...state, folder, selected: null }; notice = null; return draw(); };
  const set = (changes) => { state = { ...state, ...changes }; paint(); };

  let drag = null;
  let dragged = false;
  diagram.addEventListener('click', (e) => {
    if (dragged) return;
    const node = e.target.closest('[data-key]');
    set({ selected: node ? node.dataset.key : null });
  });
  diagram.addEventListener('dblclick', (e) => {
    const folder = e.target.closest('.box.folder');
    if (folder) open(folder.dataset.key.split('/'));
  });
  header.addEventListener('click', (e) => {
    const crumb = e.target.closest('[data-folder]');
    if (crumb) open(crumb.dataset.folder ? crumb.dataset.folder.split('/') : []);
    else if (e.target.closest('[data-toggle-arrows]')) set({ arrows: !state.arrows });
  });
  card.addEventListener('click', (e) => {
    const button = e.target.closest('[data-open]');
    if (button) open(button.dataset.open.split('/'));
  });
  doc.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const next = onEscape(state);
      if (next.folder.length !== state.folder.length) open(next.folder);
      else set(next);
    } else if (e.key === 'Enter' && state.selected) {
      const box = view.boxes.find((b) => b.key === state.selected);
      if (box && box.kind !== 'module') open(box.key.split('/'));
    } else if (e.key === '0' && svg()) {
      zoomed = null;
      paint();
    }
  });
  diagram.addEventListener('wheel', (e) => {
    if (!svg()) return;
    e.preventDefault();
    const perPixel = current()[2] / svg().clientWidth;
    if (e.ctrlKey || e.metaKey) {
      const pt = new DOMPoint(e.clientX, e.clientY).matrixTransform(svg().getScreenCTM().inverse());
      setView(zoomAt(current(), Math.exp(-e.deltaY * 0.002), pt.x, pt.y));
    } else {
      setView(panBy(current(), e.deltaX * perPixel, e.deltaY * perPixel));
    }
  }, { passive: false });
  diagram.addEventListener('pointerdown', (e) => {
    if (svg()) drag = { x: e.clientX, y: e.clientY, vb: current() };
    dragged = false;
  });
  win.addEventListener('pointermove', (e) => {
    if (!drag) return;
    const dx = e.clientX - drag.x;
    const dy = e.clientY - drag.y;
    if (Math.abs(dx) + Math.abs(dy) > 3) dragged = true;
    if (dragged) {
      const perPixel = drag.vb[2] / svg().clientWidth;
      setView(panBy(drag.vb, -dx * perPixel, -dy * perPixel));
    }
  });
  win.addEventListener('pointerup', () => { drag = null; });
  win.addEventListener('hashchange', () => {
    ({ state, notice } = restore(page, parseHash(win.location.hash)));
    draw();
  });
  draw();
}
```

- [ ] **Step 4: Run the tests**

Run: `node --test web/ && go test ./internal/page/`
Expected: PASS (the Go test re-bundles the new `app.js`).

- [ ] **Step 5: Check the interactions by hand**

```bash
go run ./cmd/umlv --no-open ../spaced-repetition
```

Open `../spaced-repetition/.umlv/index.html` in the browser pane and confirm each line, fixing `app.js` (test-first where the logic is pure) until all hold:

- Clicking `db` selects it: cream border, bold name, its arrows brighten, other arrows fade, unrelated boxes dim, the card opens.
- Clicking empty space deselects and closes the card.
- Esc with the card open closes it; a second Esc inside `routes/` goes up a level.
- Selecting `routes/ ›` and pressing Enter opens it; so does the card's Open button; so does double-click.
- The arrows switch hides every arrow except the selected box's.
- The wheel pans; ⌘ or Ctrl with the wheel (or a trackpad pinch) zooms around the pointer; `0` fits the view again.
- Hovering an arrow names its two ends; hovering a badge lists the boxes behind it; hovering a lamp shows its number.
- Refreshing keeps the folder, selection and arrows switch. Editing the URL to `#f=gone` opens the top with the notice.

- [ ] **Step 6: Commit**

```bash
git add web/app.js web/app.test.js
git commit -m "Select, focus, open, go up, toggle arrows, zoom and pan, and keep it all in the URL."
```

---

### Task 18: The look, checked in colour and greyscale

**Files:**
- Create: `web/embed_test.go`, `docs/impl/screens.sh`, screenshots under `docs/impl/screens/`
- Modify: `web/style.css` only if a check below fails

**Interfaces:**
- Produces: `docs/impl/screens.sh`, which Task 19 reruns.

- [ ] **Step 1: Pin the palette with a test** `web/embed_test.go`

This guards the CSS written in Task 11, so it passes at once; it exists to fail if a later edit drifts from the spec.

```go
package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestStyleUsesTheDesignPaletteAndNoGlow(t *testing.T) {
	css, err := fs.ReadFile(FS, "style.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, hex := range []string{"#1C1A17", "#2E2924", "#534C43", "#8A8070", "#E8DCC4", "#A89E8C",
		"#C9A86A", "#3B8A4E", "#E0A030", "#E8503A", "#221F1B"} {
		if !strings.Contains(string(css), hex) {
			t.Errorf("style.css lacks %s from the Look table", hex)
		}
	}
	for _, banned := range []string{"text-shadow", "blur(", "gradient("} {
		if strings.Contains(string(css), banned) {
			t.Errorf("style.css uses %s; the look has no glow", banned)
		}
	}
}
```

Run: `go test ./web/`
Expected: PASS.

- [ ] **Step 2: Write** `docs/impl/screens.sh`

```sh
#!/bin/sh
# Re-shoot the screenshots the visual review looks at. Run from the repo root
# after `go run ./cmd/umlv --metrics --no-open .` and
# `go run ./cmd/umlv --no-open ../spaced-repetition`.
set -e
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
OUT=docs/impl/screens
mkdir -p "$OUT"
shot() {
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --window-size=1600,1000 \
    --virtual-time-budget=10000 --screenshot="$PWD/$OUT/$1.png" "$2" 2>/dev/null
}
NEO="file://$PWD/.umlv/index.html"
SR="file://$(cd ../spaced-repetition && pwd)/.umlv/index.html"
shot sr-top "$SR"
shot sr-routes "$SR#f=routes"
shot sr-selected "$SR#s=db"
shot sr-focus "$SR#s=db&a=off"
shot neo-top "$NEO"
shot neo-module-card "$NEO#f=internal&s=internal/metrics"
shot neo-folder-card "$NEO#s=internal"
sips -m "/System/Library/ColorSync/Profiles/Generic Gray Profile.icc" \
  "$OUT/neo-top.png" --out "$OUT/neo-top-grey.png" >/dev/null
sips -m "/System/Library/ColorSync/Profiles/Generic Gray Profile.icc" \
  "$OUT/neo-module-card.png" --out "$OUT/neo-module-card-grey.png" >/dev/null
```

- [ ] **Step 3: Shoot**

```bash
chmod +x docs/impl/screens.sh
go run ./cmd/umlv --metrics --no-open .
go run ./cmd/umlv --no-open ../spaced-repetition
docs/impl/screens.sh
```

- [ ] **Step 4: Check by eye**, reading every PNG under `docs/impl/screens/`:

- In the greyscale shots, the four lamp states are distinct: red has a bright ring, amber is the brightest fill, green is mid-grey, unlit is a dark socket with a faint ring.
- The selected box's border is cream, not amber; nothing but lamps uses the saturated amber.
- Arrows are visible but quieter than box names; faded arrows nearly disappear.
- The spaced-repetition top level (no coverage) shows the legend's "not measured: run umlv --metrics".
- Text is never smaller than 11px; header labels are uppercase; identifiers are not.

Fix `web/style.css` (or the markup in `render.js`, test-first) for any line that fails, then re-run the script.

- [ ] **Step 5: Commit**

```bash
git add web/embed_test.go docs/impl/screens.sh docs/impl/screens web/style.css
git commit -m "Pin the palette in a test and shoot the page in colour and greyscale."
```

---

### Task 19: Visual review of every screen (scheduled reviewer)

**Files:**
- Create: `docs/impl/visual-review.md` (the reviewer's prompt and, below it, the findings and what was done)
- Modify: whatever the MUST findings touch, plus refreshed screenshots

**Interfaces:**
- Consumes: `docs/impl/screens.sh` and the screenshots from Task 18.

This is the visual reviewer Nick asked to have look at all the visuals once they exist.

- [ ] **Step 1: Re-shoot from the current code**

```bash
go run ./cmd/umlv --metrics --no-open . && go run ./cmd/umlv --no-open ../spaced-repetition && docs/impl/screens.sh
```

- [ ] **Step 2: Write the prompt** to `docs/impl/visual-review.md`

```markdown
# Visual review

## Prompt

You are a visual and interaction designer reviewing a built tool, from screenshots. Do not edit
files. Read docs/design/architecture.md (sections The page and Look) in
~/Documents/Repos/uml-viewer-neo, then look at every PNG in docs/impl/screens/ with the Read tool:

- sr-top: a 60-module TypeScript repo's top level, never measured (all lamps unlit).
- sr-routes: inside its routes/ folder.
- sr-selected: the db module selected: focus, fading, the card.
- sr-focus: the same with arrows switched off.
- neo-top, neo-top-grey: this repo's own top level after coverage ran, in colour and greyscale.
- neo-module-card, neo-module-card-grey: a module's card with its function table.
- neo-folder-card: a folder's card.

An earlier design review asked for: a legend that doubles as a status board; focus mode on
selection; colour-blind-safe lamps (a ring on red, amber brighter than green); amber reserved
for lamps, brass for headings, cream for selection; folder boxes reading `name/ ›` with a count
line; badges written `3+2`; orthogonal arrows pointing down; a card that spells out its grade.
Check each landed and works on screen.

Then judge, as a designer of data-dense tools: first-read comprehension, what draws the eye first,
readability at this density, the mission-control look (calm, warm, legible; no glow), type and
spacing, and anything that looks broken. Report findings ranked, each marked MUST, SHOULD or
CONSIDER, each with the screenshot it is visible in and a concrete fix (CSS values or markup).
Keep it under 900 words.
```

- [ ] **Step 3: Dispatch the reviewer**

Use the Agent tool with `model: "fable"` and the prompt above, in the background. When it reports, append its findings to `docs/impl/visual-review.md` under `## Findings (<date>)`.

- [ ] **Step 4: Fold in the MUST findings**

Fix each MUST (test-first wherever the change is logic in `render.js`, `model.js` or `app.js`; CSS-only changes are checked by eye), re-run `docs/impl/screens.sh`, and look again. Under each finding in `docs/impl/visual-review.md`, note what was done. Leave SHOULD and CONSIDER items listed for Nick rather than acting on them.

- [ ] **Step 5: Commit**

```bash
git add docs/impl web
git commit -m "Fold in the visual review of the built page."
```

---

### Task 20: Hold umlv to its own numbers, install it, document it

**Files:**
- Modify: whichever Go files the self-check flags; `README.md`

- [ ] **Step 1: Run the self-check**

```bash
go run ./cmd/umlv --metrics --no-open .
```

Expected: a summary with `0 red, 0 amber`. Modules without functions (such as `web`) are unlit and that is fine; check no unlit module has functions:

```bash
jq -r '.modules[] | select(.grade == "unlit" and (.functions | length) > 0) | .id' .umlv/data.json
```

Expected: no output.

- [ ] **Step 2: Fix anything amber or red**

List the functions to fix:

```bash
jq -r '.modules[] | select(.grade == "red" or .grade == "amber") | .id as $m
  | .functions[] | select(.crap > 8) | "\(.crap | floor)\tcc \(.cc)\tcov \(.coverage * 100 | floor)%\t\($m) \(.name)"' .umlv/data.json
```

For each: if CRAP is close to CC, split the function; if CRAP is well above CC, write the missing tests first. Keep every change test-first and re-run Step 1 until it passes. Commit each fix on its own:

```bash
git commit -am "Split <function> so <module> grades green."
```

- [ ] **Step 3: Install**

```bash
go install ./cmd/umlv
command -v umlv || echo "add $(go env GOPATH)/bin to PATH"
```

If `umlv` is not on PATH, tell Nick the line to add to `~/.zshrc`; do not edit it yourself.

- [ ] **Step 4: Rewrite** `README.md`

```markdown
# umlv

`umlv` turns a Go, TypeScript or Python repository into one HTML page: the folders and modules
as boxes, their imports as arrows, and a lamp on each box showing how risky its code is to change.

## Use

    umlv [--metrics] [--lang go|typescript|python] [--no-open] [repo]

`umlv ~/code/some-repo` scans the repo and opens `.umlv/index.html`. `--metrics` runs the repo's
own tests with coverage first, which lights the lamps; later runs reuse that report. The same data
is written to `.umlv/data.json` for scripts and agents. `.umlv/policy.toml` is written once and is
yours to edit: which libraries show as ovals, your editor, and a custom coverage command.

To keep `.umlv/` out of every repo's `git status`:

    printf '.umlv/\n' >> ~/.config/git/ignore

## Install

    go install ./cmd/umlv

Scanning needs the repo's own toolchain: `go`, `node` or `python3`. Coverage needs its test tools:
nothing extra for Go, `pytest-cov` for Python, `@vitest/coverage-v8` for vitest.

## Develop

    go vet ./... && go test ./... && node --test web/
    go run ./cmd/umlv --metrics --no-open .     # the self-check: no amber or red

Design: [docs/design/architecture.md](docs/design/architecture.md).
Plan: [docs/impl/v1-plan.md](docs/impl/v1-plan.md).

Private: the ideas come from unclebob's unlicensed [uml-viewer](https://github.com/unclebob/uml-viewer),
so this repo is not published.
```

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "Document how to use, install and develop umlv."
```

---

### Task 21: A skill that teaches agents to use umlv

**Files:**
- Create: `skills/umlv/SKILL.md`; symlink `~/.claude/skills/umlv` → `skills/umlv`

**REQUIRED SUB-SKILL:** superpowers:writing-skills. Follow its test-first loop: watch an agent fail without the skill, write the skill, watch it succeed.

- [ ] **Step 1: Baseline without the skill**

Dispatch a fresh subagent (general-purpose) with only this prompt, and save its transcript summary under `## Baseline` in `skills/umlv/TESTING.md`:

> The `umlv` command is installed on this machine. Using it, find the three riskiest modules to change in ~/Documents/Repos/uml-viewer-neo and, for each, the function that makes it risky. Say how you know, and give Nick a link that shows him the riskiest one.

Note each thing it gets wrong or wastes effort on (for example: opening a browser, parsing HTML, reading "unlit" as bad, not knowing what CRAP means, no deep link).

- [ ] **Step 2: Write** `skills/umlv/SKILL.md`

````markdown
---
name: umlv
description: Use when exploring an unfamiliar Go, TypeScript or Python repo, deciding which code is risky to change before editing it, finding what imports a module, or showing a human how a codebase fits together.
---

# umlv

`umlv` scans a repo and writes two files: `.umlv/index.html`, a diagram for humans, and
`.umlv/data.json`, the same facts for you. Read the JSON with `jq`. Never parse the HTML.

## Run it

```bash
umlv --no-open <repo>             # structure only: seconds
umlv --metrics --no-open <repo>   # also runs the repo's tests with coverage: can take minutes
```

- Always pass `--no-open`; without it a browser opens on the human's screen.
- `--metrics` runs the repo's own test suite. Do it only when running tests is acceptable here.
  A plain run reuses the last coverage report, so grades survive between runs.
- The last line of output is a summary: modules, and how many of each grade.
- Exit 1 means it stopped (read stderr: a missing tool, an unreadable policy); exit 2 means bad flags.

## data.json

| Field | Meaning |
|---|---|
| `modules[].id` | What imports point at: a file path, or a Go import path |
| `modules[].tree` | Folder path ending with the module's own name; joined with `/` it is the module's box key |
| `modules[].grade` | `green`, `amber`, `red`, or `unlit` (not measured) |
| `modules[].stats` | `mu`, `sigma`, `max` of the module's function CRAP scores; absent when unlit |
| `modules[].functions[]` | `name`, `file`, `line`, `cc`, and when measured `coverage` (0 to 1) and `crap` |
| `modules[].uses` | IDs of the modules it imports, and `lib:<name>` for libraries listed in the policy |
| `coverageAt` | When the coverage report was written; empty means never measured |
| `bands` | Each grade's cut-off, in words |

## Recipes

```bash
D=<repo>/.umlv/data.json
# Modules by risk, worst first
jq -r '.modules[] | select(.stats) | "\(.stats.mu + .stats.sigma | . * 10 | round / 10)\t\(.grade)\t\(.id)"' "$D" | sort -rn
# The ten worst functions anywhere
jq -r '[.modules[] as $m | $m.functions[] | select(.crap) | {crap, cc, coverage, at: "\($m.id) \(.name)"}]
  | sort_by(-.crap) | .[:10][] | "\(.crap | floor)\tcc \(.cc)\tcov \(.coverage * 100 | floor)%\t\(.at)"' "$D"
# What imports a module, and what it imports
jq -r --arg id src/db.ts '.modules[] | select(.uses | index($id)) | .id' "$D"
jq -r --arg id src/db.ts '.modules[] | select(.id == $id) | .uses[]' "$D"
# Never measured
jq -r '.modules[] | select(.grade == "unlit") | .id' "$D"
```

## Reading the numbers

- CRAP = CC² × (1 − coverage)³ + CC, where CC counts a function's branches. CRAP close to CC means
  tested but complex: split the function. CRAP far above CC means under-tested: write tests first.
- A module's grade reads μ + σ of its functions' CRAP (cut-offs in `bands`), so one very bad
  function can make a module red.
- `unlit` means not measured. It is not a bad grade. Say so when you report.
- Grades are only as fresh as `coverageAt`. If the code changed since, re-run with `--metrics`.
- `uses` counts type-only imports too. Libraries appear only if listed in `.umlv/policy.toml`.
- Tests, vendored code and build output are not scanned.

## Showing a human

Link straight to a box's card: `file://<absolute repo path>/.umlv/index.html#s=<box key>`, and
into a folder with `f`: `#f=routes&s=routes/review`. Or tell them to run `umlv <repo>`, which
opens the page.

## Common mistakes

| Mistake | Instead |
|---|---|
| Parsing `index.html` | Read `data.json` |
| Reporting unlit modules as risky | Report them as not measured |
| Running `--metrics` where tests are slow or touch real services | Ask first, or use the last report |
| Forgetting `--no-open` | Always pass it |
| Editing `.umlv/policy.toml` silently | It belongs to the human; say what you changed |
````

- [ ] **Step 3: Install it**

```bash
mkdir -p ~/.claude/skills
ln -s "$PWD/skills/umlv" ~/.claude/skills/umlv
```

Nick asked for this skill so agents can use it; mention the symlink in the final report.

- [ ] **Step 4: Run the same test with the skill**

Dispatch a fresh subagent with the same prompt as Step 1, told to read `~/.claude/skills/umlv/SKILL.md` first. Save its summary under `## With the skill` in `skills/umlv/TESTING.md`. It must: pass `--no-open`, use `data.json` with `jq`, name real modules and functions with their CRAP and CC, treat unlit as not measured, and give a working `#s=` link. If it misses any of these, tighten the skill's wording for that point and run again.

- [ ] **Step 5: Commit**

```bash
git add skills
git commit -m "Add a skill that teaches agents to run umlv and query data.json."
```

---

## Execution notes

- **Order:** Tasks 1–12 give a working page without metrics (the first look is at the end of Task 12); 13–16 add metrics; 17–19 finish the page and review it; 20–21 hold the tool to its own numbers and teach other agents to use it.
- **Ask Nick before:** downloading elkjs (Task 9, Step 1). Tell him, without asking, about the skill symlink in `~/.claude/skills` (Task 21) and any PATH line (Task 20).
- **Spec changes:** if a task needs a decision the spec does not make, stop and update `docs/design/architecture.md` first, in its own commit, then continue.
