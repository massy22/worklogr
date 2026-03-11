package app

import (
	"fmt"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
)

type appRuntime struct {
	loadConfig   func(string) (*config.Config, error)
	openDatabase func(string) (*database.DatabaseManager, error)
}

func newAppRuntime() *appRuntime {
	return &appRuntime{
		loadConfig:   config.LoadConfig,
		openDatabase: database.NewDatabaseManager,
	}
}

func (r *appRuntime) loadAppConfig(configPath string) (*config.Config, error) {
	cfg, err := r.loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	return cfg, nil
}

func withDatabase[T any](runtime *appRuntime, configPath string, fn func(*config.Config, *database.DatabaseManager) (T, error)) (T, error) {
	var zero T

	cfg, err := runtime.loadAppConfig(configPath)
	if err != nil {
		return zero, err
	}

	db, err := runtime.openDatabase(cfg.DatabasePath)
	if err != nil {
		return zero, fmt.Errorf("データベースの初期化に失敗しました: %w", err)
	}
	defer db.Close()

	return fn(cfg, db)
}
