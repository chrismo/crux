# Graph-view rework plan

> **STATUS: SHIPPED** (b95447e and follow-ups). All three phases landed: the
> rune-canvas renderer with real connectors, filter-first collapse with
> path-preserving dashed connectors, and visible-set selection. Since shipping,
> the interaction model evolved further (kept below for history): selection is
> **arrows** (not `h/j/k/l` — letters are now type-to-filter), pins replaced the
> isolate/focus verb, and `esc` walks a focus history stack. Kept as the record
> of the rework; live behavior is in the code + `PLAN-graph-nav.md` (forward
> proposals).

Reworking crux's Flow graph view so a complicated DAG is navigable. Decided
2026-07-01. Primary interaction model: **filter-first collapse** — typing
narrows the graph to matching nodes plus the paths connecting them.

## Problems (grounded in current code)

Three concrete defects, all in the rendering/interaction layer (the graph
*algorithms* in `internal/graph` are fine and stay untouched):

1. **Overflows off the right.** `RenderGraph` (`internal/ui/graphview.go:59`)
   builds each layer with `lipgloss.JoinHorizontal` and never receives the
   terminal width. The result is fed into a viewport whose width is the
   terminal, so anything wider just clips/scrolls off. `Screen`/`RenderGraph`
   aren't told the width at all.
2. **No pathing.** Layers are stacked with a hardcoded decorative
   `"\n   │\n"` stub (`graphview.go:73`) under column 0 — not a routed edge.
   The `graph.Edges` (real parent→child links) are never drawn, so you can't
   tell what connects to what.
3. **Filter/focus only dims, never narrows.** `dims()`
   (`graphview.go:46-54,77-86`) mutes color but renders every node full-size.
   Typing a filter doesn't reduce the graph, and a dimmed node's muted-gray is
   nearly identical to a genuinely skipped (gray) node — so in a mostly-gray run
   filtering looks like it "only affects the green nodes."

## What already works (keep)

`internal/graph`: longest-path layering + barycenter ordering (`layout.go`),
`Edges` (`build.go`, parses `use:` from RawDefinition, all three YAML forms),
critical path (`critpath.go`), failure blast radius (`failure.go`), focus cone
(`focus.go`). All fixture-tested. The `--print` body-parity invariant
(`Screen`/`RenderGraph` back both TUI and headless, guarded by snapshots forcing
`termenv.Ascii`) must be preserved.

## Foundation change

Replace the `JoinHorizontal`-of-cells approach with a **rune-canvas renderer**:
a 2D grid where node boxes are placed at `(x,y)` computed from the existing
`Pos`, and edges are drawn as orthogonal lines between a parent's bottom port
and a child's top port using `graph.Edges` + `LayoutData`. Thread terminal
`width` from `app` → `Screen` → `RenderGraph` (currently absent — the reason
nothing fits). No changes to `internal/graph`.

## Phases

### Phase 1 — Connectors + width plumbing
- Rune-canvas draw primitive (place box, draw H/V/elbow runes, layer→x/y
  mapping) with its own unit tests.
- Draw adjacent-layer edges cleanly; skip-layer edges (e.g. `build-web`→`e2e`)
  route through the inter-layer gap as a channel — v1 tolerates occasional
  crossings rather than a full orthogonal router.
- `width` param on `Screen`/`RenderGraph`.
- Regenerate snapshots; a snapshot from `sample_dag_failed.json` contains visible
  connector runes joining e.g. `go-deps`→`build-api`.
- **Fixes problem 2.**

### Phase 2 — Visible-set collapse (the payoff)
- Derive a `VisibleSet` from filter substring and/or focus cone. Replace
  `dims()` membership: nodes outside the set are **removed**, not muted.
- **Path preservation:** when two visible nodes connect only through hidden
  nodes, draw a *collapsed connector* (distinct style, e.g. dashed/`┈▶`) so
  relationships survive the collapse.
- Header shows `filter: X  (n of N shown)` + hidden count.
- Filtered-out nodes render visually distinct from skipped/gray.
- Snapshot with an active filter shows only matches (hidden absent) with
  path-preserving connectors.
- **Fixes problems 1 (collapse is the primary fit) and 3.**

### Phase 3 — Interaction + polish
- Selection (via `moveSelection` in `internal/ui/app.go`; shipped as **arrow
  keys** since letters became type-to-filter) moves only among **visible** nodes;
  `ensureSelectedVisible` pans horizontally as well as vertically (fallback pan
  for the unfiltered full view).
- Three "gray" treatments made distinct and legible: skipped (state),
  filtered-out (gone), dimmed context (one-hop neighbors of matches, if shown);
  reflect in legend/help.
- Unit tests for the new selection/visibility behavior (e.g. `moveSelection`
  skips hidden nodes).

## Testing

TDD against frozen fixtures — no network:
- `internal/rwx/testdata/sample_dag_succeeded.json` (all green)
- `internal/rwx/testdata/sample_dag_failed.json` (`chaos=true`: 15 green /
  1 failed / 15 skipped — the mostly-gray acceptance case)

Each phase re-freezes snapshots at fixed widths. Note: Phase 2's collapse changes
the frame, so it will rewrite Phase 1's snapshot — expected.

Run tests with the repo's GOPROXY workaround:
`GOPROXY=https://proxy.golang.org,direct ./build.sh test`

## Done criteria

`./build.sh test` fully green with snapshots/tests demonstrating all three:
connectors drawn, filter/focus collapses with path preservation, selection
moves among visible nodes only. `internal/graph` unchanged; no weakened or
skipped tests.
