# umlv

`umlv` turns a Go, TypeScript or Python repository into one HTML page: the folders and modules
as boxes, their imports as arrows, and a lamp on each box showing how risky its code is to change.

## Use

    umlv [--metrics] [--lang go|typescript|python] [--no-open] [repo]

`umlv ~/code/some-repo` scans the repo and opens `.umlv/index.html`. `--metrics` runs the repo's
own tests with coverage first, which lights the lamps; later runs reuse that report. The same data
is written to `.umlv/data.json` for scripts and agents. `.umlv/policy.toml` is written once and is
yours to edit: which libraries show as ovals, your editor, and a custom coverage command.

To keep `.umlv/` out of every repo's `git status`:

    printf '.umlv/\n' >> ~/.config/git/ignore

## Install

    go install ./cmd/umlv

Scanning needs the repo's own toolchain: `go`, `node` or `python3`. Coverage needs its test tools:
nothing extra for Go, `pytest-cov` for Python, `@vitest/coverage-v8` for vitest.

## Develop

    go vet ./... && go test ./... && node --test web/
    go run ./cmd/umlv --metrics --no-open .     # the self-check: no amber or red

Design: [docs/design/architecture.md](docs/design/architecture.md).
Plan: [docs/impl/v1-plan.md](docs/impl/v1-plan.md).

Private: the ideas come from unclebob's unlicensed [uml-viewer](https://github.com/unclebob/uml-viewer),
so this repo is not published.
