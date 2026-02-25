---
name: release
description: Cut a new release of mardi-gras. Runs tests, lint, vet, creates a semver tag, pushes to trigger GoReleaser CI, and monitors the workflow.
disable-model-invocation: true
argument-hint: "[version, e.g. v0.2.1 or 'patch'/'minor'/'major']"
---

# Release Mardi Gras

Cut a new release. Argument is either an explicit semver tag (e.g. `v0.2.1`) or a bump keyword (`patch`, `minor`, `major`).

## Pre-flight checks

Run ALL of these before tagging. Stop on any failure.

```bash
make test          # Full test suite
go vet ./...       # Static analysis
make fmt           # Formatting check — verify no files changed
```

If `golangci-lint` is available, also run `make lint`.

Check for uncommitted changes — the working tree MUST be clean on `main`:

```bash
git status
git diff --stat
```

If not on `main`, warn the user and confirm before proceeding.

## Determine version

1. Get the latest tag: `git tag --sort=-v:refname | head -1`
2. If argument is `patch`/`minor`/`major`, bump the appropriate component
3. If argument is an explicit `vX.Y.Z`, use it directly
4. If no argument, suggest a version based on commits since last tag:
   - Any `feat:` commits → suggest minor bump
   - Only `fix:`/`chore:`/`docs:` → suggest patch bump

## Create the release

1. Review what's shipping — show commits since last tag:
   ```bash
   git log <last-tag>..HEAD --oneline --no-merges
   ```

2. Write a concise annotated tag message summarizing the release. Group changes into categories (features, fixes, compatibility). Keep it to 3-5 lines.

3. Create the annotated tag:
   ```bash
   git tag -a <version> -m "<message>"
   ```

4. Push the tag to trigger the release workflow:
   ```bash
   git push origin <version>
   ```

## Monitor the release

1. Watch the GitHub Actions workflow:
   ```bash
   gh run list --limit 1
   gh run watch <run-id> --exit-status
   ```

2. Verify the release assets after completion:
   ```bash
   gh release view <version> --json name,assets --jq '.name, (.assets[] | "  " + .name + " (" + .state + ")")'
   ```

3. Expected assets (6 binaries + checksums):
   - `mardi-gras_<ver>_darwin_amd64.tar.gz`
   - `mardi-gras_<ver>_darwin_arm64.tar.gz`
   - `mardi-gras_<ver>_linux_amd64.tar.gz`
   - `mardi-gras_<ver>_linux_arm64.tar.gz`
   - `mardi-gras_<ver>_windows_amd64.zip`
   - `mardi-gras_<ver>_windows_arm64.zip`
   - `checksums.txt`

## Post-release

Report the release URL to the user:
```
https://github.com/quietpublish/mardi-gras/releases/tag/<version>
```

## Config reference

- **GoReleaser config**: `.goreleaser.yaml`
- **CI workflow**: `.github/workflows/release.yml`
- **Homebrew tap**: `matt-wright86/homebrew-tap` (updated automatically by GoReleaser)
- **Version injection**: `-ldflags "-X main.version=<version>"` in `cmd/mg`
