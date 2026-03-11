package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/config"
)

func loadCLIConfig(configPath string) (*config.Config, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	return cfg, nil
}
