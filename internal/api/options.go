package api

import (
	"net/http"
	"time"

	"github.com/natikgadzhi/cli-kit/ratelimit"
)

// Option configures a Client via the functional-options pattern.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithMaxRetries sets the maximum number of retries on the underlying
// RetryTransport. A value of 0 is treated as 1 (one retry, two total
// attempts) because the transport resets zero to its default of 5.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n <= 0 {
			n = 1
		}
		c.retryTransport.MaxRetries = n
	}
}

// WithPageDelay sets the delay inserted between paginated requests to
// avoid hitting rate limits proactively.
func WithPageDelay(d time.Duration) Option {
	return func(c *Client) {
		c.pageDelay = d
	}
}

// WithBaseURL overrides the Slack API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithRetryOn5xx controls whether HTTP 5xx responses are retried by the
// underlying RetryTransport. Default is true.
func WithRetryOn5xx(retry bool) Option {
	return func(c *Client) {
		c.retryTransport.RetryOn5xx = retry
	}
}

// WithTransport overrides the base HTTP transport used by the retry layer.
// This is useful for testing with httptest servers.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) {
		retryRT := ratelimit.NewRetryTransport(rt)
		// Preserve any existing settings from the default retryTransport.
		retryRT.MaxRetries = c.retryTransport.MaxRetries
		retryRT.OnRetry = c.retryTransport.OnRetry
		retryRT.RetryOn5xx = c.retryTransport.RetryOn5xx
		c.retryTransport = retryRT
		c.httpClient.Transport = retryRT
	}
}
