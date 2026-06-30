# RWX TUI — local build monitor with a better Flow graph

## Context

The RWX web UI (`cloud.rwx.com/mint/dscout/runs`) is serviceable but its Flow
dependency-graph viewer is weak — hard to see the critical path, no focused
subgraph filtering, cache/skip reasons are opaque, and tracing a failure to its
logs/blast-radius is clunky. We want a **local TUI** that:

1. Renders a **better Flow dependency-graph viewer** for RWX runs, with four
   specific wins over the web UI (critical-path, focus/filter, cache clarity,
   failure tracing).
2. Optionally fires **macOS notifications** on status changes for a watchlist of
   builds plus the current branch.

This is greenfield tooling that lives in its **own standalone personal repo
(`chrismo/rwxtui`)** — not in the dscout monorepo. Rationale: it's a tool *about*
a repo's CI, not part of any product; the monorepo has no Go toolchain and its
path-filter / tree-hash CI design would have to be taught to ignore it; and you
want to iterate freely without product-CI friction or org governance. It stays
**org-agnostic** — `org` and the target repo's `.rwx/` location are configured
(default `org: dscout`), and it reads any local checkout's configs via a `--dir`
flag. It talks to the RWX org via the already-installed `rwx` CLI (v3.16, authed
as `chris.morris@dscout.com`). All the verified facts below were captured against
the dscout monorepo checkout at `/Users/chrismo/dev/no-linear-rwx-tui`.

### Data layer reality (verified, not assumed)

RWX has **no public list-runs or full-DAG-status API**. Confirmed surface:

- **Static DAG**: parse `.rwx/*.yml` directly. `app-dscout.yml` has 78 tasks;
  each `- key: <k>` is a node, `use: [a, b]` arrays are the edges. Other attrs
  we render: `if:` (conditional/skip), `filter:`, `parallel:`, `call:`, `run:`.
  Cross-file `use:` does not exist — one file = one DAG. (per `.rwx/AGENTS.md`)
- **Live status**: `rwx results <run-id> --output json` →
  `{RunID, ResultStatus, Completed, Prompt?}` (run-level). While in-flight:
  `ResultStatus:"no_result"`. When done, `Prompt` enumerates **failed tasks with
  task-ids** in one call — cheap failure list.
- **Per-task status**: `rwx results <run-id> --task <key> --output json` →
  `{RunID, TaskID, ResultStatus, Completed}`. **~0.35–0.5s per call** (measured),
  so 78 sequential ≈ 30s — must poll concurrently with a bounded pool.
- **Run resolution**: `rwx results --branch <b> --definition .rwx/app-dscout.yml`.
  Multiple defs on one branch → `--definition` is **required** (hard error
  otherwise — handle gracefully).
- **Logs**: `rwx logs <run-id> --task <key> --output <dir>` (downloads/extracts).
- `rwx mcp serve` exposes only `get_run_test_failures` — not a graph API, skip it.
- macOS toast: no `terminal-notifier`; use `osascript -e 'display notification …'`.

## Decisions (from user)

- **Stack**: Go + Bubble Tea / Lipgloss (single static binary; matches `rwx`).
- **Polling**: smart — poll only not-yet-`completed` tasks, back off as they finish.
- **Graph wins**: all four (critical-path, focus/filter, cache clarity, failure tracing).
- **Notifications**: watchlist (pinned runs) + current repo branch, auto.

## Architecture

Standalone Go module in its own repo (`chrismo/rwxtui`), reading a target
checkout's `.rwx/` via `--dir` (default: cwd):

```
rwxtui/                       # repo root
  go.mod
  cmd/rwxtui/main.go          # entrypoint, flag parsing (--dir, --branch, --definition)
  internal/rwx/               # data layer — wraps the rwx CLI
    cli.go                    # exec rwx, parse --output json, handle multi-def error
    dag.go                    # parse .rwx/*.yml → Graph{Nodes, Edges, attrs}
    poll.go                   # bounded concurrent per-task poller w/ backoff
    model.go                  # Run, Task, Status, ResultStatus enums
  internal/graph/             # layout + analysis (pure, unit-tested)
    layout.go                 # topological layered (Sugiyama-lite) coords
    critpath.go               # longest-dependency-chain / gating analysis
    focus.go                  # ancestors+descendants subgraph of a node
  internal/ui/                # Bubble Tea models
    app.go                    # root model, keymap, view routing
    graphview.go              # the Flow viewer (pan/zoom/scroll, render nodes+edges)
    detail.go                 # task detail pane (status, why-ran, logs link)
    runlist.go                # pick run / branch / definition
    watchlist.go              # manage pinned runs
  internal/notify/
    macos.go                  # osascript toast; diff prev→curr status to fire
  internal/config/
    config.go                 # ~/.config/rwxtui/config.yml: watchlist, org, poll interval
```

### Data layer (`internal/rwx`)

- `cli.go`: thin `exec.Command("rwx", …)` wrapper. Always `--output json`.
  Strip the "new release available" stderr noise. Detect the multi-definition
  error and surface the definition choices to the UI (don't crash).
- `dag.go`: parse YAML with `gopkg.in/yaml.v3` into a `Graph`. A node carries
  `key, use[], if, filter[], parallel, call, runSnippet`. The `if`/`filter`/`call`
  fields drive the **cache-clarity** rendering (e.g. a task with an `if:` that
  resolved false renders as "skipped").
- `poll.go`: the smart poller. Cycle:
  1. One run-level call → overall status + (if done) parse `Prompt` failed-task list.
  2. For each task still `!completed`, fire `rwx results --task <key>` through a
     bounded worker pool (~8–10 concurrent). Mark completed tasks done; stop
     polling them. Emits a `StatusMsg` into Bubble Tea via a channel.
  Backoff widens the interval as the active-set shrinks.

### Graph viewer (`internal/ui/graphview.go` + `internal/graph`)

Layered top-down layout (roots → leaves) computed in `graph/layout.go`
(topological layering + simple barycenter ordering to reduce crossings). Render
with Lipgloss boxes for nodes and ASCII/Unicode connectors for edges, inside a
scrollable/pannable viewport. The four wins:

- **Critical-path**: `critpath.go` computes the longest chain by task duration
  (duration from completed-task timing where available; fall back to edge depth).
  Bold/colored that chain; show total in a status bar.
- **Focus/filter**: `/` to type-filter task keys; `f` on a selected node isolates
  its ancestor+descendant subgraph (`focus.go`) and dims the rest.
- **Cache clarity**: per-node glyph/color for `cache-hit | ran | skipped(if=false) |
  filtered | running | failed | pending` — derived from status + the parsed
  `if:`/`filter:` attrs.
- **Failure tracing**: `x` jumps to the first failed task; `enter` opens the
  detail pane with a "download logs" action (`rwx logs`) and highlights the
  downstream blast radius (descendants of the failed node).

### Notifications (`internal/notify`)

The poller keeps a prev-status map per watched run. On any task or run-level
transition (e.g. `running → failed`, run `succeeded`), fire an `osascript`
toast. Watched set = config watchlist + auto-resolved current-branch run. Toggle
with a CLI flag / config so it can be disabled.

### Config (`internal/config`)

`~/.config/rwxtui/config.yml`: `org` (default `dscout`), `defaultDir` (path to a
checkout's repo root, for resolving `.rwx/`), `defaultDefinition`,
`pollIntervalSec`, `maxConcurrentPolls`, `notify: bool`, `watchlist: [run-ids]`.
Org-agnostic by design — nothing dscout-specific is hardcoded.

## Build order (incremental, each step runnable)

The repo already exists locally at `/Users/chrismo/dev/rwx-tui` (git-initialized
today). Work happens there.

0. **Commit this plan** into `/Users/chrismo/dev/rwx-tui` (e.g. as `PLAN.md`) as
   the first commit, so the design is captured in the repo before code lands.
1. **Module + skeleton**: in `/Users/chrismo/dev/rwx-tui`, `go mod init`, Bubble Tea
   hello-world, flag parsing (`--dir`, `--branch`, `--definition`).
2. **Data layer read-only**: `dag.go` (parse `<dir>/.rwx/app-dscout.yml` → print
   node/edge counts to validate against the known 78 tasks) + `cli.go` run
   resolution. Point `--dir` at `/Users/chrismo/dev/no-linear-rwx-tui` to test.
3. **Static graph render**: layered layout + viewport, no live status yet.
4. **Live status**: smart poller → color nodes by status; status bar shows
   run-level result. Handle the multi-definition picker.
5. **Four graph wins**: critical-path, focus/filter, cache glyphs, failure jump.
6. **Detail pane + logs**: `rwx logs` integration.
7. **Notifications**: macOS toasts on transitions; watchlist UI + config.

## Verification

- **Unit tests** (`go test ./...`) for the pure logic — most valuable where bugs
  hide: `dag.go` (parse the real `.rwx/app-dscout.yml`, assert 78 nodes and a few
  known edges like `axon-compile use: [code, elixir, system-packages,
  axon-deps-get]`), `graph/layout.go` (no cycles, stable layering), `critpath.go`
  (longest chain on a hand-built fixture), `focus.go` (ancestor/descendant sets).
  No existing test suite to extend — this is new code, so tests ship with it.
- **CLI-layer tests** use a fake `rwx` (inject the exec function) returning the
  captured real JSON shapes (run-level `no_result`; per-task `succeeded`;
  completed `Prompt` with a failed task) — no network in tests.
- **End-to-end manual run** against the live org: `go run ./cmd/rwxtui
  --dir /Users/chrismo/dev/no-linear-rwx-tui --branch main
  --definition .rwx/app-dscout.yml` — confirm the graph renders,
  statuses populate, a known failed run (e.g. the captured
  `0ceabde9…` with `axon-build` failed) highlights `axon-build` and its blast
  radius, and the critical path is plausible.
- **Notification smoke test**: point at a finished run, force a synthetic
  prev→curr transition, confirm the macOS toast appears via `osascript`.

## Open risks / notes

- Per-task polling is the only live-status path and it's process-spawn heavy;
  the bounded pool + smart backoff is the mitigation. If it proves too slow on
  78 tasks, fall back to run-level polling + `Prompt`-derived failures only, and
  fetch per-task status lazily on selection.
- Timing data for true critical-path may be absent from the sparse JSON; the
  plan falls back to topological depth when durations aren't available.
- Standalone personal repo (`chrismo/rwxtui`), so it never entangles the dscout
  monorepo's path-filter / tree-hash CI. Kept org-agnostic so it could later be
  pointed at any RWX org or open-sourced.
