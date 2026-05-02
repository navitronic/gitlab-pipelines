# Plan: Distribute via Homebrew

Make `gitlab-pipelines` installable with `brew install navitronic/tap/gitlab-pipelines`.

## Steps

### 1. Add GoReleaser configuration

Create `.goreleaser.yaml` at the repo root:

- Build `cmd/gitlab-pipelines/` for darwin (arm64, amd64) and linux (arm64, amd64)
- Set binary name to `gitlab-pipelines`
- Archive as `.tar.gz` with README and LICENSE
- Generate checksums
- Create GitHub Release with changelog from commit messages
- Configure the Homebrew tap section (see step 3)

### 2. Add GitHub Actions release workflow

Create `.github/workflows/release.yml`:

- Trigger on tag push matching `v*`
- Checkout code, setup Go
- Run `goreleaser release --clean`
- Requires a `GITHUB_TOKEN` (default) and optionally a `HOMEBREW_TAP_TOKEN` for cross-repo tap updates

### 3. Create Homebrew tap repository

Create a new repo `navitronic/homebrew-tap` on GitHub:

- GoReleaser will auto-commit the formula here on each release
- Formula declares `glab` as a dependency (`depends_on "glab"`)
- Formula installs the pre-built binary (no compilation needed by the user)

### 4. Add a LICENSE file

Homebrew formulae expect a license. Add `LICENSE` (MIT or similar) to the repo root.

### 5. Tag and release

- Tag the current state: `git tag v0.1.0`
- Push the tag: `git push origin v0.1.0`
- GoReleaser builds binaries, creates the GitHub Release, and updates the tap formula

### 6. Verify installation

```sh
brew tap navitronic/tap
brew install gitlab-pipelines
gitlab-pipelines --version  # (needs version flag, see below)
```

## Additional considerations

- **Version flag**: Add `--version` / `-v` flag to `cmd/gitlab-pipelines/main.go` using `ldflags` set by GoReleaser (`-X main.version={{.Version}}`)
- **Runtime dependency**: The formula should note that `glab` must be authenticated (`glab auth login`) before first use
- **Linux support**: GoReleaser handles cross-compilation; Homebrew on Linux (Linuxbrew) works with the same tap
- **Existing `go install` path**: Keep the `go install` instructions in the README as an alternative
