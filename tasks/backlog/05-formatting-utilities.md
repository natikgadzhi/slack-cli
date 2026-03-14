# Task 5: Formatting and parsing utilities

**Phase**: 1 — Core
**Blocked by**: #1
**Blocks**: #8, #9, #10, #11

## Objective

Port `src/slack_cli/formatting.py` to Go. Pure functions, no API calls.

## Reference (Python)

- `parse_slack_url(url)` → (channel_id, message_ts, thread_ts)
- `parse_time(value)` → unix timestamp float (relative: 30m/3h/2d/1w, absolute: date/datetime, raw unix)
- `format_message(msg)` → dict with ts, time, user, text (truncated 500), reply_count, reactions, attachment
- `build_permalink(team_url, channel_id, ts)` → URL string
- `print_json(data)` → pretty-print JSON to stdout

## Acceptance criteria

- [ ] `internal/formatting/url.go`:
  - `ParseSlackURL(url string) (channelID, messageTS, threadTS string, err error)`
  - Handles `.../archives/C12345678/p1741234567123456` format
  - Handles `?thread_ts=...` query param
  - Returns clear errors for unrecognized formats
- [ ] `internal/formatting/time.go`:
  - `ParseTime(value string) (float64, error)`
  - Relative: `30m`, `3h`, `2d`, `1w`
  - Absolute: `2026-03-01`, `2026-03-01T14:00:00`, `2026-03-01 14:00:00`
  - Raw unix timestamp
  - Clear error for unparseable input
- [ ] `internal/formatting/message.go`:
  - `Message` struct with exported fields: TS, Time, User, Text, ReplyCount, Reactions, Attachment, Link
  - `FormatMessage(raw map[string]any) Message`
  - Text truncated to 500 chars
  - Empty/whitespace-only text omitted
  - Reactions formatted as `"name(count)"`
  - Attachment: title, text (truncated 300), color, action URLs (source, silence, playbook)
  - `BuildPermalink(teamURL, channelID, ts string) string`
- [ ] Port all Python tests (19 test cases across URL, time, message, permalink):
  - 4 URL parsing tests (basic, thread_ts, invalid path, invalid ts)
  - 8 time parsing tests (minutes, hours, days, weeks, absolute date, datetime, unix, invalid)
  - 8 message formatting tests (basic, truncate, empty text, reply count, zero reply count, reactions, attachment, missing ts)
  - 3 permalink tests (basic, trailing slash, ts without dot)
- [ ] `go test ./internal/formatting/...` passes

## Notes

- Use `net/url` for URL parsing
- Use `time` package for time parsing
- Use `regexp` for relative time pattern matching
- No external dependencies needed
