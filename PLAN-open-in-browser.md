# Open-in-browser — deep links out

> **STATUS: SHIPPED** (ctrl+o 16545e5, ctrl+g 917fbad, no-op notice 1d92ff2).
> Both keys work in the list and the graph/detail, verified end-to-end against a
> real git-triggered run (see "Verified" below). Kept as the design record.

Two "jump to the source of truth" affordances the user asked for (2026-07-02):

- **Open the run on cloud.rwx.com** (`ctrl+o`) — the RWX build page for the
  selected/open run.
- **Open the commit on GitHub** (`ctrl+g`) — the VCS commit the run was triggered
  from.

Both are *run-scoped* (they belong to a run, not a graph node), and both apply
in the home run list (the selected row) and the graph/detail view (the open
run).

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

- **Phase 1 — rwx build link. SHIPPED (2026-07-06).** `ctrl+o` opens the run's
  cloud.rwx.com page. Key audit confirmed bare letters are all eaten by
  type-to-filter, but ctrl-chords are wide open (only ^C/^R taken) — chose
  `ctrl+o` ("open"), not the `ctrl+b`/`ctrl+g` pair, since cloud.rwx.com is *the*
  page. Details below.
- **Phase 2 — GitHub commit link. SHIPPED (2026-07-06).** `ctrl+g` opens the
  run's commit on GitHub. `{owner}/{repo}` base inferred once at startup from the
  git remote (`GithubBaseURL(dir)`, ssh + https + ssh:// forms); non-GitHub hosts
  and missing sha/remote no-op gracefully. `commitURL(base, sha)`. Works in list
  (row's CommitSha), graph, and detail (the open run's CommitSha — no carry
  needed, `rwx results` provides it). Keybar `^g commit`. Tested:
  `parseGithubRemote`, `commitURL`, and list/graph dispatch + no-op.
  `ctrl+o` stays rwx, `ctrl+g` is GitHub.

Also in this pass: the opener moved to `browser.go` and now **waits and reports
failures** (was fire-and-forget `.Start()` swallowing errors) — a failed launch
surfaces as `error: open browser: …` in the footer (list and graph) instead of
looking like the key did nothing.

### Phase 1 as built

- `openInBrowser(url)` (browser.go) shells to `open` (darwin) / `xdg-open`
  (else), **waiting** and capturing stderr so a failure surfaces (not the
  original fire-and-forget). Injected as `App.openURL func(string) error` so
  tests assert the dispatched URL with a spy (no real browser launch).
- `openURLCmd(open, url)` runs it off the update loop, capturing only opener+url
  (not the model), and no-ops on empty url.
- List: `ctrl+o` opens `selectedRun().RunUrl` (ready-made). Graph/detail:
  `ctrl+o` opens `App.runUrl`, which is **carried** from the summary when a run
  is opened from the list (the `rwx results` payload has no URL). `--run <id>`
  startup has no summary, so graph `ctrl+o` is a no-op there (acceptable).
- Keybar shows `^o web` in the list, graph, and detail keybars; locked by
  `TestFooterKeybarByMode`. Dispatch covered by `TestCtrlOOpensRunUrlFromList`,
  `TestCtrlOOpensRunUrlFromGraph`, `TestOpeningRunFromListCarriesRunUrl`,
  `TestCtrlONoopWithoutUrl`.

### No-op notice (1d92ff2)

An advertised key that silently does nothing reads as broken. When there's
nothing to open, a transient footer note (`theme.Special`, cleared on the next
key) explains why:

- `ctrl+g`, run has no commit → `this run has no commit (CLI-triggered)`
- `ctrl+g`, no GitHub remote → `no GitHub remote — can't link a commit`
- `ctrl+o`, no RunUrl → `no rwx page for this run`

`commitNotice(base, sha)` picks the message; `App.notice` renders above the
footer. Locked by `TestCtrlGNoticeWhenRunHasNoCommit`, `TestNoticeClearsOnNextKey`.

### Verified (2026-07-06)

Discovered that **every run in this repo was `Trigger=cli`** (no commit sha), so
`ctrl+g` no-op'd everywhere — CLI runs (`rwx run`) carry no commit by design.
The fix was infra, not code: the repo's `.rwx/ci.yml` already had the
`on.github.push` trigger, but the **RWX GitHub App wasn't installed** (proven by
0 check-runs across 8 pushed commits). After installing it, a push produced a
`github.push` run with a real `CommitSha`, RWX reported `success` back to the
commit, and `ctrl+g` opened `github.com/chrismo/crux/commit/<sha>` — confirmed
live in both list and graph. **Lesson: verify a feature against real data, not a
synthetic fixture with a hand-set sha.**

## Decisions locked / open

- LOCKED: run-scoped (not node-scoped); available in list + graph + detail; build
  URL carried from the summary (not reconstructed from RunID).
- LOCKED: `ctrl+o` rwx page, `ctrl+g` GitHub commit; both shipped.
- LOCKED: GitHub base inferred from the git remote (github.com only); missing
  sha/remote → transient notice, no `--repo-url` flag needed for now.
- OPEN: whether `--print` emits the URLs as text.
