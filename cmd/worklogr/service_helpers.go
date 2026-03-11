package main

import (
	"fmt"
	"strings"

	"github.com/iriam/worklogr/internal/config"
)

var orderedServiceNames = []string{"slack", "github", "google_calendar"}

var serviceDisplayNames = map[string]string{
	"slack":           "Slack",
	"github":          "GitHub",
	"google_calendar": "Google Calendar",
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
		for _, serviceName := range orderedServiceNames {
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
