<p align="center">
  <img src="https://raw.githubusercontent.com/natikgadzhi/slack-cli-auth/main/App/Assets.xcassets/AppIcon.appiconset/icon_256x256.png" width="128" alt="Slack CLI icon" />
</p>

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

slack-cli talks to Slack's internal web API and needs an `xoxc-` workspace token
and `xoxd-` session cookie from an active browser session.

### Slack CLI Auth (recommended)

[**Slack CLI Auth**](https://github.com/natikgadzhi/slack-cli-auth) is a small,
signed + notarized macOS app that signs you in to Slack in a web view, captures
the tokens automatically, and stores them in your Keychain in the layout
slack-cli expects. SSO, 2FA, and email magic links all work.

```sh
brew install --cask natikgadzhi/taps/slack-cli-auth
```

Open the app, sign in to your workspace, and you're done — slack-cli picks up
the tokens automatically. Verify with:

```sh
slack-cli auth check
```

### Manual setup

If you prefer to set tokens by hand, grab the `xoxc` token and `xoxd` (`d`)
cookie from your browser's devtools and store them in the Keychain:

```sh
slack-cli auth set-xoxc xoxc-...
slack-cli auth set-xoxd xoxd-...
```

Keychain services: `slack-xoxc-token` / `slack-xoxd-token`, account defaults to
the current OS user (`whoami`). Override with `SLACK_KEYCHAIN_ACCOUNT`.

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
slack-cli message --channel general --ts 1741234567.123456
slack-cli channels get general --since 2d --limit 100
slack-cli channels list
slack-cli channels members general
slack-cli channels search eng
slack-cli canvases read F12345678
slack-cli canvases read 'https://yourteam.slack.com/docs/T123/F12345678'
slack-cli emojis list
slack-cli emojis search fire
slack-cli emojis new
slack-cli emojis download --download-dir ./wiki/img
slack-cli files read F12345678
slack-cli files read 'https://yourteam.slack.com/files/U123/F12345678/report.txt'
slack-cli reactions get 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli search "deployment failed" --limit 10
slack-cli saved --limit 50
slack-cli unread --limit 50
slack-cli users list
slack-cli users get alice
slack-cli users search "alice"
```

## Usage scenarios

### Catch up on what you missed

```sh
# What needs my attention right now?
slack-cli unread

# What did people save for me to look at?
slack-cli saved --limit 20

# What happened in #engineering today?
slack-cli channels get engineering --since 1d
```

### Find and read messages

```sh
# Search for a topic across all channels
slack-cli search "deployment failed" --limit 10

# Search with surrounding context (3 messages before and after each hit)
slack-cli search "deployment failed" --context 3

# Search messages from a specific person
slack-cli search --from alice "API outage" --sort recent

# Read a specific thread by URL
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'

# Read a thread by channel + timestamp (useful from scripts or other tools)
slack-cli message --channel general --ts 1741234567.123456
```

### Explore channels and people

```sh
# Find channels by name
slack-cli channels search infra

# List all channels (including DMs and group chats)
slack-cli channels list --type public_channel,private_channel,mpim,im

# Who's in a channel?
slack-cli channels members engineering

# Look up someone's full profile
slack-cli users get alice

# Find people by name or email
slack-cli users search "product manager"
```

### Work with files and content

```sh
# Read a text file inline (code snippets, configs, logs)
slack-cli files read F12345678

# Paste a file URL directly — the file ID is extracted automatically
slack-cli files read 'https://yourteam.slack.com/files/U123/F12345678/report.txt'

# Download a file to disk
slack-cli files read F12345678 --download --download-dir ./downloads

# Read a Slack canvas document
slack-cli canvases read F12345678

# Or paste the canvas URL
slack-cli canvases read 'https://yourteam.slack.com/docs/T123/F12345678'
```

### Reactions and emojis

```sh
# See who reacted to a message
slack-cli reactions get 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli reactions get --channel general --ts 1741234567.123456

# Find custom workspace emojis
slack-cli emojis search party

# List every custom emoji in the workspace
slack-cli emojis list -o json

# Watch which emojis your team added since the last run
slack-cli emojis new

# Download every custom emoji image (great for building an emoji wiki)
slack-cli emojis download --download-dir ./wiki/img
```

### Pipe and script

```sh
# Export channel history as JSON for processing
slack-cli channels get general --since 7d -o json | jq '.[] | .text'

# Get all messages from a user in JSON
slack-cli search --from alice -o json > alice-messages.json

# List user emails for a channel
slack-cli channels members engineering -o json | jq '.[].email'

# Feed search results to another tool
slack-cli search "action item" --context 2 -o json | jq '.[] | select(.context_before)'
```

## Shell completion

slack-cli ships completion scripts for bash, zsh, fish, and PowerShell via the
`completion` subcommand:

```sh
slack-cli completion fish | source                          # current session
slack-cli completion fish > ~/.config/fish/completions/slack-cli.fish  # persistent
```

For other shells, see `slack-cli completion <bash|zsh|powershell> --help`.

Beyond the static command/flag tree, completion is **dynamic** where it helps:

| Where | Completes |
|-------|-----------|
| `channels get <TAB>`, `channels members <TAB>`, `message --channel <TAB>` | Channel names from your channel list |
| `users get <TAB>` | User handles (annotated with real names) |
| `search --from <TAB>` | User handles (annotated with real names); a leading `@` is honored |
| `channels list --type <TAB>`, `channels search --type <TAB>` | `public_channel`, `private_channel`, `mpim`, `im` |
| `search --sort <TAB>` | `relevance`, `recent` |
| `-o`/`--output <TAB>` | `json`, `table` |

Dynamic channel/user lists are fetched from Slack on first use and cached for an
hour under `<derived-dir>/completion/`, so a TAB press is instant after the first
fetch. If a refresh fails (offline, rate limited, missing credentials),
completion silently falls back to the last cached list or offers nothing.

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

Fetch a single Slack message or thread by URL, or by channel and timestamp.

```sh
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456' -o json
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456' --download-files
slack-cli message --channel general --ts 1741234567.123456
slack-cli message --channel C12345 --ts 1741234567.123456 -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--channel` | | Channel name or ID (use with `--ts` instead of a URL) |
| `--ts` | | Message timestamp (use with `--channel` instead of a URL) |
| `--download-files` | `false` | Download file attachments to disk |
| `--download-dir` | `slack-files` | Directory for downloaded files |

You can identify the message either by its full Slack URL (positional argument)
or by passing `--channel` and `--ts` together. The two modes are mutually
exclusive -- providing both a URL and the flags is an error, as is providing only
one of `--channel` or `--ts`.

Fetches the message and all thread replies, resolves user IDs to display names, and generates permalinks. File attachments are always listed in the output (name, type, size, URL). Use `--download-files` to save them to disk.

### `reactions get`

Show reactions on a Slack message.

```sh
slack-cli reactions get 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli reactions get --channel general --ts 1741234567.123456
slack-cli reactions get 'https://...' -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--channel` | | Channel name or ID (use with `--ts` instead of a URL) |
| `--ts` | | Message timestamp (use with `--channel` instead of a URL) |

You can identify the message either by its full Slack URL (positional argument)
or by passing `--channel` and `--ts` together. The two modes are mutually
exclusive.

Table output shows EMOJI, COUNT, and USERS columns. EMOJI displays the reaction
shortcode (e.g. `:thumbsup:`), COUNT shows the number of users who reacted, and
USERS lists display names of those users. JSON output returns an array of
reaction objects with resolved user names.

### `channels get`

Fetch messages from a Slack channel by name or ID.

```sh
slack-cli channels get general --since 2d --limit 100
slack-cli channels get C12345678 --since 2026-03-01 --until 2026-03-10
slack-cli channels get general --with-replies
slack-cli channels get general --with-pins
slack-cli channels get general -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | | Start time (relative like `2d`, or absolute like `2026-03-01`) |
| `--until` | | End time |
| `-n`, `--limit` | `50` | Maximum number of messages to fetch |
| `--with-replies` | `false` | Expand thread replies inline under each parent message |
| `--with-pins` | `false` | Fetch and display pinned items and bookmarks |
| `--download-files` | `false` | Download file attachments to disk |
| `--download-dir` | `slack-files` | Directory for downloaded files |

File attachments are always listed in JSON output (name, type, size, URL) and
shown as `[file: name]` indicators in table output. Use `--download-files` to
save them to disk; each file's `local_path` is then included in JSON output.

In table output there is no separate link column; instead the TIME cell renders
as an OSC-8 hyperlink to the message permalink, so it stays clickable in a
capable terminal without the URL being truncated. Use `-o json` to get the raw
permalink. When `--with-replies` is set, thread replies appear after their parent
with a `↳` prefix in table output and nested under `"replies"` in JSON.

When `--with-pins` is set, pinned items and bookmarks (tabs) are fetched via
`pins.list` and `bookmarks.list` and shown as a header section before the
messages in table output. In JSON output, the response becomes an object with
`pinned_items`, `bookmarks`, and `messages` fields instead of a flat message
array. When `--with-pins` is not set (the default), the output format is
unchanged.

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

### `channels members`

List members of a channel.

```sh
slack-cli channels members general
slack-cli channels members C12345678
slack-cli channels members general --limit 50
slack-cli channels members general --include-bots --include-deactivated
slack-cli channels members general -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `100` | Maximum number of members to return |
| `--include-bots` | `false` | Include bot users |
| `--include-deactivated` | `false` | Include deactivated users |

Output columns match the `users` command: ID, NAME, REAL NAME, EMAIL.

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

### `canvases read`

Fetch and display a Slack canvas (document) by its ID or URL.

```sh
slack-cli canvases read F12345678
slack-cli canvases read 'https://yourteam.slack.com/docs/T123/F12345678'
slack-cli canvases read 'https://app.slack.com/docs/T123/F12345678'
slack-cli canvases read F12345678 -o json
```

Accepts a raw canvas ID (starting with `F`) or a Slack canvas URL -- the ID is
extracted automatically.
In table/text output the canvas content is rendered as simplified markdown
(headings, paragraphs, lists, code blocks, blockquotes, links, checklists). In
JSON output the full API response is returned, including the raw document
structure.

### `emojis search`

Search custom workspace emojis by name (case-insensitive substring match).

```sh
slack-cli emojis search fire
slack-cli emojis search party --limit 10
slack-cli emojis search logo -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `50` | Maximum number of results |

Table output columns: NAME (with colons for copy-paste, e.g. `:fire:`), TYPE
(`custom` or `alias`), VALUE (image URL for custom emojis, target name like
`:flames:` for aliases). JSON output returns an array of objects with `name`,
`type`, `value`, and `url` (the underlying image URL — for aliases this is
resolved to the target's URL, so wiki tooling can render either the alias
display name or the underlying image).

### `emojis list`

List every custom emoji in the workspace.

```sh
slack-cli emojis list
slack-cli emojis list --limit 1000
slack-cli emojis list --type custom
slack-cli emojis list --type alias -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `500` | Maximum number of results |
| `--type` | `all` | Filter by type: `custom`, `alias`, or `all` |

Output shape matches `emojis search` (NAME / TYPE / VALUE in table, JSON array
of objects with `name`, `type`, `value`, `url`). Results are sorted by name.

### `emojis new`

List emojis added since the last `emojis new` run. slack-cli persists a
snapshot of the workspace's emoji set under the data directory
(`<data-dir>/emojis/snapshot.json`); each invocation compares the live emoji
set against the snapshot, prints what's new, then updates the snapshot.

```sh
slack-cli emojis new                # since last run
slack-cli emojis new --since 7d     # only emojis first seen in the last 7 days
slack-cli emojis new --reset        # replace snapshot with the current set
slack-cli emojis new -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | | Only show emojis first seen at or after this time (e.g. `7d`, `2026-03-01`) |
| `--reset` | `false` | Replace the snapshot with the current emoji set and print nothing |
| `-n`, `--limit` | `0` | Maximum emojis to print (`0` = no limit) |

The first invocation establishes the baseline and reports zero new emojis.
Subsequent runs print emojis that weren't in the previous snapshot (or, with
`--since`, emojis whose first-seen timestamp lies within the window). The
snapshot is updated whether or not anything new was found, so re-runs don't
double-report.

### `emojis download`

Sync custom emoji images to a local directory, with full creator + creation-time
metadata captured in a manifest. Downloads are **incremental** — on every run
slack-cli loads the existing `manifest.json` and only fetches emojis it hasn't
seen before (or whose image is missing on disk). Pass `--overwrite` to re-fetch
everything.

```sh
slack-cli emojis download                                  # incremental sync of all custom emojis
slack-cli emojis download --download-dir ./wiki/img
slack-cli emojis download fire                             # one emoji
slack-cli emojis download :fire:
slack-cli emojis download --overwrite                      # re-fetch everything
slack-cli emojis download --keep-removed                   # don't prune deleted emojis
```

| Flag | Default | Description |
|------|---------|-------------|
| `--download-dir` | `slack-emojis` | Directory for downloaded files |
| `--overwrite` | `false` | Re-download files that already exist on disk |
| `--keep-removed` | `false` | Keep manifest entries for emojis that no longer exist in the workspace |

**Files written**

```
<download-dir>/
  manifest.json        # catalog of every emoji (see schema below)
  fire.png             # custom emoji image (extension derived from URL)
  party_blob.gif
  campfire.alias       # one-line text file containing "fire"
```

Aliases get a `<name>.alias` text file (one line containing the target name)
instead of duplicating the underlying image. Image filename extensions come
from the source URL (`.png`, `.gif`, `.jpg`, `.jpeg`, `.webp`, `.apng`),
falling back to `.png` for unknown types.

**Manifest schema**

```jsonc
{
  "version": 1,
  "updated_at": "2026-06-06T19:42:11Z",
  "emojis": {
    "fire": {
      "type": "custom",
      "url": "https://emoji.slack-edge.com/.../fire.png",
      "local_path": "fire.png",
      "created_at": "2021-03-12T15:42:00Z",
      "created_by_id": "U12345678",
      "created_by_name": "Alice Adams"
    },
    "campfire": {
      "type": "alias",
      "alias_for": "fire",
      "local_path": "campfire.alias",
      "created_at": "2022-07-04T12:00:00Z",
      "created_by_id": "U87654321",
      "created_by_name": "Bob Bobson"
    }
  }
}
```

Metadata is sourced from Slack's internal `emoji.adminList` endpoint, which
requires the same browser session credentials slack-cli already uses. Syncing
a large workspace (thousands of emojis) takes a few seconds — slack-cli
paginates 200 emojis per request and respects Slack's rate limits.

**Prune semantics**

By default, emojis that have been removed from the workspace are dropped from
the manifest and their local files are deleted, so the manifest stays a true
mirror of the current emoji set. Pass `--keep-removed` to retain entries (the
local file is preserved too) — useful when a wiki has historical links you
don't want to break.

A summary line is printed to stderr:
`Done. N downloaded, M aliases, K skipped, R removed, E errors.`

### `files read`

Fetch a Slack file's metadata and, for text files, its content. Accepts a raw
file ID (starting with `F`) or a Slack file URL -- the ID is extracted
automatically.

```sh
slack-cli files read F12345678
slack-cli files read 'https://yourteam.slack.com/files/U123/F12345678/report.txt'
slack-cli files read F12345678 -o json
slack-cli files read F12345678 --download
slack-cli files read F12345678 --download --download-dir ./my-files
```

| Flag | Default | Description |
|------|---------|-------------|
| `--download` | `false` | Download the file to disk |
| `--download-dir` | `slack-files` | Directory for downloaded files |

For text-like files (mimetypes starting with `text/`, plus `application/json`,
`application/xml`, etc.) the file content is fetched and printed to stdout after
the metadata table. For binary files, only metadata (name, type, size, URL) is
shown.

In JSON output, text file content is included as a `content` field. The
`--download` flag saves any file to disk regardless of type.

### `search`

Search Slack messages.

```sh
slack-cli search "deployment failed" --limit 10
slack-cli search --from alice "deployment"
slack-cli search --from U12345 --sort recent
slack-cli search "deployment failed" --context 3
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `20` | Maximum number of results |
| `--from` | | Filter by user (handle or user ID) |
| `--sort` | `relevance` | Sort order: `relevance` or `recent` |
| `-C`, `--context` | `0` | Number of surrounding messages to fetch for each hit |

At least one of a query argument or `--from` is required.

When `--context N` is set (N > 0), each search hit is enriched with up to N
messages before and after it from the same channel. In table output, context
messages are prefixed with `|` and the hit itself with `>`, with blank lines
separating groups. In JSON output, each result gains `context_before` and
`context_after` arrays of `{ts, user, text}` objects. Context fetching makes
additional API calls (two per hit); if a call is rate-limited, the hit is shown
without context and a warning is printed.

In table output there is no separate link column; instead the TIME cell renders
as an OSC-8 hyperlink to the message permalink, so it stays clickable in a
capable terminal without the URL being truncated. Use `-o json` to get the raw
permalink.

The CHANNEL column shows the conversation name. For 1:1 DM hits
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

### `users list`

List workspace users. Running `users` with no subcommand is equivalent to
`users list`.

```sh
slack-cli users list
slack-cli users list --limit 50
slack-cli users list --include-bots --include-deactivated
slack-cli users list -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `100` | Maximum number of users |
| `--include-bots` | `false` | Include bot users |
| `--include-deactivated` | `false` | Include deactivated users |

### `users get`

Look up a single user's full profile by handle or user ID.

```sh
slack-cli users get alice
slack-cli users get U12345678
slack-cli users get @alice
slack-cli users get alice -o json
```

Table output uses a two-column key-value layout showing: ID, Name, Real Name,
Display Name, Email, Title, Status, Status Emoji, Timezone, Phone, Admin, Owner,
Bot, and Deactivated. JSON output returns a single object with all fields.

When given a handle (e.g. `alice`), the command searches via Slack's
`users.search` API to resolve the user ID, then fetches the full profile via
`users.info`. When given a user ID (e.g. `U12345678`), it calls `users.info`
directly.

### `users search`

Search users by name, handle, or email.

```sh
slack-cli users search alice
slack-cli users search "alice smith"
slack-cli users search alice --limit 10
slack-cli users search alice -o json
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n`, `--limit` | `20` | Maximum number of results |

Uses Slack's internal `users.search` API. Results are displayed in the same
columnar table format as `users list` (ID, NAME, REAL NAME, EMAIL).

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

## Contributors

Thank you to everyone who has contributed to slack-cli:

- [Matt Smith (@lambdamatt)](https://github.com/lambdamatt)

## Dev

```sh
make build      # build binary to ./slack-cli
make test       # run unit tests
make vet        # go vet
make lint       # golangci-lint
make e2e        # end-to-end tests (requires valid Slack credentials)
make clean      # remove build artifacts
```
