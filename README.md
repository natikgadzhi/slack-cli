# slack-cli

Slack read-only CLI for fetching messages, threads, and history.

## Setup

```sh
cd slack-cli
uv sync
```

## Auth

Requires an `xoxc-` token and `xoxd-` cookie from an active Slack browser session.

Store them in macOS Keychain:

```sh
uv run slack auth set-xoxc xoxc-...
uv run slack auth set-xoxd xoxd-...
```

Or set env vars `SLACK_XOXC` and `SLACK_XOXD`.

Keychain services: `slack-xoxc-token` / `slack-xoxd-token`, account defaults to `natikgadzhi`.
Override with `SLACK_KEYCHAIN_ACCOUNT`, `SLACK_XOXC_SERVICE`, `SLACK_XOXD_SERVICE`.

## Usage

```sh
uv run slack --help
uv run slack auth check
uv run slack message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
uv run slack channel general --since 2d --limit 100
uv run slack channel C12345678 --since 2026-03-01 --until 2026-03-10
uv run slack search "deployment failed" --count 10
```

## Dev

```sh
uv run ruff check src/        # lint
uv run ruff format src/       # format
uv run pyright src/           # type check
```
