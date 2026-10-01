// Package page turns scan facts and scores into the page: the data the page
// reads, and the HTML that carries it along with the bundled page code.
package page

import (
	"sort"
	"strings"

	"uml-viewer-neo/internal/facts"
)

// Options are the parts of the page that do not come from the scan.
// Libraries and EditorPrefix are the policy, already resolved; Bands
// describes each grade's cut-off in words for the card.
type Options struct {
	Repo, ScannedAt, CoverageAt, EditorPrefix string
	Libraries                                 []string
	Bands                                     map[string]string
}

// Build makes the page data. Policy is applied here, so the page knows
// nothing about it: unlisted libraries and the standard library are dropped.
func Build(scan facts.Scan, scores facts.Scores, o Options) facts.Page {
	ids := map[string]bool{}
	for _, m := range scan.Modules {
		ids[m.ID] = true
	}
	used := map[string]bool{}
	p := facts.Page{
		Repo: o.Repo, ScannedAt: o.ScannedAt, CoverageAt: o.CoverageAt,
		EditorPrefix: o.EditorPrefix, Bands: o.Bands, Notes: scan.Notes,
		Modules: []facts.PageModule{}, Libraries: []facts.Library{},
	}
	for _, m := range scan.Modules {
		score, measured := scores[m.ID]
		pm := facts.PageModule{
			ID: m.ID, Tree: tree(m), Name: m.Name, Source: m.Source(), Grade: facts.Unlit,
			Functions: []facts.PageFunction{}, Uses: uses(m, ids, o.Libraries, used),
		}
		if measured {
			pm.Grade, pm.Stats = score.Grade, score.Stats
		}
		for _, f := range m.Functions {
			pf := facts.PageFunction{Name: f.Name, File: f.File, Line: f.Start, CC: f.CC}
			if fs, ok := score.Functions[f.Name]; ok {
				cov, crap := fs.Coverage, fs.CRAP
				pf.Coverage, pf.CRAP = &cov, &crap
			}
			pm.Functions = append(pm.Functions, pf)
		}
		p.Modules = append(p.Modules, pm)
	}
	sort.Slice(p.Modules, func(i, j int) bool { return p.Modules[i].ID < p.Modules[j].ID })
	for _, l := range o.Libraries {
		if used[l] {
			p.Libraries = append(p.Libraries, facts.Library{ID: "lib:" + l, Name: l})
		}
	}
	return p
}

// tree is a module's folder path ending with its own name. The repo's root
// module has an empty path and goes under its name.
func tree(m facts.Module) []string {
	if m.Path == "" {
		return []string{m.Name}
	}
	return strings.Split(m.Path, "/")
}

func uses(m facts.Module, ids map[string]bool, libs []string, used map[string]bool) []string {
	set := map[string]bool{}
	for _, imp := range m.Imports {
		switch {
		case imp.Project:
			if ids[imp.To] && imp.To != m.ID {
				set[imp.To] = true
			}
		case !imp.Std:
			if lib, ok := library(imp, libs); ok {
				set["lib:"+lib] = true
				used[lib] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// library is the listed library an import belongs to: its own name, or a
// listed name it sits under ("ai" covers "ai/test", "google" covers
// "google.cloud.storage").
func library(imp facts.Import, libs []string) (string, bool) {
	name := imp.Module
	if name == "" {
		name = imp.To
	}
	for _, l := range libs {
		if name == l || strings.HasPrefix(name, l+"/") || strings.HasPrefix(name, l+".") {
			return l, true
		}
	}
	return "", false
}
