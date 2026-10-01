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
