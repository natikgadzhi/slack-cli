# Task 1: Bootstrap Go project

**Phase**: 0 — Bootstrap
**Blocked by**: none
**Blocks**: #2, #3, #5

## Objective

Initialize the Go module, directory structure, and build tooling. Python source stays in place until the Go rewrite is verified.

## Acceptance criteria

- [ ] `go mod init github.com/natikgadzhi/slack-cli` succeeds
- [ ] Directory layout created:
  ```
  cmd/slack-cli/main.go
  internal/api/
  internal/auth/
  internal/config/
  internal/formatting/
  internal/channels/
  internal/users/
  internal/commands/
  internal/output/
  internal/cache/
  ```
- [ ] `main.go` has a minimal `func main()` that compiles
- [ ] `Makefile` has targets: `build`, `test`, `vet`, `lint` (golangci-lint), `e2e`
- [ ] `.goreleaser.yml` present with darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
- [ ] `.gitignore` updated for Go binary artifacts
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes

## Notes

- Use Go 1.22+ (or latest stable)
- Add `cobra` dependency: `go get github.com/spf13/cobra`
- Add `golangci-lint` as a dev tool (not a Go dependency)
- Keep all Python files untouched
