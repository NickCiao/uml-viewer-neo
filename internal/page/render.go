package page

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"uml-viewer-neo/internal/facts"
	"uml-viewer-neo/web"
)

const entry = "import { start } from './app.js'; start(document, window);"

// Render writes the page: the template with the styles, the page data and
// the bundled page code inside it.
func Render(p facts.Page) ([]byte, error) {
	data, err := json.Marshal(p) // escapes <, > and &, so no name can close the script tag
	if err != nil {
		return nil, err
	}
	js, err := bundle()
	if err != nil {
		return nil, err
	}
	css, err := fs.ReadFile(web.FS, "style.css")
	if err != nil {
		return nil, err
	}
	tmpl, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		return nil, err
	}
	fill := strings.NewReplacer( // one pass: inserted text is never re-scanned
		"{{TITLE}}", html.EscapeString(p.Repo),
		"{{CSS}}", string(css),
		"{{DATA}}", string(data),
		"{{JS}}", strings.ReplaceAll(js, "</script", `<\/script`),
	)
	return []byte(fill.Replace(string(tmpl))), nil
}

// bundle joins app.js and everything it imports into one script, reading
// the files from the embedded web.FS rather than from disk.
func bundle() (string, error) {
	res := api.Build(api.BuildOptions{
		Stdin:            &api.StdinOptions{Contents: entry, ResolveDir: "/", Sourcefile: "entry.js", Loader: api.LoaderJS},
		Bundle:           true,
		Write:            false,
		Format:           api.FormatIIFE,
		Target:           api.ES2020,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Plugins:          []api.Plugin{embedded(web.FS)},
	})
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("bundling the page code: %s", res.Errors[0].Text)
	}
	return string(res.OutputFiles[0].Contents), nil
}

func embedded(files fs.FS) api.Plugin {
	return api.Plugin{Name: "embedded", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `^\.\.?/`}, func(a api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: path.Join(a.ResolveDir, a.Path), Namespace: "web"}, nil
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "web"}, func(a api.OnLoadArgs) (api.OnLoadResult, error) {
			src, err := fs.ReadFile(files, strings.TrimPrefix(a.Path, "/"))
			if err != nil {
				return api.OnLoadResult{}, err
			}
			text := string(src)
			return api.OnLoadResult{Contents: &text, ResolveDir: path.Dir(a.Path), Loader: api.LoaderJS}, nil
		})
	}}
}
