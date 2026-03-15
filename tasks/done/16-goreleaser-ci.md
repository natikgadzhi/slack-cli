# Task 16: GoReleaser + GitHub Actions CI

**Phase**: 4 — Polish
**Blocked by**: #14
**Blocks**: #17

## Objective

Set up automated CI and release pipeline so the project builds, tests, and releases binaries on tag push.

## CI workflow (`.github/workflows/ci.yml`)

Triggers: push to main, pull requests

Jobs:
- [ ] `build`: `go build ./...`
- [ ] `vet`: `go vet ./...`
- [ ] `test`: `go test -race ./...`
- [ ] `lint`: `golangci-lint run`
- [ ] Matrix: Go latest stable, on ubuntu-latest

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

## Acceptance criteria

- [ ] CI workflow runs on PR and push to main
- [ ] Release workflow runs on tag push
- [ ] `.goreleaser.yml` builds correct targets
- [ ] `goreleaser check` passes locally
- [ ] Binaries are statically linked (no runtime dependencies)

## Notes

- GoReleaser has built-in Homebrew tap support — configure it here, Task #17 creates the tap repo
- Use `goreleaser/goreleaser-action@v5` in GitHub Actions
- Consider adding a `goreleaser build --snapshot` step in CI to verify the config
