# Task 10: Implement all CLI commands

**Phase**: 2 — Features
**Blocked by**: #4, #5, #6, #7, #8, #9
**Blocks**: #12, #13, #14

## Objective

Wire up all command handlers with real logic. This is the main integration point where all internal packages come together.

## Commands to implement

### `auth check`
- Get xoxc and xoxd tokens (auth package)
- Run `_check_token` diagnostics for each (sanitize, warn on bad prefix)
- Call `auth.test` API
- If fails and xoxd is URL-encoded, retry with decoded value (same fallback as Python)
- Print results: `[OK]`, `[WARN]`, `[FAIL]` lines

### `auth set-xoxc <token>` / `auth set-xoxd <token>`
- Save token to keychain via auth package
- Print confirmation with service name and account

### `message <url>`
- Parse URL → channel_id, message_ts, thread_ts
- If thread_ts: fetch `conversations.replies` with thread_ts
- Else: fetch `conversations.replies` with message_ts — if >1 message, it's a thread
- Resolve user names
- Format and output (respect `-o` flag)

### `channel <name|id> --since --until --limit`
- Resolve channel name to ID
- Parse --since/--until to timestamps
- Fetch `conversations.history` with pagination
- Get team URL for permalinks
- Resolve user names
- Add permalink to each message
- Format and output (respect `-o` flag)
- **Show progress spinner** during fetch (message count)

### `search <query> --count`
- Call `search.messages` API
- Format results (ts, channel name, user, text truncated, permalink)
- Output (respect `-o` flag)

## Acceptance criteria

- [ ] All 5 commands fully functional
- [ ] `-o json` and `-o markdown` work for message, channel, search
- [ ] Progress indicator shown during multi-page fetches
- [ ] Rate limit handling works transparently (retry + partial results)
- [ ] `go build ./cmd/slack-cli` produces working binary
- [ ] `go vet ./...` passes

## Notes

- For progress, consider a simple stderr spinner: `Fetching messages... (42 so far)`
- Don't block on stdout — progress goes to stderr, data goes to stdout
- This task is large but all the building blocks exist from tasks #4-#9
