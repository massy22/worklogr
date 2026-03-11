package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
	"github.com/iriam/worklogr/internal/exporter"
)

type ExportRequest struct {
	ConfigPath string
	StartTime  time.Time
	EndTime    time.Time
	Services   []string
	Format     string
	OutputPath string
}

type ExportResult struct {
	MatchedEventCount int
}

type ExportUsecase struct {
	runtime      *appRuntime
	exportEvents func([]*config.Event, string, string) error
}

func NewExportUsecase() *ExportUsecase {
	return &ExportUsecase{
		runtime:      newAppRuntime(),
		exportEvents: exportEvents,
	}
}

func (u *ExportUsecase) Run(request ExportRequest) (*ExportResult, error) {
	return withDatabase(u.runtime, request.ConfigPath, func(cfg *config.Config, db *database.DatabaseManager) (*ExportResult, error) {
		events, err := db.GetEvents(request.StartTime, request.EndTime, request.Services)
		if err != nil {
			return nil, fmt.Errorf("イベントの取得に失敗しました: %w", err)
		}

		result := &ExportResult{MatchedEventCount: len(events)}
		if len(events) == 0 {
			return result, nil
		}

		if err := u.exportEvents(events, request.Format, request.OutputPath); err != nil {
			return nil, err
		}

		return result, nil
	})
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
