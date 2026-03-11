package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/iriam/worklogr/internal/collector"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

var collectOrderedServiceNames = []string{"slack", "github", "google_calendar"}

type CollectRequest struct {
	ConfigPath string
	StartTime  time.Time
	EndTime    time.Time
	Services   []string
}

type CollectResult struct {
	TargetServices []string
}

type collectCoordinator interface {
	InitializeServicesFor([]string) error
	ValidateTimeRange(time.Time, time.Time) error
	CollectAndStore(time.Time, time.Time, []string) error
}

type CollectUsecase struct {
	runtime      *appRuntime
	newCollector func(*config.Config, *database.DatabaseManager) collectCoordinator
}

func NewCollectUsecase() *CollectUsecase {
	return &CollectUsecase{
		runtime: newAppRuntime(),
		newCollector: func(cfg *config.Config, db *database.DatabaseManager) collectCoordinator {
			return collector.NewEventCollector(cfg, db)
		},
	}
}

func (u *CollectUsecase) Run(request CollectRequest) (*CollectResult, error) {
	return withDatabase(u.runtime, request.ConfigPath, func(cfg *config.Config, db *database.DatabaseManager) (*CollectResult, error) {
		targetServices, err := resolveCollectServices(cfg, request.Services)
		if err != nil {
			return nil, err
		}

		eventCollector := u.newCollector(cfg, db)
		if err := eventCollector.InitializeServicesFor(targetServices); err != nil {
			return nil, fmt.Errorf("サービスの初期化に失敗しました: %w", err)
		}

		if err := eventCollector.ValidateTimeRange(request.StartTime, request.EndTime); err != nil {
			return nil, fmt.Errorf("時間範囲が無効です: %w", err)
		}

		if err := eventCollector.CollectAndStore(request.StartTime, request.EndTime, targetServices); err != nil {
			return nil, fmt.Errorf("イベント収集に失敗しました: %w", err)
		}

		return &CollectResult{TargetServices: targetServices}, nil
	})
}

func resolveCollectServices(cfg *config.Config, requested []string) ([]string, error) {
	knownServices := map[string]bool{
		"slack":           true,
		"github":          true,
		"google_calendar": true,
	}

	enabledServices := map[string]bool{
		"slack":           cfg.Slack.Enabled,
		"github":          cfg.GitHub.Enabled,
		"google_calendar": cfg.GoogleCal.Enabled,
	}

	if len(requested) == 0 {
		var targets []string
		for _, serviceName := range collectOrderedServiceNames {
			if enabledServices[serviceName] {
				targets = append(targets, serviceName)
			}
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("有効な収集対象サービスがありません")
		}
		return targets, nil
	}

	seen := make(map[string]bool)
	var targets []string
	for _, rawName := range requested {
		serviceName := strings.ToLower(strings.TrimSpace(rawName))
		if !knownServices[serviceName] {
			return nil, fmt.Errorf("未知のサービスが指定されました: %s（指定可能: slack, github, google_calendar）", rawName)
		}
		if !enabledServices[serviceName] {
			return nil, fmt.Errorf("サービス '%s' は設定で無効です。config.yaml を確認してください", serviceName)
		}
		if seen[serviceName] {
			continue
		}
		seen[serviceName] = true
		targets = append(targets, serviceName)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("収集対象サービスが指定されていません")
	}

	return targets, nil
}
