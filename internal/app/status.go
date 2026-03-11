package app

import (
	"fmt"

	"github.com/iriam/worklogr/internal/collector"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

type StatusRequest struct {
	ConfigPath string
}

type StatusResult struct {
	ServiceStatus map[string]collector.ServiceStatus
	Stats         map[string]int
	StatsError    error
}

type statusCollector interface {
	InitializeServices() error
	GetServiceStatus() map[string]collector.ServiceStatus
}

type StatusUsecase struct {
	runtime      *appRuntime
	newCollector func(*config.Config, *database.DatabaseManager) statusCollector
}

func NewStatusUsecase() *StatusUsecase {
	return &StatusUsecase{
		runtime: newAppRuntime(),
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) statusCollector {
			return collector.NewEventCollector(cfg, db)
		},
	}
}

func (u *StatusUsecase) Run(request StatusRequest) (*StatusResult, error) {
	return withDatabase(u.runtime, request.ConfigPath, func(cfg *config.Config, db *database.DatabaseManager) (*StatusResult, error) {
		eventCollector := u.newCollector(cfg, db)
		eventCollector.InitializeServices()

		result := &StatusResult{
			ServiceStatus: eventCollector.GetServiceStatus(),
			Stats:         map[string]int{},
		}

		stats, err := db.GetStats()
		if err != nil {
			result.StatsError = fmt.Errorf("データベース統計の取得に失敗しました: %w", err)
			return result, nil
		}

		result.Stats = stats
		return result, nil
	})
}
