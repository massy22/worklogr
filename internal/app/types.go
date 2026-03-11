package app

import "time"

type TimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}

type ConfiguredService struct {
	Name        string
	DisplayName string
	Enabled     bool
	Configured  bool
}

type serviceDefinition struct {
	Name        string
	DisplayName string
}

var appServiceDefinitions = []serviceDefinition{
	{Name: "slack", DisplayName: "Slack"},
	{Name: "github", DisplayName: "GitHub"},
	{Name: "google_calendar", DisplayName: "Google Calendar"},
}
