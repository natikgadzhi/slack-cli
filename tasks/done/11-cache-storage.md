# Task 11: Markdown cache / storage layer

**Phase**: 2 — Features
**Blocked by**: #5, #8
**Blocks**: #12

## Objective

Implement the Markdown caching system per PROJECT_PROMPT item 6. Each fetched object is stored as a Markdown file with YAML frontmatter.

## Design

Storage location: `~/.local/share/slack-cli/cache/` (from `config.CacheDir()`)

Directory structure:
```
~/.local/share/slack-cli/cache/
├── messages/
│   └── C12345678/
│       └── 1741234567.123456.md
├── channels/
│   └── C12345678/
│       └── 2026-03-01_2026-03-10.md
└── search/
    └── <query-hash>.md
```

Each file has YAML frontmatter:
```yaml
---
tool: slack-cli
object_type: message | channel_history | search_result
slug: C12345678/1741234567.123456
created_at: 2026-03-14T10:00:00Z
updated_at: 2026-03-14T10:00:00Z
source_url: https://myteam.slack.com/archives/C12345678/p1741234567123456
command: "slack-cli message https://..."
---
```

## Acceptance criteria

- [ ] `internal/cache/cache.go`:
  - `Cache` struct with base dir path
  - `NewCache() *Cache` — uses `config.CacheDir()`
  - `Get(objectType, slug string) (content []byte, meta Metadata, found bool, err error)`
  - `Put(objectType, slug string, content []byte, meta Metadata) error`
  - `Metadata` struct: Tool, ObjectType, Slug, CreatedAt, UpdatedAt, SourceURL, Command
- [ ] `internal/cache/frontmatter.go`:
  - `MarshalFrontmatter(meta Metadata, body []byte) []byte`
  - `UnmarshalFrontmatter(data []byte) (Metadata, []byte, error)`
- [ ] Cache key derivation:
  - Messages: `messages/<channel_id>/<ts>.md`
  - Channel history: `channels/<channel_id>/<since>_<until>.md`
  - Search: `search/<sha256(query)[:12]>.md`
- [ ] `--no-cache` flag (global) skips cache reads (still writes)
- [ ] Unit tests:
  - Write then read roundtrip
  - Frontmatter marshal/unmarshal
  - Missing cache file → found=false
  - Cache directory auto-created
  - Different object types stored in correct subdirs
- [ ] `go test ./internal/cache/...` passes

## Notes

- Use `gopkg.in/yaml.v3` for YAML frontmatter (or a simpler approach: manual marshal since the schema is fixed and small)
- Cache writes should be atomic (write to temp file, then rename) to avoid corruption
- Consider: no TTL-based invalidation for now — the `--no-cache` flag is sufficient. Users can also delete the cache dir.
