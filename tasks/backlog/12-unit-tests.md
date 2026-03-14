# Task 12: Unit tests for all packages

**Phase**: 3 — Testing
**Blocked by**: #10, #11
**Blocks**: #14

## Objective

Ensure comprehensive unit test coverage across all packages. Each package should have been tested as it was built (tasks #2-#11), but this task is for filling gaps, adding edge cases, and ensuring consistency.

## Test inventory

Review and augment tests in each package:

### `internal/config`
- Defaults when no env vars set
- Each env var override works
- Path expansion (`~` → home dir)

### `internal/auth`
- SanitizeToken: 7 cases from Python (already in #3)
- KeychainGet/Set with mocked exec
- GetXoxc/GetXoxd: env var priority over keychain

### `internal/api`
- Correct headers on request
- Params encoding, nil exclusion
- HTTP 429 → retry with backoff
- 429 during pagination → partial results + error
- Non-429 errors → APIError
- Team URL caching
- Spaced query delay (verify timing with mock)

### `internal/formatting`
- All 19 Python test cases ported (already in #5)
- Additional edge cases: very long URLs, empty strings, unicode text

### `internal/channels`
- ID passthrough, hash strip, name resolution, pagination, not-found

### `internal/users`
- Cache hit, cache miss + API, cache file creation, API failure fallback

### `internal/output`
- JSON valid, Markdown format correct, empty input, special characters

### `internal/cache`
- Roundtrip, frontmatter parsing, missing file, auto-mkdir

## Acceptance criteria

- [ ] Every package has `_test.go` files
- [ ] `go test ./...` passes with 0 failures
- [ ] `go test -race ./...` passes (no data races)
- [ ] No tests depend on external services (all mocked)
- [ ] Test coverage is reasonable (check with `go test -cover ./...`)

## Notes

- This is a review/gap-fill task. Workers on earlier tasks should have written tests alongside their code.
- Focus on edge cases and error paths that individual task workers might have missed.
