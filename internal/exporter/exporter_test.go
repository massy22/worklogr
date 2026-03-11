package exporter

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

func sampleEvents() []*config.Event {
	base := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	return []*config.Event{
		{
			ID:        "event-1",
			Service:   "slack",
			Type:      "message",
			Title:     "Daily update",
			Content:   "shared status",
			Timestamp: base,
			Metadata:  `{"channel":"dev"}`,
			UserID:    "U123",
			Attachments: []config.EventAttachment{
				{FileID: "file-1", Title: "notes", TextFull: "attachment body"},
			},
		},
		{
			ID:        "event-2",
			Service:   "github",
			Type:      "pull_request",
			Title:     "Opened PR",
			Content:   "refactor collector",
			Timestamp: base.Add(2 * time.Hour),
			Metadata:  `{"repo":"worklogr"}`,
			UserID:    "octocat",
		},
	}
}

func readCSVRecords(t *testing.T, path string) [][]string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open csv file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv file: %v", err)
	}
	return records
}

func containsRow(records [][]string, want []string) bool {
	for _, record := range records {
		if len(record) != len(want) {
			continue
		}
		match := true
		for i := range want {
			if record[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestJSONExporterExportToJSONStringIncludesSummary(t *testing.T) {
	exporter := NewJSONExporter()
	events := sampleEvents()

	jsonString, err := exporter.ExportToJSONString(events)
	if err != nil {
		t.Fatalf("ExportToJSONString returned error: %v", err)
	}

	var got ExportData
	if err := json.Unmarshal([]byte(jsonString), &got); err != nil {
		t.Fatalf("failed to unmarshal exported json: %v", err)
	}

	if got.EventCount != 2 {
		t.Fatalf("expected event count 2, got %d", got.EventCount)
	}
	if got.TimeRange == nil {
		t.Fatalf("expected time range to be present")
	}
	if !got.TimeRange.Start.Equal(events[0].Timestamp) {
		t.Fatalf("expected time range start %v, got %v", events[0].Timestamp, got.TimeRange.Start)
	}
	if !got.TimeRange.End.Equal(events[1].Timestamp) {
		t.Fatalf("expected time range end %v, got %v", events[1].Timestamp, got.TimeRange.End)
	}
	if got.Services["slack"].EventCount != 1 {
		t.Fatalf("expected slack event count 1, got %d", got.Services["slack"].EventCount)
	}
	if got.Services["github"].EventTypes[0] != "pull_request" {
		t.Fatalf("expected github event type pull_request, got %+v", got.Services["github"].EventTypes)
	}
	if got.Metadata == nil || got.Metadata.Generator != "worklogr" || got.Metadata.Format != "json" {
		t.Fatalf("unexpected export metadata: %+v", got.Metadata)
	}
}

func TestJSONExporterConvertEventsForAIIncludesMetadataAndAttachments(t *testing.T) {
	exporter := NewJSONExporter()
	events := sampleEvents()

	aiEvents := exporter.convertEventsForAI(events)
	if len(aiEvents) != 2 {
		t.Fatalf("expected 2 ai events, got %d", len(aiEvents))
	}

	first := aiEvents[0]
	if first.Context["channel"] != "dev" {
		t.Fatalf("expected metadata channel to be included, got %+v", first.Context)
	}

	attachments, ok := first.Context["attachments"].([]config.EventAttachment)
	if !ok {
		t.Fatalf("expected attachments in AI context, got %T", first.Context["attachments"])
	}
	if len(attachments) != 1 || attachments[0].FileID != "file-1" {
		t.Fatalf("unexpected attachments in AI context: %+v", attachments)
	}
}

func TestJSONExporterExportToJSONStringWithEmptyEvents(t *testing.T) {
	exporter := NewJSONExporter()

	jsonString, err := exporter.ExportToJSONString(nil)
	if err != nil {
		t.Fatalf("ExportToJSONString returned error: %v", err)
	}

	var got ExportData
	if err := json.Unmarshal([]byte(jsonString), &got); err != nil {
		t.Fatalf("failed to unmarshal exported json: %v", err)
	}

	if got.EventCount != 0 {
		t.Fatalf("expected event count 0, got %d", got.EventCount)
	}
	if got.TimeRange != nil {
		t.Fatalf("expected no time range for empty events, got %+v", got.TimeRange)
	}
}

func TestCSVExporterExportToCSVWritesHeaderAndRows(t *testing.T) {
	exporter := NewCSVExporter()
	outputPath := filepath.Join(t.TempDir(), "events.csv")

	if err := exporter.ExportToCSV(sampleEvents(), outputPath); err != nil {
		t.Fatalf("ExportToCSV returned error: %v", err)
	}

	records := readCSVRecords(t, outputPath)
	if len(records) != 3 {
		t.Fatalf("expected 3 csv records, got %d", len(records))
	}

	expectedHeader := []string{"Timestamp", "Service", "Type", "Title", "Content", "UserID", "Metadata"}
	if !containsRow(records[:1], expectedHeader) {
		t.Fatalf("expected csv header %v, got %v", expectedHeader, records[0])
	}
	if records[1][1] != "slack" || records[2][1] != "github" {
		t.Fatalf("unexpected service order in csv rows: %+v", records)
	}
}

func TestCSVExporterExportToCSVWithSummaryIncludesSections(t *testing.T) {
	exporter := NewCSVExporter()
	outputPath := filepath.Join(t.TempDir(), "summary.csv")

	if err := exporter.ExportToCSVWithSummary(sampleEvents(), outputPath); err != nil {
		t.Fatalf("ExportToCSVWithSummary returned error: %v", err)
	}

	records := readCSVRecords(t, outputPath)
	if !containsRow(records, []string{"SUMMARY"}) {
		t.Fatalf("expected SUMMARY section in csv output")
	}
	if !containsRow(records, []string{"Total Events", "2"}) {
		t.Fatalf("expected total events row in csv summary")
	}
	if !containsRow(records, []string{"SERVICE BREAKDOWN"}) {
		t.Fatalf("expected service breakdown section in csv output")
	}
	if !containsRow(records, []string{"EVENTS"}) {
		t.Fatalf("expected events section in csv output")
	}
	if !containsRow(records, []string{"slack", "1", "50.0%"}) {
		t.Fatalf("expected slack service breakdown row in csv output")
	}
	if !containsRow(records, []string{"github", "1", "50.0%"}) {
		t.Fatalf("expected github service breakdown row in csv output")
	}
}

func TestCSVExporterGenerateDailySummaryAggregatesByDateAndService(t *testing.T) {
	exporter := NewCSVExporter()
	events := []*config.Event{
		{
			ID:        "event-1",
			Service:   "slack",
			Type:      "message",
			Title:     "one",
			Timestamp: time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC),
		},
		{
			ID:        "event-2",
			Service:   "slack",
			Type:      "message",
			Title:     "two",
			Timestamp: time.Date(2026, 3, 5, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:        "event-3",
			Service:   "github",
			Type:      "review",
			Title:     "three",
			Timestamp: time.Date(2026, 3, 5, 11, 0, 0, 0, time.UTC),
		},
	}

	summary := exporter.generateDailySummary(events)
	day := summary["2026-03-05"]
	if day == nil {
		t.Fatalf("expected daily summary for 2026-03-05")
	}
	if day["slack"].EventCount != 2 {
		t.Fatalf("expected 2 slack events, got %d", day["slack"].EventCount)
	}
	if day["slack"].Summary != "2 events over 1h30m0s" {
		t.Fatalf("unexpected slack summary text: %q", day["slack"].Summary)
	}
	if day["github"].EventCount != 1 {
		t.Fatalf("expected 1 github event, got %d", day["github"].EventCount)
	}
}
