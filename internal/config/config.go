// Package config defines configuration and event types for worklogr.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iriam/worklogr/internal/utils"
	"gopkg.in/yaml.v3"
)

// ServiceConfig holds configuration for each external service
type ServiceConfig struct {
	ClientID               string        `yaml:"client_id"`
	ClientSecret           string        `yaml:"client_secret"`
	AccessToken            string        `yaml:"access_token"`
	RefreshToken           string        `yaml:"refresh_token"`
	TokenExpiresAt         *time.Time    `yaml:"token_expires_at,omitempty"`
	Enabled                bool          `yaml:"enabled"`
	UseOkta                bool          `yaml:"use_okta"`
	AutoRefresh            bool          `yaml:"auto_refresh"`
	ValidationInterval     time.Duration `yaml:"validation_interval"`
	MaxRetries             int           `yaml:"max_retries"`
	RetryBackoffMultiplier float64       `yaml:"retry_backoff_multiplier"`
}

// OktaConfig holds Okta OIDC configuration
type OktaConfig struct {
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
	Enabled      bool   `yaml:"enabled"`
}

// Config holds the application configuration
type Config struct {
	Slack        ServiceConfig `yaml:"slack"`
	GitHub       ServiceConfig `yaml:"github"`
	GoogleCal    ServiceConfig `yaml:"google_calendar"`
	GoogleCalendarOptions GoogleCalendarOptions `yaml:"google_calendar_options"`
	Okta         OktaConfig    `yaml:"okta"`
	DatabasePath string        `yaml:"database_path"`
	Timezone     string        `yaml:"timezone"`
}

// GoogleCalendarOptions controls optional calendar collection behavior.
// Note: FetchDriveAttachments defaults to true when omitted.
type GoogleCalendarOptions struct {
	FetchDriveAttachments  *bool `yaml:"fetch_drive_attachments"`
	AttachmentTextMaxChars int   `yaml:"attachment_text_max_chars"`
}

func (o GoogleCalendarOptions) ShouldFetchDriveAttachments() bool {
	if o.FetchDriveAttachments == nil {
		return true
	}
	return *o.FetchDriveAttachments
}

func (o GoogleCalendarOptions) EffectiveAttachmentTextMaxChars() int {
	if o.AttachmentTextMaxChars <= 0 {
		return 100000
	}
	return o.AttachmentTextMaxChars
}

// Event represents a collected event from any service
type Event struct {
	ID        string    `json:"id" db:"id"`
	Service   string    `json:"service" db:"service"`
	Type      string    `json:"type" db:"type"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	Metadata  string    `json:"metadata" db:"metadata"`
	UserID    string    `json:"user_id" db:"user_id"`
	// Attachments are stored separately (see DB table event_attachments).
	Attachments []EventAttachment `json:"attachments,omitempty" db:"-"`
}

// EventAttachment represents an attachment associated with an event.
// Primary use: Google Calendar Gemini notes stored as Google Docs.
type EventAttachment struct {
	FileID    string `json:"file_id" db:"file_id"`
	Title     string `json:"title,omitempty" db:"title"`
	MimeType  string `json:"mime_type,omitempty" db:"mime_type"`
	ExportAs  string `json:"export_as,omitempty" db:"export_as"`
	TextFull  string `json:"text_full,omitempty" db:"text_full"`
	Truncated bool   `json:"truncated,omitempty" db:"truncated"`
}

// LoadConfig loads configuration from the specified file
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		// First try current directory
		currentDir, err := os.Getwd()
		if err == nil {
			localConfigPath := filepath.Join(currentDir, "config.yaml")
			if _, err := os.Stat(localConfigPath); err == nil {
				configPath = localConfigPath
			}
		}
		
		// Fall back to home directory if no local config found
		if configPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			configPath = filepath.Join(homeDir, ".worklogr", "config.yaml")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config if it doesn't exist
			return createDefaultConfig(configPath)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default database path if not specified
	if config.DatabasePath == "" {
		homeDir, _ := os.UserHomeDir()
		config.DatabasePath = filepath.Join(homeDir, ".worklogr", "worklogr.db")
	}

	// Set default timezone if not specified
	if config.Timezone == "" {
		config.Timezone = "Asia/Tokyo" // Default to JST
	}

	// Defaults for optional Google Calendar behaviors
	// FetchDriveAttachments: default ON (true) when omitted
	// AttachmentTextMaxChars: default 100000 when omitted/invalid
	if config.GoogleCalendarOptions.AttachmentTextMaxChars <= 0 {
		config.GoogleCalendarOptions.AttachmentTextMaxChars = 100000
	}

	return &config, nil
}

// SaveConfig saves the configuration to the specified file
func SaveConfig(config *Config, configPath string) error {
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".worklogr", "config.yaml")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(configPath string) (*Config, error) {
	// Use current directory for database by default to avoid creating ~/.worklogr
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	config := &Config{
		Slack: ServiceConfig{
			Enabled: false,
		},
		GitHub: ServiceConfig{
			Enabled: false,
		},
		GoogleCal: ServiceConfig{
			Enabled: false,
		},
		GoogleCalendarOptions: GoogleCalendarOptions{
			// FetchDriveAttachments is intentionally left nil to mean "default ON".
			AttachmentTextMaxChars: 100000,
		},
		DatabasePath: filepath.Join(currentDir, "worklogr.db"),
		Timezone:     "Asia/Tokyo", // Default to JST
	}

	if err := SaveConfig(config, configPath); err != nil {
		return nil, fmt.Errorf("failed to save default config: %w", err)
	}

	// Validate timezone
	if err := config.ValidateTimezone(); err != nil {
		return nil, fmt.Errorf("invalid timezone configuration: %w", err)
	}

	return config, nil
}

// ValidateTimezone validates the configured timezone
func (c *Config) ValidateTimezone() error {
	validation := utils.ValidateTimezone(c.Timezone)
	if !validation.IsValid {
		return validation.Error
	}
	return nil
}

// GetTimezoneManager creates a TimezoneManager for this configuration
func (c *Config) GetTimezoneManager() (*utils.TimezoneManager, error) {
	return utils.NewTimezoneManager(c.Timezone)
}

// ToAuthConfig converts ServiceConfig to AuthConfig for authentication managers
func (sc *ServiceConfig) ToAuthConfig() *AuthConfig {
	return &AuthConfig{
		AccessToken:            sc.AccessToken,
		RefreshToken:           sc.RefreshToken,
		TokenExpiresAt:         sc.TokenExpiresAt,
		AutoRefresh:            sc.AutoRefresh,
		ValidationInterval:     sc.ValidationInterval,
		MaxRetries:             sc.MaxRetries,
		RetryBackoffMultiplier: sc.RetryBackoffMultiplier,
	}
}

// AuthConfig represents authentication configuration (compatible with auth package)
type AuthConfig struct {
	AccessToken            string        `yaml:"access_token"`
	RefreshToken           string        `yaml:"refresh_token"`
	TokenExpiresAt         *time.Time    `yaml:"token_expires_at,omitempty"`
	AutoRefresh            bool          `yaml:"auto_refresh"`
	ValidationInterval     time.Duration `yaml:"validation_interval"`
	MaxRetries             int           `yaml:"max_retries"`
	RetryBackoffMultiplier float64       `yaml:"retry_backoff_multiplier"`
}

// UpdateFromAuthConfig updates ServiceConfig from AuthConfig
func (sc *ServiceConfig) UpdateFromAuthConfig(authConfig *AuthConfig) {
	sc.AccessToken = authConfig.AccessToken
	sc.RefreshToken = authConfig.RefreshToken
	sc.TokenExpiresAt = authConfig.TokenExpiresAt
	sc.AutoRefresh = authConfig.AutoRefresh
	sc.ValidationInterval = authConfig.ValidationInterval
	sc.MaxRetries = authConfig.MaxRetries
	sc.RetryBackoffMultiplier = authConfig.RetryBackoffMultiplier
}

// SetDefaultAuthValues sets default values for authentication configuration
func (sc *ServiceConfig) SetDefaultAuthValues() {
	if sc.ValidationInterval == 0 {
		sc.ValidationInterval = 5 * time.Minute
	}
	if sc.MaxRetries == 0 {
		sc.MaxRetries = 3
	}
	if sc.RetryBackoffMultiplier == 0 {
		sc.RetryBackoffMultiplier = 2.0
	}
	// AutoRefresh defaults to false for security
}

// ValidateAuthConfig validates the authentication configuration
func (sc *ServiceConfig) ValidateAuthConfig() error {
	if sc.ValidationInterval < time.Minute {
		return fmt.Errorf("validation_interval must be at least 1 minute")
	}
	if sc.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	if sc.RetryBackoffMultiplier < 1.0 {
		return fmt.Errorf("retry_backoff_multiplier must be at least 1.0")
	}
	return nil
}

// MigrateConfig migrates old configuration format to new format
func (c *Config) MigrateConfig() {
	// Set default auth values for all services
	c.Slack.SetDefaultAuthValues()
	c.GitHub.SetDefaultAuthValues()
	c.GoogleCal.SetDefaultAuthValues()
}
