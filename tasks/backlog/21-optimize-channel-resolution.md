# Task 21: Short-circuit channel resolution

**Phase**: 4 — Polish
**Blocked by**: #12
**Blocks**: (none)

## Objective

`channels.ResolveChannel` currently fetches ALL channels via `CallPaginated` before searching for the target name. For workspaces with thousands of channels, this is many unnecessary API calls.

## Fix

Check each page as it arrives and return early when the channel is found. This requires either:
1. Manual pagination loop with per-page check (similar to what channel.go already does for messages), or
2. A `CallPaginatedUntil(predicate)` variant in the `api` package

Also consider caching the channel name→ID mapping on disk (similar to user cache).

## Acceptance criteria

- [ ] Channel resolution for common channels completes in 1-2 API calls (not 10+)
- [ ] Existing tests pass
- [ ] New test for early-termination behavior
