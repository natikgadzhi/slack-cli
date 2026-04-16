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
slack-cli channels get general --since 2d --limit 100
slack-cli channels list
slack-cli channels search eng
slack-cli saved --limit 20
slack-cli search "deployment failed" --limit 10
slack-cli users
```

## Global flags

| Flag | Description |
|------|-------------|
| `-o`, `--output` | Output format: `json` or `table` (default: auto-detected; table in TTY, json when piped) |
| `--no-cache` | Skip cache for this request |
| `--debug` | Enable debug logging to stderr |
| `-d`, `--derived` | Derived data directory (default: `~/.local/share/lambdal/derived/slack-cli`) |
| `--version` | Show version information |

## Commands

### `auth check`

Check if Slack tokens are configured and valid.

```sh
slack-cli auth check
```

### `auth set-xoxc`

Store an xoxc token in the macOS Keychain.

```sh
slack-cli auth set-xoxc xoxc-...
```

### `auth set-xoxd`

Store an xoxd cookie in the macOS Keychain.

```sh
slack-cli auth set-xoxd xoxd-...
```

### `message`

Fetch a single Slack message or thread by URL.

```sh
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456' -o json
```

Fetches the message and all thread replies, resolves user IDs to display names, and generates permalinks.

### `channels get`

Fetch messages from a Slack channel by name or ID.

```sh
slack-cli channels get general --since 2d --limit 100
slack-cli channels get C12345678 --since 2026-03-01 --until 2026-03-10
slack-cli channels get general -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | | Start time (relative like `2d`, or absolute like `2026-03-01`) |
| `--until` | | End time |
| `-n`, `--limit` | `50` | Maximum number of messages to fetch |

### `channels list`

List channels and conversations.

```sh
slack-cli channels list
slack-cli channels list --limit 50
slack-cli channels list --type public_channel,private_channel,mpim,im
slack-cli channels list --include-archived
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `100` | Maximum number of channels to return |
| `--type` | `public_channel,private_channel` | Comma-separated conversation types |
| `--include-archived` | `false` | Include archived channels |

### `channels search`

Search channels by name (case-insensitive substring match).

```sh
slack-cli channels search eng
slack-cli channels search "product" --type public_channel,private_channel,mpim,im
slack-cli channels search infra --include-archived
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `20` | Maximum number of results |
| `--type` | `public_channel,private_channel,mpim,im` | Conversation types to search |
| `--include-archived` | `false` | Include archived channels |

### `search`

Search Slack messages.

```sh
slack-cli search "deployment failed" --limit 10
slack-cli search --from alice "deployment"
slack-cli search --from U12345 --sort recent
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `20` | Maximum number of results |
| `--from` | | Filter by user (handle or user ID) |
| `--sort` | `relevance` | Sort order: `relevance` or `recent` |

At least one of a query argument or `--from` is required.

### `saved`

List saved messages via Slack's legacy stars API.

```sh
slack-cli saved
slack-cli saved --limit 20
slack-cli saved -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `20` | Maximum number of saved messages to return |

Slack's newer Later view is not available through Slack's public APIs, so `saved` may not include items you've saved recently in the Slack client.

### `users`

List workspace users.

```sh
slack-cli users
slack-cli users --limit 50
slack-cli users --include-bots --include-deactivated
slack-cli users -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `100` | Maximum number of users |
| `--include-bots` | `false` | Include bot users |
| `--include-deactivated` | `false` | Include deactivated users |

### `version`

Print version information as JSON.

```sh
slack-cli version
slack-cli --version
```

## Output formats

All commands support the `-o` flag:

| Format | Flag | Description |
|--------|------|-------------|
| Table | `-o table` | Human-readable aligned columns |
| JSON | `-o json` | Structured JSON |

When no `-o` flag is provided, slack-cli auto-detects: **table** when stdout is a TTY, **json** when piped or redirected.

## Cache

Results are cached as Markdown files with YAML frontmatter in `~/.local/share/lambdal/derived/slack-cli/`. Override with `SLACK_DATA_DIR`, `SLACK_CLI_DERIVED_DIR`, or `LAMBDAL_DERIVED_DIR` environment variables.

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
