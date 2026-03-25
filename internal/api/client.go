package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 5
	defaultPageDelay  = 100 * time.Millisecond
)

// Client is an HTTP client for the Slack Web API.
// It handles authentication headers, rate limiting with retry/backoff,
// and cursor-based pagination.
type Client struct {
	xoxc       string
	xoxd       string
	httpClient *http.Client
	baseURL    string
	maxRetries int
	pageDelay  time.Duration

	// teamURL is cached after the first successful GetTeamURL call.
	teamURL  string
	teamOnce sync.Once
	teamErr  error

	// sleepFn is an indirection for testing; defaults to time.Sleep.
	sleepFn func(time.Duration)
}

// NewClient creates a Client with the given tokens and optional configuration.
func NewClient(xoxc, xoxd string, opts ...Option) *Client {
	c := &Client{
		xoxc:       xoxc,
		xoxd:       xoxd,
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    config.SlackAPIBase,
		maxRetries: defaultMaxRetries,
		pageDelay:  defaultPageDelay,
		sleepFn:    time.Sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Call makes a single POST request to the given Slack API endpoint.
// Params with empty values are included; to omit a key, simply don't add it
// to the map. Returns the parsed JSON body as a generic map.
func (c *Client) Call(endpoint string, params map[string]string) (map[string]any, error) {
	return c.callWithRetry(endpoint, params, c.maxRetries)
}

// callWithRetry performs the HTTP call and retries on 429 up to maxRetries times.
// It uses a loop instead of recursion to avoid stacking deferred resp.Body.Close() calls.
func (c *Client) callWithRetry(endpoint string, params map[string]string, retriesLeft int) (map[string]any, error) {
	reqURL := c.baseURL + "/" + endpoint

	for {
		body := c.encodeParams(params)
		req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("creating request for %s: %w", endpoint, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.xoxc)
		req.Header.Set("Cookie", "d="+c.xoxd)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		req.Header.Set("User-Agent", config.UserAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter, hasHeader := c.parseRetryAfter(resp.Header.Get("Retry-After"))
			// Drain and close the body before retrying to avoid stacking deferred closes.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if retriesLeft <= 0 {
				return nil, &RateLimitError{RetryAfter: retryAfter}
			}
			delay := c.backoffDelay(retryAfter, hasHeader, c.maxRetries-retriesLeft)
			c.sleepFn(delay)
			retriesLeft--
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response from %s: %w", endpoint, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, clierrors.HandleHTTPError(resp.StatusCode, endpoint, "slack-cli", c.checkAuth)
		}

		var result map[string]any
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("decoding response from %s: %w", endpoint, err)
		}

		// Slack returns HTTP 200 with {"ok": false, "error": "..."} for
		// auth failures, permission errors, etc. Detect this and return a CLIError.
		if ok, exists := result["ok"]; exists {
			if okBool, isBool := ok.(bool); isBool && !okBool {
				errMsg := "unknown error"
				if e, has := result["error"].(string); has {
					errMsg = e
				}
				cliErr := clierrors.NewCLIError(clierrors.ExitError, fmt.Sprintf("slack api: %s", errMsg))
				cliErr = cliErr.WithCode(resp.StatusCode)
				return nil, cliErr
			}
		}

		return result, nil
	}
}

// CallPaginated follows Slack cursor-based pagination. It collects items found
// under collectKey from each page's response_metadata.next_cursor. If a 429
// occurs mid-pagination, it returns all data collected so far along with a
// *RateLimitError (callers can check with errors.As and still use partial data).
func (c *Client) CallPaginated(endpoint string, params map[string]string, cursorKey, collectKey string) ([]map[string]any, error) {
	var all []map[string]any

	// Copy params so we don't mutate the caller's map.
	p := make(map[string]string, len(params))
	for k, v := range params {
		p[k] = v
	}

	first := true
	for {
		if !first {
			c.sleepFn(c.pageDelay)
		}
		first = false

		result, err := c.Call(endpoint, p)
		if err != nil {
			// On rate limit, attach partial results.
			if rlErr, ok := asRateLimitError(err); ok {
				rlErr.PartialData = all
				return all, rlErr
			}
			return all, err
		}

		// Extract the collected items.
		items := ExtractItems(result, collectKey)
		all = append(all, items...)

		// Check for next cursor using the specified cursorKey
		// (e.g. "next_cursor" for standard Slack pagination).
		cursor := ExtractNextCursor(result, cursorKey)
		if cursor == "" {
			break
		}
		p["cursor"] = cursor
	}

	return all, nil
}

// GetTeamURL returns the Slack workspace base URL (e.g. "https://myteam.slack.com").
// It first checks the SLACK_TEAM_URL environment variable, then falls back to
// calling auth.test. The result is cached for the lifetime of the Client.
func (c *Client) GetTeamURL() (string, error) {
	c.teamOnce.Do(func() {
		if envURL := os.Getenv("SLACK_TEAM_URL"); envURL != "" {
			c.teamURL = strings.TrimRight(envURL, "/")
			return
		}
		data, err := c.Call("auth.test", nil)
		if err != nil {
			c.teamErr = fmt.Errorf("fetching team URL: %w", err)
			return
		}
		u, ok := data["url"].(string)
		if !ok || u == "" {
			c.teamErr = fmt.Errorf("auth.test response missing url field")
			return
		}
		c.teamURL = strings.TrimRight(u, "/")
	})
	return c.teamURL, c.teamErr
}

// checkAuth verifies whether the current credentials are valid by calling
// auth.test. Implements clierrors.AuthChecker for HandleHTTPError.
func (c *Client) checkAuth() (bool, error) {
	data, err := c.Call("auth.test", nil)
	if err != nil {
		return false, err
	}
	if ok, exists := data["ok"]; exists {
		if okBool, isBool := ok.(bool); isBool {
			return okBool, nil
		}
	}
	return false, nil
}

// --- helpers ---

// encodeParams builds a URL-encoded form body from the param map.
func (c *Client) encodeParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	return vals.Encode()
}

// parseRetryAfter reads the Retry-After header value as seconds.
// Returns the parsed duration and true if a valid header was present.
// Returns (0, false) if the header is missing or unparseable.
func (c *Client) parseRetryAfter(header string) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(header)
	if err != nil || secs <= 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

// backoffDelay returns the delay before retrying. If hasRetryAfter is true,
// retryAfter contains the server-requested delay and we add jitter on top.
// Otherwise we use exponential backoff with jitter.
func (c *Client) backoffDelay(retryAfter time.Duration, hasRetryAfter bool, attempt int) time.Duration {
	if hasRetryAfter {
		// Add 0-25% jitter on top of the server-requested delay.
		jitter := time.Duration(rand.Int64N(int64(retryAfter) / 4))
		return retryAfter + jitter
	}
	// Exponential backoff: 1s, 2s, 4s, 8s, ...
	base := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	jitter := time.Duration(rand.Int64N(int64(base) / 2))
	return base + jitter
}

// asRateLimitError unwraps err into a *RateLimitError if possible,
// using errors.As so it works with wrapped errors.
func asRateLimitError(err error) (*RateLimitError, bool) {
	var rlErr *RateLimitError
	if errors.As(err, &rlErr) {
		return rlErr, true
	}
	return nil, false
}

// ExtractItems pulls a slice of objects from result[collectKey].
func ExtractItems(result map[string]any, collectKey string) []map[string]any {
	raw, ok := result[collectKey]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	items := make([]map[string]any, 0, len(arr))
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			items = append(items, m)
		}
	}
	return items
}

// ExtractNextCursor reads the cursor from response_metadata in a Slack API response.
// cursorKey specifies which field to look up inside response_metadata (e.g.
// "next_cursor"). Most Slack endpoints use "next_cursor" by default.
func ExtractNextCursor(result map[string]any, cursorKey string) string {
	meta, ok := result["response_metadata"]
	if !ok {
		return ""
	}
	metaMap, ok := meta.(map[string]any)
	if !ok {
		return ""
	}
	cursor, ok := metaMap[cursorKey].(string)
	if !ok {
		return ""
	}
	return cursor
}
