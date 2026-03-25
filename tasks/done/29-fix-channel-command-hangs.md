# Task 29: Fix `slack-cli channel` command hanging

## Problem

`slack-cli channel <name|id>` prints "Resolving channel ..." and then appears to hang indefinitely — no progress, no error, no exit. The `message` command works fine.

## Root Cause Analysis

The hang occurs in `ResolveChannel()` (`internal/channels/resolve.go:37-53`). Likely causes, in order of probability:

### 1. Silent pagination through huge workspace (most likely)
`conversations.list` is called with `types: public_channel,private_channel,mpim,im` and paginates through ALL conversations (including DMs/group DMs). In a large workspace this could be thousands of entries with zero progress output. Rate limiting (429s) during this pagination would add silent multi-second delays between pages.

### 2. Rate limit stall
The API client retries 429s with exponential backoff (up to 5 retries per call). During `conversations.list` pagination, each page could hit rate limits, causing multi-second sleeps with no user-visible indication.

### 3. Possible infinite cursor loop
If the API returns a non-empty cursor but an empty channels list repeatedly, the loop never breaks.

## Key Files

- `internal/channels/resolve.go` — `ResolveChannel()`, pagination loop
- `internal/commands/channel.go` — `runChannel()`, command entry point
- `internal/api/client.go` — `Call()`, retry/backoff logic

## Fix Plan

### Step 1: Add progress indication to channel resolution
In `ResolveChannel()`, add stderr output showing pagination progress (e.g., "Checked N channels across M pages..."). This alone will tell us if the hang is "working but slow" or truly stuck.

### Step 2: Add a `--verbose` / `--debug` flag (optional)
Log API calls, response status, cursor state, and rate limit hits so the user (and us) can see exactly what's happening.

### Step 3: Reduce the blast radius of `conversations.list`
- If the input is a channel ID (already handled) or a URL, skip resolution entirely
- Consider using only `public_channel,private_channel` types (drop `mpim,im` — users don't refer to DMs by name)
- Add a safety valve: max pages limit (e.g., 20 pages = 4000 channels) with a clear error

### Step 4: Handle channel IDs passed as URLs or with `#` prefix
The command might receive a Slack URL (like `message` does) — detect and extract channel ID from URLs.

## Acceptance Criteria

- [ ] `slack-cli channel general` resolves and fetches messages (or errors clearly)
- [ ] `slack-cli channel C12345678` works (direct ID, no resolution needed)
- [ ] Progress is visible during channel name resolution
- [ ] If resolution takes too long or fails, a clear error is shown
- [ ] Existing tests still pass

## Interactive Debugging Required

Tokens cannot be shared with agents — the user will need to test changes locally and report results. The worker should propose changes, and the user will verify behavior.
