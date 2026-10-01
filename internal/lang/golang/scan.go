// Package golang scans Go modules. Packages and imports come from `go list`,
// so build constraints and the module path are whatever the toolchain says.
// Functions, their line ranges and cyclomatic complexity come from go/parser.
package golang

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"uml-viewer-neo/internal/facts"
)

type listed struct {
	ImportPath string
	Dir        string
	Name       string
	GoFiles    []string
	CgoFiles   []string
	Imports    []string
	Module     *struct{ Path, Dir string }
}

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

func goList(root string) ([]listed, error) {
	cmd := exec.Command("go", "list", "-e", "-json", "./...")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []listed
	for dec.More() {
		var p listed
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func modulePath(pkgs []listed) string {
	for _, p := range pkgs {
		if p.Module != nil && p.Module.Path != "" {
			return p.Module.Path
		}
	}
	return ""
}

// readRequires lists the module paths in go.mod's require directives,
// longest first, so an import resolves to its most specific module.
func readRequires(gomod string) []string {
	f, err := os.Open(gomod)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	inBlock := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := withoutComment(sc.Text())
		switch line {
		case "require (":
			inBlock = true
		case ")":
			inBlock = false
		default:
			if mod := requiredModule(line, inBlock); mod != "" {
				out = append(out, mod)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func withoutComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// requiredModule is the module a go.mod line requires: any line inside a
// require block, or the one after "require " on a single-line directive.
func requiredModule(line string, inBlock bool) string {
	fields := strings.Fields(line)
	if inBlock {
		if len(fields) == 0 {
			return ""
		}
		return fields[0]
	}
	if strings.HasPrefix(line, "require ") && len(fields) >= 2 && fields[1] != "(" {
		return fields[1]
	}
	return ""
}

func under(path, prefix string) bool {
	return prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/"))
}

func isStd(path string) bool {
	first := path
	if i := strings.Index(path, "/"); i >= 0 {
		first = path[:i]
	}
	return !strings.Contains(first, ".")
}

func imports(paths []string, prefix string, requires []string) []facts.Import {
	out := []facts.Import{}
	for _, path := range paths {
		switch {
		case path == "C" || path == "unsafe":
			continue
		case under(path, prefix):
			out = append(out, facts.Import{To: path, Project: true})
		case isStd(path):
			out = append(out, facts.Import{To: path, Std: true, Module: path})
		default:
			mod := path
			for _, r := range requires {
				if under(path, r) {
					mod = r
					break
				}
			}
			out = append(out, facts.Import{To: path, Module: mod})
		}
	}
	return out
}

func functions(root string, p listed) []facts.Function {
	out := []facts.Function{}
	files := append(append([]string{}, p.GoFiles...), p.CgoFiles...)
	sort.Strings(files)
	fset := token.NewFileSet()
	for _, name := range files {
		full := filepath.Join(p.Dir, name)
		f, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, full)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			out = append(out, facts.Function{
				Name:  funcName(fd),
				File:  filepath.ToSlash(rel),
				Start: fset.Position(fd.Pos()).Line,
				End:   fset.Position(fd.End()).Line,
				CC:    complexity(fd),
			})
		}
	}
	return out
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return recvType(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func recvType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvType(t.X)
	case *ast.IndexExpr:
		return recvType(t.X)
	case *ast.IndexListExpr:
		return recvType(t.X)
	case *ast.ParenExpr:
		return recvType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// complexity counts decision points the way gocyclo does: 1 for the function,
// plus each if, loop, non-default case, and short-circuit operator.
func complexity(fd *ast.FuncDecl) int {
	n := 1
	ast.Inspect(fd, func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			if t.List != nil {
				n++
			}
		case *ast.CommClause:
			if t.Comm != nil {
				n++
			}
		case *ast.BinaryExpr:
			if t.Op == token.LAND || t.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}
