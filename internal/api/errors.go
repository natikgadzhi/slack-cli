// Package api provides an HTTP client for the Slack Web API with
// rate-limit handling, exponential backoff, and cursor-based pagination.
package api

import (
	"fmt"
	"time"
)

// APIError represents a non-OK response from the Slack API.
// Code is the HTTP status code; Message contains the response body excerpt.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack api error (HTTP %d): %s", e.Code, e.Message)
}

// RateLimitError is returned when the API responds with HTTP 429.
// RetryAfter indicates how long the server asked us to wait.
// PartialData, when non-nil, contains results collected before the 429
// during a paginated call.
type RateLimitError struct {
	RetryAfter  time.Duration
	PartialData []map[string]any
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited: retry after %s", e.RetryAfter)
}
