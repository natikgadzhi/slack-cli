# Task 16: GoReleaser + Release Workflow

**Phase**: 4 — Polish
**Blocked by**: #14
**Blocks**: #17

## Objective

Set up GoReleaser and a GitHub Actions release workflow so the project builds and publishes binaries on tag push.

Basic CI (build, vet, test) is already handled by Task #19.

## Release workflow (`.github/workflows/release.yml`)

Triggers: push tag `v*`

Jobs:
- [ ] Run GoReleaser with `goreleaser release --clean`
- [ ] Uses `GITHUB_TOKEN` for GitHub Releases
- [ ] Produces binaries for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64

## `.goreleaser.yml`

- [ ] Binary name: `slack-cli`
- [ ] Builds: CGO_ENABLED=0 for static binaries
- [ ] Archives: tar.gz for linux, zip for darwin
- [ ] Checksum file
- [ ] Homebrew tap integration (auto-update formula in `natikgadzhi/taps`)

## CI enhancements

When this task lands, also add to the existing CI workflow (`.github/workflows/ci.yml`):
- [ ] `golangci-lint run` step
- [ ] `goreleaser build --snapshot` step to verify release config on PRs

## Acceptance criteria

- [ ] Release workflow runs on tag push
- [ ] `.goreleaser.yml` builds correct targets
- [ ] `goreleaser check` passes locally
- [ ] Binaries are statically linked (no runtime dependencies)
- [ ] golangci-lint added to CI

## Notes

- GoReleaser has built-in Homebrew tap support — configure it here, Task #17 creates the tap repo
- Use `goreleaser/goreleaser-action@v5` in GitHub Actions
