# Task 9: CLI scaffold with cobra

**Phase**: 2 — Features
**Blocked by**: #2, #3, #5
**Blocks**: #10

## Objective

Build the CLI entry point and command tree using cobra, matching the current Python CLI structure.

## Current Python CLI structure

```
slack-cli
├── auth
│   ├── check
│   ├── set-xoxc <token>
│   └── set-xoxd <token>
├── message <url>
├── channel <name|id> [--since TIME] [--until TIME] [--limit N]
└── search <query> [--count N]
```

## Acceptance criteria

- [ ] `cmd/slack-cli/main.go`: initializes root command, calls `Execute()`
- [ ] `internal/commands/root.go`:
  - Root command: `slack-cli` with description matching Python version
  - Persistent flag: `-o` / `--output` (json|markdown), default json
- [ ] `internal/commands/auth.go`:
  - `auth` command group: "Manage Slack authentication tokens"
  - `auth check` subcommand (placeholder handler)
  - `auth set-xoxc <token>` subcommand (placeholder handler)
  - `auth set-xoxd <token>` subcommand (placeholder handler)
- [ ] `internal/commands/message.go`:
  - `message <url>` command (placeholder handler)
- [ ] `internal/commands/channel.go`:
  - `channel <name|id>` command
  - Flags: `--since TIME`, `--until TIME`, `--limit N` (default 50)
  - Placeholder handler
- [ ] `internal/commands/search.go`:
  - `search <query>` command
  - Flag: `--count N` (default 20)
  - Placeholder handler
- [ ] `go build ./cmd/slack-cli` produces working binary
- [ ] `./slack-cli --help` shows all commands
- [ ] `./slack-cli auth --help` shows subcommands
- [ ] `./slack-cli channel --help` shows flags

## Notes

- Placeholder handlers should print "not yet implemented" and exit 0
- The real command logic is wired in Task #10
- Use `github.com/spf13/cobra` — already added in Task #1
- Consider adding a `--no-cache` global flag for Task #11 integration
