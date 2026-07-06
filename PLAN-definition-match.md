# --definition: make it work, and match on partial paths (proposal)

The user reported `--definition` "doesn't work AT ALL", and wants partial
matching: given `.rwx/app-dscout.yml`, typing `--definition app-dscout` should
resolve it. Captured 2026-07-02. Planner proposal, not locked.

## Grounding (why it does nothing today)

- `cmd/crux/main.go` **registers** the flag (`fs.StringVar(&o.definition,
  "definition", ...)`, main.go:45) but **nothing ever reads `o.definition`**
  after parsing — confirmed by grep: the only occurrence is the `StringVar`
  line. It is dead. That's the "doesn't work at all" bug: the value is parsed and
  discarded.
- Intended role (per the flag help + `options.definition` comment): "required
  when a branch has multiple definitions" — i.e. disambiguate which run to open
  when a branch has runs from several `.rwx/*.yml` definitions.
- Available data: every `rwx.RunSummary` has `DefinitionPath` (runs.go:14), e.g.
  `.rwx/ci.yml`, `.rwx/app-dscout.yml`. So resolution = pick the run(s) whose
  `DefinitionPath` matches `--definition`.
- Today's resolution paths (`cmd/crux/main.go run()`):
  - `--run <id>` → open that run directly.
  - else → fetch the list (`ListFilter{Limit, Branch, Mine, ...}`) and show the
    home list. `--branch` feeds the fetch; `--definition` is ignored.

## What "match on partial paths" should mean

Match `--definition <substr>` against each run's `DefinitionPath`,
case-insensitive **substring** (same semantics as the graph/list type-to-filter,
for consistency with crux's "just type" model). `app-dscout` matches
`.rwx/app-dscout.yml`; `ci` matches `.rwx/ci.yml`.

Edge cases to define:
- **No match** → error out with a clear message listing the DefinitionPaths that
  *were* seen (so the user can correct the substring), rather than silently
  showing everything.
- **Multiple matches** (substring hits several definitions) → don't guess.
  Either (a) narrow the home list to just those rows and let the user pick, or
  (b) error listing the ambiguous set. Leaning (a): it composes with the list
  power-tools and needs no new UI.
- **Exactly one match** → the interesting case. Options:
  - open that run's graph directly (matches the "resolve a single run" intent of
    the flag today), or
  - narrow the list to that definition's runs (most recent first) and let the
    user open one.
  Leaning: **one match on its own → still show the (1-definition) list** unless
  combined with something that implies "open it" — simpler, uniform with the
  multi-match case, and avoids a mode where crux jumps straight into a graph the
  user didn't explicitly select. Open question — the user may prefer
  jump-straight-in for a single match.

## Two possible layers (which is `--definition`?)

Worth deciding explicitly, because they behave differently:

1. **A fetch filter** — narrow which runs are *loaded*. `rwx runs list` has no
   `--definition` flag (checked the args in `runs.go`), so this can't be
   server-side; it'd be a **client-side** filter over the fetched page (like the
   view filter). Honest limitation: only sees loaded pages.
2. **A view seed** — exactly the existing `--list-filter` mechanism, which
   already matches `DefinitionPath` (among Title/Branch). In fact
   `--list-filter app-dscout` **already does** most of what's asked, today,
   client-side.

This raises a real design question: **is `--definition` redundant with
`--list-filter`?** Differences that might justify keeping it:
- `--definition` matches *only* the definition path (not Title/Branch), so it's
  precise — `--list-filter ci` could also match a title containing "ci".
- `--definition` carries the "resolve a single run to open" intent that
  `--list-filter` doesn't.

Recommendation to decide: either
- **(A) Wire `--definition` as a definition-only client filter** that (1) narrows
  the list to matching DefinitionPaths and (2) auto-opens when exactly one run
  matches — giving it a distinct job from `--list-filter`; or
- **(B) Drop `--definition`** and document `--list-filter` as the way to filter
  by definition, removing the dead flag entirely.

## Testing

- Unit-test the matcher: `matchDefinition(runs, "app-dscout")` →
  the `.rwx/app-dscout.yml` run(s); case-insensitive; empty term = no filter;
  no-match returns the seen-paths set for the error.
- Model-test the resolution: `--definition` with one match vs. several vs. none.

## Decisions locked / open

- OBSERVED FACT: `--definition` was dead (parsed, never read).
- LOCKED: partial = case-insensitive substring over `DefinitionPath`, consistent
  with the type-to-filter law.

## SHIPPED (2026-07-03)

Chose option (A), as a **persistent DefinitionPath-only view scope** — not an
alias for `--list-filter`, and no auto-open:

- `FilterByDefinition(runs, term)` (listview.go) matches `DefinitionPath` only,
  case-insensitive; empty = passthrough. Distinct from `FilterRunList`'s
  tri-field match, so `--definition ci` won't hit a Title containing "ci".
- `App.defFilter` (from `AppConfig.DefinitionFilter` ← `--definition`) is a
  **persistent scope**: `visibleRuns` = `FilterRunList(FilterByDefinition(runs,
  defFilter), listFilter)`. Typing narrows *within* the scope; **esc clears the
  typed filter but keeps the def scope** (that's why it isn't just a `listFilter`
  seed — a shared field would corrupt on the next keystroke).
- Single vs. multi match are **uniform**: always narrow the list, never
  auto-open. No-match shows an empty list with the scope still in the header
  (`def: X  (0 of N shown)`), not everything.
- Header via `listStatus`: `def: <term>` sits between the fetch scope and the
  typed `filter:`, with the shared `(n of N shown)` count.
- Honest limitation (same as all view filters): only sees loaded pages.

Deferred: a key to drop the def scope interactively (today it's restart-only);
non-substring/exact matching. Revisit if the persistent scope feels trapping.
