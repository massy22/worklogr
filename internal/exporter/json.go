package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

// JSONExporter handles JSON export functionality
type JSONExporter struct{}

// NewJSONExporter creates a new JSON exporter
func NewJSONExporter() *JSONExporter {
	return &JSONExporter{}
}

// ExportToJSON exports events to a JSON file
func (je *JSONExporter) ExportToJSON(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_events_%s.json", time.Now().Format("20060102_150405"))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create export data structure
	exportData := &ExportData{
		ExportedAt:   time.Now(),
		EventCount:   len(events),
		TimeRange:    je.calculateTimeRange(events),
		Services:     je.getServicesSummary(events),
		Events:       events,
		Metadata:     je.createMetadata(events),
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal events to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("Events exported to JSON: %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportToJSONString exports events to a JSON string
func (je *JSONExporter) ExportToJSONString(events []*config.Event) (string, error) {
	exportData := &ExportData{
		ExportedAt:   time.Now(),
		EventCount:   len(events),
		TimeRange:    je.calculateTimeRange(events),
		Services:     je.getServicesSummary(events),
		Events:       events,
		Metadata:     je.createMetadata(events),
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal events to JSON: %w", err)
	}

	return string(jsonData), nil
}

// ExportEventsOnly exports only the events array without metadata
func (je *JSONExporter) ExportEventsOnly(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_events_simple_%s.json", time.Now().Format("20060102_150405"))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal events directly
	jsonData, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal events to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("Events exported to JSON (simple format): %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportForAI exports events in a format optimized for AI processing
func (je *JSONExporter) ExportForAI(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_ai_ready_%s.json", time.Now().Format("20060102_150405"))
	}

	// Create AI-optimized format
	aiData := &AIExportData{
		Summary: AIExportSummary{
			TotalEvents:    len(events),
			DateRange:      je.calculateTimeRange(events),
			ServicesUsed:   je.getServiceNames(events),
			ExportedAt:     time.Now().Format(time.RFC3339),
			Purpose:        "Daily report generation for AI processing",
		},
		Events: je.convertEventsForAI(events),
		Statistics: je.generateStatistics(events),
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(aiData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal AI data to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("AI-ready events exported to JSON: %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportData represents the complete export structure
type ExportData struct {
	ExportedAt   time.Time                  `json:"exported_at"`
	EventCount   int                        `json:"event_count"`
	TimeRange    *TimeRange                 `json:"time_range"`
	Services     map[string]ServiceSummary  `json:"services"`
	Events       []*config.Event            `json:"events"`
	Metadata     *ExportMetadata            `json:"metadata"`
}

// AIExportData represents AI-optimized export structure
type AIExportData struct {
	Summary    AIExportSummary `json:"summary"`
	Events     []AIEvent       `json:"events"`
	Statistics *Statistics     `json:"statistics"`
}

// AIExportSummary provides a summary for AI processing
type AIExportSummary struct {
	TotalEvents  int      `json:"total_events"`
	DateRange    *TimeRange `json:"date_range"`
	ServicesUsed []string `json:"services_used"`
	ExportedAt   string   `json:"exported_at"`
	Purpose      string   `json:"purpose"`
}

// AIEvent represents an event optimized for AI processing
type AIEvent struct {
	Timestamp   string                 `json:"timestamp"`
	Service     string                 `json:"service"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ServiceSummary provides summary information about a service
type ServiceSummary struct {
	EventCount int      `json:"event_count"`
	EventTypes []string `json:"event_types"`
}

// ExportMetadata contains metadata about the export
type ExportMetadata struct {
	Version     string            `json:"version"`
	Generator   string            `json:"generator"`
	Format      string            `json:"format"`
	Compression string            `json:"compression"`
	Checksum    string            `json:"checksum,omitempty"`
}

// Statistics provides statistical information about the events
type Statistics struct {
	EventsByService map[string]int `json:"events_by_service"`
	EventsByType    map[string]int `json:"events_by_type"`
	EventsByHour    map[int]int    `json:"events_by_hour"`
	EventsByDay     map[string]int `json:"events_by_day"`
}

// calculateTimeRange calculates the time range of events
func (je *JSONExporter) calculateTimeRange(events []*config.Event) *TimeRange {
	if len(events) == 0 {
		return nil
	}

	start := events[0].Timestamp
	end := events[0].Timestamp

	for _, event := range events {
		if event.Timestamp.Before(start) {
			start = event.Timestamp
		}
		if event.Timestamp.After(end) {
			end = event.Timestamp
		}
	}

	return &TimeRange{
		Start: start,
		End:   end,
	}
}

// getServicesSummary creates a summary of services and their events
func (je *JSONExporter) getServicesSummary(events []*config.Event) map[string]ServiceSummary {
	services := make(map[string]ServiceSummary)

	for _, event := range events {
		summary, exists := services[event.Service]
		if !exists {
			summary = ServiceSummary{
				EventCount: 0,
				EventTypes: []string{},
			}
		}

		summary.EventCount++

		// Add event type if not already present
		typeExists := false
		for _, eventType := range summary.EventTypes {
			if eventType == event.Type {
				typeExists = true
				break
			}
		}
		if !typeExists {
			summary.EventTypes = append(summary.EventTypes, event.Type)
		}

		services[event.Service] = summary
	}

	return services
}

// getServiceNames returns a list of unique service names
func (je *JSONExporter) getServiceNames(events []*config.Event) []string {
	serviceMap := make(map[string]bool)
	for _, event := range events {
		serviceMap[event.Service] = true
	}

	var services []string
	for service := range serviceMap {
		services = append(services, service)
	}

	return services
}

// convertEventsForAI converts events to AI-optimized format
func (je *JSONExporter) convertEventsForAI(events []*config.Event) []AIEvent {
	var aiEvents []AIEvent

	for _, event := range events {
		aiEvent := AIEvent{
			Timestamp: event.Timestamp.Format(time.RFC3339),
			Service:   event.Service,
			Type:      event.Type,
			Title:     event.Title,
			Content:   event.Content,
		}

		// Parse metadata if available
		if event.Metadata != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(event.Metadata), &metadata); err == nil {
				aiEvent.Context = metadata
			}
		}

		aiEvents = append(aiEvents, aiEvent)
	}

	return aiEvents
}

// generateStatistics generates statistical information about events
func (je *JSONExporter) generateStatistics(events []*config.Event) *Statistics {
	stats := &Statistics{
		EventsByService: make(map[string]int),
		EventsByType:    make(map[string]int),
		EventsByHour:    make(map[int]int),
		EventsByDay:     make(map[string]int),
	}

	for _, event := range events {
		// Count by service
		stats.EventsByService[event.Service]++

		// Count by type
		stats.EventsByType[event.Type]++

		// Count by hour
		hour := event.Timestamp.Hour()
		stats.EventsByHour[hour]++

		// Count by day
		day := event.Timestamp.Format("2006-01-02")
		stats.EventsByDay[day]++
	}

	return stats
}

// createMetadata creates export metadata
func (je *JSONExporter) createMetadata(events []*config.Event) *ExportMetadata {
	return &ExportMetadata{
		Version:     "1.0",
		Generator:   "worklogr",
		Format:      "json",
		Compression: "none",
	}
}

// ValidateJSON validates that the exported JSON is valid
func (je *JSONExporter) ValidateJSON(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	var exportData ExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	return nil
}
