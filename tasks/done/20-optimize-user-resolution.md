# Task 20: Optimize user resolution (N+1 fix)

**Phase**: 4 — Polish
**Blocked by**: #12
**Blocks**: (none)

## Objective

The current `UserResolver.ResolveUsers` makes one `users.info` API call per unknown user ID. For channels with many unique posters, this creates an N+1 pattern (30 users = 30 serial HTTP calls = ~6s latency on first run).

## Options

1. **Parallel fetches**: Use `errgroup` with bounded concurrency (e.g. 5 workers) to fetch unknown users in parallel. Reduces wall time from N*200ms to ~N/5*200ms.
2. **Batch via users.list**: Fetch all workspace users with `users.list` (paginated) and populate the entire cache at once. Fewer round-trips for large workspaces, but downloads more data than needed.
3. **Hybrid**: Use `users.list` if >10 unknown users, individual `users.info` otherwise.

## Acceptance criteria

- [ ] First-run latency for a channel with 30 unique users is <2s (down from ~6s)
- [ ] Existing user cache behavior preserved
- [ ] Tests updated for new concurrency
- [ ] `go test -race ./...` passes
