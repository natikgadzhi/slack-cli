# Task 7: User resolution with cache

**Phase**: 2 — Features
**Blocked by**: #4
**Blocks**: #10

## Objective

Port `src/slack_cli/users.py` to Go. Replace user IDs with display names, backed by a JSON file cache.

## Reference (Python)

```python
def resolve_users(messages: list) -> list:
    cache = _load_user_cache()
    unknown = {m["user"] for m in messages if m.get("user") and m["user"] not in cache}
    for uid in unknown:
        cache[uid] = _fetch_username(uid)  # users.info API
    _save_user_cache(cache)
    # replace user IDs with names in returned messages
```

## Acceptance criteria

- [ ] `internal/users/resolve.go`:
  - `UserResolver` struct with `*api.Client` and cache path
  - `NewUserResolver(client *api.Client) *UserResolver` — uses `config.UserCachePath()`
  - `ResolveUsers(messages []map[string]any) ([]map[string]any, error)`
  - Collects unique unknown user IDs from messages
  - Fetches each via `users.info` API
  - Updates JSON cache file (create parent dirs if needed)
  - Returns messages with `user` field replaced by display name
- [ ] Cache file format: `{"U12345": "Natik Gadzhi", ...}` (simple JSON object)
- [ ] Unit tests:
  - All users in cache → no API calls
  - Unknown user → API call + cache updated
  - Cache file doesn't exist → created
  - API failure for one user → falls back to raw UID
  - Messages without user field → unchanged
- [ ] `go test ./internal/users/...` passes

## Notes

- Use temp dirs in tests for cache files
- The API client should be injectable (interface or passed as arg) for testability
