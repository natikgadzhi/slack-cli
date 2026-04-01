# Task: Adopt bordered tables with terminal-width adaptation

## Background

CLI_STANDARDS.md now requires all tools to use bordered tables with box-drawing characters that adapt to terminal width. See `~/src/natikgadzhi/template/CLI_STANDARDS.md` for the full spec.

## What to Do

1. Copy the `internal/table/` package from `~/src/natikgadzhi/claude-utils/internal/table/table.go` into this project at `internal/table/table.go`
2. Replace all `output.NewTable()` calls with `table.New()` (or `table.NewWriter()`)
3. Format timestamps in table output as `DD Mon YYYY HH:MM` (e.g., `31 Mar 2026 20:00`) — no timezone, no seconds
4. Ensure the `golang.org/x/term` dependency is available (it likely already is via cli-kit)

## Files to Change

- `internal/commands/users.go` — user list table
- `internal/commands/search.go` — search results table
- `internal/commands/channels_list.go` — channels table
- Any other files using `output.NewTable()`

## Acceptance Criteria

- All table output uses box-drawing borders (`┌┬┐├┼┤└┴┘`)
- Tables adapt to terminal width (shrink widest column, truncate with `…`)
- Timestamps in table mode use `DD Mon YYYY HH:MM` format
- `go build ./...`, `go vet ./...`, `go test ./...` all pass
