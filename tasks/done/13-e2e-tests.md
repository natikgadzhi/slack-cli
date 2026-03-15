# Task 13: End-to-end tests

**Phase**: 3 — Testing
**Blocked by**: #10
**Blocks**: (none directly, but should pass before Phase 4)

## Objective

Write E2E tests that build the real binary and run it against the Slack API. Gated behind a build tag and env vars so they don't run in CI without credentials.

## Design

- Build tag: `//go:build e2e`
- File: `tests/e2e/e2e_test.go` (or `cmd/slack-cli/e2e_test.go`)
- Required env vars:
  - `SLACK_XOXC` — valid xoxc token
  - `SLACK_XOXD` — valid xoxd token
  - `SLACK_TEST_MESSAGE_URL` — a known Slack message URL to fetch
  - `SLACK_TEST_CHANNEL` — a channel name or ID to query
  - `SLACK_TEST_SEARCH_QUERY` — a search term known to return results
- If any required env var is missing, skip the test (not fail)

## Test cases

- [ ] `slack-cli auth check` — exit code 0, output contains `[OK]` and `authenticated`
- [ ] `slack-cli message $SLACK_TEST_MESSAGE_URL` — exit 0, valid JSON output, has `ts` and `text` fields
- [ ] `slack-cli message $SLACK_TEST_MESSAGE_URL -o markdown` — exit 0, output contains markdown headers
- [ ] `slack-cli channel $SLACK_TEST_CHANNEL --since 7d --limit 5` — exit 0, valid JSON array, len <= 5
- [ ] `slack-cli channel $SLACK_TEST_CHANNEL --since 7d --limit 5 -o markdown` — exit 0, markdown output
- [ ] `slack-cli search "$SLACK_TEST_SEARCH_QUERY" --count 3` — exit 0, valid JSON array
- [ ] `slack-cli --help` — exit 0, output contains all command names
- [ ] `slack-cli auth --help` — exit 0, lists subcommands
- [ ] Invalid URL → non-zero exit code
- [ ] Missing token (unset env) → non-zero exit code with helpful error

## Acceptance criteria

- [ ] Tests in file with `//go:build e2e` tag
- [ ] `go test -tags e2e ./...` runs them when env vars are set
- [ ] `go test ./...` (without tag) skips them entirely
- [ ] `make e2e` target runs them
- [ ] Each test builds the binary fresh via `go build` in TestMain
- [ ] Tests clean up any temp files they create

## Notes

- Build the binary once in `TestMain` to a temp dir, reuse across all tests
- Use `os/exec` to run the binary as a subprocess
- Parse stdout to validate output format
- These tests are slow (real API calls) — keep the count small and focused
