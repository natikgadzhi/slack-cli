# Task 19: GitHub Actions CI Workflow

**Phase**: 1.5 — Infrastructure (should exist from day one)
**Blocked by**: none
**Blocks**: none (but all future PRs benefit)

## Objective

Add a GitHub Actions CI workflow that runs build, vet, and tests for the Go codebase. This ensures every PR is validated before merge.

Task 16 (GoReleaser + release workflow) remains separate and will be done in Phase 4.

## CI workflow (`.github/workflows/ci.yml`)

Triggers: push to `main`, pull requests to `main`

### Go job

- [ ] Set up Go 1.26.x
- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test -race ./...`
- [ ] Runs on ubuntu-latest

## Acceptance criteria

- [ ] CI triggers on push to main and on PRs
- [ ] Go build, vet, and test pass
- [ ] Workflow is minimal and fast (no unnecessary steps)

## Notes

- No golangci-lint yet — it can be added when the Go codebase is more mature (Task 16)
- This task was split out from Task 16 to get CI running early
