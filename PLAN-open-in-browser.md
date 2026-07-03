# Open-in-browser — deep links out (proposals)

Two "jump to the source of truth" affordances the user asked for (2026-07-02):

- **Open the run on cloud.rwx.com** — the RWX build page for the selected/open run.
- **Open the commit on GitHub** — the VCS commit the run was triggered from.

Both are *run-scoped* (they belong to a run, not a graph node), and both apply
in **two places**: the home run list (the selected row) and the graph
detail/logs view (the open run). Planner proposals, not locked decisions —
grounded against current code so the coder can pick one up on greenlight.

## Grounding (what data we actually have)

Checked the rwx models and fixtures (`internal/rwx/runs.go`, `model.go`, and the
captured `testdata/*.json`):

- **List rows** (`rwx runs list`, `rwx.RunSummary`) carry, per run:
  - `RunUrl` (runs.go:20) — the **ready-made** cloud.rwx.com URL, e.g.
    `https://cloud.rwx.com/mint/clabs/runs/{id}`. No construction needed.
  - `CommitSha` (runs.go:18), `RepositoryName` (runs.go:19), `Branch`.
  - Note: some rows have **empty** `CommitSha`/`RepositoryName` (seen in
    `runs_list.json`) — the affordance must no-op gracefully when a field is
    missing.
- **Detail** (`rwx results`, `rwx.Run`) carries `RunID` (model.go:11),
  `CommitSha` (model.go:17), `CommitMessage`, `Branch`, `RepositoryName` — but
  **NOT `RunUrl`** (confirmed absent from the `rwx results` payload, not just
  unmapped). And `openRunCmd` (app.go:621) opens by **`r.ID` only** (app.go:898),
  dropping the summary that had `RunUrl`.

### The two hard parts (why this isn't a one-liner)

1. **cloud.rwx.com URL in the detail view.** `RunUrl` exists only on the list
   summary. It embeds an **org slug** (`clabs`) that is *not* derivable from
   `RunID` alone, so we can't reconstruct it in the detail view from the run ID.
   → **Carry `RunSummary.RunUrl` into the opened-run state** when the list opens
   a run (thread it through `runOpenedMsg`/the model alongside the fetched
   `Run`). For a run opened directly via `--run <id>` (no summary in hand), the
   build link is simply unavailable — no org slug to build it from. Acceptable;
   no-op there.

2. **GitHub commit URL is under-specified by rwx data.** `RepositoryName` is a
   **bare name** (`"crux"`) — no owner/org, no host. `https://github.com/{owner}/
   {repo}/commit/{sha}` needs the owner and assumes github.com. rwx doesn't give
   us either. Options, best first:
   - **Infer from the local git remote.** crux already runs in a checkout
     (`--dir`, default cwd). `git remote get-url origin` →
     `git@github.com:chrismo/crux.git` / `https://github.com/chrismo/crux.git`,
     parse to a `https://github.com/{owner}/{repo}` base, append
     `/commit/{CommitSha}`. Handles github.com naturally and degrades (unknown
     host → no link). Grounded and zero-config for the common case.
   - **Explicit config**: `--repo-url <base>` flag / env for when the checkout
     isn't present or the remote isn't the trigger source.
   - Sanity-check `RepositoryName` against the parsed remote's repo when both
     exist, to avoid linking the wrong repo.
   - Open question: non-GitHub hosts (GitLab/Bitbucket) — different commit URL
     shape. v1 can be **github.com only**; other hosts → no link (log/no-op).

## Mechanism

- **Opening a URL**: Go has no stdlib opener. Shell out per-OS — macOS `open`,
  Linux `xdg-open` (crux targets macOS + Linux). One small `openBrowser(url)`
  helper in a new `internal/ui` (or `internal/sys`) file. **Inject the opener**
  (a `func(string) error` field on the model, default = real) so tests assert
  the URL without launching a browser.
- **URL building is pure** — `buildURL`-style funcs (rwx URL passthrough, GitHub
  commit URL from remote + sha) are unit-testable with no I/O. Test those; test
  the key handler dispatches the right URL to the injected opener.

## Interaction (keys) — the type-to-filter constraint

Under crux's "one law: just type to filter", **letters are filter characters**
in both list and graph, so these can't be `g`/`b`. Non-letter keys are mostly
taken (space=pin, tab=scope, enter=open, esc=back, backspace=list,
ctrl+r=refresh, ctrl+c=quit, arrows, g/G=top/bottom).

Leading proposal: **ctrl-chords**, mnemonic and conflict-free —
- `ctrl+b` → open the **build** (cloud.rwx.com)
- `ctrl+g` → open the **commit** on **GitHub**

Available in list mode (acts on `selectedRun()`) and in the graph detail/graph
mode (acts on the open run). Add both to the mode keybars (`keys.go`
`ShortHelp`), and only advertise a link when its URL is actually resolvable
(e.g. hide `ctrl+g` when there's no remote/sha). Alternative if two chords feel
heavy: a single `ctrl+o` "open…" that shows a tiny target chooser — heavier,
probably unnecessary for two targets.

## --print

`--print` has no browser. Optionally **emit the resolved URLs as text** (a
`build: <url>` / `commit: <url>` line) so the headless path still surfaces them.
Low priority; decide when building.

## Phases

- **Phase 1 — rwx build link.** Carry `RunUrl` through the open path; `ctrl+b` in
  list + detail; `openBrowser` helper with injected opener. The easy half
  (URL is ready-made). Snapshot/unit-test the wiring.
- **Phase 2 — GitHub commit link.** Git-remote inference → `{owner}/{repo}`
  base; `ctrl+g` in list + detail; graceful no-op on missing sha/remote/non-
  GitHub host. Unit-test the remote-URL parser (ssh + https forms) and the
  commit-URL builder.

## Decisions locked / open

- LOCKED: run-scoped (not node-scoped); available in both list and detail;
  build URL carried from the summary (not reconstructed from RunID).
- OPEN: key choice (`ctrl+b`/`ctrl+g` chords vs. an `ctrl+o` chooser).
- OPEN: GitHub owner/host source — git remote inference (recommended) vs.
  explicit `--repo-url`; non-GitHub hosts deferred (v1 = github.com only).
- OPEN: whether `--print` emits the URLs as text.
