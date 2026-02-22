package services

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iriam/worklogr/internal/utils"
	"github.com/slack-go/slack"
)

var retryLogger = utils.NewLogger().WithService("slack")

// RetryableSlackClient wraps the Slack client with retry functionality
type RetryableSlackClient struct {
	client    *slack.Client
	maxRetries int
}

// NewRetryableSlackClient creates a new retryable Slack client
func NewRetryableSlackClient(token string, maxRetries int) *RetryableSlackClient {
	if maxRetries <= 0 {
		maxRetries = 3 // Default to 3 retries
	}

	// Create Slack client with custom HTTP client
	httpClient := &http.Client{
		Transport: &RetryTransport{
			Transport:  http.DefaultTransport,
			MaxRetries: maxRetries,
		},
	}
	
	// Use the custom HTTP client option
	client := slack.New(token, slack.OptionHTTPClient(httpClient))

	return &RetryableSlackClient{
		client:     client,
		maxRetries: maxRetries,
	}
}

// GetClient returns the underlying Slack client
func (r *RetryableSlackClient) GetClient() *slack.Client {
	return r.client
}

// RetryTransport implements http.RoundTripper with retry logic for 429 errors
type RetryTransport struct {
	Transport  http.RoundTripper
	MaxRetries int
}

// RoundTrip implements the http.RoundTripper interface with retry logic
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= rt.MaxRetries; attempt++ {
		// Clone the request for retry attempts
		reqClone := req.Clone(req.Context())
		
		// Perform the request
		resp, err = rt.Transport.RoundTrip(reqClone)
		
		// If no error and not a 429, return immediately
		if err != nil {
			if attempt == rt.MaxRetries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", rt.MaxRetries+1, err)
			}
			// Wait before retry for network errors
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// Check for 429 Too Many Requests
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// Handle 429 error
		if attempt == rt.MaxRetries {
			return resp, nil // Return the 429 response on final attempt
		}

		// Get retry delay from Retry-After header
		retryAfter := getRetryAfterDelay(resp)
		
		retryLogger.Warnf("Rate limit (429): %v 後にリトライします (%d/%d)", retryAfter, attempt+1, rt.MaxRetries)
		
		// Close the response body before retrying
		resp.Body.Close()
		
		// Wait for the specified duration
		time.Sleep(retryAfter)
	}

	return resp, err
}

// getRetryAfterDelay extracts the retry delay from Retry-After header
func getRetryAfterDelay(resp *http.Response) time.Duration {
	retryAfterHeader := resp.Header.Get("Retry-After")
	if retryAfterHeader == "" {
		// Default to 60 seconds if no Retry-After header
		return 60 * time.Second
	}

	// Try to parse as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
		if seconds > 300 { // Cap at 5 minutes
			seconds = 300
		}
		if seconds < 1 { // Minimum 1 second
			seconds = 1
		}
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as HTTP date (RFC 1123)
	if retryTime, err := time.Parse(time.RFC1123, retryAfterHeader); err == nil {
		delay := time.Until(retryTime)
		if delay > 5*time.Minute { // Cap at 5 minutes
			delay = 5 * time.Minute
		}
		if delay < 1*time.Second { // Minimum 1 second
			delay = 1 * time.Second
		}
		return delay
	}

	// Default to 60 seconds if parsing fails
	return 60 * time.Second
}

// RetryableAPICall wraps Slack API calls with additional retry logic
func (r *RetryableSlackClient) RetryableAPICall(operation string, apiCall func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		err := apiCall()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if it's a rate limit error from Slack SDK
		if slackErr, ok := err.(*slack.RateLimitedError); ok {
			if attempt == r.maxRetries {
				break // Don't retry on final attempt
			}

			retryAfter := time.Duration(slackErr.RetryAfter) * time.Second
			if retryAfter > 5*time.Minute {
				retryAfter = 5 * time.Minute
			}

			retryLogger.Warnf("Slack API rate limit: %s は %v 後にリトライします (%d/%d)", operation, retryAfter, attempt+1, r.maxRetries)
			
			time.Sleep(retryAfter)
			continue
		}

		// For other errors, don't retry
		break
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, r.maxRetries+1, lastErr)
}
