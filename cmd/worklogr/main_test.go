package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

func writeCommandTestConfig(t *testing.T, timezone string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	body := []byte("timezone: \"" + timezone + "\"\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatalf("failed to write command test config: %v", err)
	}
	return path
}

func TestParseTimeRangeRejectsStartAfterEnd(t *testing.T) {
	configPath = writeCommandTestConfig(t, "UTC")

	_, _, err := parseTimeRange("2026-03-06 12:00:00", "2026-03-06 11:00:00")
	if err == nil {
		t.Fatalf("expected parseTimeRange to reject start after end")
	}
}

func TestParseTimeStringUsesConfiguredTimezone(t *testing.T) {
	configPath = writeCommandTestConfig(t, "UTC")

	got, err := parseTimeString("2026-03-06 12:34:56")
	if err != nil {
		t.Fatalf("parseTimeString returned error: %v", err)
	}

	expected := time.Date(2026, 3, 6, 12, 34, 56, 0, time.UTC)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestResolveCollectServicesReturnsEnabledServicesInOrder(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: false},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	got, err := resolveCollectServices(cfg, nil)
	if err != nil {
		t.Fatalf("resolveCollectServices returned error: %v", err)
	}

	want := []string{"slack", "google_calendar"}
	if len(got) != len(want) {
		t.Fatalf("expected %d services, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected services %v, got %v", want, got)
		}
	}
}

func TestResolveCollectServicesNormalizesAndDeduplicates(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: true},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	got, err := resolveCollectServices(cfg, []string{" Slack ", "github", "slack", "GOOGLE_CALENDAR"})
	if err != nil {
		t.Fatalf("resolveCollectServices returned error: %v", err)
	}

	want := []string{"slack", "github", "google_calendar"}
	if len(got) != len(want) {
		t.Fatalf("expected %d services, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected services %v, got %v", want, got)
		}
	}
}

func TestResolveCollectServicesRejectsUnknownOrDisabledServices(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: false},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	if _, err := resolveCollectServices(cfg, []string{"unknown"}); err == nil {
		t.Fatalf("expected unknown service to return error")
	}

	if _, err := resolveCollectServices(cfg, []string{"github"}); err == nil {
		t.Fatalf("expected disabled service to return error")
	}
}
