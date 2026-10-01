package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
	if err == nil || !regexp.MustCompile(`policy\.toml:\d+:`).MatchString(err.Error()) {
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

func writePolicy(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".umlv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, File), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadReadsACustomCoverageCommand(t *testing.T) {
	root := writePolicy(t, "editor = \"cursor\"\n[coverage]\ncommand = [\"make\", \"cover\"]\nreport = \"build/cover.out\"\n")
	p, ok, err := Load(root)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(p.Coverage.Command, []string{"make", "cover"}) || p.Coverage.Report != "build/cover.out" {
		t.Fatalf("coverage = %+v", p.Coverage)
	}
}

func TestLoadDefaultsALeftOutOrEmptyEditorToVscode(t *testing.T) {
	for _, text := range []string{"libraries = []\n", "editor = \"\"\n"} {
		p, _, err := Load(writePolicy(t, text))
		if err != nil || p.Editor != "vscode" {
			t.Fatalf("%q: editor = %q, err = %v", text, p.Editor, err)
		}
	}
}

func TestLoadNamesTheFileWhenAValueHasTheWrongType(t *testing.T) {
	_, ok, err := Load(writePolicy(t, "editor = 5\n"))
	if !ok || err == nil || !strings.Contains(err.Error(), "policy.toml") {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestTopLibrariesNamesAnImportByItsPathWhenItHasNoModule(t *testing.T) {
	scan := facts.Scan{Modules: []facts.Module{
		{ID: "a", Imports: []facts.Import{{To: "left-pad"}}},
		{ID: "b", Imports: []facts.Import{{To: "left-pad"}}},
	}}
	if got := TopLibraries(scan, 8); !reflect.DeepEqual(got, []string{"left-pad"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDefaultPicksTheTopLibrariesAndVscode(t *testing.T) {
	scan := facts.Scan{Modules: []facts.Module{{ID: "a", Imports: []facts.Import{lib("zod")}}}}
	p := Default(scan)
	if p.Editor != "vscode" || !reflect.DeepEqual(p.Libraries, []string{"zod"}) {
		t.Fatalf("got %+v", p)
	}
}

func TestWriteFailsWhenUmlvIsNotAFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".umlv"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, Policy{Editor: "vscode"}); err == nil {
		t.Fatal("writing under a file must fail")
	}
}
