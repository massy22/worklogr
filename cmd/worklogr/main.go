package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/iriam/worklogr/internal/auth"
	"github.com/iriam/worklogr/internal/collector"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
	"github.com/iriam/worklogr/internal/exporter"
	"github.com/iriam/worklogr/internal/utils"
	"github.com/spf13/cobra"
)

var (
	configPath string
	startDate  string
	endDate    string
	services   []string
	outputPath string
	format     string
)

func main() {
	// Cobraのデフォルトメッセージを日本語化
	rootCmd.SetUsageTemplate(`使用方法:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

利用可能なコマンド:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

フラグ:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

グローバルフラグ:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

追加のヘルプトピック:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

詳細については "{{.CommandPath}} [command] --help" を使用してください。{{end}}
`)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "worklogr",
	Short: "報告書生成のためのマルチサービスイベント収集CLI",
	Long: `worklogrは複数のサービス（Slack、GitHub、Google Calendar）からイベントを収集し、
SQLiteに保存して、JSON/CSV（AI向けJSONを含む）で出力するCLIツールです。

Google Calendarはgcloud認証（ADC）を使用し、イベントに添付されたGoogleドキュメント（Geminiメモ等）の本文テキストも収集できます。
添付本文はイベント本体とは別テーブル（event_attachments）に保存され、AI向けJSONでは context.attachments に含まれます。`,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "設定ファイルのパス")

	// Cobraの自動生成コマンドを日本語化
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// カスタムヘルプコマンド
	helpCmd := &cobra.Command{
		Use:   "help [command]",
		Short: "任意のコマンドのヘルプ",
		Long: `任意のコマンドのヘルプ情報を表示します。
引数なしで実行すると、利用可能なすべてのコマンドを表示します。`,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := c.Root().Find(args)
			if cmd == nil || e != nil {
				c.Printf("不明なコマンド \"%s\" です。\n", strings.Join(args, " "))
				c.Root().Usage()
			} else {
				cmd.InitDefaultHelpFlag() // make possible 'help' flag to be shown
				cmd.Help()
			}
		},
	}
	rootCmd.SetHelpCommand(helpCmd)

	// Add subcommands
	rootCmd.AddCommand(gcloudCmd)
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
}

// GCloudコマンド
var gcloudCmd = &cobra.Command{
	Use:   "gcloud",
	Short: "Google（Calendar/Drive）のgcloud認証（ADC）を確認/案内",
	Long:  "Google Calendar（およびGeminiメモ等のGoogleドキュメント添付取得）に必要なgcloud認証（Application Default Credentials）を確認/案内します。",
}

var gcloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "gcloud認証状態を確認",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 統合されたCalendarAuthManagerを使用
		calendarAuth := auth.NewCalendarAuthManagerWithGCloud(&auth.AuthConfig{}, nil)

		if calendarAuth.IsGCloudAuthenticated() {
			fmt.Println("gcloud認証が利用可能です")
			fmt.Println("Google Calendar/Driveにgcloud認証でアクセスできます")
		} else {
			fmt.Println("gcloud認証が利用できません")
			fmt.Println("\ngcloud認証をセットアップするには:")
			fmt.Println(calendarAuth.SetupGCloudAuth())
		}

		return nil
	},
}

var gcloudSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "gcloudセットアップ手順を表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 統合されたCalendarAuthManagerを使用
		calendarAuth := auth.NewCalendarAuthManagerWithGCloud(&auth.AuthConfig{}, nil)
		fmt.Println(calendarAuth.SetupGCloudAuth())
		return nil
	},
}

// Collectコマンド
var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "サービスからイベントを収集",
	Long: `設定されたサービスから指定期間のイベントを収集し、SQLiteに保存します。

Google Calendarが有効な場合、イベントに添付されたGoogleドキュメント（Geminiメモ等）の本文テキストも取得できます（デフォルトON）。
添付本文は event_attachments テーブルに保存され、動画などの添付は対象外です。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 時間範囲を解析
		startTime, endTime, err := parseTimeRange(startDate, endDate)
		if err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

		// 終了日全体を含むように終了時刻を調整
		// 終了時刻が00:00:00の場合、同日の23:59:59まで延長
		// ただし未来の時刻にならないよう確認
		if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
			adjustedEndTime := endTime.Add(24*time.Hour - time.Second)
			// 未来の時刻にならない場合のみ調整
			if !adjustedEndTime.After(time.Now()) {
				endTime = adjustedEndTime
			} else {
				// 今日の場合は現在時刻に設定
				endTime = time.Now()
			}
		}

		// 設定を読み込み
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
		}

		// データベースを初期化
		db, err := database.NewDatabaseManager(cfg.DatabasePath)
		if err != nil {
			return fmt.Errorf("データベースの初期化に失敗しました: %w", err)
		}
		defer db.Close()

		// イベントコレクターを初期化
		eventCollector := collector.NewEventCollector(cfg, db)
		targetServices, err := resolveCollectServices(cfg, services)
		if err != nil {
			return err
		}

		if err := eventCollector.InitializeServicesFor(targetServices); err != nil {
			return fmt.Errorf("サービスの初期化に失敗しました: %w", err)
		}

		// 時間範囲を検証
		if err := eventCollector.ValidateTimeRange(startTime, endTime); err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

		// イベントを収集・保存
		fmt.Printf("%s から %s までのイベントを収集中...\n",
			startTime.Format("2006-01-02 15:04:05"),
			endTime.Format("2006-01-02 15:04:05"))

		if err := eventCollector.CollectAndStore(startTime, endTime, targetServices); err != nil {
			return fmt.Errorf("イベント収集に失敗しました: %w", err)
		}

		fmt.Println("イベント収集が正常に完了しました！")
		return nil
	},
}

// Exportコマンド
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "収集したイベントをエクスポート",
	Long: `SQLiteからイベントを取得して、指定形式でエクスポートします。

json-ai 形式では、イベントのmetadataに加えて、添付本文（Googleドキュメント等）がある場合は context.attachments に含めます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 時間範囲を解析
		startTime, endTime, err := parseTimeRange(startDate, endDate)
		if err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

		// エクスポート用に終了日全体を含むよう終了時刻を調整
		// 終了時刻が00:00:00の場合、同日の23:59:59まで延長
		// ただし未来の時刻にならないよう確認
		if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
			adjustedEndTime := endTime.Add(24*time.Hour - time.Second)
			// 未来の時刻にならない場合のみ調整
			if !adjustedEndTime.After(time.Now()) {
				endTime = adjustedEndTime
			} else {
				// 今日の場合は現在時刻に設定
				endTime = time.Now()
			}
		}

		fmt.Printf("%s から %s までのイベントをエクスポート中...\n",
			startTime.Format("2006-01-02 15:04:05"),
			endTime.Format("2006-01-02 15:04:05"))

		// 設定を読み込み
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
		}

		// データベースを初期化
		db, err := database.NewDatabaseManager(cfg.DatabasePath)
		if err != nil {
			return fmt.Errorf("データベースの初期化に失敗しました: %w", err)
		}
		defer db.Close()

		// データベースからイベントを取得
		events, err := db.GetEvents(startTime, endTime, services)
		if err != nil {
			return fmt.Errorf("イベントの取得に失敗しました: %w", err)
		}

		if len(events) == 0 {
			fmt.Println("指定された期間にイベントが見つかりませんでした")
			return nil
		}

		fmt.Printf("エクスポート対象のイベントが %d 件見つかりました\n", len(events))

		// 形式に基づいてエクスポート
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
	},
}

// Statusコマンド
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "サービス状態を表示",
	Long:  "設定されたすべてのサービスの状態を表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 設定を読み込み
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
		}

		// データベースを初期化
		db, err := database.NewDatabaseManager(cfg.DatabasePath)
		if err != nil {
			return fmt.Errorf("データベースの初期化に失敗しました: %w", err)
		}
		defer db.Close()

		// イベントコレクターを初期化
		eventCollector := collector.NewEventCollector(cfg, db)
		eventCollector.InitializeServices() // 初期化エラーでは失敗しない

		// サービス状態を取得
		status := eventCollector.GetServiceStatus()

		// 状態を表示
		fmt.Println("サービス状態:")
		fmt.Println("=============")
		serviceNames := map[string]string{
			"slack":           "Slack",
			"github":          "GitHub",
			"google_calendar": "Google Calendar",
		}

		for _, serviceName := range []string{"slack", "github", "google_calendar"} {
			if serviceStatus, exists := status[serviceName]; exists {
				fmt.Printf("%-15s | 有効: %-5t | 認証済み: %-5t | 初期化済み: %-5t\n",
					serviceNames[serviceName],
					serviceStatus.Enabled,
					serviceStatus.Authenticated,
					serviceStatus.Initialized)
			}
		}

		// データベース統計を表示
		fmt.Println("\nデータベース統計:")
		fmt.Println("=================")
		stats, err := db.GetStats()
		if err != nil {
			fmt.Printf("データベース統計の取得に失敗しました: %v\n", err)
		} else {
			for service, count := range stats {
				fmt.Printf("%-15s: %d イベント\n", serviceNames[service], count)
			}
		}

		return nil
	},
}

// Configコマンド
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "設定を管理",
	Long:  "worklogrの設定を表示・管理します",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "現在の設定を表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
		}

		fmt.Println("現在の設定:")
		fmt.Println("===========")
		fmt.Printf("データベースパス: %s\n", cfg.DatabasePath)
		fmt.Printf("タイムゾーン: %s\n", cfg.Timezone)
		fmt.Println("\nサービス:")

		services := map[string]config.ServiceConfig{
			"Slack":           cfg.Slack,
			"GitHub":          cfg.GitHub,
			"Google Calendar": cfg.GoogleCal,
		}

		for name, service := range services {
			fmt.Printf("  %-15s: 有効=%t, 設定済み=%t\n",
				name,
				service.Enabled,
				service.ClientID != "" && service.ClientSecret != "")
		}

		return nil
	},
}

func init() {
	// Collect command flags
	collectCmd.Flags().StringVarP(&startDate, "start", "s", "", "開始日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	collectCmd.Flags().StringVarP(&endDate, "end", "e", "", "終了日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	collectCmd.Flags().StringSliceVar(&services, "services", []string{}, "収集対象サービス（例: slack,github,google_calendar）")
	collectCmd.MarkFlagRequired("start")
	collectCmd.MarkFlagRequired("end")

	// Export command flags
	exportCmd.Flags().StringVarP(&startDate, "start", "s", "", "開始日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	exportCmd.Flags().StringVarP(&endDate, "end", "e", "", "終了日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	exportCmd.Flags().StringSliceVar(&services, "services", []string{}, "エクスポート対象サービス（例: slack,github,google_calendar）")
	exportCmd.Flags().StringVarP(&outputPath, "output", "o", "", "出力ファイルパス")
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "エクスポート形式 (json, json-ai, csv, csv-summary)")
	exportCmd.MarkFlagRequired("start")
	exportCmd.MarkFlagRequired("end")

	// Config subcommands
	configCmd.AddCommand(configShowCmd)

	// GCloud subcommands
	gcloudCmd.AddCommand(gcloudStatusCmd)
	gcloudCmd.AddCommand(gcloudSetupCmd)

}

// ヘルパー関数

func parseTimeRange(startStr, endStr string) (time.Time, time.Time, error) {
	var startTime, endTime time.Time
	var err error

	// 開始時刻を解析
	if startTime, err = parseTimeString(startStr); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("開始時刻が無効です: %w", err)
	}

	// 終了時刻を解析
	if endTime, err = parseTimeString(endStr); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("終了時刻が無効です: %w", err)
	}

	// 時間範囲を検証
	if startTime.After(endTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("開始時刻は終了時刻より後にできません")
	}

	return startTime, endTime, nil
}

func parseTimeString(timeStr string) (time.Time, error) {
	// タイムゾーン設定を取得するため設定を読み込み
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// 設定が読み込めない場合はJSTにフォールバック
		cfg = &config.Config{Timezone: "Asia/Tokyo"}
	}

	// タイムゾーンマネージャーを作成
	timezoneManager, err := utils.NewTimezoneManager(cfg.Timezone)
	if err != nil {
		// 無効な場合はデフォルトタイムゾーンにフォールバック
		timezoneManager, _ = utils.NewTimezoneManager("Asia/Tokyo")
	}

	// タイムゾーンマネージャーで解析を試行
	if t, err := timezoneManager.ParseTimeInTimezone(timeStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("時刻の解析に失敗しました: %s", timeStr)
}

func parseRelativeTime(timeStr, timezone string) (time.Time, error) {
	// 設定されたタイムゾーンを取得
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// タイムゾーンが利用できない場合はUTC+9にフォールバック
		loc = time.FixedZone("JST", 9*60*60)
	}

	// 設定されたタイムゾーンでの現在時刻を取得
	now := time.Now().In(loc)

	switch strings.ToLower(timeStr) {
	case "now":
		return now, nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, loc), nil
	}

	// 期間ベースの相対時刻を解析（例: "1d", "2h", "30m"）
	if len(timeStr) > 1 {
		unit := timeStr[len(timeStr)-1:]
		valueStr := timeStr[:len(timeStr)-1]

		value, err := strconv.Atoi(valueStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("相対時刻の値が無効です: %s", valueStr)
		}

		switch unit {
		case "d":
			return now.AddDate(0, 0, -value), nil
		case "h":
			return now.Add(time.Duration(-value) * time.Hour), nil
		case "m":
			return now.Add(time.Duration(-value) * time.Minute), nil
		}
	}

	return time.Time{}, fmt.Errorf("サポートされていない相対時刻形式です: %s", timeStr)
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
		for _, serviceName := range []string{"slack", "github", "google_calendar"} {
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
