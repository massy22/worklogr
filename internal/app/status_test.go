package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/collector"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

type stubStatusCollector struct {
	status map[string]collector.ServiceStatus
}

func (s *stubStatusCollector) InitializeServices() error {
	return nil
}

func (s *stubStatusCollector) GetServiceStatus() map[string]collector.ServiceStatus {
	return s.status
}

func TestStatusUsecaseRunReturnsStatusesAndStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "status.db")
	db, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	if err := db.InsertEvent(&config.Event{
		ID:        "event-1",
		Service:   "slack",
		Type:      "message",
		Title:     "title",
		Content:   "content",
		Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("failed to seed test database: %v", err)
	}

	usecase := &StatusUsecase{
		loadConfig: func(path string) (*config.Config, error) {
			return &config.Config{DatabasePath: dbPath}, nil
		},
		openDatabase: func(path string) (*database.DatabaseManager, error) {
			return db, nil
		},
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) statusCollector {
			return &stubStatusCollector{
				status: map[string]collector.ServiceStatus{
					"slack": {
						Name:          "slack",
						Enabled:       true,
						Authenticated: true,
						Initialized:   true,
					},
				},
			}
		},
	}

	result, err := usecase.Run(StatusRequest{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.StatsError != nil {
		t.Fatalf("expected no stats error, got %v", result.StatsError)
	}
	if result.Stats["total"] != 1 {
		t.Fatalf("expected total stats 1, got %v", result.Stats)
	}
	if !result.ServiceStatus["slack"].Initialized {
		t.Fatalf("expected slack status to be initialized, got %+v", result.ServiceStatus["slack"])
	}
}

func TestStatusUsecaseRunReturnsConfigLoadError(t *testing.T) {
	usecase := &StatusUsecase{
		loadConfig: func(path string) (*config.Config, error) {
			return nil, fmt.Errorf("boom")
		},
		openDatabase: database.NewDatabaseManager,
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) statusCollector {
			return &stubStatusCollector{}
		},
	}

	if _, err := usecase.Run(StatusRequest{}); err == nil {
		t.Fatalf("expected Run to return config load error")
	}
}
