# Task 31: Add `slack-cli users` command

## Objective

List workspace users with their display names, emails, and Slack user IDs.

```sh
slack-cli users
slack-cli users --limit 50
slack-cli users -o json | jq '.[] | select(.email | contains("@example.com"))'
```

## Context

There's no way to look up user IDs or browse the workspace directory from the CLI today. This is useful for:
- Finding a user's Slack ID to use with `slack-cli search --from U12345678`
- Scripting bulk lookups (e.g. "who has this email?")
- Quick directory browsing

## Implementation Plan

### 1. Add `users` command

New file: `internal/commands/users.go`

- `slack-cli users` lists workspace members
- Uses `users.list` API endpoint with cursor-based pagination
- Default limit: 100 users
- Flags: `--limit` / `-n`

### 2. Fields to extract per user

From each user object in the `users.list` response:
- `id` — Slack user ID (e.g. U12345678)
- `name` — username/handle
- `real_name` — display name
- `profile.email` — email (may be empty for some users)
- `deleted` — whether the user is deactivated
- `is_bot` — skip bots by default, add `--include-bots` flag to show them

### 3. Output formats

- Table mode: columns for ID, NAME, REAL NAME, EMAIL
- JSON mode: array of objects with all fields above
- Suppress stderr progress in JSON mode (consistent with other commands)

### 4. Optional filters

- `--include-bots` to include bot users (excluded by default)
- `--include-deactivated` to include deactivated users (excluded by default)

## Acceptance Criteria

- [ ] `slack-cli users` lists active human users with ID, name, real_name, email
- [ ] `slack-cli users -o json` outputs clean JSON (no stderr noise)
- [ ] `slack-cli users --limit 10` limits results
- [ ] Bots and deactivated users excluded by default
- [ ] `--include-bots` and `--include-deactivated` flags work
- [ ] Pagination works for large workspaces
- [ ] Tests cover output building and flag behavior
