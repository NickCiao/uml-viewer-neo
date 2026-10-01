// Package facts is the vocabulary every other package shares: what a scanner
// reports, the scores computed from coverage, and the page data embedded in
// index.html and written to data.json.
package facts

import "fmt"

// Import is one import of a module, as a scanner reports it. A project import
// names another module's ID in To; anything else is a library or the standard
// library, named by its import path and, when the scanner knows it, its
// package name in Module.
type Import struct {
	To      string `json:"to"`
	Project bool   `json:"project"`
	Std     bool   `json:"std,omitempty"`
	Module  string `json:"module,omitempty"`
}

// Function is one function or method. CoverStart and CoverCol, when set, are
// where its body starts: coverage before that point belongs to the
// declaration, not the body.
type Function struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	CoverStart int    `json:"cover_start,omitempty"`
	CoverCol   int    `json:"cover_col,omitempty"`
	CC         int    `json:"cc"`
}

// Module is the unit a language imports. ID is what imports point at; Path
// is its place in the folder tree ("" for the repo's root module).
type Module struct {
	ID        string     `json:"ns"`
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	File      string     `json:"file,omitempty"`
	Dir       string     `json:"dir,omitempty"`
	Imports   []Import   `json:"imports"`
	Functions []Function `json:"functions"`
}

// Source is the repo-relative path a source link should open: the module's
// file, or for a Go package its directory.
func (m Module) Source() string {
	if m.File != "" {
		return m.File
	}
	return m.Dir
}

// Scan is everything one scanner reports about a repo.
type Scan struct {
	Lang    string   `json:"lang"`
	Prefix  string   `json:"prefix"`
	Modules []Module `json:"modules"`
	Notes   []string `json:"notes,omitempty"`
}

// Grade is a module's lamp.
type Grade string

const (
	Green Grade = "green"
	Amber Grade = "amber"
	Red   Grade = "red"
	Unlit Grade = "unlit"
)

// Stats summarises a module's measured CRAP scores.
type Stats struct {
	Mu    float64 `json:"mu"`
	Sigma float64 `json:"sigma"`
	Max   float64 `json:"max"`
}

// FunctionScore is one measured function.
type FunctionScore struct {
	Coverage float64
	CRAP     float64
}

// ModuleScore is one module with at least one measured function. Functions
// is keyed by function name; a function missing from it was not measured.
type ModuleScore struct {
	Grade     Grade
	Stats     *Stats
	Functions map[string]FunctionScore
}

// Scores is keyed by module ID. A module missing from it is unlit.
type Scores map[string]ModuleScore

// Page is the document embedded in index.html and written to data.json.
// model.js is its only reader in the page.
type Page struct {
	Repo         string            `json:"repo"`
	ScannedAt    string            `json:"scannedAt"`
	CoverageAt   string            `json:"coverageAt,omitempty"`
	EditorPrefix string            `json:"editorPrefix"`
	Bands        map[string]string `json:"bands,omitempty"`
	Modules      []PageModule      `json:"modules"`
	Libraries    []Library         `json:"libraries"`
	Notes        []string          `json:"notes,omitempty"`
}

// PageModule is a module as the page sees it. Tree is its folder path, ending
// with its own name; Uses holds module IDs and library IDs ("lib:<name>").
type PageModule struct {
	ID        string         `json:"id"`
	Tree      []string       `json:"tree"`
	Name      string         `json:"name"`
	Source    string         `json:"source"`
	Grade     Grade          `json:"grade"`
	Stats     *Stats         `json:"stats,omitempty"`
	Functions []PageFunction `json:"functions"`
	Uses      []string       `json:"uses"`
}

// PageFunction is a function as the page sees it; Coverage and CRAP are
// absent when it was not measured.
type PageFunction struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	CC       int      `json:"cc"`
	Coverage *float64 `json:"coverage,omitempty"`
	CRAP     *float64 `json:"crap,omitempty"`
}

// Library is an outside library drawn as an oval.
type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MissingToolError means a program umlv needs to run is not installed.
type MissingToolError struct{ Tool string }

func (e *MissingToolError) Error() string {
	return fmt.Sprintf("%s is not installed or not on PATH", e.Tool)
}
