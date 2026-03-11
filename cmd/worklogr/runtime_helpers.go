package main

import (
	"fmt"
	"strings"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
	"github.com/iriam/worklogr/internal/exporter"
)

func loadCLIConfig(configPath string) (*config.Config, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	return cfg, nil
}

func loadCLIConfigAndDatabase(configPath string) (*config.Config, *database.DatabaseManager, error) {
	cfg, err := loadCLIConfig(configPath)
	if err != nil {
		return nil, nil, err
	}

	db, err := database.NewDatabaseManager(cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("データベースの初期化に失敗しました: %w", err)
	}

	return cfg, db, nil
}

func exportEvents(events []*config.Event, format, outputPath string) error {
	switch strings.ToLower(format) {
	case "json":
		jsonExporter := exporter.NewJSONExporter()
		if err := jsonExporter.ExportToJSON(events, outputPath); err != nil {
			return fmt.Errorf("JSONエクスポートに失敗しました: %w", err)
		}
	case "json-ai":
		jsonExporter := exporter.NewJSONExporter()
		if err := jsonExporter.ExportForAI(events, outputPath); err != nil {
			return fmt.Errorf("AI用JSONエクスポートに失敗しました: %w", err)
		}
	case "csv":
		csvExporter := exporter.NewCSVExporter()
		if err := csvExporter.ExportToCSV(events, outputPath); err != nil {
			return fmt.Errorf("CSVエクスポートに失敗しました: %w", err)
		}
	case "csv-summary":
		csvExporter := exporter.NewCSVExporter()
		if err := csvExporter.ExportToCSVWithSummary(events, outputPath); err != nil {
			return fmt.Errorf("サマリー付きCSVエクスポートに失敗しました: %w", err)
		}
	default:
		return fmt.Errorf("サポートされていない形式です: %s。対応形式: json, json-ai, csv, csv-summary", format)
	}

	return nil
}
