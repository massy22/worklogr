package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

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

func init() {
	exportCmd.Flags().StringVarP(&startDate, "start", "s", "", "開始日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	exportCmd.Flags().StringVarP(&endDate, "end", "e", "", "終了日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	exportCmd.Flags().StringSliceVar(&services, "services", []string{}, "エクスポート対象サービス（例: slack,github,google_calendar）")
	exportCmd.Flags().StringVarP(&outputPath, "output", "o", "", "出力ファイルパス")
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "エクスポート形式 (json, json-ai, csv, csv-summary)")
	exportCmd.MarkFlagRequired("start")
	exportCmd.MarkFlagRequired("end")
}
