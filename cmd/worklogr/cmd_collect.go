package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/collector"
	"github.com/spf13/cobra"
)

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

		if err := eventCollector.ValidateTimeRange(startTime, endTime); err != nil {
			return fmt.Errorf("時間範囲が無効です: %w", err)
		}

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

func init() {
	collectCmd.Flags().StringVarP(&startDate, "start", "s", "", "開始日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	collectCmd.Flags().StringVarP(&endDate, "end", "e", "", "終了日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	collectCmd.Flags().StringSliceVar(&services, "services", []string{}, "収集対象サービス（例: slack,github,google_calendar）")
	collectCmd.MarkFlagRequired("start")
	collectCmd.MarkFlagRequired("end")
}
