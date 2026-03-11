package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func newJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestValidateSlackTokenReturnsValidStatus(t *testing.T) {
	tv := NewTokenValidator()
	tv.httpClient = newTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://slack.com/api/auth.test" {
			t.Fatalf("unexpected slack validation url: %s", req.URL.String())
		}
		return newJSONResponse(http.StatusOK, `{"ok":true,"user":"kazuki","team":"dev"}`), nil
	})

	status, err := tv.ValidateSlackToken(context.Background(), "xoxp-token")
	if err != nil {
		t.Fatalf("ValidateSlackToken returned error: %v", err)
	}
	if !status.IsValid || status.ErrorMessage != "" {
		t.Fatalf("expected valid slack status, got %+v", status)
	}
}

func TestValidateGitHubTokenReturnsAPIErrorStatus(t *testing.T) {
	tv := NewTokenValidator()
	tv.httpClient = newTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.github.com/user" {
			t.Fatalf("unexpected github validation url: %s", req.URL.String())
		}
		return newJSONResponse(http.StatusUnauthorized, `{"message":"Bad credentials"}`), nil
	})

	status, err := tv.ValidateGitHubToken(context.Background(), "ghp-token")
	if err != nil {
		t.Fatalf("ValidateGitHubToken returned error: %v", err)
	}
	if status.IsValid {
		t.Fatalf("expected invalid github status, got %+v", status)
	}
	if !strings.Contains(status.ErrorMessage, "Bad credentials") {
		t.Fatalf("expected bad credentials message, got %q", status.ErrorMessage)
	}
}

func TestValidateGoogleTokenSetsExpiry(t *testing.T) {
	tv := NewTokenValidator()
	tv.httpClient = newTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.String(), "https://www.googleapis.com/oauth2/v1/tokeninfo") {
			t.Fatalf("unexpected google validation url: %s", req.URL.String())
		}
		return newJSONResponse(http.StatusOK, `{"audience":"client","scope":"calendar.readonly","expires_in":3600}`), nil
	})

	before := time.Now()
	status, err := tv.ValidateGoogleToken(context.Background(), "google-token")
	if err != nil {
		t.Fatalf("ValidateGoogleToken returned error: %v", err)
	}
	if !status.IsValid {
		t.Fatalf("expected valid google status, got %+v", status)
	}
	if status.ExpiresAt == nil || status.ExpiresAt.Before(before.Add(59*time.Minute)) {
		t.Fatalf("expected expiry around one hour in future, got %+v", status.ExpiresAt)
	}
}

func TestValidateTokenByServiceRejectsUnsupportedService(t *testing.T) {
	tv := NewTokenValidator()

	if _, err := tv.ValidateTokenByService(context.Background(), "notion", "token"); err == nil {
		t.Fatalf("expected unsupported service validation to fail")
	}
}

func TestCheckServiceHealthReturnsSuggestionOnInvalidToken(t *testing.T) {
	hc := NewHealthChecker()
	hc.validator.httpClient = newTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial tcp timeout")
	})

	result, err := hc.CheckServiceHealth(context.Background(), "slack", "xoxp-token")
	if err != nil {
		t.Fatalf("CheckServiceHealth returned error: %v", err)
	}
	if result.Status.IsValid {
		t.Fatalf("expected invalid health status, got %+v", result.Status)
	}
	if len(result.Suggestions) == 0 {
		t.Fatalf("expected health check suggestions for invalid token")
	}
}
