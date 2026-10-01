// Command umlv scans a Go, TypeScript or Python repo and writes one HTML
// page of its modules, imports and grades to <repo>/.umlv/index.html, with
// the same data in .umlv/data.json.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/internal/lang"
	"uml-viewer-neo/internal/lang/golang"
	"uml-viewer-neo/internal/lang/python"
	"uml-viewer-neo/internal/lang/typescript"
	"uml-viewer-neo/internal/metrics"
	"uml-viewer-neo/internal/page"
	"uml-viewer-neo/internal/policy"
)

// languages, in the order detection tries them.
var languages = []lang.Language{golang.Lang, python.Lang, typescript.Lang}

const usage = "usage: umlv [--metrics] [--lang go|typescript|python] [--no-open] [repo]"

type options struct {
	root, lang      string
	metrics, noOpen bool
}

// usageError is a mistake on the command line, exit code 2. msg is empty
// when the flag package has already explained it.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, openBrowser, time.Now))
}

func run(args []string, stdout, stderr io.Writer, open func(string) error, now func() time.Time) int {
	o, err := parseArgs(args, stderr)
	if err != nil {
		return fail(err, stderr)
	}
	out, p, err := generate(o, stderr, now)
	if err != nil {
		return fail(err, stderr)
	}
	fmt.Fprintln(stdout, summary(p, out))
	openPage(o, out, open, stderr)
	return 0
}

func parseArgs(args []string, stderr io.Writer) (options, error) {
	var o options
	flags := flag.NewFlagSet("umlv", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, usage); flags.PrintDefaults() }
	flags.BoolVar(&o.metrics, "metrics", false, "run the repo's tests with coverage first")
	flags.StringVar(&o.lang, "lang", "", "go, typescript or python; overrides detection")
	flags.BoolVar(&o.noOpen, "no-open", false, "write the page without opening a browser")
	if err := flags.Parse(args); err != nil {
		return o, usageError{}
	}
	if flags.NArg() > 1 {
		return o, usageError{usage}
	}
	root, err := filepath.Abs(flags.Arg(0)) // "" means the current directory
	if err != nil {
		return o, err
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return o, fmt.Errorf("%s is not a directory", root)
	}
	o.root = root
	return o, nil
}

func fail(err error, stderr io.Writer) int {
	var u usageError
	if errors.As(err, &u) {
		if u.msg != "" {
			fmt.Fprintln(stderr, "umlv:", u.msg)
		}
		return 2
	}
	fmt.Fprintln(stderr, "umlv:", explain(err))
	return 1
}

// generate scans the repo, applies its policy and writes the page.
func generate(o options, stderr io.Writer, now func() time.Time) (string, facts.Page, error) {
	l, err := pickLanguage(o.root, o.lang)
	if err != nil {
		return "", facts.Page{}, err
	}
	scan, err := l.Scan(o.root)
	if err != nil {
		return "", facts.Page{}, err
	}
	for _, n := range scan.Notes {
		fmt.Fprintln(stderr, "umlv: note:", n)
	}
	pol, err := loadPolicy(o.root, scan)
	if err != nil {
		return "", facts.Page{}, err
	}
	scores, coverageAt := coverage(o, l, pol, scan, stderr)
	p := page.Build(scan, scores, page.Options{
		Repo: filepath.Base(o.root), ScannedAt: now().UTC().Format(time.RFC3339), CoverageAt: coverageAt,
		EditorPrefix: pol.EditorPrefix(o.root), Libraries: pol.Libraries, Bands: metrics.Bands,
	})
	out, err := write(o.root, p)
	return out, p, err
}

// coverage returns the scores, and when their report was written: from a
// fresh test run with --metrics, else from the last report on disk.
// Problems only warn; the lamps stay unlit.
func coverage(o options, l lang.Language, pol policy.Policy, scan facts.Scan, stderr io.Writer) (facts.Scores, string) {
	c := coverageCommand(l, pol, o.root)
	if o.metrics {
		runTests(o.root, c, stderr)
	}
	cov, at, err := readReport(o.root, l, c, scan)
	if err != nil {
		if o.metrics {
			fmt.Fprintf(stderr, "umlv: %v. %s\n", err, l.CoverageHint)
		}
		return facts.Scores{}, ""
	}
	return metrics.Score(scan, &cov), at
}

// coverageCommand is the language's command, with the policy's overrides.
func coverageCommand(l lang.Language, pol policy.Policy, root string) metrics.Command {
	c := l.Coverage(root)
	if len(pol.Coverage.Command) > 0 {
		c.Args = pol.Coverage.Command
	}
	if pol.Coverage.Report != "" {
		c.Report = pol.Coverage.Report
	}
	return c
}

func runTests(root string, c metrics.Command, stderr io.Writer) {
	if len(c.Args) == 0 {
		fmt.Fprintln(stderr, "umlv: no known way to run this repo's tests; set [coverage] in .umlv/policy.toml")
		return
	}
	fmt.Fprintln(stderr, "umlv: running", strings.Join(c.Args, " "))
	var exit *exec.ExitError
	if err := metrics.Run(root, c, stderr); errors.As(err, &exit) {
		fmt.Fprintf(stderr, "umlv: tests exited with code %d; using whatever report they wrote\n", exit.ExitCode())
	} else if err != nil {
		fmt.Fprintln(stderr, "umlv:", explain(err))
	}
}

// readReport reads the report c names, and when it was written.
func readReport(root string, l lang.Language, c metrics.Command, scan facts.Scan) (metrics.Coverage, string, error) {
	path := filepath.Join(root, c.Report)
	st, err := os.Stat(path)
	if c.Report == "" || err != nil {
		return metrics.Coverage{}, "", fmt.Errorf("no coverage report at %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return metrics.Coverage{}, "", err
	}
	cov, err := l.Read(data, root, scan)
	if err != nil {
		return metrics.Coverage{}, "", fmt.Errorf("could not read the coverage report: %w", err)
	}
	return cov, st.ModTime().UTC().Format(time.RFC3339), nil
}

func pickLanguage(root, name string) (lang.Language, error) {
	if name != "" {
		l, err := lang.ByName(name, languages)
		if err != nil {
			return l, usageError{err.Error()}
		}
		return l, nil
	}
	l, err := lang.Detect(root, languages)
	if err != nil {
		return l, fmt.Errorf("%w; pass --lang to choose one", err)
	}
	return l, nil
}

// loadPolicy reads the repo's policy, writing the default on a first scan.
func loadPolicy(root string, scan facts.Scan) (policy.Policy, error) {
	p, ok, err := policy.Load(root)
	if err != nil || ok {
		return p, err
	}
	p = policy.Default(scan)
	return p, policy.Write(root, p)
}

var installHints = map[string]string{
	"go":      "install Go from https://go.dev/dl",
	"node":    "install Node.js from https://nodejs.org",
	"python3": "install Python 3 from https://www.python.org/downloads",
}

// explain adds an install hint when the error is a missing tool.
func explain(err error) string {
	var missing *facts.MissingToolError
	if errors.As(err, &missing) && installHints[missing.Tool] != "" {
		return fmt.Sprintf("%v; %s", err, installHints[missing.Tool])
	}
	return err.Error()
}

// write puts index.html and data.json in root's .umlv/ and returns the page's path.
func write(root string, p facts.Page) (string, error) {
	dir := filepath.Join(root, ".umlv")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	html, err := page.Render(p)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	out := filepath.Join(dir, "index.html")
	return out, os.WriteFile(out, html, 0o644)
}

func summary(p facts.Page, out string) string {
	n := map[facts.Grade]int{}
	for _, m := range p.Modules {
		n[m.Grade]++
	}
	if n[facts.Unlit] == len(p.Modules) {
		return fmt.Sprintf("umlv: %d modules, none measured (run with --metrics) → %s", len(p.Modules), out)
	}
	return fmt.Sprintf("umlv: %d modules: %d red, %d amber, %d green, %d unlit → %s",
		len(p.Modules), n[facts.Red], n[facts.Amber], n[facts.Green], n[facts.Unlit], out)
}

func openPage(o options, out string, open func(string) error, stderr io.Writer) {
	if o.noOpen {
		return
	}
	if err := open(out); err != nil {
		fmt.Fprintf(stderr, "umlv: could not open a browser (%v); open %s yourself\n", err, out)
	}
}

func openBrowser(path string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, path).Start()
}
