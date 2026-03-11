package app

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

type stubCollectCoordinator struct {
	initializedWith []string
	collectedWith   []string
	validateCalls   int
}

func (s *stubCollectCoordinator) InitializeServicesFor(serviceNames []string) error {
	s.initializedWith = append([]string(nil), serviceNames...)
	return nil
}

func (s *stubCollectCoordinator) ValidateTimeRange(startTime, endTime time.Time) error {
	s.validateCalls++
	return nil
}

func (s *stubCollectCoordinator) CollectAndStore(startTime, endTime time.Time, serviceNames []string) error {
	s.collectedWith = append([]string(nil), serviceNames...)
	return nil
}

func TestResolveCollectServicesReturnsEnabledServicesInOrder(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: false},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	got, err := resolveCollectServices(cfg, nil)
	if err != nil {
		t.Fatalf("resolveCollectServices returned error: %v", err)
	}

	want := []string{"slack", "google_calendar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected services %v, got %v", want, got)
	}
}

func TestResolveCollectServicesNormalizesAndDeduplicates(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: true},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	got, err := resolveCollectServices(cfg, []string{" Slack ", "github", "slack", "GOOGLE_CALENDAR"})
	if err != nil {
		t.Fatalf("resolveCollectServices returned error: %v", err)
	}

	want := []string{"slack", "github", "google_calendar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected services %v, got %v", want, got)
	}
}

func TestResolveCollectServicesRejectsUnknownOrDisabledServices(t *testing.T) {
	cfg := &config.Config{
		Slack:     config.ServiceConfig{Enabled: true},
		GitHub:    config.ServiceConfig{Enabled: false},
		GoogleCal: config.ServiceConfig{Enabled: true},
	}

	if _, err := resolveCollectServices(cfg, []string{"unknown"}); err == nil {
		t.Fatalf("expected unknown service to return error")
	}
	if _, err := resolveCollectServices(cfg, []string{"github"}); err == nil {
		t.Fatalf("expected disabled service to return error")
	}
}

func TestCollectUsecaseRunInitializesAndCollectsResolvedServices(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "collect.db")
	db, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	coordinator := &stubCollectCoordinator{}
	usecase := &CollectUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return &config.Config{
					DatabasePath: dbPath,
					Slack:        config.ServiceConfig{Enabled: true},
					GitHub:       config.ServiceConfig{Enabled: true},
				}, nil
			},
			openDatabase: func(path string) (*database.DatabaseManager, error) {
				return db, nil
			},
		},
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) collectCoordinator {
			return coordinator
		},
	}

	result, err := usecase.Run(CollectRequest{
		StartTime: time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 3, 10, 18, 0, 0, 0, time.UTC),
		Services:  []string{" github ", "slack"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	want := []string{"github", "slack"}
	if !reflect.DeepEqual(result.TargetServices, want) {
		t.Fatalf("expected target services %v, got %v", want, result.TargetServices)
	}
	if result.CollectedRange.StartTime != time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC) || result.CollectedRange.EndTime != time.Date(2026, 3, 10, 18, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected collected range: %+v", result.CollectedRange)
	}
	if !reflect.DeepEqual(coordinator.initializedWith, want) {
		t.Fatalf("expected InitializeServicesFor to receive %v, got %v", want, coordinator.initializedWith)
	}
	if !reflect.DeepEqual(coordinator.collectedWith, want) {
		t.Fatalf("expected CollectAndStore to receive %v, got %v", want, coordinator.collectedWith)
	}
	if coordinator.validateCalls != 1 {
		t.Fatalf("expected ValidateTimeRange to be called once, got %d", coordinator.validateCalls)
	}
}

func TestCollectUsecaseRunReturnsConfigLoadError(t *testing.T) {
	usecase := &CollectUsecase{
		runtime: &appRuntime{
			loadConfig: func(path string) (*config.Config, error) {
				return nil, fmt.Errorf("boom")
			},
			openDatabase: database.NewDatabaseManager,
		},
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) collectCoordinator {
			return &stubCollectCoordinator{}
		},
	}

	if _, err := usecase.Run(CollectRequest{}); err == nil {
		t.Fatalf("expected Run to return config load error")
	}
}
