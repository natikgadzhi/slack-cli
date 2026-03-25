// Package api provides an HTTP client for the Slack Web API with
// rate-limit handling, exponential backoff, and cursor-based pagination.
package api

import (
	"errors"
	"fmt"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
)

// APIError represents a non-OK response from the Slack API.
// Code is the HTTP status code; Message contains the response body excerpt.
// Deprecated: new code should use clierrors.CLIError via AsCLIError instead.
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

// AsAPIError unwraps err into an *APIError if possible.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// AsRateLimitError unwraps err into a *RateLimitError if possible.
func AsRateLimitError(err error) (*RateLimitError, bool) {
	var rlErr *RateLimitError
	if errors.As(err, &rlErr) {
		return rlErr, true
	}
	return nil, false
}

// AsCLIError unwraps err into a *clierrors.CLIError if possible.
func AsCLIError(err error) (*clierrors.CLIError, bool) {
	var cliErr *clierrors.CLIError
	if errors.As(err, &cliErr) {
		return cliErr, true
	}
	return nil, false
}
