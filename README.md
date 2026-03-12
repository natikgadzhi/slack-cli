# slack-cli

Slack read-only CLI for fetching messages, threads, and history.

## Auth

Requires an `xoxc-` token and `xoxd-` cookie from an active Slack browser session.

Store them in macOS Keychain:

```sh
slack set-xoxc xoxc-...
slack set-xoxd xoxd-...
```

Or set env vars `SLACK_XOXC` and `SLACK_XOXD`.

Keychain account defaults to `natikgadzhi`. Override with `SLACK_KEYCHAIN_ACCOUNT`.

## Setup

```sh
cd slack-cli
uv sync
```

## Run

```sh
uv run slack --help
uv run slack check
uv run slack message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
uv run slack history general --since 2d --limit 100
uv run slack history C12345678 --since 2026-03-01 --until 2026-03-10
uv run slack search "deployment failed" --count 10
```

## Dev

```sh
uv run ruff check src/        # lint
uv run ruff format src/       # format
uv run pyright src/           # type check
```
