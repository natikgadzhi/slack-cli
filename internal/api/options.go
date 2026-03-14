package api

import "time"

// Option configures a Client via the functional-options pattern.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithMaxRetries sets the maximum number of retries on HTTP 429.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
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
