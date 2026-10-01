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
	if err := findProgram(root, c.Args[0]); err != nil {
		return err
	}
	report := filepath.Join(root, c.Report)
	if err := clearReport(report); err != nil {
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

// findProgram checks that prog can be run: a relative path with a separator,
// like "./scripts/cov.sh", is looked for under root; anything else on PATH.
func findProgram(root, prog string) error {
	if strings.ContainsAny(prog, string(filepath.Separator)) && !filepath.IsAbs(prog) {
		if _, err := os.Stat(filepath.Join(root, prog)); err != nil {
			return &facts.MissingToolError{Tool: prog}
		}
	} else if _, err := exec.LookPath(prog); err != nil {
		return &facts.MissingToolError{Tool: prog}
	}
	return nil
}

// clearReport makes the report's folder and removes any report left there.
func clearReport(report string) error {
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		return err
	}
	if err := os.Remove(report); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
