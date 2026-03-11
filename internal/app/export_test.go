package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

func TestExportUsecaseRunExportsMatchedEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "export.db")
	db, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	timestamp := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	if err := db.InsertEvent(&config.Event{
		ID:        "event-1",
		Service:   "slack",
		Type:      "message",
		Title:     "title",
		Content:   "content",
		Timestamp: timestamp,
	}); err != nil {
		t.Fatalf("failed to seed test database: %v", err)
	}

	var exportedCount int
	var exportedFormat string
	var exportedPath string

	usecase := &ExportUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return &config.Config{DatabasePath: dbPath}, nil
			},
			openDatabase: func(path string) (*database.DatabaseManager, error) {
				return db, nil
			},
		},
		exportEvents: func(events []*config.Event, format, outputPath string) error {
			exportedCount = len(events)
			exportedFormat = format
			exportedPath = outputPath
			return nil
		},
	}

	result, err := usecase.Run(ExportRequest{
		StartTime:  timestamp.Add(-time.Hour),
		EndTime:    timestamp.Add(time.Hour),
		Services:   []string{"slack"},
		Format:     "json",
		OutputPath: "/tmp/output.json",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.EventCount != 1 {
		t.Fatalf("expected event count 1, got %d", result.EventCount)
	}
	if exportedCount != 1 || exportedFormat != "json" || exportedPath != "/tmp/output.json" {
		t.Fatalf("unexpected export call: count=%d format=%q path=%q", exportedCount, exportedFormat, exportedPath)
	}
}

func TestExportUsecaseRunSkipsExportWhenNoEventsMatched(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "export-empty.db")
	db, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	called := false
	usecase := &ExportUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return &config.Config{DatabasePath: dbPath}, nil
			},
			openDatabase: func(path string) (*database.DatabaseManager, error) {
				return db, nil
			},
		},
		exportEvents: func(events []*config.Event, format, outputPath string) error {
			called = true
			return nil
		},
	}

	result, err := usecase.Run(ExportRequest{
		StartTime: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 3, 10, 23, 59, 59, 0, time.UTC),
		Format:    "json",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.EventCount != 0 {
		t.Fatalf("expected event count 0, got %d", result.EventCount)
	}
	if called {
		t.Fatalf("expected export function not to be called when no events matched")
	}
}

func TestExportUsecaseRunReturnsConfigLoadError(t *testing.T) {
	usecase := &ExportUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return nil, fmt.Errorf("boom")
			},
			openDatabase: database.NewDatabaseManager,
		},
		exportEvents: exportEvents,
	}

	if _, err := usecase.Run(ExportRequest{}); err == nil {
		t.Fatalf("expected Run to return config load error")
	}
}
