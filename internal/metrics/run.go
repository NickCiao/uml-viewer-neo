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
	prog := c.Args[0]
	if strings.ContainsAny(prog, string(filepath.Separator)) && !filepath.IsAbs(prog) {
		// Relative path like "./scripts/cov.sh"; check against root
		if _, err := os.Stat(filepath.Join(root, prog)); err != nil {
			return &facts.MissingToolError{Tool: prog}
		}
	} else if _, err := exec.LookPath(prog); err != nil {
		return &facts.MissingToolError{Tool: prog}
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
