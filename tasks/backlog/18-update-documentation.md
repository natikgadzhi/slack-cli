# Task 18: Update README and documentation

**Phase**: 4 — Polish
**Blocked by**: #15

## Objective

Rewrite all documentation to reflect the Go version. Remove Python references.

## README.md

Rewrite to cover:

- [ ] Project description (same: "Slack read-only CLI for fetching messages, threads, and history")
- [ ] Installation:
  - Homebrew: `brew tap natikgadzhi/taps && brew install slack-cli`
  - From source: `go install github.com/natikgadzhi/slack-cli/cmd/slack-cli@latest`
  - From release: download binary from GitHub Releases
- [ ] Auth setup (same as before — keychain or env vars)
- [ ] Usage examples (same commands but no `uv run` prefix):
  ```
  slack-cli --help
  slack-cli auth check
  slack-cli message 'https://...'
  slack-cli channel general --since 2d --limit 100
  slack-cli channel C12345678 --since 2026-03-01 --until 2026-03-10
  slack-cli search "deployment failed" --count 10
  ```
- [ ] Output formats: `-o json` (default), `-o markdown`
- [ ] Cache: explain `~/.local/share/slack-cli/cache/`, `--no-cache` flag
- [ ] Dev section:
  ```
  go build ./...
  go test ./...
  go test -race ./...
  go vet ./...
  golangci-lint run
  make e2e  # requires SLACK_XOXC, SLACK_XOXD env vars
  ```

## CLAUDE.md

- [ ] Update `@README.md` reference (it auto-includes)
- [ ] Verify worker/reviewer/lead agent instructions still make sense for Go
- [ ] Update quality check commands from Python to Go

## Acceptance criteria

- [ ] README.md fully rewritten for Go
- [ ] No references to Python, uv, pip, pyproject.toml, ruff, pyright
- [ ] CLAUDE.md updated
- [ ] PROJECT_PROMPT.md left untouched
