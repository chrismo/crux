# Releasing crux

Releases are cut by [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) via
the `release` GitHub Actions workflow, triggered by a `vX.Y.Z` tag. It builds
darwin/linux × amd64/arm64 archives + checksums, publishes a GitHub Release, and
commits the Homebrew cask to `chrismo/homebrew-crux`.

Install target once released: `brew install chrismo/crux/crux`.

## One-time setup

1. **Create the tap repo** `chrismo/homebrew-crux` on GitHub (public, empty),
   then push the scaffold in `../homebrew-crux`:
   ```sh
   cd ../homebrew-crux
   git remote add origin git@github.com:chrismo/homebrew-crux.git
   git push -u origin main
   ```
2. **Add a PAT secret** so GoReleaser can push the cask cross-repo: create a
   fine-grained token with **Contents: read/write** on `chrismo/homebrew-crux`,
   then add it to *this* repo as the secret `HOMEBREW_TAP_GITHUB_TOKEN`
   (`gh secret set HOMEBREW_TAP_GITHUB_TOKEN`).

## Cut a release

```sh
# validate + local dry run (no publish)
goreleaser check
HOMEBREW_TAP_GITHUB_TOKEN=x goreleaser release --snapshot --clean

# real release
git tag v0.1.0
git push origin v0.1.0        # the workflow does the rest
```

## Notes

- The GitHub Release is published on `chrismo/crux` (this repo).
- macOS binaries are unsigned; the cask's postflight strips the Gatekeeper
  quarantine attribute so `crux` runs without a prompt.
