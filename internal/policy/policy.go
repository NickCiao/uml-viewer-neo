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
	Omit      []string `toml:"omit"`
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
	text := "# umlv policy. Written once; edit freely, umlv never overwrites it.\n\n" +
		"# Outside libraries drawn as ovals. A name also covers its sub-packages.\n" +
		"libraries = " + list(p.Libraries) + "\n\n" +
		"# Folders or modules to leave out of the diagram and its grades, named as\n" +
		"# the page names them (\"orders/legacy\") or by their path in the repo\n" +
		"# (\"src/generated\"). Each also covers everything inside it.\n" +
		"omit = " + list(p.Omit) + "\n\n" +
		"# Editor for source links: \"vscode\" or \"cursor\".\n" +
		"editor = " + strconv.Quote(p.Editor) + "\n\n" +
		"# To replace the language's coverage command, uncomment and edit.\n" +
		"# {report} in the command becomes the report path.\n" +
		"# [coverage]\n# command = [\"make\", \"cover\"]\n# report = \"build/cover.out\"\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

func list(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = strconv.Quote(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
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

// OmitFrom leaves out the modules the omit list names, by their place in the
// folder tree or their path in the repo; an entry also covers what is under
// it. Notes say how many were left out and name any entry that matched
// nothing, which is most likely a typo.
func (p Policy) OmitFrom(scan facts.Scan) facts.Scan {
	entries := omitEntries(p.Omit)
	if len(entries) == 0 {
		return scan
	}
	matched := map[string]bool{}
	kept := []facts.Module{}
	for _, m := range scan.Modules {
		if e, ok := omitted(m, entries); ok {
			matched[e] = true
			continue
		}
		kept = append(kept, m)
	}
	notes := append([]string{}, scan.Notes...)
	if n := len(scan.Modules) - len(kept); n > 0 {
		notes = append(notes, fmt.Sprintf("Left out %d module%s named under omit in %s.", n, plural(n), File))
	}
	for _, e := range entries {
		if !matched[e] {
			notes = append(notes, fmt.Sprintf("Nothing matches %q under omit in %s.", e, File))
		}
	}
	scan.Modules, scan.Notes = kept, notes
	return scan
}

// omitEntries are the omit list without leading "./", surrounding slashes or
// empty entries, which would otherwise cover the whole repo.
func omitEntries(omit []string) []string {
	var out []string
	for _, e := range omit {
		if e = strings.Trim(strings.TrimPrefix(e, "./"), "/"); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// omitted is the first entry that covers m.
func omitted(m facts.Module, entries []string) (string, bool) {
	for _, e := range entries {
		if within(m.Place(), e) || within(m.Source(), e) {
			return e, true
		}
	}
	return "", false
}

func within(path, entry string) bool {
	return path == entry || strings.HasPrefix(path, entry+"/")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
