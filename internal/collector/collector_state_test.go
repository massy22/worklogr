package collector

import (
	"slices"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

func TestInitializeServicesForRejectsInvalidRequests(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: false},
		GitHub:    config.ServiceConfig{Enabled: true},
		GoogleCal: config.ServiceConfig{Enabled: false},
	}
	ec := NewEventCollector(cfg, nil)

	testCases := []struct {
		name         string
		serviceNames []string
	}{
		{name: "empty service list", serviceNames: nil},
		{name: "unknown service", serviceNames: []string{"unknown"}},
		{name: "disabled service", serviceNames: []string{"slack"}},
		{name: "missing token", serviceNames: []string{"github"}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := ec.InitializeServicesFor(tc.serviceNames); err == nil {
				t.Fatalf("expected InitializeServicesFor to return error for %v", tc.serviceNames)
			}
		})
	}
}

func TestPrioritizeCalendarServiceMovesCalendarFirst(t *testing.T) {
	got := prioritizeCalendarService([]string{"slack", "google_calendar", "github"})
	want := []string{"google_calendar", "slack", "github"}

	if len(got) != len(want) {
		t.Fatalf("expected %d services, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected order %v, got %v", want, got)
		}
	}
}

func TestGetEnabledAndAvailableServices(t *testing.T) {
	ec := &EventCollector{
		config: &config.Config{
			Slack:     config.ServiceConfig{Enabled: true},
			GitHub:    config.ServiceConfig{Enabled: false},
			GoogleCal: config.ServiceConfig{Enabled: true},
		},
		services: map[string]ServiceClient{
			"github": &mockServiceClient{},
			"slack":  &mockServiceClient{},
		},
	}

	enabled := ec.GetEnabledServices()
	wantEnabled := []string{"slack", "google_calendar"}
	if len(enabled) != len(wantEnabled) {
		t.Fatalf("expected enabled services %v, got %v", wantEnabled, enabled)
	}
	for i := range wantEnabled {
		if enabled[i] != wantEnabled[i] {
			t.Fatalf("expected enabled services %v, got %v", wantEnabled, enabled)
		}
	}

	available := ec.GetAvailableServices()
	slices.Sort(available)
	wantAvailable := []string{"github", "slack"}
	if !slices.Equal(available, wantAvailable) {
		t.Fatalf("expected available services %v, got %v", wantAvailable, available)
	}
}

func TestGetServiceStatusReflectsConfigAndInitialization(t *testing.T) {
	ec := &EventCollector{
		config: &config.Config{
			Slack: config.ServiceConfig{
				Enabled:     true,
				AccessToken: "xoxp-token",
			},
			GitHub: config.ServiceConfig{
				Enabled: false,
			},
			GoogleCal: config.ServiceConfig{
				Enabled: true,
			},
		},
		services: map[string]ServiceClient{
			"slack": &mockServiceClient{},
		},
	}

	status := ec.GetServiceStatus()

	if !status["slack"].Enabled || !status["slack"].Authenticated || !status["slack"].Initialized {
		t.Fatalf("unexpected slack status: %+v", status["slack"])
	}
	if status["github"].Enabled || status["github"].Authenticated || status["github"].Initialized {
		t.Fatalf("unexpected github status: %+v", status["github"])
	}
	if !status["google_calendar"].Enabled || !status["google_calendar"].Authenticated || status["google_calendar"].Initialized {
		t.Fatalf("unexpected google_calendar status: %+v", status["google_calendar"])
	}
}

func TestValidateTimeRangeRejectsInvalidRanges(t *testing.T) {
	ec := &EventCollector{}
	now := time.Now()

	if err := ec.ValidateTimeRange(now.Add(-time.Hour), now); err != nil {
		t.Fatalf("expected valid range, got %v", err)
	}
	if err := ec.ValidateTimeRange(now, now.Add(-time.Hour)); err == nil {
		t.Fatalf("expected start after end to fail")
	}
	if err := ec.ValidateTimeRange(now.Add(-time.Hour), now.Add(time.Hour)); err == nil {
		t.Fatalf("expected future end time to fail")
	}
	if err := ec.ValidateTimeRange(now.Add(-366*24*time.Hour), now); err == nil {
		t.Fatalf("expected range over one year to fail")
	}
}
