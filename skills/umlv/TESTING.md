# Testing the umlv skill

The skill is tested the way `superpowers:writing-skills` asks: run a fresh agent on a realistic
task without the skill, write the skill against what went wrong, then run the same task with it.

The task prompt, used for both runs:

> The `umlv` command is installed on this machine. Using it, find the three riskiest modules to
> change in ~/Documents/Repos/uml-viewer-neo and, for each, the function that makes it risky. Say
> how you know, and give Nick a link that shows him the riskiest one.

## Baseline

No skill, 2026-10-01, Sonnet 5.5 subagent, same prompt as above.

Outcome: correct answer, at a high cost and only because the target was umlv's own repo.

- Answer: all 10 modules green; relatively riskiest internal/policy (μ+σ 7.96, TopLibraries CC 10), internal/page (7.36, Build CC 8), internal/lang/golang (7.02, complexity CC 9); correctly called `web` "unlit = not measured, not safe"; gave link `file:///…/uml-viewer-neo/.umlv/index.html#f=internal&s=internal%2Fpolicy` and a vscode:// link.
- 18 commands. It ran `umlv --help` to discover flags (found --no-open there), then read `.umlv/data.json` raw, grepped docs/design/architecture.md for the grading rule, and read web/app.js and web/model.js source to learn the URL fragment format and how box keys are built.
- It wrote three ad-hoc Python scripts to list grades, recompute μ/σ per module and print per-function CC/coverage/CRAP — work a jq recipe does in one line.
- It could not have learnt the data.json fields, the grading rule or the fragment format in any other repo (no design doc there).
- It ran `--metrics` (the repo's tests) without weighing whether that was acceptable.
- It did not open a browser (it found --no-open via --help), but nothing told it to prefer that.

### What in the skill answers each weakness

| Baseline weakness | Where the skill answers it |
|---|---|
| Ran `--help` to find `--no-open` | Run it: the flag first, with the reason (a browser opens on the human's screen) |
| Nothing told it to prefer `--no-open` | Run it: "Always pass `--no-open`"; Common mistakes row |
| Ran `--metrics` without weighing it | Run it: says it runs the repo's own tests, only when that is acceptable; Common mistakes row; a plain run reuses the last report |
| Read `data.json` raw; three ad-hoc Python scripts for μ/σ and per-function CRAP | Recipes: modules by risk, ten worst functions, one module's functions, all in `jq` |
| Grepped the design doc for the grading rule | Reading the numbers (CRAP, μ + σ, `bands` row); no design doc exists in other repos |
| Read `web/app.js` and `web/model.js` to learn the link format | Showing a human: the fragment recipe and the rule that `f` must be the parent folders |
| Could not learn the field names elsewhere | The data.json table |
| Treated `web` as "unlit = not measured" correctly | Kept: Reading the numbers and Common mistakes, so it does not regress |

## With the skill

2026-10-01, Sonnet 5.5 subagent, same prompt, told to read ~/.claude/skills/umlv/SKILL.md first.

Outcome: the same correct answer in 3 tool calls (read SKILL.md, one `umlv --no-open` run, two batches of the skill's jq recipes) instead of 18.

- Answer: internal/policy 7.96 (TopLibraries CC 10, 100%, CRAP 10), internal/page 7.36 (Build CC 8, tied with uses), internal/lang/golang 7.02 (complexity CC 9) — real modules and functions with CRAP, CC and coverage; noted CRAP = CC means "split the function", per the skill.
- Passed `--no-open`; used data.json with jq only; never parsed the HTML or read source to learn the format.
- Checked `coverageAt` for freshness and did NOT run `--metrics` because nobody had said running tests was acceptable — the skill's guidance.
- Reported `web` as unlit = not measured, not ranked as risky.
- Link built with the skill's fragment recipe: `file:///…/uml-viewer-neo/.umlv/index.html#f=internal&s=internal/policy` (raw `/`, parent folder in `f`) — the form verified to work in a browser.
- Success criteria from Task 21 Step 4: all met. No wording needed tightening.

Against the baseline: the same answer in 3 tool calls instead of 18, with no source reading, and `--metrics` correctly not run.

## Checked against the tool

Every claim was checked on 2026-10-01 against `umlv --help`, this repo's `.umlv/data.json` (from a
`--metrics` run), scratch copies of the Go, TypeScript and Python test shops, and the page in a
browser. Four things in the first draft were wrong or missing and are corrected:

- `coverageAt` is absent, not empty, when nothing was measured (`omitempty`; `jq` reads `null`).
- `tree` is an array and is not the file path: `src/cart/basket.ts` has tree `["cart","basket"]`
  (`src` levels and the extension are dropped, `index.ts` takes its folder's name), so always read it from `data.json`.
- "`tree` joined with `/` is the box key" holds only inside the module's parent folder. A box is
  selected only in the `f` view that holds it: `#s=internal/policy` alone selects nothing, and
  `#f=internal&s=internal/policy` shows the card. A raw `/` in `s` and `f` works; the page parses
  with `URLSearchParams` and rewrites it to `%2F`.
- `data.json` also has `source` and `notes`, now in the table. Stdout is a single summary line.
