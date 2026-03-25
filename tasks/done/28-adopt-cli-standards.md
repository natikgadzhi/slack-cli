# Task 28: Adopt CLI Standards and cli-kit Library

## Objective

Migrate `slack-cli` to use the `cli-kit` shared library and conform to the unified CLI UX standards defined in `../template/CLI_STANDARDS.md`.

## Context

We are standardizing CLI UX across all Lambda L tools. The standards document is at `../template/CLI_STANDARDS.md`. The shared Go library is at `../template/cli-kit/` (module: `github.com/natikgadzhi/cli-kit`).

Read `../template/CLI_STANDARDS.md` thoroughly before starting.

## Changes Required

### 1. Output Format (`-o/--output`)

**Current:** `-o` flag with values `json`, `markdown`
**Target:** `-o/--output` flag with values `json`, `table`

- Replace the output formatting with `cli-kit/output`
- Add `table` format (human-readable aligned columns for messages, search results, etc.)
- Drop `markdown` as an output format (markdown is for derived cache files, not terminal output)
- Add TTY detection: default to `table` in interactive terminals, `json` when piped
- The `-o` short flag is already correct

### 2. Derived Data Directory (`-d/--derived`)

**Current:** `--output-dir` / `-d` flag, `SLACK_DATA_DIR` env, default `~/.local/share/slack-cli/`
**Target:** `-d/--derived` flag, `SLACK_CLI_DERIVED_DIR` env, `LAMBDAL_DERIVED_DIR` env, default `~/.local/share/lambdal/derived/slack-cli/`

- Replace directory logic with `cli-kit/derived`
- Rename `--output-dir` to `--derived` (short flag `-d` stays the same)
- Rename env var from `SLACK_DATA_DIR` to `SLACK_CLI_DERIVED_DIR` (also respect `LAMBDAL_DERIVED_DIR`)
- Update default path to `~/.local/share/lambdal/derived/slack-cli/`
- Update frontmatter fields to match the standard: `tool`, `object_type`, `slug`, `source_url`, `created_at`, `updated_at`, `command`

### 3. Error Handling

- Replace custom `APIError` and `RateLimitError` types with `cli-kit/errors`
- Use `errors.HandleHTTPError()` for HTTP error responses
- Ensure HTTP 401/403 triggers an auth check and suggests re-authentication
- Ensure HTTP 429 returns all data fetched so far with `"partial": true`
- Use standard exit codes: 0 (success), 1 (error), 2 (auth error)
- Errors in JSON mode should be JSON on stderr

### 4. Version Command

- Replace custom version implementation with `cli-kit/version`
- Ensure both `slack-cli version` and `slack-cli --version` output the same JSON

### 5. Progress Indicators

- Add progress indicators using `cli-kit/progress` for multi-page fetches (channel history, search)
- Progress indicators only show in `table` mode
- Completely suppressed in `json` mode

### 6. Help Text

- Ensure running `slack-cli` with no arguments shows help
- Every flag must show accepted values, default value, and example
- Add usage examples to each subcommand

### 7. Consistent Flag Names

Ensure these global flags exist with exact names:
- `-o, --output` (json, table) — already `-o`, change long form and values
- `-d, --derived` (path) — rename from `--output-dir`
- `--debug` (bool) — add if not present
- `-n, --limit` (integer) — rename from `--limit` on channel, `--count` on search
- `--no-cache` (bool) — already exists

## Acceptance Criteria

- [ ] `slack-cli` uses `cli-kit/output`, `cli-kit/errors`, `cli-kit/derived`, `cli-kit/version`, `cli-kit/progress`
- [ ] `-o json` and `-o table` work correctly
- [ ] TTY detection defaults to `table` interactively, `json` when piped
- [ ] `-d/--derived` sets derived directory, respects `SLACK_CLI_DERIVED_DIR` and `LAMBDAL_DERIVED_DIR`
- [ ] Default derived path is `~/.local/share/lambdal/derived/slack-cli/`
- [ ] HTTP errors handled consistently via cli-kit
- [ ] Partial results returned on 429 with `"partial": true`
- [ ] `slack-cli` with no args shows help
- [ ] Progress indicators show during multi-page fetches in table mode
- [ ] All existing tests pass or are updated
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all clean

## Notes

- The `--count` flag on `search` should be unified with `--limit` on `channel` → both become `-n/--limit`.
- The existing `internal/output` package is small and can be fully replaced.
- The `internal/api/errors.go` with `APIError`/`RateLimitError` should be replaced with cli-kit error types. The rate limit retry logic in `internal/api/client.go` stays, but its error returns integrate with cli-kit.
- The `internal/cache` package keeps file I/O but uses `cli-kit/derived` for paths and frontmatter.
- The user cache (`users.json`) can stay as-is — it's not a derived data file, it's an internal optimization cache.
