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
