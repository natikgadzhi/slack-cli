# slack-cli

Slack read-only CLI for fetching messages, threads, and history.

## Setup

```sh
cd slack-cli
go build -o slack-cli ./cmd/slack-cli
```

Or install directly:

```sh
go install github.com/natikgadzhi/slack-cli/cmd/slack-cli@latest
```

## Auth

Requires an `xoxc-` token and `xoxd-` cookie from an active Slack browser session.

Store them in macOS Keychain:

```sh
slack-cli auth set-xoxc xoxc-...
slack-cli auth set-xoxd xoxd-...
```

Or set env vars `SLACK_XOXC` and `SLACK_XOXD`.

Keychain services: `slack-xoxc-token` / `slack-xoxd-token`, account defaults to `natikgadzhi`.
Override with `SLACK_KEYCHAIN_ACCOUNT`, `SLACK_XOXC_SERVICE`, `SLACK_XOXD_SERVICE`.

## Usage

```sh
slack-cli --help
slack-cli auth check
slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
slack-cli channel general --since 2d --limit 100
slack-cli channel C12345678 --since 2026-03-01 --until 2026-03-10
slack-cli search "deployment failed" --count 10
```

## Dev

```sh
make build      # build binary
make test       # run unit tests
make vet        # go vet
make lint       # golangci-lint
make e2e        # end-to-end tests (requires valid Slack credentials)
```
