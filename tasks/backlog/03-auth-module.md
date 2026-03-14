# Task 3: Auth module (keychain + env vars)

**Phase**: 1 — Core
**Blocked by**: #1
**Blocks**: #4, #9

## Objective

Port `src/slack_cli/auth.py` to Go. Provide keychain read/write via macOS `security` CLI, env var fallback, and token sanitization.

## Reference (Python)

- `_keychain_get(service)` — calls `security find-generic-password`
- `_keychain_set(service, token)` — deletes existing, then `security add-generic-password`
- `_sanitize_token(token)` — strips whitespace, quotes, "Bearer " prefix; returns (clean, warnings)
- `get_xoxc()` / `get_xoxd()` — env var first, keychain fallback

## Acceptance criteria

- [ ] `internal/auth/keychain.go`:
  - `KeychainGet(service string) (string, error)` — exec `security find-generic-password -a <account> -s <service> -w`
  - `KeychainSet(service, token string) error` — delete-then-add pattern
  - Account comes from `config.KeychainAccount()`
- [ ] `internal/auth/sanitize.go`:
  - `SanitizeToken(token string) (clean string, warnings []string)`
  - Strips leading/trailing whitespace
  - Strips surrounding single or double quotes
  - Strips "Bearer " prefix (case-insensitive)
- [ ] `internal/auth/auth.go`:
  - `GetXoxc() (string, error)` — checks `SLACK_XOXC` env, falls back to keychain
  - `GetXoxd() (string, error)` — checks `SLACK_XOXD` env, falls back to keychain
- [ ] Unit tests for `SanitizeToken` — port all 7 Python test cases:
  - clean token unchanged
  - strips double quotes
  - strips single quotes
  - strips Bearer prefix
  - strips bearer (lowercase) prefix
  - strips whitespace
  - strips multiple artifacts at once
- [ ] Unit tests for keychain functions (mock `exec.Command`)
- [ ] `go test ./internal/auth/...` passes

## Notes

- Use `os/exec` for shelling out to `security`
- Make the command executor injectable (interface or function field) for testability
