# Task 22: Parallelize team URL fetch with message pagination

**Phase**: 4 — Polish
**Blocked by**: #10
**Blocks**: (none)

## Objective

In `channel.go` and `message.go`, `client.GetTeamURL()` is called after all messages are fetched. Since it's an independent API call (`auth.test`), it could run concurrently with message fetching to save ~100-300ms.

## Fix

Fire `GetTeamURL()` in a goroutine before/during the pagination loop, collect the result after pagination completes. The `sync.Once` inside `GetTeamURL` already makes it safe to call from multiple goroutines.

## Acceptance criteria

- [ ] Team URL fetch runs concurrently with message pagination
- [ ] No data races (`go test -race`)
- [ ] Measurable latency improvement for multi-page fetches
