---
name: umlv
description: Use when exploring an unfamiliar Go, TypeScript or Python repo, deciding which code is risky to change before editing it, finding what imports a module, or showing a human how a codebase fits together.
---

# umlv

`umlv` scans a repo and writes two files: `.umlv/index.html`, a diagram for humans, and
`.umlv/data.json`, the same facts for you. Read the JSON with `jq`. Never parse the HTML.

## Run it

```bash
umlv --no-open <repo>             # structure only: seconds
umlv --metrics --no-open <repo>   # also runs the repo's tests with coverage: can take minutes
```

- Always pass `--no-open`; without it a browser opens on the human's screen.
- `--metrics` runs the repo's own test suite. Do it only when running tests is acceptable here.
  A plain run reuses the last coverage report, so grades survive between runs.
- Stdout is one summary line: how many modules, and how many of each grade.
- It writes only inside `<repo>/.umlv/` and does not git-ignore it. Keep it out of commits.
- A repo with several languages is read as the first of Go, Python, TypeScript; `--lang` picks another.
- Exit 1 means it stopped (read stderr: a missing tool, an unreadable policy); exit 2 means bad flags.

## data.json

| Field | Meaning |
|---|---|
| `modules[].id` | What imports point at: a repo-relative file path (TypeScript, Python), or a Go import path |
| `modules[].tree` | Array: the module's place in the folder tree, ending with its own name, e.g. `["cart","basket"]` for `src/cart/basket.ts`. Not the file path |
| `modules[].source` | Repo-relative file, or directory for a Go package |
| `modules[].grade` | `green`, `amber`, `red`, or `unlit` (not measured) |
| `modules[].stats` | `mu`, `sigma`, `max` of the module's function CRAP scores; absent when unlit |
| `modules[].functions[]` | `name`, `file`, `line`, `cc`, and when measured `coverage` (0 to 1) and `crap` |
| `modules[].uses` | IDs of the modules it imports, and `lib:<name>` for libraries listed in the policy |
| `coverageAt` | When the coverage report was written; absent means never measured |
| `bands` | Each grade's cut-off on μ + σ, in words |
| `notes` | Optional: what the scan left out (also on stderr as `umlv: note:`) |

## Recipes

```bash
D=<repo>/.umlv/data.json
# Modules by risk, worst first
jq -r '.modules[] | select(.stats) | "\(.stats.mu + .stats.sigma | . * 100 | round / 100)\t\(.grade)\t\(.id)"' "$D" | sort -rn
# The ten worst functions anywhere
jq -r '[.modules[] as $m | $m.functions[] | select(.crap) | {crap, cc, coverage, at: "\($m.id) \(.name) \(.file):\(.line)"}]
  | sort_by(-.crap) | .[:10][] | "\(.crap | floor)\tcc \(.cc)\tcov \(.coverage * 100 | floor)%\t\(.at)"' "$D"
# One module's functions, worst first
jq -c --arg id src/db.ts '.modules[] | select(.id == $id) | .functions | sort_by(-(.crap // 0))[] | {name, line, cc, coverage, crap}' "$D"
# What imports a module, and what it imports
jq -r --arg id src/db.ts '.modules[] | select(.uses | index($id)) | .id' "$D"
jq -r --arg id src/db.ts '.modules[] | select(.id == $id) | .uses[]' "$D"
# Never measured
jq -r '.modules[] | select(.grade == "unlit") | .id' "$D"
```

## Reading the numbers

- CRAP = CC² × (1 − coverage)³ + CC, where CC counts a function's branches. CRAP close to CC means
  tested but complex: split the function. CRAP far above CC means under-tested: write tests first.
- A module's grade reads μ + σ of its functions' CRAP (cut-offs in `bands`), so one very bad
  function can make a module red.
- `unlit` means not measured: no coverage report yet, or none covers the module's files. It is not a
  bad grade. Say so when you report.
- Grades are only as fresh as `coverageAt`. If the code changed since, re-run with `--metrics`.
- `uses` counts type-only imports too. Libraries appear only if listed in `.umlv/policy.toml`.
- Tests, vendored code and build output are not scanned.

## Showing a human

Link to a module's card: `file://<absolute repo path>/.umlv/index.html` followed by the fragment this
prints (percent-encode spaces in the path; a raw `/` in the fragment is fine):

```bash
jq -r --arg id src/cart/basket.ts '.modules[] | select(.id == $id) | .tree
  | "#" + (if length > 1 then "f=" + (.[:-1] | join("/")) + "&" else "" end) + "s=" + join("/")' "$D"
# #f=cart&s=cart/basket
```

`s` names a box and selects it only in the folder view `f` that holds it. So `f` is the module's
parent folders, and `#s=cart/basket` alone selects nothing. `#s=cart` is the card of the folder
`cart/`. Or tell them to run `umlv <repo>`, which opens the page.

## Common mistakes

| Mistake | Instead |
|---|---|
| Parsing `index.html` | Read `data.json` |
| Reporting unlit modules as risky | Report them as not measured |
| Running `--metrics` where tests are slow or touch real services | Ask first, or use the last report |
| Forgetting `--no-open` | Always pass it |
| Building a link from the file path or `#s=` alone | Use the fragment recipe |
| Editing `.umlv/policy.toml` silently | It belongs to the human; say what you changed |
