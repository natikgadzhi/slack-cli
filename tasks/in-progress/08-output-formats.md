# Task 8: Output format support (JSON and Markdown)

**Phase**: 2 — Features
**Blocked by**: #5
**Blocks**: #10, #11

## Objective

Implement `-o json` and `-o markdown` output formats per PROJECT_PROMPT item 5.

## Acceptance criteria

- [ ] `internal/output/output.go`:
  - `Format` type (string enum): `JSON`, `Markdown`
  - `ParseFormat(s string) (Format, error)` — accepts "json", "markdown", "md"
  - `RenderMessages(w io.Writer, messages []formatting.Message, format Format) error`
  - `RenderSearchResults(w io.Writer, results []map[string]any, format Format) error`
  - `RenderSingle(w io.Writer, msg formatting.Message, format Format) error`
- [ ] JSON output: pretty-printed JSON (same as current Python behavior)
- [ ] Markdown output for messages:
  ```markdown
  ## 2026-03-01 14:00 UTC — @username

  Message text here.

  > Attachment title
  > Attachment text

  Reactions: thumbsup(3), heart(1)
  [Thread: 5 replies] | [Link](https://...)
  ---
  ```
- [ ] Default format: JSON (backward compatible)
- [ ] Unit tests:
  - JSON rendering produces valid JSON
  - Markdown rendering produces expected format
  - Single message vs list rendering
  - Messages with attachments, reactions, links
  - Empty message list
- [ ] `go test ./internal/output/...` passes

## Notes

- The `-o` / `--output` flag itself is wired in Task #9 (CLI scaffold)
- This package is pure rendering, no I/O beyond writing to the provided `io.Writer`
