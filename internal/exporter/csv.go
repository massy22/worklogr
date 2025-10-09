package exporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

// CSVExporter handles CSV export functionality
type CSVExporter struct{}

// NewCSVExporter creates a new CSV exporter
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{}
}

// ExportToCSV exports events to a CSV file
func (ce *CSVExporter) ExportToCSV(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_events_%s.csv", time.Now().Format("20060102_150405"))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp",
		"Service",
		"Type",
		"Title",
		"Content",
		"UserID",
		"Metadata",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write events
	for _, event := range events {
		record := []string{
			event.Timestamp.Format(time.RFC3339),
			event.Service,
			event.Type,
			event.Title,
			event.Content,
			event.UserID,
			event.Metadata,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	fmt.Printf("Events exported to CSV: %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportToCSVWithSummary exports events to CSV with a summary sheet
func (ce *CSVExporter) ExportToCSVWithSummary(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_events_summary_%s.csv", time.Now().Format("20060102_150405"))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write summary section
	if err := ce.writeSummarySection(writer, events); err != nil {
		return fmt.Errorf("failed to write summary section: %w", err)
	}

	// Write empty line
	writer.Write([]string{})

	// Write events section
	if err := ce.writeEventsSection(writer, events); err != nil {
		return fmt.Errorf("failed to write events section: %w", err)
	}

	fmt.Printf("Events with summary exported to CSV: %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportForSpreadsheet exports events in a format optimized for spreadsheet analysis
func (ce *CSVExporter) ExportForSpreadsheet(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_spreadsheet_%s.csv", time.Now().Format("20060102_150405"))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write enhanced header for spreadsheet analysis
	header := []string{
		"Date",
		"Time",
		"Timestamp",
		"Service",
		"Type",
		"Title",
		"Content",
		"UserID",
		"Hour",
		"DayOfWeek",
		"ContentLength",
		"HasMetadata",
		"Metadata",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write events with additional analysis columns
	for _, event := range events {
		record := []string{
			event.Timestamp.Format("2006-01-02"),
			event.Timestamp.Format("15:04:05"),
			event.Timestamp.Format(time.RFC3339),
			event.Service,
			event.Type,
			event.Title,
			event.Content,
			event.UserID,
			strconv.Itoa(event.Timestamp.Hour()),
			event.Timestamp.Weekday().String(),
			strconv.Itoa(len(event.Content)),
			strconv.FormatBool(event.Metadata != ""),
			event.Metadata,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	fmt.Printf("Spreadsheet-optimized events exported to CSV: %s\n", outputPath)
	fmt.Printf("Total events: %d\n", len(events))
	return nil
}

// ExportDailySummary exports a daily summary in CSV format
func (ce *CSVExporter) ExportDailySummary(events []*config.Event, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("worklogr_daily_summary_%s.csv", time.Now().Format("20060102_150405"))
	}

	// Group events by date and service
	dailySummary := ce.generateDailySummary(events)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Date",
		"Service",
		"EventCount",
		"EventTypes",
		"FirstEvent",
		"LastEvent",
		"Summary",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write daily summary records
	for date, services := range dailySummary {
		for service, summary := range services {
			record := []string{
				date,
				service,
				strconv.Itoa(summary.EventCount),
				ce.joinStrings(summary.EventTypes, "; "),
				summary.FirstEvent.Format("15:04:05"),
				summary.LastEvent.Format("15:04:05"),
				summary.Summary,
			}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("failed to write CSV record: %w", err)
			}
		}
	}

	fmt.Printf("Daily summary exported to CSV: %s\n", outputPath)
	return nil
}

// DailySummary represents a summary for a specific date and service
type DailySummary struct {
	EventCount int
	EventTypes []string
	FirstEvent time.Time
	LastEvent  time.Time
	Summary    string
}

// writeSummarySection writes the summary section to CSV
func (ce *CSVExporter) writeSummarySection(writer *csv.Writer, events []*config.Event) error {
	// Write summary header
	writer.Write([]string{"SUMMARY"})
	writer.Write([]string{})

	// Basic statistics
	writer.Write([]string{"Total Events", strconv.Itoa(len(events))})
	
	if len(events) > 0 {
		timeRange := ce.calculateTimeRange(events)
		writer.Write([]string{"Date Range", fmt.Sprintf("%s to %s", 
			timeRange.Start.Format("2006-01-02"), 
			timeRange.End.Format("2006-01-02"))})
	}

	// Service breakdown
	serviceStats := ce.getServiceStatistics(events)
	writer.Write([]string{})
	writer.Write([]string{"SERVICE BREAKDOWN"})
	writer.Write([]string{"Service", "Event Count", "Percentage"})
	
	total := len(events)
	for service, count := range serviceStats {
		percentage := float64(count) / float64(total) * 100
		writer.Write([]string{
			service, 
			strconv.Itoa(count), 
			fmt.Sprintf("%.1f%%", percentage),
		})
	}

	return nil
}

// writeEventsSection writes the events section to CSV
func (ce *CSVExporter) writeEventsSection(writer *csv.Writer, events []*config.Event) error {
	// Write events header
	writer.Write([]string{"EVENTS"})
	writer.Write([]string{})

	// Write column headers
	header := []string{
		"Timestamp",
		"Service",
		"Type",
		"Title",
		"Content",
		"UserID",
		"Metadata",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write events
	for _, event := range events {
		record := []string{
			event.Timestamp.Format(time.RFC3339),
			event.Service,
			event.Type,
			event.Title,
			event.Content,
			event.UserID,
			event.Metadata,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// calculateTimeRange calculates the time range of events
func (ce *CSVExporter) calculateTimeRange(events []*config.Event) *TimeRange {
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

// getServiceStatistics returns event count by service
func (ce *CSVExporter) getServiceStatistics(events []*config.Event) map[string]int {
	stats := make(map[string]int)
	for _, event := range events {
		stats[event.Service]++
	}
	return stats
}

// generateDailySummary generates a daily summary grouped by date and service
func (ce *CSVExporter) generateDailySummary(events []*config.Event) map[string]map[string]*DailySummary {
	summary := make(map[string]map[string]*DailySummary)

	for _, event := range events {
		date := event.Timestamp.Format("2006-01-02")
		service := event.Service

		if summary[date] == nil {
			summary[date] = make(map[string]*DailySummary)
		}

		if summary[date][service] == nil {
			summary[date][service] = &DailySummary{
				EventCount: 0,
				EventTypes: []string{},
				FirstEvent: event.Timestamp,
				LastEvent:  event.Timestamp,
				Summary:    "",
			}
		}

		dailySummary := summary[date][service]
		dailySummary.EventCount++

		// Update time range
		if event.Timestamp.Before(dailySummary.FirstEvent) {
			dailySummary.FirstEvent = event.Timestamp
		}
		if event.Timestamp.After(dailySummary.LastEvent) {
			dailySummary.LastEvent = event.Timestamp
		}

		// Add event type if not already present
		typeExists := false
		for _, eventType := range dailySummary.EventTypes {
			if eventType == event.Type {
				typeExists = true
				break
			}
		}
		if !typeExists {
			dailySummary.EventTypes = append(dailySummary.EventTypes, event.Type)
		}

		// Generate summary text
		dailySummary.Summary = ce.generateSummaryText(dailySummary)
	}

	return summary
}

// generateSummaryText generates a summary text for a daily summary
func (ce *CSVExporter) generateSummaryText(summary *DailySummary) string {
	duration := summary.LastEvent.Sub(summary.FirstEvent)
	return fmt.Sprintf("%d events over %v", summary.EventCount, duration.Round(time.Minute))
}

// joinStrings joins a slice of strings with a separator
func (ce *CSVExporter) joinStrings(strings []string, separator string) string {
	if len(strings) == 0 {
		return ""
	}
	
	result := strings[0]
	for i := 1; i < len(strings); i++ {
		result += separator + strings[i]
	}
	return result
}

// ValidateCSV validates that the exported CSV is valid
func (ce *CSVExporter) ValidateCSV(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Validate header
	expectedHeader := []string{"Timestamp", "Service", "Type", "Title", "Content", "UserID", "Metadata"}
	if len(records[0]) < len(expectedHeader) {
		return fmt.Errorf("CSV header is incomplete")
	}

	return nil
}

// ConvertJSONToCSV converts a JSON export file to CSV format
func (ce *CSVExporter) ConvertJSONToCSV(jsonPath, csvPath string) error {
	// Read JSON file
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse JSON
	var exportData struct {
		Events []*config.Event `json:"events"`
	}
	if err := json.Unmarshal(jsonData, &exportData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Export to CSV
	return ce.ExportToCSV(exportData.Events, csvPath)
}
