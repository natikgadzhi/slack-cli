package api

import (
	"encoding/json"
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
func (c *Client) callWithRetry(endpoint string, params map[string]string, retriesLeft int) (map[string]any, error) {
	body := c.encodeParams(params)
	reqURL := c.baseURL + "/" + endpoint

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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := c.parseRetryAfter(resp.Header.Get("Retry-After"))
		if retriesLeft <= 0 {
			return nil, &RateLimitError{RetryAfter: retryAfter}
		}
		delay := c.backoffDelay(retryAfter, c.maxRetries-retriesLeft)
		c.sleepFn(delay)
		return c.callWithRetry(endpoint, params, retriesLeft-1)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", endpoint, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt := string(respBody)
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
		}
		return nil, &APIError{Code: resp.StatusCode, Message: excerpt}
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response from %s: %w", endpoint, err)
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
			if rlErr, ok := asRateLimitError(err); ok {
				rlErr.PartialData = all
				return all, rlErr
			}
			return all, err
		}

		// Extract the collected items.
		items := extractItems(result, collectKey)
		all = append(all, items...)

		// Check for next cursor.
		cursor := extractNextCursor(result, cursorKey)
		if cursor == "" {
			break
		}
		p[cursorKey] = cursor
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
		if u, ok := data["url"].(string); ok {
			c.teamURL = strings.TrimRight(u, "/")
		}
	})
	return c.teamURL, c.teamErr
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
// Returns 1s if the header is missing or unparseable.
func (c *Client) parseRetryAfter(header string) time.Duration {
	if header == "" {
		return time.Second
	}
	secs, err := strconv.Atoi(header)
	if err != nil || secs <= 0 {
		return time.Second
	}
	return time.Duration(secs) * time.Second
}

// backoffDelay returns the delay before retrying. If retryAfter is positive
// (from the Retry-After header), it adds jitter. Otherwise it uses exponential
// backoff with jitter.
func (c *Client) backoffDelay(retryAfter time.Duration, attempt int) time.Duration {
	if retryAfter > 0 {
		// Add 0-25% jitter on top of the server-requested delay.
		jitter := time.Duration(rand.Int64N(int64(retryAfter) / 4))
		return retryAfter + jitter
	}
	// Exponential backoff: 1s, 2s, 4s, 8s, ...
	base := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	jitter := time.Duration(rand.Int64N(int64(base) / 2))
	return base + jitter
}

// asRateLimitError unwraps err into a *RateLimitError if possible.
func asRateLimitError(err error) (*RateLimitError, bool) {
	if rlErr, ok := err.(*RateLimitError); ok {
		return rlErr, true
	}
	return nil, false
}

// extractItems pulls a slice of objects from result[collectKey].
func extractItems(result map[string]any, collectKey string) []map[string]any {
	raw, ok := result[collectKey]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var items []map[string]any
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			items = append(items, m)
		}
	}
	return items
}

// extractNextCursor reads response_metadata.next_cursor from a Slack API response.
// cursorKey is the parameter name used to pass the cursor back in the next request
// (e.g. "cursor"); the response always stores it under response_metadata.next_cursor.
func extractNextCursor(result map[string]any, _ string) string {
	meta, ok := result["response_metadata"]
	if !ok {
		return ""
	}
	metaMap, ok := meta.(map[string]any)
	if !ok {
		return ""
	}
	cursor, ok := metaMap["next_cursor"].(string)
	if !ok {
		return ""
	}
	return cursor
}
