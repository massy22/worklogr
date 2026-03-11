package app

import (
	"fmt"
	"testing"

	"github.com/iriam/worklogr/internal/config"
)

func TestConfigShowUsecaseRunBuildsDisplayResult(t *testing.T) {
	usecase := &ConfigShowUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return &config.Config{
					DatabasePath: "/tmp/worklogr.db",
					Timezone:     "UTC",
					Slack: config.ServiceConfig{
						Enabled:      true,
						ClientID:     "slack-id",
						ClientSecret: "slack-secret",
					},
					GitHub: config.ServiceConfig{
						Enabled: true,
					},
					GoogleCal: config.ServiceConfig{
						Enabled:      true,
						ClientID:     "google-id",
						ClientSecret: "google-secret",
					},
				}, nil
			},
		},
	}

	result, err := usecase.Run(ConfigShowRequest{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.DatabasePath != "/tmp/worklogr.db" || result.Timezone != "UTC" {
		t.Fatalf("unexpected config show result: %+v", result)
	}
	if len(result.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(result.Services))
	}
	if result.Services[0].Name != "slack" || result.Services[0].DisplayName != "Slack" || !result.Services[0].Enabled || !result.Services[0].Configured {
		t.Fatalf("unexpected slack config show service: %+v", result.Services[0])
	}
	if result.Services[1].Name != "github" || result.Services[1].DisplayName != "GitHub" || !result.Services[1].Enabled || result.Services[1].Configured {
		t.Fatalf("unexpected github config show service: %+v", result.Services[1])
	}
}

func TestConfigShowUsecaseRunReturnsConfigLoadError(t *testing.T) {
	usecase := &ConfigShowUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return nil, fmt.Errorf("boom")
			},
		},
	}

	if _, err := usecase.Run(ConfigShowRequest{}); err == nil {
		t.Fatalf("expected Run to return config load error")
	}
}
