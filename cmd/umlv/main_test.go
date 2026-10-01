package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"uml-viewer-neo/internal/facts"
)

func copyShop(t *testing.T) string { return copySample(t, "golang") }

// copySample copies a language's sample repo to a temp dir whose name has a space.
func copySample(t *testing.T, language string) string {
	t.Helper()
	src := filepath.Join("../../internal/lang", language, "testdata/shop")
	dst := filepath.Join(t.TempDir(), "my shop")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

var fixed = func() time.Time { return time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC) }

func umlv(t *testing.T, args ...string) (code int, stdout, stderr string, opened []string) {
	t.Helper()
	var out, errOut bytes.Buffer
	open := func(p string) error { opened = append(opened, p); return nil }
	code = run(args, &out, &errOut, open, fixed)
	return code, out.String(), errOut.String(), opened
}

func TestScansARepoAndWritesThePageAndData(t *testing.T) {
	dir := copyShop(t)
	code, out, errOut, opened := umlv(t, "--no-open", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, f := range []string{".umlv/index.html", ".umlv/data.json", ".umlv/policy.toml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s missing", f)
		}
	}
	var p facts.Page
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/data.json"))
	if err := json.Unmarshal(b, &p); err != nil || len(p.Modules) != 2 || p.Repo != "my shop" || p.ScannedAt != "2026-09-30T20:00:00Z" {
		t.Fatalf("data = %+v, err = %v", p, err)
	}
	if !strings.Contains(out, "2 modules, none measured") || !strings.Contains(out, ".umlv/index.html") {
		t.Fatalf("summary = %q", out)
	}
	if len(opened) != 0 {
		t.Fatal("opened a browser despite --no-open")
	}
}

func TestScansEverySampleRepoEndToEnd(t *testing.T) {
	for language, modules := range map[string]int{"golang": 2, "python": 5, "typescript": 5} {
		dir := copySample(t, language)
		if code, _, errOut, _ := umlv(t, "--no-open", dir); code != 0 {
			t.Fatalf("%s: exit %d: %s", language, code, errOut)
		}
		var p facts.Page
		b, _ := os.ReadFile(filepath.Join(dir, ".umlv/data.json"))
		if err := json.Unmarshal(b, &p); err != nil || len(p.Modules) != modules {
			t.Fatalf("%s: %d modules, err %v", language, len(p.Modules), err)
		}
		html, _ := os.ReadFile(filepath.Join(dir, ".umlv/index.html"))
		if !strings.Contains(string(html), `<script type="application/json" id="data">`) || !strings.Contains(string(html), p.Modules[0].ID) {
			t.Fatalf("%s: the page does not carry its data", language)
		}
	}
}

func TestExplainNamesTheToolAndHowToInstallIt(t *testing.T) {
	if got := explain(&facts.MissingToolError{Tool: "node"}); !strings.Contains(got, "node is not installed") || !strings.Contains(got, "nodejs.org") {
		t.Fatalf("got %q", got)
	}
}

func TestOpensThePageByDefault(t *testing.T) {
	dir := copyShop(t)
	if code, _, errOut, opened := umlv(t, dir); code != 0 || len(opened) != 1 || !strings.HasSuffix(opened[0], "index.html") {
		t.Fatalf("exit %d, opened %v: %s", code, opened, errOut)
	}
}

func TestKeepsAnEditedPolicy(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("editor = \"cursor\"\n"), 0o644)
	if code, _, errOut, _ := umlv(t, "--no-open", dir); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/policy.toml"))
	if string(b) != "editor = \"cursor\"\n" {
		t.Fatal("umlv overwrote the policy")
	}
}

func TestStopsWhenNoLanguageIsRecognised(t *testing.T) {
	code, _, errOut, _ := umlv(t, "--no-open", t.TempDir())
	if code != 1 || !strings.Contains(errOut, "--lang") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestRejectsAnUnknownLanguage(t *testing.T) {
	if code, _, errOut, _ := umlv(t, "--lang", "cobol", t.TempDir()); code != 2 || !strings.Contains(errOut, "cobol") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestStopsOnABrokenPolicyWithItsLine(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("editor = \"vscode\"\nlibraries = [\n"), 0o644)
	if code, _, errOut, _ := umlv(t, "--no-open", dir); code != 1 || !regexp.MustCompile(`policy\.toml:\d+:`).MatchString(errOut) {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func readPage(t *testing.T, dir string) facts.Page {
	t.Helper()
	var p facts.Page
	b, _ := os.ReadFile(filepath.Join(dir, ".umlv/data.json"))
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMetricsRunsTheTestsAndLightsTheLamps(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the sample repo's own go test")
	}
	dir := copyShop(t)
	code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "2 modules: 0 red, 0 amber, 2 green, 0 unlit") {
		t.Fatalf("summary = %q", out)
	}
	p := readPage(t, dir)
	if p.CoverageAt == "" || p.Bands["red"] != "over 12" {
		t.Fatalf("coverageAt = %q, bands = %v", p.CoverageAt, p.Bands)
	}
}

func TestAPlainRunReusesTheLastReport(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the sample repo's own go test")
	}
	dir := copyShop(t)
	umlv(t, "--metrics", "--no-open", dir)
	if _, out, _, _ := umlv(t, "--no-open", dir); !strings.Contains(out, "2 green") {
		t.Fatalf("summary = %q", out)
	}
}

func TestFailingTestsWarnButStillWriteThePage(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the sample repo's own go test")
	}
	dir := copyShop(t)
	os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte("package shop\n\nimport \"testing\"\n\nfunc TestBad(t *testing.T) { t.Fatal(\"no\") }\n"), 0o644)
	code, _, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 || !strings.Contains(errOut, "tests exited with code") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestNoReportWarnsWithTheLanguagesHint(t *testing.T) {
	dir := copyShop(t)
	os.MkdirAll(filepath.Join(dir, ".umlv"), 0o755)
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("[coverage]\ncommand = [\"true\"]\n"), 0o644)
	code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 || !strings.Contains(errOut, "no coverage report") || !strings.Contains(errOut, "Go needs nothing extra") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "none measured") {
		t.Fatalf("summary = %q", out)
	}
}

func TestMissingProgramLeavesLampsUnlitEvenWithAnOldReport(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the sample repo's own go test")
	}
	dir := copyShop(t)
	if code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir); code != 0 || !strings.Contains(out, "2 green") {
		t.Fatalf("exit %d, summary %q: %s", code, out, errOut)
	}
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("[coverage]\ncommand = [\"no-such-tool-xyz\"]\n"), 0o644)
	code, out, errOut, _ := umlv(t, "--metrics", "--no-open", dir)
	if code != 0 || !strings.Contains(out, "none measured") {
		t.Fatalf("exit %d, summary %q: %s", code, out, errOut)
	}
	if !strings.Contains(errOut, "no-such-tool-xyz is not installed") || !strings.Contains(errOut, "Go needs nothing extra") {
		t.Fatalf("stderr = %q", errOut)
	}
	if p := readPage(t, dir); p.CoverageAt != "" {
		t.Fatalf("coverageAt = %q, want none", p.CoverageAt)
	}
}

func TestSummaryPointsToTheWarningRatherThanAskingForMetricsAfterMetricsRan(t *testing.T) {
	dir := copyShop(t)
	if _, out, _, _ := umlv(t, "--no-open", dir); !strings.Contains(out, "none measured (run with --metrics)") {
		t.Fatalf("plain run summary = %q", out)
	}
	os.WriteFile(filepath.Join(dir, ".umlv/policy.toml"), []byte("[coverage]\ncommand = [\"true\"]\n"), 0o644)
	_, out, _, _ := umlv(t, "--metrics", "--no-open", dir)
	if !strings.Contains(out, "none measured (see the warning above)") || strings.Contains(out, "run with --metrics") {
		t.Fatalf("--metrics summary = %q", out)
	}
}
