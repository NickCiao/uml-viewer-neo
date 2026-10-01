// Package metrics turns coverage into scores: CRAP for each function, and
// for each module the mean, spread, worst and grade. The grade cut-offs live
// here and nowhere else.
package metrics

import (
	"fmt"
	"math"

	"uml-viewer-neo/internal/facts"
)

// NoCol marks a unit from a report that gives no column.
const NoCol = -1

// Unit is one piece of a coverage report: a Go block, an istanbul statement
// or a coverage.py line. Weight is how many statements it stands for.
type Unit struct {
	File      string
	Line, Col int
	Weight    int
	Hit       bool
}

// Coverage is one report: the files it covered, and their units. A file
// missing from Files was not measured at all.
type Coverage struct {
	Files map[string]bool
	Units []Unit
}

// Command runs a repo's tests with coverage. {report} in Args becomes the
// report's absolute path; Report is relative to the repo root.
type Command struct {
	Args, Env []string
	Report    string
}

const greenMax, amberMax = 8.0, 12.0

// Bands describes each grade's cut-off on μ + σ, for the card.
var Bands = map[string]string{
	"green": fmt.Sprintf("%.0f or less", greenMax),
	"amber": fmt.Sprintf("over %.0f, up to %.0f", greenMax, amberMax),
	"red":   fmt.Sprintf("over %.0f", amberMax),
}

// CRAP is CC² × (1 − coverage)³ + CC, with coverage from 0 to 1.
func CRAP(cc int, coverage float64) float64 {
	c, miss := float64(cc), 1-coverage
	return c + c*c*miss*miss*miss
}

// covers says whether u belongs to f's body. Coverage before the body's
// start (a def line, the `const f =` of an arrow function) ran at import.
func covers(f facts.Function, u Unit) bool {
	from, col := f.CoverStart, f.CoverCol
	if from == 0 {
		from, col = f.Start, 0
	}
	if u.File != f.File || u.Line > f.End {
		return false
	}
	return u.Line > from || (u.Line == from && (u.Col == NoCol || u.Col >= col))
}

// FunctionCoverage is the weighted share of f's body that ran. A function
// with nothing to cover counts as covered.
func FunctionCoverage(f facts.Function, units []Unit) float64 {
	total, hit := 0, 0
	for _, u := range units {
		if covers(f, u) {
			total += u.Weight
			if u.Hit {
				hit += u.Weight
			}
		}
	}
	if total == 0 {
		return 1
	}
	return float64(hit) / float64(total)
}

// StatsOf is the mean, population standard deviation and worst of scores.
func StatsOf(crap []float64) *facts.Stats {
	if len(crap) == 0 {
		return nil
	}
	sum, worst := 0.0, crap[0]
	for _, c := range crap {
		sum += c
		worst = math.Max(worst, c)
	}
	mu := sum / float64(len(crap))
	sq := 0.0
	for _, c := range crap {
		sq += (c - mu) * (c - mu)
	}
	return &facts.Stats{Mu: mu, Sigma: math.Sqrt(sq / float64(len(crap))), Max: worst}
}

// GradeOf reads μ + σ against the cut-offs.
func GradeOf(s *facts.Stats) facts.Grade {
	switch {
	case s == nil:
		return facts.Unlit
	case s.Mu+s.Sigma <= greenMax:
		return facts.Green
	case s.Mu+s.Sigma <= amberMax:
		return facts.Amber
	default:
		return facts.Red
	}
}

// Score measures every function whose file is in the report. A module with
// no measured function is left out, which leaves its lamp unlit.
func Score(scan facts.Scan, cov *Coverage) facts.Scores {
	scores := facts.Scores{}
	if cov == nil {
		return scores
	}
	byFile := map[string][]Unit{}
	for _, u := range cov.Units {
		byFile[u.File] = append(byFile[u.File], u)
	}
	for _, m := range scan.Modules {
		fns := map[string]facts.FunctionScore{}
		var craps []float64
		for _, f := range m.Functions {
			if !cov.Files[f.File] {
				continue
			}
			c := FunctionCoverage(f, byFile[f.File])
			crap := CRAP(f.CC, c)
			fns[f.Name] = facts.FunctionScore{Coverage: c, CRAP: crap}
			craps = append(craps, crap)
		}
		if len(craps) > 0 {
			stats := StatsOf(craps)
			scores[m.ID] = facts.ModuleScore{Grade: GradeOf(stats), Stats: stats, Functions: fns}
		}
	}
	return scores
}
