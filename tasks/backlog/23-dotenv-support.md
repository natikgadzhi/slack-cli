# Task 23: Local .env file support for E2E tests

**Phase**: 4 — Polish
**Blocked by**: #13
**Blocks**: (none)

## Objective

Support loading environment variables from a `.env` file for local E2E test runs, so developers don't need to export 6+ vars manually each time.

## Implementation

1. Add `.env` to `.gitignore` (prevent accidental commit of tokens)
2. Create `.env.example` with all supported vars (empty values, with comments)
3. Update `Makefile` `e2e` target to source `.env` if it exists:
   ```makefile
   e2e:
   	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
   	go test -tags e2e -v -timeout 120s ./tests/
   ```
4. Document the `.env` workflow in a comment at the top of `tests/e2e_test.go`

## .env.example contents

```
# Slack credentials (required for E2E tests)
SLACK_XOXC=
SLACK_XOXD=

# Optional — avoids an extra auth.test API call
SLACK_TEAM_URL=

# Test data
SLACK_TEST_CHANNEL=general
SLACK_TEST_MESSAGE_URL=
SLACK_TEST_SEARCH_QUERY=wat
```

## Acceptance criteria

- [ ] `.env` is in `.gitignore`
- [ ] `.env.example` exists with documented vars
- [ ] `make e2e` sources `.env` automatically if present
- [ ] `go test ./...` (without tag) still works normally
- [ ] No tokens committed to repo
