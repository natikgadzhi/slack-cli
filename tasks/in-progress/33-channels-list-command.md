# Task 33: Add `channels list` subcommand

## Objective

Add a `channels list` command that returns a paginated list of conversations (channels, groups, DMs, MPIMs) the authenticated user has access to.

## Depends On

- Task 32 (channels command group must exist)

## Acceptance Criteria

1. **`channels list` subcommand** registered under the `channels` parent command:
   - `Use: "list"`, `Short: "List channels and conversations"`
   - No positional args required

2. **Flags**:
   - `--limit, -n` (int, default 100) — max channels to return
   - `--type` (string, default "public_channel,private_channel") — comma-separated conversation types to include. Valid: `public_channel`, `private_channel`, `mpim`, `im`
   - `--include-archived` (bool, default false) — include archived channels

3. **API**: Uses `conversations.list` endpoint with cursor-based pagination:
   - Params: `limit` (page size, use 200), `types`, `exclude_archived` (inverse of --include-archived), `cursor`
   - Collects channels from `channels` key in response
   - Pagination via `response_metadata.next_cursor`

4. **Output fields** per channel:
   - `id` — channel ID
   - `name` — channel name (or computed name for IMs/MPIMs)
   - `topic` — channel topic (from `topic.value`)
   - `purpose` — channel purpose (from `purpose.value`)
   - `num_members` — member count
   - `is_archived` — whether archived
   - `type` — derived: "public_channel", "private_channel", "mpim", or "im"

5. **Table output**: Columns: ID, NAME, TYPE, MEMBERS, TOPIC (truncated to 60 chars)

6. **JSON output**: Array of channel objects with all fields above

7. **Progress indicator**: Use `progress.NewCounter("Fetching channels", format)` pattern from users.go

8. **Rate limit handling**: Warn and return partial results (same pattern as users.go)

9. **Unit tests** in `channels_list_test.go`:
   - Test `extractChannelFields` helper
   - Test `deriveChannelType` helper
   - Test filtering logic

10. **Quality gates**: `go build ./...`, `go vet ./...`, `go test ./...` all pass

## Files to Create/Modify

- `internal/commands/channels_list.go` — new file with list subcommand
- `internal/commands/channels_list_test.go` — new file with unit tests
- `internal/commands/channels.go` — register `list` subcommand (from task 32)

## Reference

- Follow the pagination pattern from `users.go` (lines 56-95)
- Follow the table rendering pattern from `users.go` (`renderUsersTable`)
- Slack API: `conversations.list` returns `channels[]` with `response_metadata.next_cursor`
