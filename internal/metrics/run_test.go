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
