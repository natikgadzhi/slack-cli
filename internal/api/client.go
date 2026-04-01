package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/natikgadzhi/cli-kit/debug"
	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/ratelimit"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultPageDelay = 100 * time.Millisecond
)

// Client is an HTTP client for the Slack Web API.
// It handles authentication headers, rate limiting via cli-kit/ratelimit
// RetryTransport, and cursor-based pagination.
type Client struct {
	xoxc       string
	xoxd       string
	httpClient *http.Client
	baseURL    string
	pageDelay  time.Duration

	// teamURL is cached after the first successful GetTeamURL call.
	teamURL  string
	teamOnce sync.Once
	teamErr  error

	// sleepFn is an indirection for testing; defaults to time.Sleep.
	sleepFn func(time.Duration)

	// retryTransport is exposed so callers can set OnRetry for progress feedback.
	retryTransport *ratelimit.RetryTransport
}

// NewClient creates a Client with the given tokens and optional configuration.
// Rate-limit retry is handled by wrapping the HTTP transport in a
// cli-kit/ratelimit.RetryTransport.
func NewClient(xoxc, xoxd string, opts ...Option) *Client {
	rt := ratelimit.NewRetryTransport(http.DefaultTransport)

	c := &Client{
		xoxc:           xoxc,
		xoxd:           xoxd,
		httpClient:     &http.Client{Timeout: defaultTimeout, Transport: rt},
		baseURL:        config.SlackAPIBase,
		pageDelay:      defaultPageDelay,
		sleepFn:        time.Sleep,
		retryTransport: rt,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetOnRetry wires a callback that fires before each retry sleep.
// This lets callers (e.g. progress spinners) react to rate-limit waits.
func (c *Client) SetOnRetry(fn func(attempt int, delay time.Duration, statusCode int)) {
	c.retryTransport.OnRetry = fn
}

// Call makes a single POST request to the given Slack API endpoint.
// Params with empty values are included; to omit a key, simply don't add it
// to the map. Returns the parsed JSON body as a generic map.
func (c *Client) Call(endpoint string, params map[string]string) (map[string]any, error) {
	reqURL := c.baseURL + "/" + endpoint

	body := c.encodeParams(params)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", endpoint, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.xoxc)
	req.Header.Set("Cookie", "d="+c.xoxd)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("User-Agent", config.UserAgent)

	debug.Log("HTTP POST %s", reqURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}

	debug.Log("HTTP %d %s", resp.StatusCode, reqURL)

	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", endpoint, err)
	}

	// After the RetryTransport has exhausted retries, a 429 will still
	// come through here. Surface it as a RateLimitError.
	if resp.StatusCode == http.StatusTooManyRequests {
		ra := ratelimit.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &RateLimitError{RetryAfter: ra}
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
			if rlErr, ok := AsRateLimitError(err); ok {
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
