# slack-cli

Slack read-only CLI for fetching messages, threads, and history.

## Installation

### Homebrew

```sh
brew install natikgadzhi/taps/slack-cli
```

### From source

```sh
go install github.com/natikgadzhi/slack-cli/cmd/slack-cli@latest
```

### From releases

Download a pre-built binary from [GitHub Releases](https://github.com/natikgadzhi/slack-cli/releases). Binaries are available for macOS and Linux on amd64 and arm64.

## Auth

Requires an `xoxc-` token and `xoxd-` cookie from an active Slack browser session.

### macOS Keychain

Store tokens in the Keychain:

```sh
slack-cli auth set-xoxc xoxc-...
slack-cli auth set-xoxd xoxd-...
```

Verify the stored credentials:

```sh
slack-cli auth check
```

Keychain services: `slack-xoxc-token` / `slack-xoxd-token`, account defaults to `natikgadzhi`.

Override with environment variables:

| Variable                | Description                          |
|-------------------------|--------------------------------------|
| `SLACK_KEYCHAIN_ACCOUNT`| Keychain account name                |
| `SLACK_XOXC_SERVICE`   | Keychain service name for xoxc token |
| `SLACK_XOXD_SERVICE`   | Keychain service name for xoxd cookie|

### Environment variables

Alternatively, set the tokens directly as environment variables:

```sh
export SLACK_XOXC=xoxc-...
export SLACK_XOXD=xoxd-...
```

Environment variables take precedence over Keychain values.

## Usage

```sh
slack-cli --help
slack-cli auth check
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli channel general --since 2d --limit 100
slack-cli channel C12345678 --since 2026-03-01 --until 2026-03-10
slack-cli search "deployment failed" --count 10
```

### Commands

| Command   | Description                                  |
|-----------|----------------------------------------------|
| `auth`    | Manage Slack authentication tokens           |
| `message` | Fetch a single Slack message or thread by URL|
| `channel` | Fetch messages from a Slack channel          |
| `search`  | Search Slack messages                        |

### Output formats

Use the `-o` flag to choose an output format:

- `-o json` (default) -- pretty-printed JSON
- `-o markdown` (or `-o md`) -- human-readable Markdown

### Cache

Results are cached as Markdown files with YAML frontmatter in `~/.local/share/slack-cli/`. Override the data directory with the `SLACK_DATA_DIR` environment variable.

To skip the cache for a request, pass the `--no-cache` flag:

```sh
slack-cli message 'https://...' --no-cache
```

## Dev

```sh
make build      # build binary to ./slack-cli
make test       # run unit tests
make vet        # go vet
make lint       # golangci-lint
make e2e        # end-to-end tests (requires valid Slack credentials)
make clean      # remove build artifacts
```

Or run Go commands directly:

```sh
go build ./cmd/slack-cli
go test ./...
go vet ./...
```
