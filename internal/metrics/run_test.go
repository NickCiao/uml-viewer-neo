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
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestRunHandlesRelativeProgramPaths(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "x.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ran > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origCwd)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	c := Command{Args: []string{"./x.sh", "{report}"}, Report: "report.txt"}
	if err := Run(root, c, &bytes.Buffer{}); err != nil {
		t.Fatalf("err = %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "ran") {
		t.Fatalf("report = %q", b)
	}
}

func TestRunRefusesAnEmptyCommand(t *testing.T) {
	err := Run(t.TempDir(), Command{Report: "r.txt"}, &bytes.Buffer{})
	if err == nil || err.Error() != "no coverage command" {
		t.Fatalf("err = %v", err)
	}
}

func TestRunNamesAMissingScriptInTheRepo(t *testing.T) {
	err := Run(t.TempDir(), Command{Args: []string{"./scripts/cov.sh"}, Report: "r.txt"}, &bytes.Buffer{})
	var missing *facts.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "./scripts/cov.sh" {
		t.Fatalf("err = %v", err)
	}
}

func TestRunFailsWhenTheReportFolderCannotBeMade(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Run(root, Command{Args: []string{"sh", "-c", "echo ran"}, Report: "file/r.txt"}, &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("err = %v, out = %q: the command must not run without a place for its report", err, out.String())
	}
}

func TestRunFailsWhenTheStaleReportCannotBeRemoved(t *testing.T) {
	root := t.TempDir()
	stuck := filepath.Join(root, "r.txt") // a folder with something in it
	if err := os.MkdirAll(filepath.Join(stuck, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Run(root, Command{Args: []string{"sh", "-c", "echo ran"}, Report: "r.txt"}, &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("err = %v, out = %q: a stale report left in place must stop the run", err, out.String())
	}
}
