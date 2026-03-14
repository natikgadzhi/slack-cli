# Task 6: Channel resolution

**Phase**: 2 — Features
**Blocked by**: #4
**Blocks**: #10

## Objective

Port `src/slack_cli/channels.py` to Go. Resolve channel names to IDs via the Slack API.

## Reference (Python)

```python
def resolve_channel(name_or_id: str) -> str:
    name_or_id = name_or_id.lstrip("#")
    if re.match(r"^[A-Z0-9]{8,}$", name_or_id):
        return name_or_id
    # paginate conversations.list to find by name
```

## Acceptance criteria

- [ ] `internal/channels/resolve.go`:
  - `ResolveChannel(client *api.Client, nameOrID string) (string, error)`
  - Strip leading `#`
  - If matches `[A-Z0-9]{8,}` → return as-is (already an ID)
  - Otherwise paginate `conversations.list` (limit=200, types=public_channel,private_channel,mpim,im, exclude_archived=true) using cursor
  - Return channel ID when name matches
  - Return error if not found after exhausting all pages
- [ ] Unit tests with mocked API client:
  - ID passthrough (e.g. `C12345678` → `C12345678`)
  - Hash-prefixed ID (e.g. `#C12345678` → `C12345678`)
  - Name resolution found on first page
  - Name resolution found on second page (cursor pagination)
  - Name not found → error
- [ ] `go test ./internal/channels/...` passes
