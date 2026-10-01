# Visual review

## Prompt

You are a visual and interaction designer reviewing a built tool, from screenshots. Do not edit
files. Read docs/design/architecture.md (sections The page and Look) in
~/Documents/Repos/uml-viewer-neo, then look at every PNG in docs/impl/screens/ with the Read tool:

- sr-top: a 60-module TypeScript repo's top level, never measured (all lamps unlit).
- sr-routes: inside its routes/ folder.
- sr-selected: the db module selected: focus, fading, the card.
- sr-focus: the same with arrows switched off.
- neo-top, neo-top-grey: this repo's own top level after coverage ran, in colour and greyscale.
- neo-module-card, neo-module-card-grey: a module's card with its function table.
- neo-folder-card: a folder's card.

An earlier design review asked for: a legend that doubles as a status board; focus mode on
selection; colour-blind-safe lamps (a ring on red, amber brighter than green); amber reserved
for lamps, brass for headings, cream for selection; folder boxes reading `name/ ›` with a count
line; badges written `3+2`; orthogonal arrows pointing down; a card that spells out its grade.
Check each landed and works on screen.

Then judge, as a designer of data-dense tools: first-read comprehension, what draws the eye first,
readability at this density, the mission-control look (calm, warm, legible; no glow), type and
spacing, and anything that looks broken. Report findings ranked, each marked MUST, SHOULD or
CONSIDER, each with the screenshot it is visible in and a concrete fix (CSS values or markup).
Keep it under 900 words.

## Findings (2026-10-01)

Reviewer: Fable 5.1, from the nine screenshots in docs/impl/screens/ at commit 4ec7d31.

### Did the earlier review's asks land?

All eight landed on screen: legend as status board; focus mode on selection (the strongest screen); colour-blind-safe lamps (red ring, amber brighter than green; four distinct lamps in greyscale — though no red lamp appears inside a box in any shot, so the in-diagram ring is unverified); amber only for lamps, brass headings, cream selection; `name/ ›` with a count line; badges `3+2`; orthogonal arrows pointing down; the card spells out its grade. The look holds: warm, dim, no glow, type at the four specified sizes.

### Ranked

1. **MUST — Badges sit in the arrow fan and get struck through.** sr-top: the `16` under `routes/` has a line through it; the `8` over `db` sits on eight stacked arrowheads; neo-module-card: `4+1` over `facts` touches the cream arrowhead. Badges are placed at `y = −5` and `y = p.h + 14`, exactly where ELK lands edges; the 7px arrowhead occupies y ∈ [−7, 0]. Fix: `.badge { paint-order: stroke; stroke: var(--charcoal); stroke-width: 3px; stroke-linejoin: round; }` (a knocked-out halo so lines break around the digits), and move the top badge to `y="-9"` to clear the arrowhead.
   - **Done:** `.badge` in `web/style.css` now has the charcoal halo (`paint-order: stroke; stroke: var(--charcoal); stroke-width: 3px; stroke-linejoin: round`); the used-by badge in `web/render.js` moved from `y="-5"` to `y="-9"`. `web/render.test.js` pins both badge positions (-9 above; box height + 14 below). The CSS was checked by eye.
2. **MUST — Header times are UTC with no zone.** `SCANNED 2026-10-01 12:08` for a shoot taken at 08:08 local: a non-UTC reader sees a scan hours in the future, and this is the page's one freshness signal. Fix: build `YYYY-MM-DD HH:MM` in `stamp()` from the Date's local getters and put the ISO string in a `title`; run the Node tests under `TZ=UTC`. Minimum alternative: append ` UTC`.
3. **SHOULD — The card overlay hides the selected box's own partners.** sr-selected: db is used by exporter, but exporter and interop are under the card. Keep the overlay but, when the view is fitted, fit into the uncovered width: `fitted([x,y,w,h], stageW, stageH, covered = 0)` with `s = min(1, (stageW − covered)/w, stageH/h)`; pass 380 while the card is shown; zoomed or panned views are left alone.
4. **SHOULD — `0 high` is the brightest mark on a clean repo.** Render zero legend entries with class `zero` at 50% opacity.
5. **SHOULD — Badges are the one mark the legend does not explain.** Add a legend span after `→ imports`: `3+2 partners — above: used by · below: uses · +outside`.
6. **SHOULD — `1 modules · 1 measured`.** Singularise in model.js.
7. **CONSIDER — The legend counts boxes, not modules.** neo-top says `1 medium` for an 8-module folder. Counting members would make the top level a true status board. A design call for Nick.
8. **CONSIDER — Bundle shared-source edges** with `'elk.layered.mergeEdges': 'true'` in dense views (sixteen parallel lines leave `routes/` on sr-top).
9. **CONSIDER — Underlines on every function row** in the card; underline on hover only.
10. **CONSIDER — `arrows on` reads as a tag, not a switch.** `arrows: on` or a check-box glyph.

### The build's four decisions

- Card as overlay: stay, with finding 3's fit change.
- Small diagrams at 1:1, top-centre: stay. Text keeps one size at every level; the empty space below reads as "that is all there is".
- UTC header times: change (finding 2).
- Badges touching arrows: change (finding 1).
