package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/iriam/worklogr/internal/auth"
	"github.com/iriam/worklogr/internal/collector"
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
		startTime, endTime, err := parseAdjustedTimeRange(startDate, endDate)
		if err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

		cfg, db, err := loadCLIConfigAndDatabase()
		if err != nil {
			return err
		}
		defer db.Close()

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
		startTime, endTime, err := parseAdjustedTimeRange(startDate, endDate)
		if err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

		fmt.Printf("%s から %s までのイベントをエクスポート中...\n",
			startTime.Format("2006-01-02 15:04:05"),
			endTime.Format("2006-01-02 15:04:05"))

		_, db, err := loadCLIConfigAndDatabase()
		if err != nil {
			return err
		}
		defer db.Close()

		events, err := db.GetEvents(startTime, endTime, services)
		if err != nil {
			return fmt.Errorf("イベントの取得に失敗しました: %w", err)
		}

		if len(events) == 0 {
			fmt.Println("指定された期間にイベントが見つかりませんでした")
			return nil
		}

		fmt.Printf("エクスポート対象のイベントが %d 件見つかりました\n", len(events))

		return exportEvents(events, format, outputPath)
	},
}

// Statusコマンド
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "サービス状態を表示",
	Long:  "設定されたすべてのサービスの状態を表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, db, err := loadCLIConfigAndDatabase()
		if err != nil {
			return err
		}
		defer db.Close()

		eventCollector := collector.NewEventCollector(cfg, db)
		eventCollector.InitializeServices()
		status := eventCollector.GetServiceStatus()

		fmt.Println("サービス状態:")
		fmt.Println("=============")

		for _, serviceName := range orderedServiceNames {
			if serviceStatus, exists := status[serviceName]; exists {
				fmt.Printf("%-15s | 有効: %-5t | 認証済み: %-5t | 初期化済み: %-5t\n",
					serviceDisplayNames[serviceName],
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
				fmt.Printf("%-15s: %d イベント\n", serviceDisplayNames[service], count)
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
		cfg, err := loadCLIConfig()
		if err != nil {
			return err
		}

		fmt.Println("現在の設定:")
		fmt.Println("===========")
		fmt.Printf("データベースパス: %s\n", cfg.DatabasePath)
		fmt.Printf("タイムゾーン: %s\n", cfg.Timezone)
		fmt.Println("\nサービス:")

		services := map[string]struct {
			displayName string
			configured  bool
			enabled     bool
		}{
			"slack": {
				displayName: serviceDisplayNames["slack"],
				configured:  cfg.Slack.ClientID != "" && cfg.Slack.ClientSecret != "",
				enabled:     cfg.Slack.Enabled,
			},
			"github": {
				displayName: serviceDisplayNames["github"],
				configured:  cfg.GitHub.ClientID != "" && cfg.GitHub.ClientSecret != "",
				enabled:     cfg.GitHub.Enabled,
			},
			"google_calendar": {
				displayName: serviceDisplayNames["google_calendar"],
				configured:  cfg.GoogleCal.ClientID != "" && cfg.GoogleCal.ClientSecret != "",
				enabled:     cfg.GoogleCal.Enabled,
			},
		}

		for _, serviceName := range orderedServiceNames {
			service := services[serviceName]
			fmt.Printf("  %-15s: 有効=%t, 設定済み=%t\n",
				service.displayName,
				service.enabled,
				service.configured)
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
