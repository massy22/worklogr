package services

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetRetryAfterDelayParsesAndClampsValues(t *testing.T) {
	resp := &http.Response{
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader("")),
	}

	resp.Header.Set("Retry-After", "0")
	if got := getRetryAfterDelay(resp); got != time.Second {
		t.Fatalf("expected minimum retry delay 1s, got %v", got)
	}

	resp.Header.Set("Retry-After", "600")
	if got := getRetryAfterDelay(resp); got != 5*time.Minute {
		t.Fatalf("expected capped retry delay 5m, got %v", got)
	}

	resp.Header.Del("Retry-After")
	if got := getRetryAfterDelay(resp); got != 60*time.Second {
		t.Fatalf("expected default retry delay 60s, got %v", got)
	}
}

func TestRetryTransportReturnsResponseWithoutRetryOnSuccess(t *testing.T) {
	rt := &RetryTransport{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
		MaxRetries: 2,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRetryTransportReturnsErrorAfterNetworkFailure(t *testing.T) {
	rt := &RetryTransport{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network down")
		}),
		MaxRetries: 0,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatalf("expected RoundTrip to return network error")
	}
}

func TestRetryableAPICallRetriesRateLimitAndEventuallySucceeds(t *testing.T) {
	client := &RetryableSlackClient{maxRetries: 2}
	attempts := 0

	err := client.RetryableAPICall("search.messages", func() error {
		attempts++
		if attempts == 1 {
			return &slack.RateLimitedError{RetryAfter: 0}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryableAPICall returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryableAPICallStopsOnNonRetryableError(t *testing.T) {
	client := &RetryableSlackClient{maxRetries: 2}
	attempts := 0

	err := client.RetryableAPICall("auth.test", func() error {
		attempts++
		return fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatalf("expected RetryableAPICall to return error")
	}
	if attempts != 1 {
		t.Fatalf("expected non-retryable error to stop after 1 attempt, got %d", attempts)
	}
}
