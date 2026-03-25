# Task 32: Refactor `channel` to `channels` command group with `get` subcommand

## Objective

Refactor the existing `channel` command into a `channels` command group. The current `channel <name|id>` behavior becomes `channels get <name|id>`. Keep `channel` as a deprecated alias for backward compatibility.

## Acceptance Criteria

1. **New `channels` parent command** in `internal/commands/channels.go`:
   - `Use: "channels"`, `Short: "Manage and view Slack channels"`
   - Registers as a subcommand of `rootCmd`
   - Has subcommands: `get` (this task), `list` (task 33), `search` (task 34)

2. **`channels get` subcommand**:
   - Moves the existing `runChannel` logic from `channel.go` into a `get` subcommand
   - `Use: "get <name|id>"`, same flags (`--since`, `--until`, `--limit`)
   - Identical behavior to current `channel` command

3. **Backward compatibility**:
   - Keep `channel` as a hidden alias that behaves identically to `channels get`
   - The old `channel <name|id>` usage should still work
   - Mark the alias with `Hidden: true` and `Deprecated: "use 'channels get' instead"`

4. **Update root command examples** in `root.go` to use `channels get` instead of `channel`

5. **Update e2e tests** in `tests/e2e_test.go`:
   - `TestHelp` should check for `channels` instead of `channel`
   - `TestChannelSmoke` and `TestChannelMarkdown` should use `channels get` instead of `channel`

6. **Quality gates**: `go build ./...`, `go vet ./...`, `go test ./...` all pass

## Files to Modify

- `internal/commands/channel.go` → rename to `internal/commands/channels.go`, restructure
- `internal/commands/root.go` — update examples
- `tests/e2e_test.go` — update test commands

## Notes

- The `renderMessagesTable`, `renderSearchTable`, `truncate` helper functions currently in `channel.go` should move to a shared location or stay in the channels file since they're used by search too.
- `renderSearchTable` is used by `search.go` — keep it accessible (it can stay in channel.go/channels.go or move to a helpers file).
- The cache slug uses "channel" — keep that unchanged for cache compatibility.
