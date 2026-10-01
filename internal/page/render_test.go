package page

import (
	"encoding/json"
	"strings"
	"testing"

	"uml-viewer-neo/internal/facts"
)

func dataOf(t *testing.T, html string) facts.Page {
	t.Helper()
	const open = `<script type="application/json" id="data">`
	i := strings.Index(html, open)
	if i < 0 {
		t.Fatal("no data script")
	}
	rest := html[i+len(open):]
	var p facts.Page
	if err := json.Unmarshal([]byte(rest[:strings.Index(rest, "</script>")]), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderEmbedsTheDataAndTheBundledCode(t *testing.T) {
	out, err := Render(Build(shopScan(), shopScores(), shopOptions()))
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if p := dataOf(t, html); p.Repo != "shop" || len(p.Modules) != 5 {
		t.Fatalf("data = %+v", p)
	}
	if !strings.Contains(html, "<title>shop · umlv</title>") || !strings.Contains(html, "--charcoal: #1C1A17") {
		t.Fatal("title or styles missing")
	}
	if !strings.Contains(html, "elk.algorithm") || !strings.Contains(html, "org.eclipse.elk") {
		t.Fatal("the bundle lacks layout.js or ELK.js")
	}
}

func TestRenderKeepsMarkupInNamesInert(t *testing.T) {
	scan := shopScan()
	scan.Modules[0].Name = `</script><script>alert(1)</script>`
	p := Build(scan, nil, shopOptions())
	p.Repo = "<b>repo</b>"
	out, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if strings.Contains(html, "<script>alert") {
		t.Fatal("a name broke out of the data script")
	}
	if !strings.Contains(html, "&lt;b&gt;repo&lt;/b&gt; · umlv") {
		t.Fatal("the title is not escaped")
	}
	if dataOf(t, html).Repo != "<b>repo</b>" {
		t.Fatal("the data no longer round-trips")
	}
}
