# slack-cli

This is a CLI tool that allows read-only access to Slack given `xoxc` and `xoxd` tokens form Mac OS secret store.

See more details in `README.md` on the current functionality that is already implemented.

We need to make several changes to this project:

1. Rewrite in Go. Keep the same command structure and same functionality. Make sure we have extensive set of unit tests, as well as end to end tests (the user will provide their tokens in the environment for tests to run.)
2. Provide installation path with a Homebrew tap (we have multiple tools like this, so I propose a separate repository in `../taps` that would be pushed up to `github.com/natikgadzhi/taps.git`). Release should be binary and should not require the user to have Go installed on their OS.
3. Fix a bug where when API returns HTTP 429, no information is returned to the user at all - we should return everything that was successfully fetched to date.
4. Fix the rate limiting bug - implement rate limiting and backoff. Implement spaced queries. Implement CLI progress indicators to show the user that the operation is still in progress, perhaps with the counter of data already fetched.
5. Implement output in JSON or Markdown, `-o json` or `-o markdown` in CLI options.
6. Implement Markdown caching with the principles below:

## Conventions

### Version command

Every CLI tool should have a `version` subcommand that outputs JSON:

```json
{
  "version": "0.0.2",
  "commit": "6b739723c64dc3d3ee5745064663e7ad69dbb078",
  "date": "2026-03-15T03:35:31Z"
}
```

Implementation:
- Three build-time variables injected via ldflags: `Version`, `Commit`, `Date`
- Defaults: `Version="dev"`, `Commit="unknown"`, `Date="unknown"`
- GoReleaser sets them automatically via `{{.Version}}`, `{{.Commit}}`, `{{.Date}}`
- Makefile sets `Commit` and `Date` from git/system for local builds
- `--version` flag should also be supported (Cobra's built-in `Version` field), outputs a single line

## Guiding principles behind the rewrite

- Build in typed language with good tooling (already covered, Go)
- Build with easy install systems so others can benefit (Homebrew)
- Tools are portable (no language runtime required to use)
- Tool calls are logged in file system by the tools themselves
- Strongly prefer read-only tools. Tool should request explicit approval if it wants to write anything.
- Tools should output JSON or Markdown or both
- Tools should keep cache output database in a shared location. Markdown files with frontmatter. We could prefer ~/.local or something that you find a better best practice.
- Frontmatter includes tool name, object name, unique object slug, creation or last update time, source URL, specific command and agent that requested that object.
