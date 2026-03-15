# Task 14: Integration wiring and smoke test

**Phase**: 3 — Testing
**Blocked by**: #10, #12
**Blocks**: #15, #16

## Objective

Final verification that everything works together. Fix any integration issues discovered by running the full test suite and manual testing.

## Acceptance criteria

- [ ] `go build ./...` — compiles cleanly, no warnings
- [ ] `go vet ./...` — passes
- [ ] `go test ./...` — all unit tests pass
- [ ] `go test -race ./...` — no data races
- [ ] `golangci-lint run` — no lint issues (or only accepted exceptions)
- [ ] Manual smoke test of each command with real tokens:
  - `./slack-cli auth check`
  - `./slack-cli message <real-url>`
  - `./slack-cli channel <real-channel> --since 1d --limit 5`
  - `./slack-cli channel <real-channel> --since 1d -o markdown`
  - `./slack-cli search <query> --count 3`
- [ ] Rate limiting works: if 429 encountered during pagination, partial results returned
- [ ] Progress spinner visible during multi-page fetches
- [ ] Cache files written to `~/.local/share/slack-cli/cache/` after commands
- [ ] Binary size is reasonable (< 20MB)

## Notes

- This is a gatekeeper task. Phase 4 (cleanup, CI, Homebrew) should not start until this passes.
- If issues are found, create follow-up tasks or fix them inline.
- The worker on this task should have access to real Slack tokens for manual testing.
