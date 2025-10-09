package utils

import (
	"fmt"
	"strings"
	"time"
)

// TimezoneManager manages timezone operations for the application
type TimezoneManager struct {
	timezone string
	location *time.Location
}

// TimezoneValidation represents the result of timezone validation
type TimezoneValidation struct {
	IsValid  bool
	Error    error
	Location *time.Location
}

// TimeRange represents a time range with timezone information
type TimeRange struct {
	Start    time.Time
	End      time.Time
	Timezone string
}

// NewTimezoneManager creates a new TimezoneManager with the specified timezone
func NewTimezoneManager(timezone string) (*TimezoneManager, error) {
	if timezone == "" {
		timezone = "Asia/Tokyo" // Default timezone
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone '%s': %w", timezone, err)
	}

	return &TimezoneManager{
		timezone: timezone,
		location: location,
	}, nil
}

// GetTimezone returns the configured timezone string
func (tm *TimezoneManager) GetTimezone() string {
	return tm.timezone
}

// GetLocation returns the time.Location for the configured timezone
func (tm *TimezoneManager) GetLocation() *time.Location {
	return tm.location
}

// ConvertToTimezone converts a time to the configured timezone
func (tm *TimezoneManager) ConvertToTimezone(t time.Time) time.Time {
	return t.In(tm.location)
}

// ParseTimeInTimezone parses a time string in the configured timezone
func (tm *TimezoneManager) ParseTimeInTimezone(timeStr string) (time.Time, error) {
	// Try different time formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, timeStr, tm.location); err == nil {
			return t, nil
		}
	}

	// Try relative time parsing
	if relativeTime, err := tm.parseRelativeTime(timeStr); err == nil {
		return relativeTime, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse time '%s' in timezone '%s'", timeStr, tm.timezone)
}

// parseRelativeTime parses relative time strings like "today", "yesterday", etc.
func (tm *TimezoneManager) parseRelativeTime(timeStr string) (time.Time, error) {
	now := time.Now().In(tm.location)
	
	switch strings.ToLower(timeStr) {
	case "now":
		return now, nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tm.location), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, tm.location), nil
	}

	// Parse duration-based relative time (e.g., "1d", "2h", "30m")
	if len(timeStr) > 1 {
		unit := timeStr[len(timeStr)-1:]
		valueStr := timeStr[:len(timeStr)-1]
		
		var value int
		if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
			return time.Time{}, fmt.Errorf("invalid relative time value: %s", valueStr)
		}

		switch unit {
		case "d":
			return now.AddDate(0, 0, -value), nil
		case "h":
			return now.Add(time.Duration(-value) * time.Hour), nil
		case "m":
			return now.Add(time.Duration(-value) * time.Minute), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported relative time format: %s", timeStr)
}

// CreateTimeRange creates a TimeRange with proper timezone handling
func (tm *TimezoneManager) CreateTimeRange(startTime, endTime time.Time) *TimeRange {
	return &TimeRange{
		Start:    tm.ConvertToTimezone(startTime),
		End:      tm.ConvertToTimezone(endTime),
		Timezone: tm.timezone,
	}
}

// ValidateTimeRange validates that a time range is reasonable
func (tm *TimezoneManager) ValidateTimeRange(startTime, endTime time.Time) error {
	start := tm.ConvertToTimezone(startTime)
	end := tm.ConvertToTimezone(endTime)

	if start.After(end) {
		return fmt.Errorf("start time cannot be after end time")
	}

	now := time.Now().In(tm.location)
	if end.After(now) {
		return fmt.Errorf("end time cannot be in the future")
	}

	// Check if time range is too large (more than 1 year)
	if end.Sub(start) > 365*24*time.Hour {
		return fmt.Errorf("time range cannot exceed 1 year")
	}

	return nil
}

// FormatTimeForAPI formats time for API calls (RFC3339)
func (tm *TimezoneManager) FormatTimeForAPI(t time.Time) string {
	return tm.ConvertToTimezone(t).Format(time.RFC3339)
}

// FormatTimeForDisplay formats time for user display
func (tm *TimezoneManager) FormatTimeForDisplay(t time.Time) string {
	return tm.ConvertToTimezone(t).Format("2006-01-02 15:04:05")
}

// ValidateTimezone validates a timezone string and returns validation result
func ValidateTimezone(timezone string) TimezoneValidation {
	if timezone == "" {
		timezone = "Asia/Tokyo" // Default timezone
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return TimezoneValidation{
			IsValid:  false,
			Error:    fmt.Errorf("invalid timezone '%s': %w", timezone, err),
			Location: nil,
		}
	}

	return TimezoneValidation{
		IsValid:  true,
		Error:    nil,
		Location: location,
	}
}

// GetSupportedTimezones returns a list of commonly used timezones
func GetSupportedTimezones() []string {
	return []string{
		"Asia/Tokyo",
		"UTC",
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Paris",
		"Asia/Shanghai",
		"Asia/Seoul",
		"Australia/Sydney",
		"America/Chicago",
	}
}

// IsTimezoneSupported checks if a timezone is in the commonly supported list
func IsTimezoneSupported(timezone string) bool {
	supported := GetSupportedTimezones()
	for _, tz := range supported {
		if tz == timezone {
			return true
		}
	}
	return false
}
