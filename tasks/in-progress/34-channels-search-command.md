# Task 34: Add `channels search` subcommand

## Objective

Add a `channels search <query>` command that searches for channels by name substring. Returns matching channels including public, private, mpim, and im conversations.

## Depends On

- Task 32 (channels command group must exist)
- Task 33 (reuse `extractChannelFields` and `deriveChannelType` helpers, table renderer)

## Acceptance Criteria

1. **`channels search` subcommand** registered under `channels` parent:
   - `Use: "search <query>"`, `Short: "Search channels by name"`
   - Requires exactly 1 positional arg (the search string)

2. **Flags**:
   - `--limit, -n` (int, default 20) — max results to return
   - `--type` (string, default "public_channel,private_channel,mpim,im") — types to include (note: broader default than `list` since you're searching for something specific)
   - `--include-archived` (bool, default false) — include archived channels

3. **Implementation**:
   - Paginates `conversations.list` (same API as `channels list`)
   - Client-side case-insensitive substring match on channel `name` and `name_normalized` fields
   - Stops when `--limit` matches are found or all channels exhausted

4. **Output**: Same fields and table format as `channels list`

5. **Progress indicator**: Use spinner: `progress.NewSpinner("Searching channels", format)`

6. **Rate limit handling**: Warn and return partial results

7. **Unit tests** in `channels_search_test.go`:
   - Test `matchesChannelName` helper (case-insensitive substring matching)
   - Test with various channel types

8. **Quality gates**: `go build ./...`, `go vet ./...`, `go test ./...` all pass

## Files to Create/Modify

- `internal/commands/channels_search.go` — new file with search subcommand
- `internal/commands/channels_search_test.go` — new file with unit tests
- `internal/commands/channels.go` — register `search` subcommand (from task 32)

## Notes

- Reuse `extractChannelFields`, `deriveChannelType`, and `renderChannelsTable` from channels_list.go (task 33)
- The search is client-side because Slack doesn't have a channels search API
- Match against both `name` and `name_normalized` fields (same approach as `channels.ResolveChannel`)
