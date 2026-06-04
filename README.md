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

Keychain services: `slack-xoxc-token` / `slack-xoxd-token`, account defaults to the current OS user (`whoami`). Override with `SLACK_KEYCHAIN_ACCOUNT`.

`auth set-xoxd` accepts the URL-encoded form of the xoxd cookie (with `%XX`
escapes), the raw decoded form, or the full copied `d=xoxd-...` cookie pair.
Raw values are auto-encoded before storage and `[WARN]` lines tell you what was
changed.

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
slack-cli search "deployment failed" --limit 10
slack-cli saved --limit 50
slack-cli unread --limit 50
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

It reports whether each token came from the environment or Keychain, explains
Slack auth errors in plain English, and saves a working Keychain xoxd form when
it can prove one with `auth.test`.

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

In the results, the CHANNEL column shows the conversation name. For 1:1 DM hits
(which Slack reports only as the partner's user ID) it shows the other person's
display name prefixed with `@` (e.g. `@Alice Adams`); group DMs keep their
`mpdm-…` name.

### `saved`

List messages saved from the Slack "Later" / saved-items view, sorted in
reverse-chronological order by when each item was saved.

```sh
slack-cli saved
slack-cli saved --limit 100
slack-cli saved -o json | jq '.[] | .text'
```

Columns (table output): conversation name, date, message. In a capable terminal,
the conversation cell and date cell render as OSC-8 hyperlinks (the conversation
opens the channel on Slack, the date opens the message permalink). Channel
references and user mentions inside message text are replaced with readable
names; multi-person DMs are named by their participant list; emoji shortcodes
(`:thread:` → 🧵) are substituted.

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `50` | Maximum number of saved messages to return |

### `unread`

List messages in your unread queue that you'd be **notified** about — not every
unread message in every channel, only the ones worth a ping. Specifically:

- **Channel @-mentions** — `@you`, `@user-group`, `@here`, `@channel`, `@everyone`
- **Keyword highlights** — messages containing your configured "My keywords"
- **Channel invitations** — being invited/added to a channel
- **1:1 DMs** (all unread)
- **Group DMs / MPIMs** (all unread)
- **Thread replies** in threads you follow (all unread replies)

Reactions to your messages and bot/app DMs are excluded by default; opt in with
`--include-reactions` and `--include-apps`.

```sh
slack-cli unread
slack-cli unread --limit 100
slack-cli unread --include-reactions --include-apps
slack-cli unread -o json | jq '.[] | {kind, conversation, text}'
```

Reading is **non-destructive**: running this command never marks anything as read —
the reported messages stay unread/bold/badged in Slack afterward.

When you're mentioned inside a thread, the rest of that thread's unread messages are
pulled in too (via `conversations.replies`), so you see the surrounding replies, not
just the one message that mentioned you.

In the **table**, a conversation stream with several unread messages collapses to a
single line — the latest message, with a `[+N]` badge for the others. This applies to
threads and to DMs / group DMs / app DMs (so a chatty DM is one row, not many); channel
mentions and reactions stay on their own rows. **JSON** and the cached/`--derived`
**Markdown** always keep every individual message.

Columns (table output): conversation name, date, message. As with `saved`, the
conversation and date cells render as OSC-8 hyperlinks in a capable terminal,
channel/user references and emoji shortcodes are resolved, HTML entities (`&gt;` →
`>`) are decoded, and multi-person DMs are named by their participant list. JSON rows
additionally carry a `kind` field (`mention` / `keyword` / `invite` / `dm` /
`group_dm` / `thread` / `reaction` / `app`) for programmatic filtering, plus
`conversation_url`, `permalink`, `user`, and `thread_ts` (for thread replies).

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `50` | Maximum number of unread messages to return |
| `--include-reactions` | `false` | Include reactions to your messages |
| `--include-apps` | `false` | Include direct messages from bots and apps |

Data sources: mentions, keyword highlights, channel invitations (and opt-in reactions)
come from Slack's internal Activity feed (`activity.feed`); subscribed-thread replies
from `subscriptions.thread.getView`; the messages surrounding a thread mention from
`conversations.replies`; 1:1 DMs, group DMs, and (with `--include-apps`) bot/app DMs
from `client.counts` + `conversations.history`. These are the same browser endpoints
the Slack web client uses.

Notes / limitations: only the most recent page (up to 100) of unread messages per DM
is fetched, so very large unread backlogs may be truncated; muted channels are not
specially handled; thread-reply permalinks open the reply's channel (the `thread_ts`
is included in JSON for context).

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

## Slack API endpoint

All Slack HTTP calls go through a single base URL, which defaults to
`https://slack.com/api`. Override it with `SLACK_BASE_URL` — useful for tests
that stub the Slack API via `httptest`:

```sh
SLACK_BASE_URL=http://127.0.0.1:12345/api slack-cli saved
```

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
