package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestConfig(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for test config: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
}

func TestLoadConfigPrefersLocalConfigOverHomeConfig(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()

	localConfigPath := filepath.Join(workDir, "config.yaml")
	homeConfigPath := filepath.Join(homeDir, ".worklogr", "config.yaml")

	writeTestConfig(t, localConfigPath, `
timezone: "UTC"
database_path: "./local.db"
google_calendar_options:
  attachment_text_max_chars: 2048
`)
	writeTestConfig(t, homeConfigPath, `
timezone: "Asia/Seoul"
database_path: "./home.db"
google_calendar_options:
  attachment_text_max_chars: 4096
`)

	t.Setenv("HOME", homeDir)
	t.Chdir(workDir)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Timezone != "UTC" {
		t.Fatalf("expected local config timezone, got %q", cfg.Timezone)
	}
	if cfg.DatabasePath != "./local.db" {
		t.Fatalf("expected local config database path, got %q", cfg.DatabasePath)
	}
	if got := cfg.GoogleCalendarOptions.AttachmentTextMaxChars; got != 2048 {
		t.Fatalf("expected local attachment_text_max_chars, got %d", got)
	}
}

func TestLoadConfigFallsBackToHomeConfig(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()
	homeConfigPath := filepath.Join(homeDir, ".worklogr", "config.yaml")

	writeTestConfig(t, homeConfigPath, `
timezone: "Europe/London"
database_path: "./home.db"
slack:
  enabled: true
  access_token: "xoxp-home"
`)

	t.Setenv("HOME", homeDir)
	t.Chdir(workDir)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Timezone != "Europe/London" {
		t.Fatalf("expected home config timezone, got %q", cfg.Timezone)
	}
	if cfg.DatabasePath != "./home.db" {
		t.Fatalf("expected home config database path, got %q", cfg.DatabasePath)
	}
	if !cfg.Slack.Enabled {
		t.Fatalf("expected slack enabled from home config")
	}
	if cfg.Slack.AccessToken != "xoxp-home" {
		t.Fatalf("expected slack token from home config, got %q", cfg.Slack.AccessToken)
	}
}

func TestLoadConfigCreatesDefaultConfigWhenMissing(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()
	defaultConfigPath := filepath.Join(homeDir, ".worklogr", "config.yaml")

	t.Setenv("HOME", homeDir)
	t.Chdir(workDir)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Timezone != "Asia/Tokyo" {
		t.Fatalf("expected default timezone Asia/Tokyo, got %q", cfg.Timezone)
	}
	expectedDBPath := filepath.Join(workDir, "worklogr.db")
	if cfg.DatabasePath != expectedDBPath {
		t.Fatalf("expected default database path %q, got %q", expectedDBPath, cfg.DatabasePath)
	}
	if got := cfg.GoogleCalendarOptions.AttachmentTextMaxChars; got != 100000 {
		t.Fatalf("expected default attachment_text_max_chars 100000, got %d", got)
	}
	if !cfg.GoogleCalendarOptions.ShouldFetchDriveAttachments() {
		t.Fatalf("expected fetch_drive_attachments default to true")
	}
	if _, err := os.Stat(defaultConfigPath); err != nil {
		t.Fatalf("expected default config file to be created: %v", err)
	}
}

func TestLoadConfigAppliesDefaultsForOptionalFields(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	writeTestConfig(t, configPath, `
slack:
  enabled: true
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Timezone != "Asia/Tokyo" {
		t.Fatalf("expected default timezone Asia/Tokyo, got %q", cfg.Timezone)
	}
	if cfg.DatabasePath == "" {
		t.Fatalf("expected default database path to be populated")
	}
	if got := cfg.GoogleCalendarOptions.AttachmentTextMaxChars; got != 100000 {
		t.Fatalf("expected default attachment_text_max_chars 100000, got %d", got)
	}
	if !cfg.GoogleCalendarOptions.ShouldFetchDriveAttachments() {
		t.Fatalf("expected fetch_drive_attachments default to true")
	}
}

func TestServiceConfigValidateAuthConfig(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name    string
		cfg     ServiceConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: ServiceConfig{
				AccessToken:            "token",
				TokenExpiresAt:         &now,
				ValidationInterval:     time.Minute,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.0,
			},
			wantErr: false,
		},
		{
			name: "validation interval below minimum",
			cfg: ServiceConfig{
				ValidationInterval:     30 * time.Second,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.0,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			cfg: ServiceConfig{
				ValidationInterval:     time.Minute,
				MaxRetries:             -1,
				RetryBackoffMultiplier: 1.0,
			},
			wantErr: true,
		},
		{
			name: "backoff below minimum",
			cfg: ServiceConfig{
				ValidationInterval:     time.Minute,
				MaxRetries:             1,
				RetryBackoffMultiplier: 0.9,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.ValidateAuthConfig()
			if tc.wantErr && err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
