# Task 24: Markdown file-per-item output with configurable output directory

**Phase**: 2 — Implementation
**Blocked by**: (none — cache system already exists)
**Blocks**: (none)

## Objective

Extend the existing cache system so that when `--output-dir <path>` is specified, each fetched item (message, search result, user) is written as its own markdown file in that directory. This enables multiple tools to write content they fetched from different systems into the same directory, building up a shared knowledge base of markdown files.

## Design

### Flag

Add a persistent flag `--output-dir` (short: `-d`) on the root command. When set:
- Each item is written as a **separate** `.md` file in the given directory
- The existing `--output` format flag still controls stdout rendering
- Cache behavior is unchanged (still writes the aggregate file to `~/.local/share/slack-cli/`)

### File-per-item layout

```
<output-dir>/
  slack/
    messages/
      C12345678/
        1741234567.123456.md    # single message or thread root + replies
    channels/
      general/
        1741234567.123456.md    # one file per message in channel fetch
        1741234568.234567.md
    search/
      <query-hash>/
        1741234567.123456.md    # one file per search match
```

All messages in a thread go in the **same file** (thread root + replies = one logical item).

### File format

Each file uses the existing `cache.MarshalFrontmatter` format:

```yaml
---
tool: slack-cli
object_type: message
slug: C12345678/1741234567.123456
created_at: 2026-03-14T10:00:00Z
updated_at: 2026-03-14T10:00:00Z
source_url: https://team.slack.com/archives/C12345678/p1741234567123456
command: "slack-cli message https://..."
channel: general
channel_id: C12345678
---

## 2026-03-14 14:00 UTC — @alice

This is the message text.

Reactions: thumbsup(3)

[Link](https://team.slack.com/archives/...)
```

The body is the **markdown rendering** of that single item (using the existing `output.RenderSingle` or equivalent), not JSON.

### Implementation

1. **`internal/commands/root.go`**: Add `--output-dir` persistent flag (string, default empty)
2. **`internal/cache/cache.go`**: Add `PutItem(objectType, slug, markdownBody []byte, meta Metadata)` — writes a single item file. Reuses existing `Put` plumbing (atomic write, path validation, frontmatter marshal).
3. **`internal/commands/helpers.go`**: Add `writeItemFiles(outputDir, objectType, items []formatting.Message, channelID, channelName string)` helper that:
   - Creates `<outputDir>/slack/<objectType>/<channelName or channelID>/` directory
   - For each item, renders it to markdown and calls `cache.PutItem`
   - File name is `<ts>.md` (sanitized — dots kept, slashes removed)
4. **`internal/commands/message.go`**: After rendering to stdout, if `--output-dir` is set, call `writeItemFiles`. For threads, write all messages into one file.
5. **`internal/commands/channel.go`**: Same — write each message as its own file.
6. **`internal/commands/search.go`**: Write each search match as its own file, under `search/<query-hash>/`.

### Frontmatter extensions

Add optional frontmatter fields relevant to per-item files:
- `channel` — resolved channel name (if available)
- `channel_id` — Slack channel ID
- `user` — message author display name
- `thread_ts` — thread parent timestamp (if applicable)

These go into `Metadata` struct in `frontmatter.go`.

## Acceptance criteria

- [ ] `--output-dir /tmp/notes` writes individual markdown files per item
- [ ] Files use existing frontmatter format with extended metadata
- [ ] `message` command: thread root + replies = one file
- [ ] `channel` command: one file per message
- [ ] `search` command: one file per search result
- [ ] Directory structure is `<output-dir>/slack/<type>/<context>/`
- [ ] Existing cache behavior is unchanged when `--output-dir` is not set
- [ ] Existing stdout rendering is unchanged regardless of `--output-dir`
- [ ] Atomic writes (temp file + rename)
- [ ] Path traversal protection on output-dir
