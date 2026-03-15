# Task 4: API client with rate limiting and backoff

**Phase**: 1 — Core
**Blocked by**: #2, #3
**Blocks**: #6, #7, #10

## Objective

Port `src/slack_cli/api.py` to Go. Add rate limiting, backoff, and partial-result handling (fixes PROJECT_PROMPT bugs #3 and #4).

## Reference (Python)

```python
def _api_call_raw(endpoint, xoxc, xoxd, **params) -> dict:
    # POST to SLACK_API/endpoint with headers, return JSON
    # On HTTPError → ClickException
    # On URLError → ClickException

def api_call(endpoint, **params) -> dict:
    return _api_call_raw(endpoint, xoxc=get_xoxc(), xoxd=get_xoxd(), **params)

def get_team_url() -> str:
    # env SLACK_TEAM_URL or auth.test API call, cached
```

### Bugs to fix

1. **HTTP 429 returns nothing** — when API returns 429 during pagination, all previously fetched data is lost. Must return partial results.
2. **No rate limiting** — no backoff, no spaced queries, no progress indication.

## Acceptance criteria

- [ ] `internal/api/client.go`:
  - `Client` struct holding xoxc, xoxd, `*http.Client`, rate limit config
  - `NewClient(xoxc, xoxd string, opts ...Option) *Client` — functional options for timeouts, spacing
  - `Call(endpoint string, params map[string]string) (map[string]any, error)` — single API call
  - `CallPaginated(endpoint string, params map[string]string, cursorKey string, collectKey string) ([]map[string]any, error)` — handles cursor-based pagination, collects results across pages
- [ ] Headers set correctly: `Authorization: Bearer <xoxc>`, `Cookie: d=<xoxd>`, `Content-Type: application/x-www-form-urlencoded`, `User-Agent`
- [ ] Rate limiting:
  - On HTTP 429: read `Retry-After` header, sleep that duration (with jitter), retry
  - Max retries configurable (default 5)
  - Exponential backoff if no `Retry-After` header
- [ ] Partial results on 429:
  - `CallPaginated` returns all data collected before the 429, plus an error wrapping `RateLimitError`
  - Callers can check `errors.As(err, &rateLimitErr)` and still use the partial data
- [ ] Spaced queries: configurable delay between paginated requests (default 100ms)
- [ ] `GetTeamURL() (string, error)` — env `SLACK_TEAM_URL` or `auth.test`, cached
- [ ] Error types: `APIError{Code int, Message string}`, `RateLimitError{RetryAfter time.Duration, PartialData []map[string]any}`
- [ ] Unit tests:
  - Successful call with correct headers
  - Params encoding (nil params excluded)
  - HTTP 429 triggers retry
  - HTTP 429 during pagination returns partial results
  - Non-429 HTTP errors return APIError
  - Team URL caching
- [ ] `go test ./internal/api/...` passes

## Notes

- Use `net/http` standard library (no external HTTP client needed)
- For testing, use `httptest.NewServer` to mock the Slack API
- The progress indicator integration happens in Task #9/#10 (CLI layer), not here. This package should accept a callback or channel for progress reporting.
