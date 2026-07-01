# Releasing crux

Releases are cut **locally** with [GoReleaser](https://goreleaser.com)
(`.goreleaser.yaml`) — no CI, no PAT. It cross-compiles darwin/linux ×
amd64/arm64, publishes a GitHub Release on `chrismo/crux`, and commits the
Homebrew cask to `chrismo/homebrew-crux`. GoReleaser authenticates with your
local `gh` token (`gh auth token`), which has `repo` scope and can write both
repos. (This mirrors how grdy releases: build locally, push the tap from your
own machine.)

Install target once released: `brew install chrismo/crux/crux`.

## One-time setup

The tap repo `chrismo/homebrew-crux` already exists. Push the local scaffold once
so it has its README (GoReleaser adds `Casks/crux.rb` on the first release):

```sh
cd ../homebrew-crux
git remote add origin git@github.com:chrismo/homebrew-crux.git   # if not set
git push -u origin main
```

## Cut a release

```sh
./build.sh snapshot          # optional pre-flight: build all platforms, no publish
./build.sh release v0.1.0    # validate + test, then tag, push, and publish
```

`release` refuses to run on a dirty tree or an existing tag, runs
`goreleaser check` + vet + tests before tagging, then publishes the GitHub
Release and updates the tap using your `gh` token. Afterward:
`brew install chrismo/crux/crux` (or `brew upgrade crux`).

Equivalent raw commands, if you'd rather not use the script:

```sh
git tag v0.1.0 && git push origin v0.1.0
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

## Notes

- macOS binaries are unsigned; the cask's postflight strips the Gatekeeper
  quarantine attribute so `crux` runs without a prompt.
- To move releases to CI later: re-add a workflow and a cross-repo PAT
  (`HOMEBREW_TAP_GITHUB_TOKEN`) as the cask `repository.token` in
  `.goreleaser.yaml` — CI's default `GITHUB_TOKEN` can't push to another repo.
