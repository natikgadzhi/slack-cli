# Task 30: Add `--from` flag to `slack-cli search`

## Objective

Make it easy to search messages from a specific person:

```sh
slack-cli search --from @username "optional query"
slack-cli search --from U12345678 "optional query"
slack-cli search --from @username              # all recent messages from user
```

## Context

Slack's `search.messages` API already supports `from:@handle` as part of the query string, so `slack-cli search "from:@username"` technically works today. This task makes it a first-class flag for better ergonomics and discoverability.

## Current State

- `internal/commands/search.go` — `runSearch()` takes a single `<query>` arg and passes it directly to `search.messages`
- The command currently requires exactly 1 arg (`cobra.ExactArgs(1)`)
- Results are sorted by Slack's default relevance ranking
- `--count` flag already exists (default 20)

## Implementation Plan

### 1. Add `--from` flag
Add a `--from` string flag to `searchCmd`. When set, prepend `from:@<value>` to the query string sent to `search.messages`.

### 2. Make query arg optional when `--from` is set
Change args validation from `cobra.ExactArgs(1)` to `cobra.MaximumNArgs(1)`. When `--from` is set with no query, search for `from:@username` alone (returns all messages from that user).

### 3. Add `--sort` flag for recency
Add a `--sort` flag with values `relevance` (default, current behavior) and `recent`. Map to `search.messages` API params:
- `sort=timestamp&sort_dir=desc` for recent
- Default (no sort params) for relevance

When `--from` is used without a query, default to `--sort recent` (most useful for "show me what this person said").

### 4. Handle `@` prefix
Strip leading `@` from the `--from` value if present, since Slack's `from:` modifier works with bare handles.

## Key File

- `internal/commands/search.go` — all changes here

## Acceptance Criteria

- [ ] `slack-cli search --from @username` returns recent messages from that user
- [ ] `slack-cli search --from username "deployment"` searches that user's messages for "deployment"
- [ ] `slack-cli search --sort recent "query"` sorts by timestamp descending
- [ ] `slack-cli search "from:@username"` still works (no regression)
- [ ] Bare `slack-cli search "query"` unchanged
- [ ] `slack-cli search` with no args and no `--from` errors clearly
