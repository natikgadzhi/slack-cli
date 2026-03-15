# Task 27: JSON version output with commit and build date

**Phase**: 4 — Polish
**Blocked by**: (none)
**Blocks**: (none)

## Objective

Make `slack-cli version` output JSON with version, commit hash, and build date — consistent with gdrive-cli.

## Expected output

```json
{
  "version": "0.0.2",
  "commit": "6b739723c64dc3d3ee5745064663e7ad69dbb078",
  "date": "2026-03-15T03:35:31Z"
}
```

## Implementation

1. **`internal/commands/root.go`**: Add `Commit` and `Date` variables alongside `Version`, all set via ldflags:
   ```go
   var (
       Version = "dev"
       Commit  = "unknown"
       Date    = "unknown"
   )
   ```

2. **`internal/commands/version.go`**: Output JSON:
   ```go
   func runVersion(cmd *cobra.Command, args []string) {
       info := map[string]string{
           "version": Version,
           "commit":  Commit,
           "date":    Date,
       }
       enc := json.NewEncoder(os.Stdout)
       enc.SetIndent("", "  ")
       enc.Encode(info)
   }
   ```

3. **`.goreleaser.yml`**: Update ldflags to inject all three:
   ```yaml
   ldflags:
     - >-
       -s -w
       -X github.com/natikgadzhi/slack-cli/internal/commands.Version={{.Version}}
       -X github.com/natikgadzhi/slack-cli/internal/commands.Commit={{.Commit}}
       -X github.com/natikgadzhi/slack-cli/internal/commands.Date={{.Date}}
   ```

4. **`Makefile`**: Update build target:
   ```makefile
   VERSION ?= dev
   COMMIT  ?= $(shell git rev-parse --short HEAD)
   DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

   build:
   	go build -ldflags "-X .../commands.Version=$(VERSION) -X .../commands.Commit=$(COMMIT) -X .../commands.Date=$(DATE)" ...
   ```

5. **`PROJECT_PROMPT.md`**: Add a section on version command convention (see below).

## PROJECT_PROMPT.md addition

Add a "Conventions" section with version command guidance:

```markdown
## Conventions

### Version command

Every CLI tool should have a `version` subcommand that outputs JSON:

    {
      "version": "0.0.2",
      "commit": "6b739723c64dc3d3ee5745064663e7ad69dbb078",
      "date": "2026-03-15T03:35:31Z"
    }

Implementation:
- Three build-time variables injected via ldflags: Version, Commit, Date
- Defaults: Version="dev", Commit="unknown", Date="unknown"
- GoReleaser sets them automatically via {{.Version}}, {{.Commit}}, {{.Date}}
- Makefile sets Commit and Date from git/system for local builds
- `--version` flag should also be supported (Cobra's built-in Version field)
```

## Acceptance criteria

- [ ] `slack-cli version` outputs pretty-printed JSON with version, commit, date
- [ ] `slack-cli --version` still works (prints single line)
- [ ] GoReleaser injects all three values on release builds
- [ ] `make build` injects commit and date for local builds
- [ ] PROJECT_PROMPT.md documents the convention
