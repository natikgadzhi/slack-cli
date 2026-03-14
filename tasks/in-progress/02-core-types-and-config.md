# Task 2: Core types and config

**Phase**: 1 — Core
**Blocked by**: #1
**Blocks**: #4, #9

## Objective

Port `src/slack_cli/config.py` to Go. Establish shared constants, environment variable overrides, and cache path conventions.

## Reference (Python)

```python
# config.py
SLACK_API = "https://slack.com/api"
USER_CACHE_PATH = Path(os.environ.get("SLACK_USER_CACHE", Path.home() / ".cache" / "slack-users.json"))
KC_ACCOUNT = os.environ.get("SLACK_KEYCHAIN_ACCOUNT", "natikgadzhi")
KC_XOXC_SERVICE = os.environ.get("SLACK_XOXC_SERVICE", "slack-xoxc-token")
KC_XOXD_SERVICE = os.environ.get("SLACK_XOXD_SERVICE", "slack-xoxd-token")
_USER_AGENT = "Mozilla/5.0 ..."
```

## Acceptance criteria

- [ ] `internal/config/config.go` defines:
  - `SlackAPIBase` constant (`https://slack.com/api`)
  - `UserAgent` constant (same browser UA string)
  - `KeychainAccount()` — returns env `SLACK_KEYCHAIN_ACCOUNT` or `"natikgadzhi"`
  - `KeychainXoxcService()` — returns env `SLACK_XOXC_SERVICE` or `"slack-xoxc-token"`
  - `KeychainXoxdService()` — returns env `SLACK_XOXD_SERVICE` or `"slack-xoxd-token"`
  - `UserCachePath()` — returns env `SLACK_USER_CACHE` or `~/.local/share/slack-cli/users.json`
  - `CacheDir()` — returns `~/.local/share/slack-cli/cache/`
- [ ] Unit tests verify env var overrides work (set env, call func, check result)
- [ ] Unit tests verify defaults when env vars are unset
- [ ] `go build ./...` and `go test ./internal/config/...` pass

## Notes

- Python used `~/.cache/slack-users.json`; we're upgrading to `~/.local/share/slack-cli/` per PROJECT_PROMPT preference for a shared location
- Functions (not package-level vars) so tests can manipulate env cleanly
