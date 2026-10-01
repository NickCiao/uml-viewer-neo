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
// decodes the scan facts it prints. Nothing is written to disk. The script
// runs inside root, so it is handed root as an absolute path.
func RunScript(interpreter string, script io.Reader, root string) (facts.Scan, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return facts.Scan{}, err
	}
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
