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

func TestRunScriptPassesAnAbsoluteRootEvenWhenGivenARelativeOne(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "my repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(parent)
	script := `import json, sys; print(json.dumps({"lang": "x", "prefix": sys.argv[1], "modules": []}))`
	s, err := RunScript("python3", strings.NewReader(script), "my repo")
	if err != nil || !filepath.IsAbs(s.Prefix) || filepath.Base(s.Prefix) != "my repo" {
		t.Fatalf("prefix = %q, err = %v", s.Prefix, err)
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
